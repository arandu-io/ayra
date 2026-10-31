package live

import (
	"image"
	"sync"
	"time"
)

// Frame is the picture a window draws.
//
// Image is the buffer itself and not a copy: it is the one the decoder wrote
// into, and it stays unchanged until the next call to [Player.Frame]. A window
// draws it during one layout, and the engine reads it again when the frame is
// rasterised, so it cannot be written to in between.
type Frame struct {
	// Image is the picture, or nil before the first one.
	Image *image.RGBA
	// Seq counts pictures: it differs from the Seq of the previous call exactly
	// when Image is a new picture, so a window knows when to upload a texture
	// and when to draw the one it has.
	Seq uint64
	// PTS is the presentation time of the picture from the stream's start.
	PTS time.Duration
	// Wall is when the node had the picture. An overlay from the same node is
	// aligned against this clock, not against PTS.
	Wall time.Time
}

// Size is the picture's size in pixels, zero before the first one.
func (f Frame) Size() image.Point {
	if f.Image == nil {
		return image.Point{}
	}
	return f.Image.Bounds().Size()
}

// frames is three buffers: the one the window is drawing, the newest picture
// not yet drawn, and a spare for the decoder to write into.
//
// That is enough that a decoder never waits for a window and a window never
// sees a buffer change under it. A decoder that is faster than the window loses
// the older of two undrawn pictures, which is what a live view should do: the
// latest picture is the only one worth drawing.
type frames struct {
	mu sync.Mutex

	buffers [3]*image.RGBA
	pinned  int // the buffer the window holds, or -1
	latest  int // the newest undrawn picture, or -1
	writing int // the buffer a decoder holds, or -1

	seq     uint64
	meta    [3]Frame // Seq, PTS and Wall of the picture in each buffer
	drawn   uint64
	walls   map[time.Duration]time.Time
	wallFIF []time.Duration
}

const wallMemory = 64

func newFrames() *frames {
	return &frames{pinned: -1, latest: -1, writing: -1, walls: map[time.Duration]time.Time{}}
}

// remember records when the node had the sample with this presentation time, so
// that the frame decoded from it can say.
func (f *frames) remember(pts time.Duration, wall time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, seen := f.walls[pts]; !seen {
		f.wallFIF = append(f.wallFIF, pts)
		if len(f.wallFIF) > wallMemory {
			delete(f.walls, f.wallFIF[0])
			f.wallFIF = f.wallFIF[1:]
		}
	}
	f.walls[pts] = wall
}

// Acquire implements [FrameSink].
func (f *frames) Acquire(width, height int) *image.RGBA {
	if width <= 0 || height <= 0 {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writing >= 0 {
		return f.fit(f.writing, width, height)
	}
	for i := range f.buffers {
		if i != f.pinned && i != f.latest {
			f.writing = i
			return f.fit(i, width, height)
		}
	}
	return nil
}

// fit makes buffer i the size asked for.
func (f *frames) fit(i, width, height int) *image.RGBA {
	if b := f.buffers[i]; b != nil && b.Rect.Dx() == width && b.Rect.Dy() == height {
		return b
	}
	f.buffers[i] = image.NewRGBA(image.Rect(0, 0, width, height))
	return f.buffers[i]
}

// Present implements [FrameSink].
func (f *frames) Present(img *image.RGBA, pts time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writing < 0 || f.buffers[f.writing] != img {
		// Not the buffer that was handed out: a decoder presenting something it
		// did not acquire is dropped rather than shown.
		return
	}
	f.seq++
	f.meta[f.writing] = Frame{Seq: f.seq, PTS: pts, Wall: f.walls[pts]}
	f.latest = f.writing
	f.writing = -1
}

// next answers the newest picture, pinning it, and releases the one that was
// pinned before.
func (f *frames) next() Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.latest >= 0 {
		f.pinned = f.latest
		f.latest = -1
	}
	if f.pinned < 0 {
		return Frame{}
	}
	m := f.meta[f.pinned]
	m.Image = f.buffers[f.pinned]
	return m
}

// reset forgets the undrawn picture, for a stream that starts over. A buffer a
// decoder still holds is left to it: its picture is presented or dropped on its
// own terms.
func (f *frames) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.latest = -1
}
