package main

import (
	"embed"
	"log"

	"github.com/tachitachi/RuneCooldownTracker/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed frontend/src/assets/images/logo-universal.png
var trayIcon []byte

func main() {
	appService := app.NewApp()

	app := application.New(application.Options{
		Name:        "RuneCooldownTracker",
		Description: "RuneScape ability cooldown overlay",
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Services: []application.Service{
			application.NewService(appService),
		},
		OnShutdown: func() {
			appService.Shutdown()
		},
	})

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		appService.Startup(app)
	})

	appService.CreateOverlayWindow(app)
	appService.CreateConfigWindow(app)

	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("RuneCooldownTracker")

	menu := app.NewMenu()
	menu.Add("Open Config").OnClick(func(_ *application.Context) {
		appService.CreateConfigWindow(app)
	})
	menu.Add("Cooldown Tracker Config").OnClick(func(_ *application.Context) {
		appService.CreateCooldownTrackerConfigWindow(app)
	})
	menu.Add("Quit").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
