package app

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tachitachi/RuneCooldownTracker/internal/capture"
	"github.com/tachitachi/RuneCooldownTracker/internal/detection"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/TKMAX777/winapi"
	"github.com/lxn/win"
)

func (a *App) CreateOverlayWindow(app *application.App) {
	a.overlayWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "RuneCooldownTracker",
		Width:            3840, // for some reason this needs to start large, so that the snipping can interact with the whole screen after resizsing
		Height:           2160,
		Frameless:        true,
		AlwaysOnTop:      true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		URL:              "/",
		Windows: application.WindowsWindow{
			BackdropType: application.None,
		},
	})

	a.overlayWindow.SetIgnoreMouseEvents(true)
	a.overlayWindow.SetAlwaysOnTop(true)
}

// followWindow tracks the target window's position and size, keeping the
// overlay aligned with it. Also caches the DPI scale for snipping coordinate math.
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

// captureGraphics finds the target window and starts the D3D11 capture loop.
func (a *App) captureGraphics(targetWindow string) {
	var rdHwnd win.HWND
	rdHwnd = winapi.FindWindowEx(0, rdHwnd, nil, winapi.MustUTF16PtrFromString(targetWindow))
	if rdHwnd == 0 {
		fmt.Printf("Could not find window: %s\n", targetWindow)
		return
	}
	fmt.Printf("Found target window: %d\n", rdHwnd)

	a.detector = &detection.AbilityDetector{}
	a.detector.OnLayoutDetected = func(layout detection.SlotLayout) {
		a.currentLayout = &layout
		a.emitGridLines(layout)
		// If tracking was active when the layout changed, re-identify slots.
		if a.detector.IsTracking() {
			refs := detection.LoadReferenceIcons()
			refImages := detection.BuildRefImages(refs)
			a.detector.StartTracking(refImages)
		}
	}

	a.handler = &capture.CaptureHandler{
		Processor: a.detector,
	}

	go func() {
		if err := a.handler.StartCapture(rdHwnd); err != nil {
			fmt.Println("Capture Error:", err)
		}
	}()
}

func (a *App) closeGraphics() {
	if a.handler != nil {
		fmt.Println("Closing capture handler...")
		if err := a.handler.Close(); err != nil {
			fmt.Printf("Error closing handler: %v\n", err)
		}
	}
}
