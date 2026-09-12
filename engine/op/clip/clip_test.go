package clip_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/gpu/headless"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
)

// ink is the colour every test here paints with.
//
// Opaque, so that a pixel it reached and a pixel it did not are told apart by
// alpha alone: the surface starts transparent, and anything with alpha at all
// is something this drawing put there.
var ink = color.NRGBA{R: 255, A: 255}

// picture draws one frame off-screen and hands back the pixels.
//
// This is the only way to assert what a clip does rather than what it says.
// The instruction stream can be read without a GPU and says which area was
// claimed; whether that area is the one the ink reached is a question only a
// rasteriser answers, and a clip that encodes perfectly and lets the wrong
// pixels through is exactly the fault this package exists to not have.
func picture(t *testing.T, size int, draw func(o *op.Ops)) *image.RGBA {
	t.Helper()

	window, err := headless.NewWindow(size, size)
	if err != nil {
		t.Skipf("no drawing surface on this machine: %v", err)
	}
	defer window.Release()

	o := new(op.Ops)
	draw(o)
	if err := window.Frame(o); err != nil {
		t.Fatalf("the drawing was refused: %v", err)
	}

	pixels := image.NewRGBA(image.Rect(0, 0, size, size))
	if err := window.Screenshot(pixels); err != nil {
		t.Fatalf("the picture could not be read back: %v", err)
	}
	return pixels
}

// painted reports whether the ink reached a point.
func painted(pixels *image.RGBA, x, y int) bool {
	_, _, _, a := pixels.At(x, y).RGBA()
	return a > 0
}

// inked is the smallest rectangle holding everything the ink reached, and the
// empty rectangle when it reached nothing.
func inked(pixels *image.RGBA) image.Rectangle {
	var reached image.Rectangle
	for y := pixels.Rect.Min.Y; y < pixels.Rect.Max.Y; y++ {
		for x := pixels.Rect.Min.X; x < pixels.Rect.Max.X; x++ {
			if !painted(pixels, x, y) {
				continue
			}
			reached = reached.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return reached
}

// fillThrough paints everything the given areas let through, pushing them in
// order and releasing them in reverse.
func fillThrough(o *op.Ops, areas ...clip.Rect) {
	stack := make([]clip.Stack, len(areas))
	for i, area := range areas {
		stack[i] = area.Push(o)
	}
	paint.Fill(o, ink)
	for i := len(stack) - 1; i >= 0; i-- {
		stack[i].Pop()
	}
}

// TestAnEmptyClipLetsNothingThrough fixes the answer to a zero sized area.
//
// A zero rectangle is what a layout produces when it has nothing to show: a
// list with no rows, a panel collapsed to nothing, a column that was given no
// width. Every one of those paints through its own clip on the frame it turns
// empty, and an empty area that let the fill through would put the panel's
// background across the whole screen.
func TestAnEmptyClipLetsNothingThrough(t *testing.T) {
	pixels := picture(t, 100, func(o *op.Ops) {
		fillThrough(o, clip.Rect{})
	})

	if reached := inked(pixels); !reached.Empty() {
		t.Errorf("an empty clip let the ink reach %v, want nothing", reached)
	}
}

// TestABackwardsClipLetsNothingThrough fixes the answer to a rectangle turned
// inside out.
//
// It comes out of arithmetic rather than out of a call: an inset wider than
// the room it was applied to, a right edge computed from a width that went
// negative. Read as an area it has negative extent, and the only honest
// meaning of that is nothing -- the tempting alternative is to put the corners
// back in order, which quietly paints a rectangle the caller never asked for,
// somewhere else on the screen.
func TestABackwardsClipLetsNothingThrough(t *testing.T) {
	pixels := picture(t, 100, func(o *op.Ops) {
		fillThrough(o, clip.Rect{Min: image.Pt(60, 60), Max: image.Pt(20, 20)})
	})

	if reached := inked(pixels); !reached.Empty() {
		t.Errorf("a backwards clip let the ink reach %v, want nothing", reached)
	}
}

// TestClippingARectangleWithItselfIsTheRectangle is the identity that lets
// controls nest.
//
// A panel narrows the area, then the control inside it narrows the same area
// again to the same size, and so on down. If each step cost a pixel, a screen
// would lose a border for every layer it happens to be built from -- and how
// many layers that is, is not something any caller knows.
func TestClippingARectangleWithItselfIsTheRectangle(t *testing.T) {
	area := clip.Rect(image.Rect(10, 10, 80, 80))

	once := picture(t, 100, func(o *op.Ops) { fillThrough(o, area) })
	four := picture(t, 100, func(o *op.Ops) { fillThrough(o, area, area, area, area) })

	for i := range once.Pix {
		if once.Pix[i] != four.Pix[i] {
			t.Fatalf("clipping the rectangle with itself changed the picture: byte %d is %d, want %d", i, four.Pix[i], once.Pix[i])
		}
	}
}

// TestFourNestedClipsAreTheirIntersection is what nesting means, checked at
// the pixel rather than at the encoding.
//
// Each area narrows the one before it, and the result is the region inside all
// four. One area dropped on the way down is not a small error: it is the whole
// of whatever that area was hiding, drawn over its neighbours. Four levels
// rather than two because the third is where a stack that keeps only the
// current area, instead of intersecting with what it inherited, still looks
// right.
func TestFourNestedClipsAreTheirIntersection(t *testing.T) {
	areas := []image.Rectangle{
		image.Rect(10, 10, 80, 80),
		image.Rect(20, 5, 90, 70),
		image.Rect(5, 20, 70, 90),
		image.Rect(25, 25, 60, 60),
	}

	want := areas[0]
	for _, area := range areas[1:] {
		want = want.Intersect(area)
	}

	pixels := picture(t, 100, func(o *op.Ops) {
		fillThrough(o, clip.Rect(areas[0]), clip.Rect(areas[1]), clip.Rect(areas[2]), clip.Rect(areas[3]))
	})

	if reached := inked(pixels); reached != want {
		t.Fatalf("the ink reached %v, want the intersection %v", reached, want)
	}
	for y := pixels.Rect.Min.Y; y < pixels.Rect.Max.Y; y++ {
		for x := pixels.Rect.Min.X; x < pixels.Rect.Max.X; x++ {
			inside := image.Pt(x, y).In(want)
			if painted(pixels, x, y) != inside {
				t.Fatalf("the pixel at (%d, %d) is painted=%v, want %v", x, y, !inside, inside)
			}
		}
	}
}

// TestClipsThatDoNotOverlapLetNothingThrough is the intersection's other end.
//
// Two areas with nothing in common describe nothing, and that happens on real
// screens all the time: a row scrolled past the end of the list it is in, a
// tooltip positioned outside the panel that clips it. Anything at all painted
// there is painted somewhere nobody is looking for it.
func TestClipsThatDoNotOverlapLetNothingThrough(t *testing.T) {
	pixels := picture(t, 100, func(o *op.Ops) {
		fillThrough(o, clip.Rect(image.Rect(10, 10, 30, 30)), clip.Rect(image.Rect(60, 60, 90, 90)))
	})

	if reached := inked(pixels); !reached.Empty() {
		t.Errorf("two separate clips let the ink reach %v, want nothing", reached)
	}
}

// TestAnOutlineOfAClosedPathIsAccepted keeps a path that returns to its start
// from being mistaken for one that never left it.
//
// A contour is closed by drawing back to where it began, and a shape assembled
// out of arcs and lines ends up back there by arithmetic rather than by a
// caller saying so. Treating the coincidence as an error would refuse every
// shape that is actually closed, which is every shape worth filling.
func TestAnOutlineOfAClosedPathIsAccepted(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			t.Errorf("outlining a closed path panicked: %v", err)
		}
	}()

	var p clip.Path
	p.Begin(new(op.Ops))
	p.MoveTo(f32.Pt(300, 200))
	p.LineTo(f32.Pt(150, 200))
	p.MoveTo(f32.Pt(150, 200))
	p.ArcTo(f32.Pt(300, 200), f32.Pt(300, 200), 3*math.Pi/4)
	p.LineTo(f32.Pt(300, 200))
	p.Close()
	clip.Outline{Path: p.End()}.Op()
}

// TestAPathBegunWithoutAMoveReachesTheRenderer fixes the path that starts
// where the pen already is.
//
// A caller that draws from the origin has no move to make, so the first thing
// the path records is a segment. The list that produces is the one shape whose
// commands begin before anything has positioned the pen, and a renderer handed
// it has to draw rather than reject it -- a frame refused is a window that
// stops updating, not a shape that comes out wrong.
func TestAPathBegunWithoutAMoveReachesTheRenderer(t *testing.T) {
	picture(t, 100, func(o *op.Ops) {
		var p clip.Path
		p.Begin(o)
		p.LineTo(f32.Pt(10, 10))
		p.Close()
		area := clip.Outline{Path: p.End()}.Op().Push(o)
		paint.Fill(o, ink)
		area.Pop()
	})
}

// TestReleasingAnAreaAcrossARecordingPanics keeps a clip inside the recording
// it was made in.
//
// Operations recorded for later replay are a separate list, and an area
// claimed in one list cannot be released in another: the renderer would
// restore a state that was never saved where it is looking. The panic names
// the mistake at the call that made it; without it the frame draws, and what
// goes wrong is the clipping of everything afterwards.
func TestReleasingAnAreaAcrossARecordingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("releasing an area across a recording did not panic")
		}
	}()

	var o op.Ops
	area := clip.Op{}.Push(&o)
	op.Record(&o)
	area.Pop()
}
