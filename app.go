package main

import (
	"fmt"

	"github.com/tachitachi/RuneCooldownTracker/internal/capture"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App is the Wails service that bridges the frontend and backend.
type App struct {
	app           *application.App
	handler       *capture.CaptureHandler
	overlayWindow *application.WebviewWindow
	configWindow  *application.WebviewWindow
	dpiScale      int64 // atomic; stores math.Float64bits of current DPI scale
}

func NewApp() *App {
	return &App{}
}

// startup is called from the ApplicationStarted event in main.go.
func (a *App) startup(app *application.App) {
	a.app = app
	a.followWindow(app.Context(), "RuneScape")
	a.captureGraphics("RuneScape")
}

func (a *App) shutdown() {
	fmt.Println("Application shutting down")
	a.closeGraphics()
}
