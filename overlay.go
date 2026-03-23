package main

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tachitachi/RuneCooldownTracker/internal/capture"

	"github.com/TKMAX777/winapi"
	"github.com/lxn/win"
)

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

	a.handler = &capture.CaptureHandler{}

	go func() {
		if err := a.handler.StartCapture(rdHwnd); err != nil {
			fmt.Println("Capture Error:", err)
		}
	}()
}
