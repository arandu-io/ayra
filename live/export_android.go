//go:build android

package live

import "C"

import (
	"runtime/cgo"
	"time"

	"github.com/arandu-io/ayra/internal/jni"
)

// onFrame is the native method AyraDecoder's drain thread calls with a decoded
// picture. The names are the ones the virtual machine looks the function up by.
//
//export Java_io_arandu_ayra_live_AyraDecoder_onFrame
func Java_io_arandu_ayra_live_AyraDecoder_onFrame(env, class uintptr, handle int64, y, u, v uintptr, yStride, uvStride, uvPixel, cropX, cropY, width, height int32, pts int64) {
	d, ok := cgo.Handle(handle).Value().(*mcDecoder)
	if !ok {
		return
	}
	e := jni.EnvOf(env)
	d.frame(yuvPlanes{
		y: e.Buffer(y), u: e.Buffer(u), v: e.Buffer(v),
		yStride: int(yStride), uvStride: int(uvStride), uvPixelStride: int(uvPixel),
		cropX: int(cropX), cropY: int(cropY), width: int(width), height: int(height),
	}, time.Duration(pts)*time.Microsecond)
}
