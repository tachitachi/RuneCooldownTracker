package app

import (
	"fmt"
	"image"
	"math"
	"sync/atomic"

	"github.com/tachitachi/RuneCooldownTracker/internal/capture"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func (a *App) CreateConfigWindow(app *application.App) {
	if a.configWindow != nil {
		return
	}

	a.configWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "RuneCooldownTracker Config",
		Width:          840,
		Height:         1200,
		Hidden:         false,
		BackgroundType: application.BackgroundTypeSolid,
		URL:            "/config.html",
	})

	a.configWindow.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		fmt.Println("closing...")
		a.configWindow = nil
	})

	a.configWindow.OnWindowEvent(events.Common.WindowShow, func(_ *application.WindowEvent) {
		if a.handler != nil {
			a.emitCropRegion()
		}
		if a.currentLayout != nil {
			a.emitGridLines(*a.currentLayout)
		}
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

// ConfirmSnip receives the logical-pixel rect and the first click offset from JS,
// converts to physical pixels, stores the crop region, and restores click-through.
// firstClickX/Y are the first click position relative to the crop top-left in logical pixels.
func (a *App) ConfirmSnip(x, y, w, h, firstClickX, firstClickY float64) {
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
	a.clickInCrop = image.Point{
		X: int(math.Round(firstClickX * scale)),
		Y: int(math.Round(firstClickY * scale)),
	}
	if a.detector != nil {
		a.detector.ClickHint = &a.clickInCrop
		a.detector.UnlockLayout()
	}
	if a.handler != nil {
		a.handler.SetCropRegion(region)
	}
	a.overlayWindow.SetIgnoreMouseEvents(true)
	a.emitCropRegion()
	a.saveConfig()
}

// CancelSnipping aborts the snipping flow and restores click-through.
func (a *App) CancelSnipping() {
	a.overlayWindow.SetIgnoreMouseEvents(true)
	a.app.Event.Emit("snipping:cancelled")
}
