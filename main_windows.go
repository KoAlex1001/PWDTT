//go:build windows

package main

import (
	"context"
	"embed"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"pwdtt/backend"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/icons/icon.png
var appIcon []byte

//go:embed assets/icons/icon.ico
var trayIcon []byte

//go:embed assets/icons/icon-active.ico
var trayIconActive []byte

//go:embed assets/wintun.dll
var wintunDLL []byte

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

func startTray(ctx context.Context, app *backend.App) {
	// systray на Windows создаёт скрытое окно и крутит GetMessage в той же
	// горутине. Окно и очередь сообщений привязаны к потоку ОС, поэтому
	// горутину нужно закрепить за потоком: иначе планировщик Go может
	// перенести цикл трея на другой поток и иконка перестанет отвечать
	// на клики (в том числе на пункт «Выход»).
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			if trayStopping.Load() {
				systray.Quit()
				return
			}

			// Клик по значку открывает меню с кнопкой подключения/отключения,
			// двойной клик — окно приложения.
			systray.SetOnClick(func(menu systray.IMenu) {
				if menu != nil {
					_ = menu.ShowMenu()
				}
			})
			systray.SetOnDClick(func(systray.IMenu) { showWindow(ctx) })

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
			trayMu.Unlock()

			trayReady.Store(true)

			trayMu.Lock()
			trayApplyLocked()
			trayMu.Unlock()

			go hideMinimisedWindow(ctx)
		}, func() {
			trayReady.Store(false)
			trayExitOnce.Do(func() { close(trayExited) })
			if !trayStopping.Load() {
				showWindow(ctx)
			}
		})
	}()
}

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
	if trayReady.Load() {
		systray.Quit()
		select {
		case <-trayExited:
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func main() {
	backend.InitWintun(wintunDLL)
	app := backend.NewApp()
	app.TrayNotify = setTrayState
	secondInstanceLaunch := make(chan struct{}, 1)
	startMinimized := hasMinimizedFlag()

	err := wails.Run(&options.App{
		Title:       "PWDTT",
		Width:       defaultWindowWidth,
		Height:      defaultWindowHeight,
		MinWidth:    minWindowWidth,
		MinHeight:   minWindowHeight,
		Frameless:   false,
		StartHidden: startMinimized,
		// Закрытие окна скрывает его в трей, приложение продолжает работать.
		// Полностью выйти можно через пункт «Выход» в меню трея.
		HideWindowOnClose: true,
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
			UniqueId: "pwdtt-windows-client",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				select {
				case secondInstanceLaunch <- struct{}{}:
				default:
				}
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		panic(err)
	}
}
