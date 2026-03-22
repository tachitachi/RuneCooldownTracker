package capture

import (
	"syscall"
	"unsafe"

	"github.com/TKMAX777/winapi/dx11"
	"github.com/go-ole/go-ole"
)

// D3D11 usage / access constants
const (
	D3D11_USAGE_STAGING   uint32 = 3
	D3D11_CPU_ACCESS_READ uint32 = 0x20000
	D3D11_MAP_READ        uint32 = 1
)

// DXGI_SAMPLE_DESC mirrors the C struct of the same name.
type DXGI_SAMPLE_DESC struct {
	Count   uint32
	Quality uint32
}

// D3D11_TEXTURE2D_DESC mirrors the C struct of the same name.
type D3D11_TEXTURE2D_DESC struct {
	Width          uint32
	Height         uint32
	MipLevels      uint32
	ArraySize      uint32
	Format         uint32 // DXGI_FORMAT
	SampleDesc     DXGI_SAMPLE_DESC
	Usage          uint32 // D3D11_USAGE
	BindFlags      uint32
	CPUAccessFlags uint32
	MiscFlags      uint32
}

// D3D11_MAPPED_SUBRESOURCE mirrors the C struct of the same name.
// PData is a raw pointer into GPU-accessible CPU memory after Map().
type D3D11_MAPPED_SUBRESOURCE struct {
	PData      unsafe.Pointer
	RowPitch   uint32
	DepthPitch uint32
}

// --------------------------------------------------------------------------
// ID3D11Texture2D
//
// VTable layout (IUnknown → ID3D11DeviceChild → ID3D11Resource → ID3D11Texture2D):
//   0  QueryInterface
//   1  AddRef
//   2  Release
//   3  GetDevice           (ID3D11DeviceChild)
//   4  GetPrivateData
//   5  SetPrivateData
//   6  SetPrivateDataInterface
//   7  GetType             (ID3D11Resource)
//   8  SetEvictionPriority
//   9  GetEvictionPriority
//  10  GetDesc             (ID3D11Texture2D)
// --------------------------------------------------------------------------

var ID3D11Texture2DGUID = ole.NewGUID("{6f15aaf2-d208-4e89-9ab4-489535d34f9c}")

type ID3D11Texture2D struct {
	ole.IUnknown
}

type ID3D11Texture2DVtbl struct {
	ole.IUnknownVtbl
	// ID3D11DeviceChild
	GetDevice               uintptr
	GetPrivateData          uintptr
	SetPrivateData          uintptr
	SetPrivateDataInterface uintptr
	// ID3D11Resource
	GetType             uintptr
	SetEvictionPriority uintptr
	GetEvictionPriority uintptr
	// ID3D11Texture2D
	GetDesc uintptr
}

func (v *ID3D11Texture2D) VTable() *ID3D11Texture2DVtbl {
	return (*ID3D11Texture2DVtbl)(unsafe.Pointer(v.RawVTable))
}

// GetDesc returns the texture description (dimensions, format, etc.).
func (v *ID3D11Texture2D) GetDesc() D3D11_TEXTURE2D_DESC {
	var desc D3D11_TEXTURE2D_DESC
	syscall.SyscallN(
		v.VTable().GetDesc,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&desc)),
	)
	return desc
}

// --------------------------------------------------------------------------
// IDirect3DDXGIInterfaceAccess
//
// Bridge from a WinRT IDirect3DSurface to the underlying DXGI/D3D11 object.
// GUID: {704C2307-2399-4A34-B3AF-61D027F4D677}
//
// VTable layout (IUnknown → IDirect3DDxgiInterfaceAccess):
//   0  QueryInterface
//   1  AddRef
//   2  Release
//   3  GetInterface
// --------------------------------------------------------------------------

type IDirect3DDXGIInterfaceAccess struct {
	ole.IUnknown
}

type IDirect3DDXGIInterfaceAccessVtbl struct {
	ole.IUnknownVtbl
	GetInterface uintptr
}

func (v *IDirect3DDXGIInterfaceAccess) VTable() *IDirect3DDXGIInterfaceAccessVtbl {
	return (*IDirect3DDXGIInterfaceAccessVtbl)(unsafe.Pointer(v.RawVTable))
}

// GetInterface queries the underlying DXGI/D3D11 object by GUID.
// ppvObject must point to a pointer of the desired interface type.
func (v *IDirect3DDXGIInterfaceAccess) GetInterface(riid *ole.GUID, ppvObject unsafe.Pointer) error {
	r1, _, _ := syscall.SyscallN(
		v.VTable().GetInterface,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(riid)),
		uintptr(ppvObject),
	)
	if r1 != 0 {
		return ole.NewError(r1)
	}
	return nil
}

// --------------------------------------------------------------------------
// Helpers that wrap the unwrapped VTable methods on dx11 package types
// --------------------------------------------------------------------------

// D3D11CreateTexture2D calls ID3D11Device::CreateTexture2D.
// pInitialData is NULL (staging textures have no initial data).
func D3D11CreateTexture2D(device *dx11.ID3D11Device, desc *D3D11_TEXTURE2D_DESC) (*ID3D11Texture2D, error) {
	var tex *ID3D11Texture2D
	r1, _, _ := syscall.SyscallN(
		device.VTable().CreateTexture2D,
		uintptr(unsafe.Pointer(device)),
		uintptr(unsafe.Pointer(desc)),
		0, // pInitialData = NULL
		uintptr(unsafe.Pointer(&tex)),
	)
	if r1 != 0 {
		return nil, ole.NewError(r1)
	}
	return tex, nil
}

// D3D11_BOX defines a rectangular sub-region of a texture for CopySubresourceRegion.
// For 2D textures: Front=0, Back=1; Left/Right are X bounds, Top/Bottom are Y bounds.
type D3D11_BOX struct {
	Left   uint32
	Top    uint32
	Front  uint32
	Right  uint32
	Bottom uint32
	Back   uint32
}

// D3D11CopyResource calls ID3D11DeviceContext::CopyResource (returns void).
func D3D11CopyResource(ctx *dx11.ID3D11DeviceContext, dst, src *ID3D11Texture2D) {
	syscall.SyscallN(
		ctx.VTable().CopyResource,
		uintptr(unsafe.Pointer(ctx)),
		uintptr(unsafe.Pointer(dst)),
		uintptr(unsafe.Pointer(src)),
	)
}

// D3D11CopySubresourceRegion calls ID3D11DeviceContext::CopySubresourceRegion.
// Copies srcBox from src into dst at (dstX, dstY, dstZ). For 2D textures dstZ=0.
func D3D11CopySubresourceRegion(ctx *dx11.ID3D11DeviceContext, dst *ID3D11Texture2D, dstSubresource, dstX, dstY, dstZ uint32, src *ID3D11Texture2D, srcSubresource uint32, srcBox *D3D11_BOX) {
	syscall.SyscallN(
		ctx.VTable().CopySubresourceRegion,
		uintptr(unsafe.Pointer(ctx)),
		uintptr(unsafe.Pointer(dst)),
		uintptr(dstSubresource),
		uintptr(dstX),
		uintptr(dstY),
		uintptr(dstZ),
		uintptr(unsafe.Pointer(src)),
		uintptr(srcSubresource),
		uintptr(unsafe.Pointer(srcBox)),
	)
}

// D3D11Map calls ID3D11DeviceContext::Map.
func D3D11Map(ctx *dx11.ID3D11DeviceContext, resource *ID3D11Texture2D, subresource, mapType, mapFlags uint32) (D3D11_MAPPED_SUBRESOURCE, error) {
	var mapped D3D11_MAPPED_SUBRESOURCE
	r1, _, _ := syscall.SyscallN(
		ctx.VTable().Map,
		uintptr(unsafe.Pointer(ctx)),
		uintptr(unsafe.Pointer(resource)),
		uintptr(subresource),
		uintptr(mapType),
		uintptr(mapFlags),
		uintptr(unsafe.Pointer(&mapped)),
	)
	if r1 != 0 {
		return D3D11_MAPPED_SUBRESOURCE{}, ole.NewError(r1)
	}
	return mapped, nil
}

// D3D11Unmap calls ID3D11DeviceContext::Unmap (returns void).
func D3D11Unmap(ctx *dx11.ID3D11DeviceContext, resource *ID3D11Texture2D, subresource uint32) {
	syscall.SyscallN(
		ctx.VTable().Unmap,
		uintptr(unsafe.Pointer(ctx)),
		uintptr(unsafe.Pointer(resource)),
		uintptr(subresource),
	)
}
