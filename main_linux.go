//go:build linux

package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"pwdtt/backend"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/icons/icon.png
var appIcon []byte

func main() {
	app := backend.NewApp()
	startMinimized := hasMinimizedFlag()

	err := wails.Run(&options.App{
		Title:     "PWDTT",
		Width:     defaultWindowWidth,
		Height:    defaultWindowHeight,
		MinWidth:  minWindowWidth,
		MinHeight: minWindowHeight,
		Frameless: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			if startMinimized {
				minimiseWindowSoon(ctx)
			}
		},
		OnShutdown: app.Shutdown,
		Bind:       []interface{}{app},
		Linux: &linux.Options{
			ProgramName: "PWDTT",
			Icon:        appIcon,
		},
	})
	if err != nil {
		panic(err)
	}
}
