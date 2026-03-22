package main

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"RuneCooldownTracker/internal/capture"

	"github.com/TKMAX777/winapi"
	"github.com/lxn/win"
)

// App struct
type App struct {
	ctx     context.Context
	handler *capture.CaptureHandler
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.EnableClickThrough()
	a.FollowWindow(ctx, "RuneScape")
	a.CaptureGraphics(ctx, "RuneScape")
}

func (a *App) shutdown(ctx context.Context) {
	fmt.Println("Application shutting down, closing capture handler...")
	if a.handler != nil {
		err := a.handler.Close()
		if err != nil {
			fmt.Printf("Error closing handler: %v\n", err)
		}
	}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) EnableClickThrough() {
	hwnd := win.FindWindow(nil, syscall.StringToUTF16Ptr("RuneCooldownTracker"))
	if hwnd == 0 {
		return
	}

	// 1. Get current extended styles
	exStyle := win.GetWindowLong(hwnd, win.GWL_EXSTYLE)

	// 2. Explicitly ADD Layered and Transparent (Click-through)
	// We also add TOOLWINDOW to hide it from the taskbar if you want a pure overlay
	exStyle |= win.WS_EX_LAYERED | win.WS_EX_TRANSPARENT

	// 3. Force the style
	win.SetWindowLong(hwnd, win.GWL_EXSTYLE, exStyle)

	// 4. CRITICAL: Set Layered Window Attributes
	// This tells Windows to treat the 0-alpha pixels as truly transparent/clickable
	// You may need to import "github.com/lxn/win" or use syscall for this
	// SetLayeredWindowAttributes(hwnd, 0, 255, LWA_ALPHA)

	// 5. Refresh the window position and flags
	win.SetWindowPos(hwnd, win.HWND_TOPMOST, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_SHOWWINDOW|win.SWP_FRAMECHANGED)
}

func (a *App) FollowWindow(ctx context.Context, targetTitle string) {
	go func() {
		// Give the app time to initialize
		// time.Sleep(1 * time.Second)

		select {
		case <-ctx.Done():
			fmt.Println("Cleaning up window tracker...")
			return
		default:
			// Find OUR window (the overlay)
			myHwnd := win.FindWindow(nil, syscall.StringToUTF16Ptr("RuneCooldownTracker"))
			// Find THE TARGET window (e.g., the game)
			targetTitle := syscall.StringToUTF16Ptr(targetTitle) // Replace with actual title

			for {
				targetHwnd := win.FindWindow(nil, targetTitle)

				if targetHwnd != 0 && myHwnd != 0 {
					var rect win.RECT
					// Get the position of the target window
					if win.GetWindowRect(targetHwnd, &rect) {
						width := rect.Right - rect.Left
						height := rect.Bottom - rect.Top

						// Move our window to match exactly
						// SWP_NOZORDER ensures we don't mess with AlwaysOnTop
						// SWP_NOACTIVATE ensures we don't steal focus
						win.SetWindowPos(myHwnd, 0,
							rect.Left, rect.Top, width, height,
							win.SWP_NOZORDER|win.SWP_NOACTIVATE)
					}
				}

				// Poll every 16ms (~60fps) or 100ms for lower CPU usage
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (a *App) CaptureGraphics(ctx context.Context, targetWindow string) {
	var rdHwnd win.HWND
	rdHwnd = winapi.FindWindowEx(0, rdHwnd, nil, winapi.MustUTF16PtrFromString(targetWindow))
	if rdHwnd == 0 {
		fmt.Printf("Could not find window: %s\n", targetWindow)
		return
	}
	fmt.Printf("hello %d\n", rdHwnd)

	a.handler = &capture.CaptureHandler{}

	go func() {
		err := a.handler.StartCapture(rdHwnd)
		if err != nil {
			fmt.Println("Capture Error:", err)
		}
	}()
}
