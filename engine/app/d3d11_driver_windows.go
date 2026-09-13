//go:build !ayra_d3d11_warp

package app

import (
	"github.com/arandu-io/ayra/engine/internal/d3d11"
	"golang.org/x/sys/windows"
)

func createD3D11DeviceAndSwapChain(flags uint32, hwnd windows.Handle) (*d3d11.Device, *d3d11.DeviceContext, *d3d11.IDXGISwapChain, uint32, error) {
	return d3d11.CreateDeviceAndSwapChainWithFallback(flags, hwnd)
}
