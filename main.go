package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed frontend/src/assets/images/logo-universal.png
var trayIcon []byte

func main() {
	appService := NewApp()

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
			appService.shutdown()
		},
	})

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		appService.startup(app)
	})

	appService.overlayWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "RuneCooldownTracker",
		Width:            1024,
		Height:           768,
		Frameless:        true,
		AlwaysOnTop:      false,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		URL:              "/",
		Windows: application.WindowsWindow{
			BackdropType: application.None,
		},
	})

	appService.overlayWindow.SetIgnoreMouseEvents(true)
	appService.overlayWindow.SetAlwaysOnTop(true)

	appService.configWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "RuneCooldownTracker Config",
		Width:          480,
		Height:         320,
		Hidden:         true,
		BackgroundType: application.BackgroundTypeSolid,
		URL:            "/config.html",
	})

	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("RuneCooldownTracker")

	menu := app.NewMenu()
	menu.Add("Open Config").OnClick(func(_ *application.Context) {
		appService.configWindow.Show()
	})
	menu.Add("Quit").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
