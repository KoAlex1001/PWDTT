package backend_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"pwdtt/backend"
)

// ═══════════════════════════════════════════════════
// NormalizeSplitRoutes
// ═══════════════════════════════════════════════════

func TestNormalizeSplitRoutes(t *testing.T) {
	got, err := backend.NormalizeSplitRoutes([]string{
		" 10.0.0.1 ",
		"192.168.1.5/24", // host-биты должны замаскироваться
		"10.0.0.0/8",
		"10.0.0.1/32", // дубликат первого
		"",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"10.0.0.1/32", "192.168.1.0/24", "10.0.0.0/8"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("route[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeSplitRoutes_Invalid(t *testing.T) {
	got, err := backend.NormalizeSplitRoutes([]string{"999.1.1.1", "10.0.0.0/33", "abc"})
	if err == nil {
		t.Fatal("expected error for invalid routes")
	}
	if len(got) != 0 {
		t.Errorf("valid routes should be empty, got %v", got)
	}
	for _, bad := range []string{"999.1.1.1", "10.0.0.0/33", "abc"} {
		if !strings.Contains(err.Error(), bad) {
			t.Errorf("error should mention %q: %v", bad, err)
		}
	}
}

func TestNormalizeSplitRoutes_IPv6Rejected(t *testing.T) {
	if _, err := backend.NormalizeSplitRoutes([]string{"::1"}); err == nil {
		t.Error("expected error for IPv6 address")
	}
	if _, err := backend.NormalizeSplitRoutes([]string{"2001:db8::/32"}); err == nil {
		t.Error("expected error for IPv6 prefix")
	}
}

func TestNormalizeSplitRoutes_KeepsValidOnPartialError(t *testing.T) {
	got, err := backend.NormalizeSplitRoutes([]string{"10.0.0.0/8", "bad"})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(got) != 1 || got[0] != "10.0.0.0/8" {
		t.Errorf("valid routes should be preserved, got %v", got)
	}
}

// ═══════════════════════════════════════════════════
// NormalizeSplitDomains
// ═══════════════════════════════════════════════════

func TestNormalizeSplitDomains(t *testing.T) {
	got, err := backend.NormalizeSplitDomains([]string{
		"  Example.COM. ",
		"https://ozon.ru/path?x=1",
		"yandex.ru:443",
		"example.com", // дубликат
		"",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"example.com", "ozon.ru", "yandex.ru"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("domain[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeSplitDomains_IDN(t *testing.T) {
	got, err := backend.NormalizeSplitDomains([]string{"сбербанк.рф"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "xn--") {
		t.Errorf("expected punycode domain, got %v", got)
	}
}

func TestNormalizeSplitDomains_Invalid(t *testing.T) {
	got, err := backend.NormalizeSplitDomains([]string{
		"10.0.0.0/8",
		"1.2.3.4",
		"*.example.com",
		"bad_domain",
		"-bad.com",
	})
	if err == nil {
		t.Fatal("expected error for invalid domains")
	}
	if len(got) != 0 {
		t.Errorf("valid domains should be empty, got %v", got)
	}
	for _, bad := range []string{"10.0.0.0/8", "1.2.3.4", "*.example.com", "bad_domain", "-bad.com"} {
		if !strings.Contains(err.Error(), bad) {
			t.Errorf("error should mention %q: %v", bad, err)
		}
	}
}

func TestNormalizeSplitDomains_KeepsValidOnPartialError(t *testing.T) {
	got, err := backend.NormalizeSplitDomains([]string{"example.com", "*.bad.com"})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(got) != 1 || got[0] != "example.com" {
		t.Errorf("valid domains should be preserved, got %v", got)
	}
}

// ═══════════════════════════════════════════════════
// ResolveSplitDomains
// ═══════════════════════════════════════════════════

type fakeSplitResolver struct {
	ips  map[string][]netip.Addr
	errs map[string]error
}

func (f fakeSplitResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if err := f.errs[host]; err != nil {
		return nil, err
	}
	return f.ips[host], nil
}

func TestResolveSplitDomains(t *testing.T) {
	resolver := fakeSplitResolver{ips: map[string][]netip.Addr{
		"a.example": {netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("2001:db8::1")},
		"b.example": {netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("5.6.7.8")},
	}}

	got, errs := backend.ResolveSplitDomains(context.Background(), []string{"a.example", "b.example"}, resolver)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	want := []string{"1.2.3.4/32", "5.6.7.8/32"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("route[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveSplitDomains_PartialFailure(t *testing.T) {
	resolver := fakeSplitResolver{
		ips:  map[string][]netip.Addr{"ok.example": {netip.MustParseAddr("9.9.9.9")}},
		errs: map[string]error{"bad.example": errors.New("no such host")},
	}

	got, errs := backend.ResolveSplitDomains(context.Background(), []string{"ok.example", "bad.example"}, resolver)
	if len(got) != 1 || got[0] != "9.9.9.9/32" {
		t.Errorf("resolved: got %v", got)
	}
	if errs["bad.example"] == nil {
		t.Errorf("expected error for bad.example, got %v", errs)
	}
}

// ═══════════════════════════════════════════════════
// NormalizeSplitTunnel
// ═══════════════════════════════════════════════════

func TestNormalizeSplitTunnel_DefaultMode(t *testing.T) {
	got, err := backend.NormalizeSplitTunnel(backend.SplitTunnel{Enabled: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Mode != backend.SplitModeInclude {
		t.Errorf("mode: got %q, want %q", got.Mode, backend.SplitModeInclude)
	}
}

func TestNormalizeSplitTunnel_InvalidModeFallsBackToInclude(t *testing.T) {
	got, err := backend.NormalizeSplitTunnel(backend.SplitTunnel{Mode: "something"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Mode != backend.SplitModeInclude {
		t.Errorf("mode: got %q, want include", got.Mode)
	}
}

// ═══════════════════════════════════════════════════
// ResolveSplit
// ═══════════════════════════════════════════════════

func TestResolveSplit_Disabled(t *testing.T) {
	allowed := []string{"0.0.0.0/0"}
	tunnels, excludes := backend.ResolveSplit(allowed, backend.SplitTunnel{Enabled: false, Routes: []string{"10.0.0.0/8"}})
	if len(tunnels) != 1 || tunnels[0] != "0.0.0.0/0" {
		t.Errorf("tunnels: got %v", tunnels)
	}
	if excludes != nil {
		t.Errorf("excludes should be nil, got %v", excludes)
	}
}

func TestResolveSplit_Include(t *testing.T) {
	allowed := []string{"0.0.0.0/0", "::/0"}
	split := backend.SplitTunnel{
		Enabled: true,
		Mode:    backend.SplitModeInclude,
		Routes:  []string{"10.0.0.0/8", "1.2.3.4"},
	}
	tunnels, excludes := backend.ResolveSplit(allowed, split)

	want := []string{"10.0.0.0/8", "1.2.3.4/32"}
	if len(tunnels) != len(want) {
		t.Fatalf("tunnels: got %v, want %v", tunnels, want)
	}
	for i := range want {
		if tunnels[i] != want[i] {
			t.Errorf("tunnels[%d]: got %q, want %q", i, tunnels[i], want[i])
		}
	}
	if len(excludes) != 0 {
		t.Errorf("excludes should be empty, got %v", excludes)
	}
}

func TestResolveSplit_IncludeIntersectsAllowedIPs(t *testing.T) {
	// Пир принимает только 10.0.0.0/8 — маршрут на 192.168.0.0/16 в туннель
	// добавлять нельзя, пакеты были бы выброшены WireGuard.
	allowed := []string{"10.0.0.0/8"}
	split := backend.SplitTunnel{
		Enabled: true,
		Mode:    backend.SplitModeInclude,
		Routes:  []string{"10.1.0.0/16", "192.168.0.0/16"},
	}
	tunnels, _ := backend.ResolveSplit(allowed, split)

	if len(tunnels) != 1 || tunnels[0] != "10.1.0.0/16" {
		t.Errorf("tunnels: got %v, want [10.1.0.0/16]", tunnels)
	}
}

func TestResolveSplit_IncludeRouteWiderThanAllowed(t *testing.T) {
	// Пользователь указал 0.0.0.0/0, а пир принимает только 10.0.0.0/8:
	// маршрут сужается до разрешённой подсети.
	allowed := []string{"10.0.0.0/8"}
	split := backend.SplitTunnel{
		Enabled: true,
		Mode:    backend.SplitModeInclude,
		Routes:  []string{"0.0.0.0/0"},
	}
	tunnels, _ := backend.ResolveSplit(allowed, split)

	if len(tunnels) != 1 || tunnels[0] != "10.0.0.0/8" {
		t.Errorf("tunnels: got %v, want [10.0.0.0/8]", tunnels)
	}
}

func TestResolveSplit_IncludeEmptyListTunnelsNothing(t *testing.T) {
	allowed := []string{"0.0.0.0/0"}
	split := backend.SplitTunnel{Enabled: true, Mode: backend.SplitModeInclude}
	tunnels, excludes := backend.ResolveSplit(allowed, split)

	if len(tunnels) != 0 {
		t.Errorf("tunnels should be empty for empty include list, got %v", tunnels)
	}
	if excludes != nil {
		t.Errorf("excludes should be nil, got %v", excludes)
	}
}

func TestResolveSplit_IncludeDomainOnly(t *testing.T) {
	allowed := []string{"0.0.0.0/0"}
	split := backend.SplitTunnel{Enabled: true, Mode: backend.SplitModeInclude}
	tunnels, _ := backend.ResolveSplit(allowed, split, "1.2.3.4/32")

	if len(tunnels) != 1 || tunnels[0] != "1.2.3.4/32" {
		t.Errorf("tunnels: got %v, want [1.2.3.4/32]", tunnels)
	}
}

func TestResolveSplit_IncludeUnresolvedDomainsTunnelsNothing(t *testing.T) {
	// Домены не разрешились → маршрутов нет, но полный туннель включать нельзя.
	allowed := []string{"0.0.0.0/0"}
	split := backend.SplitTunnel{
		Enabled: true,
		Mode:    backend.SplitModeInclude,
		Domains: []string{"unresolved.example"},
	}
	tunnels, _ := backend.ResolveSplit(allowed, split)

	if len(tunnels) != 0 {
		t.Errorf("tunnels should be empty, got %v", tunnels)
	}
}

func TestResolveSplit_ExcludeEverything(t *testing.T) {
	allowed := []string{"0.0.0.0/0"}
	split := backend.SplitTunnel{
		Enabled: true,
		Mode:    backend.SplitModeExclude,
		Routes:  []string{"0.0.0.0/0"},
	}
	tunnels, excludes := backend.ResolveSplit(allowed, split)

	if len(tunnels) != 0 {
		t.Errorf("tunnels should be empty, got %v", tunnels)
	}
	if len(excludes) != 0 {
		t.Errorf("excludes should be empty, got %v", excludes)
	}
}

func TestResolveSplit_Exclude(t *testing.T) {
	allowed := []string{"0.0.0.0/0"}
	split := backend.SplitTunnel{
		Enabled: true,
		Mode:    backend.SplitModeExclude,
		Routes:  []string{"10.0.0.0/8", "192.168.0.0/16"},
	}
	tunnels, excludes := backend.ResolveSplit(allowed, split)

	if len(tunnels) != 1 || tunnels[0] != "0.0.0.0/0" {
		t.Errorf("tunnels: got %v", tunnels)
	}
	want := []string{"10.0.0.0/8", "192.168.0.0/16"}
	if len(excludes) != len(want) {
		t.Fatalf("excludes: got %v, want %v", excludes, want)
	}
	for i := range want {
		if excludes[i] != want[i] {
			t.Errorf("excludes[%d]: got %q, want %q", i, excludes[i], want[i])
		}
	}
}

func TestResolveSplit_IncludeWithDomainRoutes(t *testing.T) {
	allowed := []string{"0.0.0.0/0"}
	split := backend.SplitTunnel{
		Enabled: true,
		Mode:    backend.SplitModeInclude,
		Routes:  []string{"10.0.0.0/8"},
	}
	tunnels, _ := backend.ResolveSplit(allowed, split, "1.2.3.4/32")

	want := []string{"10.0.0.0/8", "1.2.3.4/32"}
	if len(tunnels) != len(want) {
		t.Fatalf("tunnels: got %v, want %v", tunnels, want)
	}
	for i := range want {
		if tunnels[i] != want[i] {
			t.Errorf("tunnels[%d]: got %q, want %q", i, tunnels[i], want[i])
		}
	}
}

func TestResolveSplit_ExcludeWithDomainRoutes(t *testing.T) {
	allowed := []string{"0.0.0.0/0"}
	split := backend.SplitTunnel{
		Enabled: true,
		Mode:    backend.SplitModeExclude,
		Routes:  []string{"10.0.0.0/8"},
	}
	tunnels, excludes := backend.ResolveSplit(allowed, split, "1.2.3.4/32")

	if len(tunnels) != 1 || tunnels[0] != "0.0.0.0/0" {
		t.Errorf("tunnels: got %v", tunnels)
	}
	want := []string{"10.0.0.0/8", "1.2.3.4/32"}
	if len(excludes) != len(want) {
		t.Fatalf("excludes: got %v, want %v", excludes, want)
	}
	for i := range want {
		if excludes[i] != want[i] {
			t.Errorf("excludes[%d]: got %q, want %q", i, excludes[i], want[i])
		}
	}
}

// ═══════════════════════════════════════════════════
// AppSettings — сериализация splitTunnel
// ═══════════════════════════════════════════════════

func TestAppSettings_SplitTunnelRoundtrip(t *testing.T) {
	s := backend.AppSettings{
		AutoStart: true,
		ObfsMode:  "audio",
		SplitTunnel: backend.SplitTunnel{
			Enabled: true,
			Mode:    backend.SplitModeExclude,
			Routes:  []string{"10.0.0.0/8", "1.2.3.4/32"},
			Domains: []string{"example.com"},
		},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"splitTunnel"`) {
		t.Errorf("JSON should contain splitTunnel: %s", data)
	}

	var got backend.AppSettings
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !got.SplitTunnel.Enabled {
		t.Error("SplitTunnel.Enabled should be true")
	}
	if got.SplitTunnel.Mode != backend.SplitModeExclude {
		t.Errorf("SplitTunnel.Mode: got %q", got.SplitTunnel.Mode)
	}
	if len(got.SplitTunnel.Routes) != 2 {
		t.Errorf("SplitTunnel.Routes: got %v", got.SplitTunnel.Routes)
	}
	if len(got.SplitTunnel.Domains) != 1 || got.SplitTunnel.Domains[0] != "example.com" {
		t.Errorf("SplitTunnel.Domains: got %v", got.SplitTunnel.Domains)
	}
}

func TestAppSettings_NoSplitTunnelInOldConfig(t *testing.T) {
	// Старый config.json без splitTunnel должен читаться без ошибок.
	var s backend.AppSettings
	if err := json.Unmarshal([]byte(`{"autoStart":true,"obfsMode":"video"}`), &s); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if s.SplitTunnel.Enabled {
		t.Error("split tunneling should be disabled by default")
	}
	if len(s.SplitTunnel.Routes) != 0 {
		t.Errorf("routes should be empty, got %v", s.SplitTunnel.Routes)
	}
}
