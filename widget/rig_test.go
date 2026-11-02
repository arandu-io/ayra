package widget_test

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/gpu/headless"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// rig draws a screen in a headless window through a real router, so a test
// can tap, type and paste and then look at the picture.
type rig struct {
	t       *testing.T
	win     *headless.Window
	router  input.Router
	ops     op.Ops
	shaper  *text.Shaper
	size    image.Point
	density float32
	scheme  theme.Scheme
	now     time.Time
}

func newRig(t *testing.T, w, h int, density float32, scheme theme.Scheme) *rig {
	t.Helper()
	px := image.Pt(int(float32(w)*density), int(float32(h)*density))
	win, err := headless.NewWindow(px.X, px.Y)
	if err != nil {
		t.Skipf("no drawing surface: %v", err)
	}
	t.Cleanup(win.Release)
	return &rig{t: t, win: win, shaper: text.NewShaper(text.WithCollection(gofont.Collection())), size: px, density: density, scheme: scheme, now: time.Unix(1_700_000_000, 0)}
}

func (r *rig) draw(screen func(ayra.Context) ayra.Dimensions) ayra.Dimensions {
	r.ops.Reset()
	r.now = r.now.Add(150 * time.Millisecond)
	gtx := layout.Context{
		Ops:         &r.ops,
		Source:      r.router.Source(),
		Metric:      unit.Metric{PxPerDp: r.density, PxPerSp: r.density},
		Now:         r.now,
		Constraints: layout.Exact(r.size),
	}
	palette := theme.New(r.scheme)
	c := ayra.Context{Context: gtx, Theme: palette, Shaper: r.shaper, Invalidate: func() {}}
	c.Fill(palette.Colours.Background)
	dims := screen(c)
	r.router.Frame(&r.ops)
	return dims
}

// settle draws a few frames, which is what a window does after an input.
func (r *rig) settle(screen func(ayra.Context) ayra.Dimensions) {
	for range 3 {
		r.draw(screen)
	}
}

func (r *rig) tap(screen func(ayra.Context) ayra.Dimensions, x, y float32) {
	at := f32.Pt(x*r.density, y*r.density)
	r.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 1, Position: at},
	)
	r.settle(screen)
}

func (r *rig) typed(screen func(ayra.Context) ayra.Dimensions, s string) {
	r.router.Queue(key.EditEvent{Range: key.Range{}, Text: s})
	r.settle(screen)
}

func (r *rig) shot(name string) {
	dir := os.Getenv("AYRA_AUDIT")
	if dir == "" {
		return
	}
	img := image.NewRGBA(image.Rectangle{Max: r.size})
	if err := r.win.Screenshot(img); err != nil {
		r.t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		r.t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		r.t.Fatal(err)
	}
}

// render is Frame of the last drawn ops to the window, then a shot.
func (r *rig) snap(name string, screen func(ayra.Context) ayra.Dimensions) {
	r.settle(screen)
	if err := r.win.Frame(&r.ops); err != nil {
		r.t.Fatal(err)
	}
	r.shot(name)
}
