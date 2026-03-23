package main

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/tachitachi/RuneCooldownTracker/internal/capture"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func (a *App) createConfigWindow(app *application.App) {
	a.configWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "RuneCooldownTracker Config",
		Width:          480,
		Height:         320,
		Hidden:         false,
		BackgroundType: application.BackgroundTypeSolid,
		URL:            "/config.html",
	})

	a.configWindow.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		fmt.Println("closing...")
		a.configWindow = nil
	})
}

// StartSnipping makes the overlay interactive so the user can drag a capture region.
func (a *App) StartSnipping() {
	a.overlayWindow.SetIgnoreMouseEvents(false)
	a.app.Event.Emit("snipping:start")
}

func (a *App) emitCropRegion() {
	region := a.handler.GetCropRegion()
	if region != nil {
		a.app.Event.Emit("snipping:confirmed", map[string]any{
			"x": region.X, "y": region.Y, "w": region.W, "h": region.H,
		})
	}
}

// ConfirmSnip receives the logical-pixel rect from JS, converts to physical pixels,
// stores the crop region, and restores click-through on the overlay.
func (a *App) ConfirmSnip(x, y, w, h float64) {
	scale := math.Float64frombits(uint64(atomic.LoadInt64(&a.dpiScale)))
	if scale == 0 {
		scale = 1.0
	}
	region := &capture.CropRegion{
		X: int(math.Round(x * scale)),
		Y: int(math.Round(y * scale)),
		W: int(math.Round(w * scale)),
		H: int(math.Round(h * scale)),
	}
	if a.handler != nil {
		a.handler.SetCropRegion(region)
	}
	a.overlayWindow.SetIgnoreMouseEvents(true)
	a.emitCropRegion()
}

// CancelSnipping aborts the snipping flow and restores click-through.
func (a *App) CancelSnipping() {
	a.overlayWindow.SetIgnoreMouseEvents(true)
	a.app.Event.Emit("snipping:cancelled")
}
