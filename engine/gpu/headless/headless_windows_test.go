package headless

import (
	"testing"
	"unsafe"

	"github.com/arandu-io/ayra/engine/internal/d3d11"
)

// TestWARPHeadless proves that the software D3D11 driver does real rendering,
// rather than merely existing as the second name in a fallback list.
func TestWARPHeadless(t *testing.T) {
	primary, fallback := newContextPrimary, newContextFallback
	newContextPrimary = func() (context, error) {
		device, immediate, _, err := d3d11.CreateDevice(d3d11.DRIVER_TYPE_WARP, 0)
		if err != nil {
			return nil, err
		}
		d3d11.IUnknownRelease(unsafe.Pointer(immediate), immediate.Vtbl.Release)
		return &d3d11Context{dev: device}, nil
	}
	newContextFallback = nil
	t.Cleanup(func() {
		newContextPrimary, newContextFallback = primary, fallback
	})
	testHeadless(t)
}
