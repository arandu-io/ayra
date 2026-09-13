//go:build ayra_d3d11_warp

package app

import (
	"github.com/arandu-io/ayra/engine/internal/d3d11"
	"golang.org/x/sys/windows"
)

// ayra_d3d11_warp is a runtime-proof build tag. The Windows CI catalogue uses
// it to exercise WARP plus swap-chain creation instead of merely accepting
// whichever driver happened to be first on the hosted runner.
func createD3D11DeviceAndSwapChain(flags uint32, hwnd windows.Handle) (*d3d11.Device, *d3d11.DeviceContext, *d3d11.IDXGISwapChain, uint32, error) {
	return d3d11.CreateDeviceAndSwapChainForWindow(d3d11.DRIVER_TYPE_WARP, flags, hwnd)
}
