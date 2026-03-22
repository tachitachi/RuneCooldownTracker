package main

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"syscall"
	"time"

	"RuneCooldownTracker/internal/capture"

	"github.com/TKMAX777/winapi"
	"github.com/lxn/win"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// App struct
type App struct {
	app           *application.App
	handler       *capture.CaptureHandler
	overlayWindow *application.WebviewWindow
	configWindow  *application.WebviewWindow
	dpiScale      int64 // atomic; stores math.Float64bits of current DPI scale
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called from the ApplicationStarted event in main.go.
func (a *App) startup(app *application.App) {
	a.app = app
	// a.EnableClickThrough()
	a.followWindow(app.Context(), "RuneScape")
	a.captureGraphics(app.Context(), "RuneScape")
}

func (a *App) shutdown() {
	fmt.Println("Application shutting down, closing capture handler...")
	if a.handler != nil {
		if err := a.handler.Close(); err != nil {
			fmt.Printf("Error closing handler: %v\n", err)
		}
	}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// followWindow is private — only called from startup, not bound to the frontend.
func (a *App) followWindow(ctx context.Context, targetTitle string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	getDpiForWindow := user32.NewProc("GetDpiForWindow")

	go func() {
		titlePtr := syscall.StringToUTF16Ptr(targetTitle)

		for {
			select {
			case <-ctx.Done():
				fmt.Println("Window tracker stopping...")
				return
			default:
			}

			targetHwnd := win.FindWindow(nil, titlePtr)
			if targetHwnd != 0 {
				var rect win.RECT
				if win.GetWindowRect(targetHwnd, &rect) {
					dpi, _, _ := getDpiForWindow.Call(uintptr(targetHwnd))
					if dpi == 0 {
						dpi = 96
					}
					scale := float64(dpi) / 96.0
					atomic.StoreInt64(&a.dpiScale, int64(math.Float64bits(scale)))
					a.overlayWindow.SetPosition(int(float64(rect.Left)/scale), int(float64(rect.Top)/scale))
					a.overlayWindow.SetSize(int(float64(rect.Right-rect.Left)/scale), int(float64(rect.Bottom-rect.Top)/scale))
				}
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()
}

// StartSnipping makes the overlay interactive so the user can drag a capture region.
func (a *App) StartSnipping() {
	a.overlayWindow.SetIgnoreMouseEvents(false)
	a.app.Event.Emit("snipping:start")
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
	a.app.Event.Emit("snipping:confirmed", map[string]any{
		"x": region.X, "y": region.Y, "w": region.W, "h": region.H,
	})
}

// CancelSnipping aborts the snipping flow and restores click-through.
func (a *App) CancelSnipping() {
	a.overlayWindow.SetIgnoreMouseEvents(true)
	a.app.Event.Emit("snipping:cancelled")
}

// captureGraphics is private — only called from startup, not bound to the frontend.
func (a *App) captureGraphics(ctx context.Context, targetWindow string) {
	var rdHwnd win.HWND
	rdHwnd = winapi.FindWindowEx(0, rdHwnd, nil, winapi.MustUTF16PtrFromString(targetWindow))
	if rdHwnd == 0 {
		fmt.Printf("Could not find window: %s\n", targetWindow)
		return
	}
	fmt.Printf("Found target window: %d\n", rdHwnd)

	a.handler = &capture.CaptureHandler{}

	go func() {
		if err := a.handler.StartCapture(rdHwnd); err != nil {
			fmt.Println("Capture Error:", err)
		}
	}()
}
