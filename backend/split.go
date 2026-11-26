package backend

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/idna"
)

// Режимы раздельного туннелирования.
const (
	// SplitModeInclude — в туннель идёт только список (всё остальное напрямую).
	SplitModeInclude = "include"
	// SplitModeExclude — в туннель идёт всё, кроме списка.
	SplitModeExclude = "exclude"
)

// NormalizeSplitRoutes приводит пользовательский список адресов к каноническим
// IPv4 CIDR: одиночный адрес превращается в /32, host-биты маскируются,
// дубликаты и пустые строки отбрасываются. IPv6 не поддерживается — маршруты
// в приложении IPv4-only, поэтому такие записи считаются ошибкой.
//
// Возвращает нормализованный список и ошибку со всеми невалидными записями
// (валидные при этом сохраняются, чтобы UI мог показать их пользователю).
func NormalizeSplitRoutes(list []string) ([]string, error) {
	var out []string
	var bad []string
	seen := make(map[string]bool, len(list))

	for _, raw := range list {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		cidr := entry
		if !strings.Contains(entry, "/") {
			if addr, err := netip.ParseAddr(entry); err == nil && addr.Is4() {
				cidr = entry + "/32"
			}
		}

		prefix, err := netip.ParsePrefix(cidr)
		if err != nil || !prefix.Addr().Is4() {
			bad = append(bad, entry)
			continue
		}

		canonical := prefix.Masked().String()
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		out = append(out, canonical)
	}

	if len(bad) > 0 {
		return out, fmt.Errorf("неверный адрес или подсеть (нужен IPv4/CIDR): %s", strings.Join(bad, ", "))
	}
	return out, nil
}

// NormalizeSplitDomains приводит список доменов к каноническому виду:
// нижний регистр, без завершающей точки, IDN → punycode, дубликаты
// отбрасываются. Принимает также URL (https://example.com/path) и адрес
// с портом. Маски (*.example.com) и IP-адреса здесь запрещены — для них есть
// список маршрутов.
func NormalizeSplitDomains(list []string) ([]string, error) {
	var out []string
	var bad []string
	seen := make(map[string]bool, len(list))

	for _, raw := range list {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		host, reason := domainHost(entry)
		if reason != "" {
			bad = append(bad, fmt.Sprintf("%s (%s)", entry, reason))
			continue
		}

		ascii, err := idna.Lookup.ToASCII(host)
		if err != nil || !validDomainName(ascii) {
			bad = append(bad, fmt.Sprintf("%s (неверный домен)", entry))
			continue
		}
		ascii = strings.ToLower(ascii)
		if seen[ascii] {
			continue
		}
		seen[ascii] = true
		out = append(out, ascii)
	}

	if len(bad) > 0 {
		return out, fmt.Errorf("неверные домены: %s", strings.Join(bad, ", "))
	}
	return out, nil
}

// domainHost извлекает имя хоста из пользовательской записи.
// Возвращает причину отказа в виде текста (пустая строка — всё в порядке).
func domainHost(entry string) (host, reason string) {
	// URL: https://example.com/path?x=1 → example.com
	if i := strings.Index(entry, "://"); i >= 0 {
		entry = entry[i+3:]
	}
	entry = strings.TrimPrefix(entry, "//")
	if i := strings.IndexAny(entry, "/?#"); i >= 0 {
		entry = entry[:i]
	}
	// Порт: example.com:443 (IPv6 в этом списке не поддерживается).
	if h, _, err := net.SplitHostPort(entry); err == nil {
		entry = h
	}
	entry = strings.TrimSuffix(entry, ".")

	if strings.ContainsAny(entry, "*") {
		return "", "маски не поддерживаются, укажите конкретный домен"
	}
	if strings.Contains(entry, "/") {
		return "", "это подсеть, укажите её в списке адресов"
	}
	if _, err := netip.ParseAddr(entry); err == nil {
		return "", "это IP-адрес, укажите его в списке адресов"
	}
	if entry == "" {
		return "", "пустой домен"
	}
	return entry, ""
}

// validDomainName проверяет ASCII-имя домена: метки 1–63 символа,
// буквы/цифры/дефис, без дефиса по краям, общая длина до 253 символов.
func validDomainName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
			if !ok {
				return false
			}
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
	}
	return true
}

// SplitResolver — минимальный интерфейс DNS-резолвера (его реализует
// *net.Resolver). Вынесен для тестируемости.
type SplitResolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// ResolveSplitDomains резолвит домены в IPv4 /32-маршруты. Запросы идут
// параллельно; ошибки возвращаются по каждому домену отдельно, чтобы
// подключение не срывалось из-за одного недоступного имени.
func ResolveSplitDomains(ctx context.Context, domains []string, resolver SplitResolver) ([]string, map[string]error) {
	normalized, _ := NormalizeSplitDomains(domains)
	if len(normalized) == 0 {
		return nil, nil
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	type result struct {
		domain string
		addrs  []netip.Addr
		err    error
	}

	ch := make(chan result, len(normalized))
	sem := make(chan struct{}, 16) // не больше 16 одновременных DNS-запросов
	var wg sync.WaitGroup
	for _, domain := range normalized {
		wg.Add(1)
		go func(domain string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			addrs, err := resolver.LookupNetIP(ctx, "ip4", domain)
			ch <- result{domain: domain, addrs: addrs, err: err}
		}(domain)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	seen := make(map[string]bool)
	var out []string
	errs := make(map[string]error)
	for res := range ch {
		if res.err != nil {
			errs[res.domain] = res.err
			continue
		}
		for _, addr := range res.addrs {
			addr = addr.Unmap()
			if !addr.Is4() {
				continue
			}
			route := netip.PrefixFrom(addr, 32).String()
			if seen[route] {
				continue
			}
			seen[route] = true
			out = append(out, route)
		}
	}
	sort.Strings(out)
	if len(errs) == 0 {
		errs = nil
	}
	return out, errs
}

// NormalizeSplitTunnel валидирует и нормализует настройки раздельного
// туннелирования. Режим по умолчанию — include.
func NormalizeSplitTunnel(s SplitTunnel) (SplitTunnel, error) {
	if s.Mode != SplitModeExclude {
		s.Mode = SplitModeInclude
	}
	routes, routeErr := NormalizeSplitRoutes(s.Routes)
	domains, domainErr := NormalizeSplitDomains(s.Domains)
	s.Routes = routes
	s.Domains = domains
	switch {
	case routeErr != nil && domainErr != nil:
		return s, fmt.Errorf("%v; %v", routeErr, domainErr)
	case routeErr != nil:
		return s, routeErr
	default:
		return s, domainErr
	}
}

// ResolveSplit вычисляет итоговые маршруты с учётом split tunneling.
//
// tunnels — подсети, которые нужно направить в WireGuard-интерфейс;
// excludes — подсети, которые должны идти через физический шлюз.
// domainRoutes — IPv4/CIDR, полученные из доменов при подключении; они
// обрабатываются так же, как записи из списка маршрутов.
//
// В режиме include список пересекается с AllowedIPs из конфига пира: маршрут
// в туннель имеет смысл только для тех сетей, которые WireGuard принимает,
// иначе такие пакеты были бы выброшены криптоключевой маршрутизацией.
func ResolveSplit(allowedIPs []string, split SplitTunnel, domainRoutes ...string) (tunnels, excludes []string) {
	tunnels = allowedIPs
	if !split.Enabled {
		return tunnels, nil
	}

	allRoutes := make([]string, 0, len(split.Routes)+len(domainRoutes))
	allRoutes = append(allRoutes, split.Routes...)
	allRoutes = append(allRoutes, domainRoutes...)

	routes, _ := NormalizeSplitRoutes(allRoutes)

	if split.Mode == SplitModeExclude {
		// 0.0.0.0/0 в списке исключений = не туннелировать ничего.
		for _, r := range routes {
			if r == "0.0.0.0/0" {
				return nil, nil
			}
		}
		return tunnels, routes
	}

	// include: пустой список (в том числе если домены не разрешились) не должен
	// превращаться в полный туннель вопреки настройке — не отправляем ничего.
	if len(routes) == 0 {
		return nil, nil
	}

	var out []string
	for _, route := range routes {
		rp, err := netip.ParsePrefix(route)
		if err != nil {
			continue
		}
		for _, allowed := range allowedIPs {
			ap, err := netip.ParsePrefix(allowed)
			if err != nil || !ap.Addr().Is4() {
				continue
			}
			if inter, ok := intersectPrefix(rp, ap); ok {
				s := inter.String()
				if !containsString(out, s) {
					out = append(out, s)
				}
			}
		}
	}
	return out, nil
}

// intersectPrefix возвращает пересечение двух IPv4-префиксов.
// Префиксы либо не пересекаются, либо один полностью содержит другой.
func intersectPrefix(a, b netip.Prefix) (netip.Prefix, bool) {
	switch {
	case a.Bits() <= b.Bits() && a.Contains(b.Addr()):
		return b, true
	case b.Bits() <= a.Bits() && b.Contains(a.Addr()):
		return a, true
	}
	return netip.Prefix{}, false
}

// SplitSummary — короткое описание активного режима для лога.
func SplitSummary(split SplitTunnel) string {
	if !split.Enabled {
		return ""
	}
	mode := "только список"
	if split.Mode == SplitModeExclude {
		mode = "всё кроме списка"
	}
	routes, _ := NormalizeSplitRoutes(split.Routes)
	domains, _ := NormalizeSplitDomains(split.Domains)
	if len(domains) > 0 {
		return fmt.Sprintf("Раздельное туннелирование: %s (%d записей, %d доменов)", mode, len(routes), len(domains))
	}
	return fmt.Sprintf("Раздельное туннелирование: %s (%d записей)", mode, len(routes))
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
