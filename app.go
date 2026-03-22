package main

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"RuneCooldownTracker/internal/capture"

	"github.com/TKMAX777/winapi"
	"github.com/lxn/win"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// App struct
type App struct {
	app     *application.App
	handler *capture.CaptureHandler
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called from the ApplicationStarted event in main.go.
func (a *App) startup(app *application.App) {
	a.app = app
	a.EnableClickThrough()
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

func (a *App) EnableClickThrough() {
	hwnd := win.FindWindow(nil, syscall.StringToUTF16Ptr("RuneCooldownTracker"))
	if hwnd == 0 {
		return
	}

	exStyle := win.GetWindowLong(hwnd, win.GWL_EXSTYLE)
	exStyle |= win.WS_EX_LAYERED | win.WS_EX_TRANSPARENT
	win.SetWindowLong(hwnd, win.GWL_EXSTYLE, exStyle)

	win.SetWindowPos(hwnd, win.HWND_TOPMOST, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_SHOWWINDOW|win.SWP_FRAMECHANGED)
}

// followWindow is private — only called from startup, not bound to the frontend.
func (a *App) followWindow(ctx context.Context, targetTitle string) {
	go func() {
		myHwnd := win.FindWindow(nil, syscall.StringToUTF16Ptr("RuneCooldownTracker"))
		titlePtr := syscall.StringToUTF16Ptr(targetTitle)

		for {
			select {
			case <-ctx.Done():
				fmt.Println("Window tracker stopping...")
				return
			default:
			}

			targetHwnd := win.FindWindow(nil, titlePtr)
			if targetHwnd != 0 && myHwnd != 0 {
				var rect win.RECT
				if win.GetWindowRect(targetHwnd, &rect) {
					width := rect.Right - rect.Left
					height := rect.Bottom - rect.Top
					win.SetWindowPos(myHwnd, 0,
						rect.Left, rect.Top, width, height,
						win.SWP_NOZORDER|win.SWP_NOACTIVATE)
				}
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()
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
