//go:build darwin

package main

import (
	"context"
	"embed"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"pwdtt/backend"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/icons/icon-tray.png
var trayIcon []byte

//go:embed assets/icons/icon-tray-active.png
var trayIconActive []byte

var (
	trayReady    atomic.Bool
	trayStopping atomic.Bool
	trayStop     = make(chan struct{})
	trayExited   = make(chan struct{})
	trayStopOnce sync.Once
	trayExitOnce sync.Once

	trayMu         sync.Mutex
	trayState      = "idle"
	trayStatusItem *systray.MenuItem
	trayToggleItem *systray.MenuItem
)

// showWindow разворачивает и показывает окно приложения.
func showWindow(ctx context.Context) {
	wailsruntime.WindowUnminimise(ctx)
	wailsruntime.WindowShow(ctx)
}

// setTrayState вызывается из backend при смене состояния туннеля.
func setTrayState(state string) {
	trayMu.Lock()
	trayState = state
	if trayReady.Load() {
		trayApplyLocked()
	}
	trayMu.Unlock()
}

// trayApplyLocked обновляет иконку, подсказку и пункты меню под текущее
// состояние. Вызывать с захваченным trayMu.
func trayApplyLocked() {
	if trayStatusItem == nil || trayToggleItem == nil {
		return
	}
	switch trayState {
	case "connected":
		systray.SetIcon(trayIconActive)
		systray.SetTooltip("PWDTT — подключено")
		trayStatusItem.SetTitle("Статус: подключено")
		trayToggleItem.SetTitle("Отключиться")
		trayToggleItem.Enable()
	case "connecting":
		systray.SetIcon(trayIconActive)
		systray.SetTooltip("PWDTT — подключение…")
		trayStatusItem.SetTitle("Статус: подключение…")
		trayToggleItem.SetTitle("Отмена")
		trayToggleItem.Enable()
	case "disconnecting":
		systray.SetIcon(trayIcon)
		systray.SetTooltip("PWDTT — отключение…")
		trayStatusItem.SetTitle("Статус: отключение…")
		trayToggleItem.SetTitle("Отключиться")
		trayToggleItem.Disable()
	default:
		systray.SetIcon(trayIcon)
		systray.SetTooltip("PWDTT — отключено")
		trayStatusItem.SetTitle("Статус: отключено")
		trayToggleItem.SetTitle("Подключиться")
		trayToggleItem.Enable()
	}
}

// startTray запускает трей через внешний цикл Wails: на macOS systray не должен
// самостоятельно запускать NSApplication.
func startTray(ctx context.Context, app *backend.App) {
	start, _ := systray.RunWithExternalLoop(func() {
		if trayStopping.Load() {
			return
		}

		trayMu.Lock()
		trayStatusItem = systray.AddMenuItem("Статус: отключено", "")
		trayStatusItem.Disable()
		trayToggleItem = systray.AddMenuItem("Подключиться", "Подключить или отключить туннель")
		trayToggleItem.Click(func() { app.ToggleTunnel() })
		systray.AddSeparator()
		show := systray.AddMenuItem("Открыть PWDTT", "Показать окно PWDTT")
		show.Click(func() { showWindow(ctx) })
		systray.AddSeparator()
		exit := systray.AddMenuItem("Выход", "Закрыть PWDTT")
		exit.Click(func() { wailsruntime.Quit(ctx) })

		// На macOS меню.attach'ится к иконке в меню-баре: левый клик открывает
		// меню нативно (ShowMenu() вне OnRClick на macOS не поддерживается).
		systray.CreateMenu()
		trayMu.Unlock()

		trayReady.Store(true)
		trayMu.Lock()
		trayApplyLocked()
		trayMu.Unlock()

		go hideMinimisedWindow(ctx)
	}, func() {
		trayReady.Store(false)
		trayExitOnce.Do(func() { close(trayExited) })
	})

	start()
}

// hideMinimisedWindow сворачивает окно в трей, когда пользователь нажал
// жёлтую кнопку в macOS — приложение продолжает работать в меню-баре.
func hideMinimisedWindow(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-trayStop:
			return
		case <-ticker.C:
			if trayReady.Load() && wailsruntime.WindowIsMinimised(ctx) {
				wailsruntime.WindowHide(ctx)
			}
		}
	}
}

func stopTray() {
	trayStopping.Store(true)
	trayStopOnce.Do(func() { close(trayStop) })
	// macOS: systray.Quit() вызвал бы [NSApp terminate] во время уже идущего
	// завершения Wails, поэтому просто снимаем флаг готовности.
	trayReady.Store(false)
	trayExitOnce.Do(func() { close(trayExited) })
}

func main() {
	// Привилегированный режим настройки туннеля (запускается от root через osascript).
	// Перехватываем до инициализации GUI.
	if len(os.Args) > 1 && os.Args[1] == "--wg-helper" {
		backend.RunWGHelperDarwin(os.Args[2:])
		return
	}

	app := backend.NewApp()
	app.TrayNotify = setTrayState
	secondInstanceLaunch := make(chan struct{}, 1)
	startMinimized := hasMinimizedFlag()

	err := wails.Run(&options.App{
		Title:     "PWDTT",
		Width:     defaultWindowWidth,
		Height:    defaultWindowHeight,
		MinWidth:  minWindowWidth,
		MinHeight: minWindowHeight,
		Frameless: false,
		// Закрытие окна скрывает его в трей, приложение продолжает работать.
		// Полностью выйти можно через пункт «Выход» в меню трея.
		HideWindowOnClose: true,
		StartHidden:       startMinimized,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			startTray(ctx, app)
			go func() {
				for range secondInstanceLaunch {
					showWindow(ctx)
				}
			}()
		},
		OnShutdown: func(ctx context.Context) {
			stopTray()
			app.Shutdown(ctx)
		},
		Bind: []interface{}{app},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "pwdtt-darwin-client",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				select {
				case secondInstanceLaunch <- struct{}{}:
				default:
				}
			},
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       false,
			},
			About: &mac.AboutInfo{
				Title:   "PWDTT",
				Message: "© 2026 PWDTT",
			},
		},
	})
	if err != nil {
		panic(err)
	}
}
