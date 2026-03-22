package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

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

	application.NewWindow(application.WebviewWindowOptions{
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

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
