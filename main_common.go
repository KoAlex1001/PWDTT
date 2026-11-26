package main

import (
	"context"
	"os"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Размеры окна по умолчанию: 440 в ширину и 720 в высоту.
const (
	defaultWindowWidth  = 440
	defaultWindowHeight = 720
	minWindowWidth      = 440
	minWindowHeight     = 480
)

// hasMinimizedFlag сообщает, запущено ли приложение с флагом --minimized.
// Именно так его запускает автозапуск, чтобы не показывать окно при старте.
func hasMinimizedFlag() bool {
	for _, arg := range os.Args[1:] {
		switch strings.TrimSpace(arg) {
		case "--minimized", "-minimized", "--tray":
			return true
		}
	}
	return false
}

// minimiseWindowSoon сворачивает окно после старта (Linux/macOS). Небольшая
// задержка нужна, чтобы окно успело создаться и попало в панель задач/Dock —
// оттуда пользователь сможет его развернуть.
func minimiseWindowSoon(ctx context.Context) {
	go func() {
		time.Sleep(600 * time.Millisecond)
		wailsruntime.WindowMinimise(ctx)
	}()
}
