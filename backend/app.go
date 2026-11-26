package backend

import (
	"context"
	"fmt"
	"strings"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"
)

const Version = "1.6.0"

// App — главный объект приложения.
// Wails привязывает его методы к frontend через Bind().
type App struct {
	ctx    context.Context
	bridge *Bridge
	store  *Store
	// TrayNotify — хук в системный трей (реализуется в main_windows.go и main_darwin.go).
	// nil на платформах без трея. Поле, а не метод, чтобы Wails не пытался
	// экспортировать его во frontend.
	TrayNotify func(state string)
}

// NewApp создаёт App. Вызывается из main() до wails.Run().
func NewApp() *App {
	return &App{
		store: NewStore(),
	}
}

// ═══════════════════════════════════════════════════
// WAILS LIFECYCLE
// ═══════════════════════════════════════════════════

// Startup вызывается Wails после создания webview.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	settings := a.store.LoadSettings()
	a.bridge = NewBridge(ctx, a.store, a.onBridgeEvent)

	if settings.AutoStart {
		a.SetAutoStart(true)
	}

	// Уборка маршрутов, оставшихся от краша прошлого запуска
	go CleanupStaleExcludeRoutes(func(msg string) {
		a.onBridgeEvent("log", "INFO", "[WG] "+msg)
	})
}

// Shutdown вызывается Wails при закрытии приложения.
func (a *App) Shutdown(ctx context.Context) {
	if a.bridge != nil {
		a.bridge.Disconnect()
	}
}

// ═══════════════════════════════════════════════════
// WAILS BINDINGS
// ═══════════════════════════════════════════════════

func (a *App) Connect(params ConnectParams) error {
	return a.bridge.Connect(params)
}

func (a *App) Disconnect() {
	a.bridge.Disconnect()
}

// SetLastProfile запоминает профиль, который трей будет подключать по клику.
func (a *App) SetLastProfile(name string) error {
	name = strings.TrimSpace(name)
	settings := a.store.LoadSettings()
	if settings.LastProfile == name {
		return nil
	}
	settings.LastProfile = name
	return a.store.SaveSettings(settings)
}

// ToggleTunnel — действие пункта «Подключиться/Отключиться» в трее.
// Не блокирует UI-поток: подключение идёт в фоне.
func (a *App) ToggleTunnel() {
	go func() {
		if a.bridge == nil {
			return
		}
		if a.bridge.IsRunning() {
			a.onBridgeEvent("state_changed", "disconnecting")
			a.bridge.Disconnect()
			return
		}
		a.onBridgeEvent("state_changed", "connecting")
		if err := a.connectLastProfile(); err != nil {
			a.onBridgeEvent("error", err.Error())
			a.onBridgeEvent("state_changed", "disconnected")
		}
	}()
}

// connectLastProfile подключается к профилю, сохранённому фронтендом как
// последний выбранный. Используется треем, когда окно скрыто.
func (a *App) connectLastProfile() error {
	name := a.store.LoadSettings().LastProfile
	if name == "" {
		// Запасной вариант: если профиль ровно один — подключаемся к нему.
		profiles := a.store.ListProfiles()
		if len(profiles) == 1 {
			for n := range profiles {
				name = n
			}
		}
	}
	if name == "" {
		return fmt.Errorf("сервер не выбран — откройте PWDTT и выберите сервер")
	}
	p, err := a.store.LoadProfile(name)
	if err != nil {
		return fmt.Errorf("профиль %q: %w", name, err)
	}
	hashes := make([]string, 0, len(p.Hashes))
	for _, h := range p.Hashes {
		if strings.TrimSpace(h) != "" {
			hashes = append(hashes, h)
		}
	}
	if len(hashes) == 0 {
		return fmt.Errorf("в профиле %q нет хешей VK", name)
	}
	workers := p.Workers
	if workers <= 0 {
		workers = len(hashes) * 9
		if workers < 9 {
			workers = 9
		}
	}
	settings := a.store.LoadSettings()
	return a.bridge.Connect(ConnectParams{
		PeerAddr:    p.PeerAddr,
		Password:    p.Password,
		Hashes:      hashes,
		DeviceID:    p.DeviceID,
		Workers:     workers,
		CaptchaMode: "auto",
		ObfsMode:    settings.ObfsMode,
		TurnTCP:     settings.TurnTCP,
	})
}

func (a *App) IsRunning() bool {
	return a.bridge.IsRunning()
}

func (a *App) GetVersion() string {
	return Version
}

// ═══════════════════════════════════════════════════
// PROFILES
// ═══════════════════════════════════════════════════

func (a *App) GetProfile(name string) (*ProfileData, error) {
	return a.store.LoadProfile(name)
}

func (a *App) SaveProfile(name string, p ProfileData) error {
	return a.store.SaveProfile(name, p)
}

func (a *App) DeleteProfile(name string) error {
	return a.store.DeleteProfile(name)
}

func (a *App) ListProfiles() map[string]ProfileData {
	return a.store.ListProfiles()
}

// ═══════════════════════════════════════════════════
// SETTINGS
// ═══════════════════════════════════════════════════

func (a *App) GetAutoStart() bool {
	return a.store.LoadSettings().AutoStart
}

func (a *App) SetAutoStart(v bool) error {
	settings := a.store.LoadSettings()
	settings.AutoStart = v
	a.store.SaveSettings(settings)
	return setAutoStart(v, settings.StartMinimized)
}

// GetStartMinimized — стартовать ли при автозапуске в свёрнутом режиме.
func (a *App) GetStartMinimized() bool {
	return a.store.LoadSettings().StartMinimized
}

// SetStartMinimized сохраняет настройку и, если автозапуск включён,
// перерегистрирует его с новым аргументом командной строки.
func (a *App) SetStartMinimized(v bool) error {
	settings := a.store.LoadSettings()
	settings.StartMinimized = v
	if err := a.store.SaveSettings(settings); err != nil {
		return err
	}
	if settings.AutoStart {
		return setAutoStart(true, v)
	}
	return nil
}

func (a *App) GetObfsMode() string {
	return a.store.LoadSettings().ObfsMode
}

func (a *App) SetObfsMode(mode string) error {
	settings := a.store.LoadSettings()
	settings.ObfsMode = mode
	return a.store.SaveSettings(settings)
}

func (a *App) GetObfsAccepted() bool {
	return a.store.LoadSettings().ObfsAccepted
}

func (a *App) SetObfsAccepted(v bool) error {
	settings := a.store.LoadSettings()
	settings.ObfsAccepted = v
	return a.store.SaveSettings(settings)
}

// GetSplitTunnel возвращает нормализованные настройки раздельного туннелирования.
func (a *App) GetSplitTunnel() SplitTunnel {
	split, _ := NormalizeSplitTunnel(a.store.LoadSettings().SplitTunnel)
	return split
}

// SetSplitTunnel валидирует, нормализует и сохраняет настройки раздельного
// туннелирования. Возвращает нормализованный вариант (например, одиночные
// адреса превращаются в /32). При ошибке валидации настройки не сохраняются.
func (a *App) SetSplitTunnel(split SplitTunnel) (SplitTunnel, error) {
	normalized, err := NormalizeSplitTunnel(split)
	if err != nil {
		return normalized, err
	}
	settings := a.store.LoadSettings()
	settings.SplitTunnel = normalized
	if err := a.store.SaveSettings(settings); err != nil {
		return normalized, err
	}
	return normalized, nil
}

func (a *App) CheckUpdate() *UpdateInfo {
	info, err := CheckUpdate(Version)
	if err != nil {
		return &UpdateInfo{Available: false}
	}
	return info
}

// ═══════════════════════════════════════════════════
// INTERNAL
// ═══════════════════════════════════════════════════

func (a *App) onBridgeEvent(name string, args ...any) {
	if name == "state_changed" && len(args) > 0 {
		if s, ok := args[0].(string); ok && a.TrayNotify != nil {
			a.TrayNotify(s)
		}
	}
	wails.EventsEmit(a.ctx, name, args...)
}
