package capture

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/TKMAX777/winapi/dx11"
	"github.com/TKMAX777/winapi/winrt"
	"github.com/go-ole/go-ole"
	"github.com/lxn/win"
	"github.com/pkg/errors"
)

type FrameProcessor func(img *image.RGBA)

// CropRegion defines a rectangular sub-region in physical pixels.
// A nil pointer means no crop — the full frame is used.
type CropRegion struct{ X, Y, W, H int }

type CaptureHandler struct {
	device                 *winrt.IDirect3DDevice
	deviceDx               *dx11.ID3D11Device
	graphicsCaptureItem    *winrt.IGraphicsCaptureItem
	framePool              *winrt.IDirect3D11CaptureFramePool
	graphicsCaptureSession *winrt.IGraphicsCaptureSession
	framePoolToken         *winrt.EventRegistrationToken
	isRunning              bool
	savedFirstFrame        bool
	OnFrame                FrameProcessor
	cropMu                 sync.Mutex
	cropRegion             *CropRegion
}

func (c *CaptureHandler) SetCropRegion(r *CropRegion) {
	c.cropMu.Lock()
	c.cropRegion = r
	c.savedFirstFrame = false
	c.cropMu.Unlock()
}

func (c *CaptureHandler) StartCapture(hwnd win.HWND) error {
	type resultAttr struct {
		err error
	}

	var result = make(chan resultAttr)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// Initialize Windows Runtime
		err := winrt.RoInitialize(winrt.RO_INIT_MULTITHREADED)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "RoInitialize")}
			return
		}
		defer winrt.RoUninitialize()

		// Create capture device
		var featureLevels = []dx11.D3D_FEATURE_LEVEL{
			dx11.D3D_FEATURE_LEVEL_11_0,
			dx11.D3D_FEATURE_LEVEL_10_1,
			dx11.D3D_FEATURE_LEVEL_10_0,
			dx11.D3D_FEATURE_LEVEL_9_3,
			dx11.D3D_FEATURE_LEVEL_9_2,
			dx11.D3D_FEATURE_LEVEL_9_1,
		}

		err = dx11.D3D11CreateDevice(
			// nil, dx11.D3D_DRIVER_TYPE_HARDWARE, 0, dx11.D3D11_CREATE_DEVICE_BGRA_SUPPORT|dx11.D3D11_CREATE_DEVICE_DEBUG,
			nil, dx11.D3D_DRIVER_TYPE_HARDWARE, 0, dx11.D3D11_CREATE_DEVICE_BGRA_SUPPORT,
			&featureLevels[0], len(featureLevels),
			dx11.D3D11_SDK_VERSION, &c.deviceDx, nil, nil,
		)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "D3DCreateDevice")}
			return
		}
		defer c.deviceDx.Release()

		// Query interface of DXGIDevice
		var dxgiDevice *dx11.IDXGIDevice
		err = c.deviceDx.PutQueryInterface(dx11.IDXGIDeviceID, &dxgiDevice)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "PutQueryInterface")}
			return
		}

		var deviceRT *ole.IInspectable

		// convert D3D11Device(Dx11) to Direct3DDevice(WinRT)
		err = dx11.CreateDirect3D11DeviceFromDXGIDevice(dxgiDevice, &deviceRT)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "CreateDirect3D11DeviceFromDXGIDevice")}
			return
		}
		defer deviceRT.Release()

		// Query interface of IDirect3DDevice
		err = deviceRT.PutQueryInterface(winrt.IDirect3DDeviceID, &c.device)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "QueryInterface: IDirect3DDeviceID")}
			return
		}
		defer c.device.Release()

		// Create Capture Settings
		factory, err := ole.RoGetActivationFactory(winrt.GraphicsCaptureItemClass, winrt.IGraphicsCaptureItemInteropID)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "RoGetActivationFactory: IGraphicsCaptureItemID")}
			return
		}
		defer factory.Release()

		var interop *winrt.IGraphicsCaptureItemInterop
		err = factory.PutQueryInterface(winrt.IGraphicsCaptureItemInteropID, &interop)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "QueryInterface: IGraphicsCaptureItemInteropID")}
			return
		}
		defer interop.Release()

		var captureItemDispatch *ole.IInspectable
		// Capture for the window specified
		err = interop.CreateForWindow(hwnd, winrt.IGraphicsCaptureItemID, &captureItemDispatch)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "CreateForWindow")}
			return
		}
		defer captureItemDispatch.Release()

		// Capture for the monitor specified
		// var hmoni = win.MonitorFromWindow(hwnd, win.MONITORINFOF_PRIMARY)

		// err = interop.CreateForMonitor(hmoni, winrt.IGraphicsCaptureItemID, &captureItemDispatch)
		// if err != nil {
		// 	result <- resultAttr{errors.Wrap(err, "CreateForMonitor")}
		// 	return
		// }
		// defer captureItemDispatch.Release()

		// Get Interface of IGraphicsCaptureItem
		err = captureItemDispatch.PutQueryInterface(winrt.IGraphicsCaptureItemID, &c.graphicsCaptureItem)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "PutQueryInterface captureItemDispatch")}
			return
		}

		// Get Capture objects size
		size, err := c.graphicsCaptureItem.Size()
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "Size")}
			return
		}

		// Get object of Direct3D11CaptureFramePoolClass
		ins, err := ole.RoGetActivationFactory(winrt.Direct3D11CaptureFramePoolClass, winrt.IDirect3D11CaptureFramePoolStaticsID)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "RoGetActivationFactory: IDirect3D11CaptureFramePoolStatics Class Instance")}
			return
		}
		defer ins.Release()

		// Get Interface of Direct3D11CaptureFramePoolClass
		var framePoolStatic *winrt.IDirect3D11CaptureFramePoolStatics2
		err = ins.PutQueryInterface(winrt.IDirect3D11CaptureFramePoolStatics2ID, &framePoolStatic)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "PutQueryInterface: IDirect3D11CaptureFramePoolStaticsID")}
			return
		}
		defer framePoolStatic.Release()

		// Create frame pool.
		// The winrt library's CreateFreeThreaded packs SizeInt32 into a single
		// register with Width<<32|Height, but x64 little-endian expects Width in
		// the low DWORD. Pre-swap so the bug cancels out and the right dimensions
		// reach the OS.
		swappedSize := &winrt.SizeInt32{Width: size.Height, Height: size.Width}
		c.framePool, err = framePoolStatic.CreateFreeThreaded(c.device, winrt.DirectXPixelFormat_B8G8R8A8UIntNormalized, 1, swappedSize)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "CreateFramePool")}
			return
		}

		// Set frame settings
		var eventObject = NewDirect3D11CaptureFramePool(c.onFrameArrived)

		c.framePoolToken, err = c.framePool.AddFrameArrived(unsafe.Pointer(eventObject))
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "AddFrameArrived")}
			return
		}
		defer eventObject.Release()

		c.graphicsCaptureSession, err = c.framePool.CreateCaptureSession(c.graphicsCaptureItem)
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "CreateCaptureSession")}
			return
		}
		defer c.graphicsCaptureSession.Release()

		// Start capturing
		err = c.graphicsCaptureSession.StartCapture()
		if err != nil {
			result <- resultAttr{errors.Wrap(err, "StartCapture")}
			return
		}

		c.isRunning = true

		result <- resultAttr{nil}

		for c.isRunning {
			var msg win.MSG
			if win.GetMessage(&msg, 0, 0, 0) == win.TRUE {
				win.TranslateMessage(&msg)
				win.DispatchMessage(&msg)
			}
			time.Sleep(10 * time.Millisecond) // Don't peg the CPU
			// time.Sleep(time.Second)
		}
	}()

	var res = <-result
	close(result)

	fmt.Println("Start Capturing")

	return res.err
}

// IDirect3DSurface is the WinRT wrapper for a D3D surface returned by a capture frame.
type IDirect3DSurface struct {
	ole.IInspectable
}

var (
	// IDirect3DSurfaceID is the IID for the WinRT IDirect3DSurface interface.
	IDirect3DSurfaceID = ole.NewGUID("{0BFAD9C1-D19E-45BB-A2AC-998E3FCB1175}")

	// IDirect3DDXGIInterfaceAccessID is the IID for IDirect3DDxgiInterfaceAccess,
	// the bridge that lets you unwrap a WinRT surface to its underlying D3D11 object.
	// Source: windows.graphics.directx.direct3d11.interop.h
	IDirect3DDXGIInterfaceAccessID = ole.NewGUID("{A9B3D012-3DF2-4EE3-B8D1-8695F457D3C1}")
)

func Surface(v *winrt.IDirect3D11CaptureFrame) (*IDirect3DSurface, error) {
	var surface *IDirect3DSurface
	// We call the 'Surface' function pointer from the VTable (index 0 after IInspectable)
	r1, _, _ := syscall.SyscallN(
		v.VTable().Surface,                // The function address
		uintptr(unsafe.Pointer(v)),        // 'this' pointer
		uintptr(unsafe.Pointer(&surface)), // where to store the result
	)

	if r1 != 0 { // win.S_OK is 0
		return nil, ole.NewError(r1)
	}
	return surface, nil
}

func (c *CaptureHandler) onFrameArrived(this_ *uintptr, sender *winrt.IDirect3D11CaptureFramePool, args *ole.IInspectable) uintptr {
	_ = (*Direct3D11CaptureFramePool)(unsafe.Pointer(this_))

	frame, err := sender.TryGetNextFrame()
	if err != nil {
		fmt.Fprintln(os.Stderr, "onFrameArrived: TryGetNextFrame:", err)
		return 0
	}
	defer func() {
		var closable *winrt.IClosable
		if e := frame.PutQueryInterface(winrt.IClosableID, &closable); e == nil {
			closable.Close()
			closable.Release()
		}
	}()

	// Skip all GPU work until the user has defined a capture region.
	c.cropMu.Lock()
	crop := c.cropRegion
	c.cropMu.Unlock()
	if crop == nil {
		return 0
	}

	// -----------------------------------------------------------------------
	// Step 1 – WinRT surface → IDirect3DDXGIInterfaceAccess → ID3D11Texture2D
	// -----------------------------------------------------------------------

	surface, err := Surface(frame)
	if err != nil {
		fmt.Fprintln(os.Stderr, "onFrameArrived: Surface:", err)
		return 0
	}
	defer surface.Release()

	// QI the WinRT surface for the DXGI bridge interface.
	var dxgiAccess *IDirect3DDXGIInterfaceAccess
	if err = surface.PutQueryInterface(IDirect3DDXGIInterfaceAccessID, &dxgiAccess); err != nil {
		fmt.Fprintln(os.Stderr, "onFrameArrived: QI IDirect3DDXGIInterfaceAccess:", err)
		return 0
	}
	defer dxgiAccess.Release()

	// Use the bridge to get the underlying ID3D11Texture2D.
	var gpuTexture *ID3D11Texture2D
	if err = dxgiAccess.GetInterface(ID3D11Texture2DGUID, unsafe.Pointer(&gpuTexture)); err != nil {
		fmt.Fprintln(os.Stderr, "onFrameArrived: GetInterface ID3D11Texture2D:", err)
		return 0
	}
	defer gpuTexture.Release()

	// -----------------------------------------------------------------------
	// Step 2 – Clamp crop to the actual frame bounds
	// -----------------------------------------------------------------------

	desc := gpuTexture.GetDesc()
	frameW, frameH := int(desc.Width), int(desc.Height)

	x0 := max(0, crop.X)
	y0 := max(0, crop.Y)
	x1 := min(frameW, crop.X+crop.W)
	y1 := min(frameH, crop.Y+crop.H)
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	cropW, cropH := x1-x0, y1-y0

	// -----------------------------------------------------------------------
	// Step 3 – Create a CPU-readable staging texture sized to the crop region
	// -----------------------------------------------------------------------

	stagingDesc := desc
	stagingDesc.Width = uint32(cropW)
	stagingDesc.Height = uint32(cropH)
	stagingDesc.Usage = D3D11_USAGE_STAGING
	stagingDesc.CPUAccessFlags = D3D11_CPU_ACCESS_READ
	stagingDesc.BindFlags = 0
	stagingDesc.MiscFlags = 0
	stagingDesc.MipLevels = 1

	stagingTex, err := D3D11CreateTexture2D(c.deviceDx, &stagingDesc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "onFrameArrived: CreateTexture2D (staging):", err)
		return 0
	}
	defer stagingTex.Release()

	// -----------------------------------------------------------------------
	// Step 4 – GPU-side copy: only the crop region → staging texture
	// -----------------------------------------------------------------------

	dctx := c.deviceDx.GetImmediateContext()
	defer dctx.Release()

	srcBox := D3D11_BOX{
		Left: uint32(x0), Right: uint32(x1),
		Top: uint32(y0), Bottom: uint32(y1),
		Front: 0, Back: 1,
	}
	D3D11CopySubresourceRegion(dctx, stagingTex, 0, 0, 0, 0, gpuTexture, 0, &srcBox)

	// -----------------------------------------------------------------------
	// Step 5 – Map the (small) staging texture and convert BGRA → RGBA
	// -----------------------------------------------------------------------

	mapped, err := D3D11Map(dctx, stagingTex, 0, D3D11_MAP_READ, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "onFrameArrived: Map:", err)
		return 0
	}
	defer D3D11Unmap(dctx, stagingTex, 0)

	rowPitch := int(mapped.RowPitch)
	totalBytes := rowPitch * cropH
	src := unsafe.Slice((*byte)(mapped.PData), totalBytes)
	img := image.NewRGBA(image.Rect(0, 0, cropW, cropH))

	for y := 0; y < cropH; y++ {
		srcRow := y * rowPitch
		dstRow := y * img.Stride
		for x := 0; x < cropW; x++ {
			s := srcRow + x*4
			d := dstRow + x*4
			img.Pix[d+0] = src[s+2] // R ← B
			img.Pix[d+1] = src[s+1] // G ← G
			img.Pix[d+2] = src[s+0] // B ← R
			img.Pix[d+3] = src[s+3] // A ← A
		}
	}

	// -----------------------------------------------------------------------
	// Step 6 – Debug: save the first processed frame to disk
	// -----------------------------------------------------------------------

	if !c.savedFirstFrame {
		if f, ferr := os.Create("frame.png"); ferr != nil {
			fmt.Fprintln(os.Stderr, "onFrameArrived: os.Create:", ferr)
		} else {
			defer f.Close()
			if ferr = png.Encode(f, img); ferr != nil {
				fmt.Fprintln(os.Stderr, "onFrameArrived: png.Encode:", ferr)
			} else {
				fmt.Println("onFrameArrived: saved frame.png")
			}
		}
		c.savedFirstFrame = true
	}

	// -----------------------------------------------------------------------
	// Step 7 – Dispatch to OnFrame
	// -----------------------------------------------------------------------

	if c.OnFrame != nil {
		c.OnFrame(img)
	}

	return 0
}

func (c *CaptureHandler) Close() error {
	if !c.isRunning {
		return nil
	}

	if c.framePool != nil {
		err := c.framePool.RemoveFrameArrived(c.framePoolToken)
		if err != nil {
			return errors.Wrap(err, "RemoveFrameArrived")
		}

		var closable *winrt.IClosable
		err = c.framePool.PutQueryInterface(winrt.IClosableID, &closable)
		if err != nil {
			return errors.Wrap(err, "PutQueryInterface: graphicsCaptureSession")
		}
		defer closable.Release()

		closable.Close()

		c.framePool = nil
	}

	var closable *winrt.IClosable
	err := c.graphicsCaptureSession.PutQueryInterface(winrt.IClosableID, &closable)
	if err != nil {
		return errors.Wrap(err, "PutQueryInterface: graphicsCaptureSession")
	}
	defer closable.Release()

	closable.Close()

	c.graphicsCaptureItem = nil
	c.isRunning = false

	return nil
}
