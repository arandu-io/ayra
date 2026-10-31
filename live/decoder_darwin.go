//go:build darwin

package live

/*
#cgo LDFLAGS: -framework CoreFoundation -framework CoreMedia -framework CoreVideo -framework VideoToolbox

#include <stdlib.h>
#include "vt_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime/cgo"
	"sync"
	"time"
	"unsafe"
)

// The Apple decoder is VideoToolbox, which decodes in the hardware the phone or
// the Mac has. It is the same code for iOS and macOS: the framework is the same,
// and so a test run on a Mac exercises what runs on a phone.
//
// A picture goes in as the node sent it, length-prefixed NAL units, and comes
// out as a BGRA pixel buffer that is converted to RGBA into the buffer the
// window draws from -- the one copy between the decoder and the screen.

// vtDecoder is the VideoToolbox backend.
type vtDecoder struct {
	sink   FrameSink
	handle cgo.Handle

	mu       sync.Mutex
	c        *C.vt_decoder
	lengthSz int
}

func newDecoder(sink FrameSink) (Decoder, error) {
	d := &vtDecoder{sink: sink}
	d.handle = cgo.NewHandle(d)
	d.c = C.vt_new(C.uintptr_t(d.handle))
	return d, nil
}

// Configure implements [Decoder].
func (d *vtDecoder) Configure(t Track) error {
	var family C.int
	switch t.Family() {
	case CodecH264:
		family = 1
	case CodecHEVC:
		family = 2
	default:
		return fmt.Errorf("%w: %q", ErrUnsupported, t.Codec)
	}
	sets, err := t.ParameterSets()
	if err != nil {
		return err
	}
	length, err := t.NALLengthSize()
	if err != nil {
		return err
	}

	// The sets are copied into C memory: a Go pointer to Go pointers cannot be
	// passed to C, and the sets are small.
	pointers := C.malloc(C.size_t(len(sets)) * C.size_t(unsafe.Sizeof(uintptr(0))))
	sizes := C.malloc(C.size_t(len(sets)) * C.size_t(unsafe.Sizeof(C.size_t(0))))
	defer C.free(pointers)
	defer C.free(sizes)
	ptrs := unsafe.Slice((**C.uint8_t)(pointers), len(sets))
	lens := unsafe.Slice((*C.size_t)(sizes), len(sets))
	for i, set := range sets {
		ptrs[i] = (*C.uint8_t)(C.CBytes(set))
		lens[i] = C.size_t(len(set))
		defer C.free(unsafe.Pointer(ptrs[i]))
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.c == nil {
		return errors.New("live: the decoder is closed")
	}
	d.lengthSz = length
	status := C.vt_configure(d.c, family, (**C.uint8_t)(pointers), (*C.size_t)(sizes), C.int(len(sets)), C.int(length))
	if status != 0 {
		return fmt.Errorf("live: VideoToolbox could not start for %s: status %d", t.Codec, int(status))
	}
	return nil
}

// Decode implements [Decoder].
func (d *vtDecoder) Decode(s Sample) error {
	if len(s.Data) == 0 {
		return nil
	}
	d.mu.Lock()
	c := d.c
	d.mu.Unlock()
	if c == nil {
		return errors.New("live: the decoder is closed")
	}
	status := C.vt_decode(c, (*C.uint8_t)(unsafe.Pointer(&s.Data[0])), C.size_t(len(s.Data)), C.int64_t(s.PTS/time.Microsecond))
	switch status {
	case 0:
		return nil
	case -1:
		return ErrBusy
	case -2:
		return errors.New("live: the decoder was not configured")
	}
	return fmt.Errorf("live: VideoToolbox rejected a picture: status %d", int(status))
}

// Close implements [Decoder].
func (d *vtDecoder) Close() error {
	d.mu.Lock()
	c := d.c
	d.c = nil
	d.mu.Unlock()
	if c != nil {
		// vt_close waits for the pictures still in the decoder, and their
		// callbacks need the handle, so it is released after.
		C.vt_close(c)
	}
	d.handle.Delete()
	return nil
}

// ayraVTFrame is called by the decoder, on its own thread, with a decoded
// picture. The pixel buffer is valid only until it returns, so the copy into the
// sink's buffer happens here.
//
//export ayraVTFrame
func ayraVTFrame(handle C.uintptr_t, image unsafe.Pointer, ptsMicros C.int64_t) {
	d, ok := cgo.Handle(handle).Value().(*vtDecoder)
	if !ok {
		return
	}
	var w, h C.int
	C.vt_size(image, &w, &h)
	img := d.sink.Acquire(int(w), int(h))
	if img == nil {
		return
	}
	if C.vt_copy(image, (*C.uint8_t)(unsafe.Pointer(&img.Pix[0])), C.size_t(img.Stride)) != 0 {
		return
	}
	d.sink.Present(img, time.Duration(ptsMicros)*time.Microsecond)
}
