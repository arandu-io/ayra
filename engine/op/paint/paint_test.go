package paint_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/gpu/headless"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
)

// ground is what every picture here is painted over before it is painted on.
//
// A surface starts transparent, and on a transparent surface "nothing was
// painted" and "something was painted and came out invisible" are the same
// picture. Against a ground they are two, which is the whole reason a test
// about not painting can be written at all.
var ground = color.NRGBA{R: 0xff, A: 0xff}

// opaque is what a colour comes back as once it has been through the
// rasteriser.
//
// Every colour in this file is fully opaque, and for those the premultiplied
// form is the colour itself -- so the conversion is written out here rather
// than imported, and a test that used a translucent colour would be forced to
// say what it expects instead of trusting this.
func opaque(c color.NRGBA) color.RGBA {
	if c.A != 0xff {
		panic("opaque is only correct for a fully opaque colour")
	}
	return color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// picture paints the ground, runs draw, and reads back what came out.
//
// It rasterises rather than reading the operation list, because the list is
// the one thing these tests must not be allowed to agree with. A colour
// written to the wrong offset, a length that counts one byte short, a brush
// that never reaches the shader -- each of them produces a list a decoder in
// this repository would read back exactly as it was meant, and a picture that
// is wrong. Only a pixel can tell them apart.
//
// The test is skipped where there is no drawing surface, which is what a build
// machine with no GPU is. Skipping is honest and failing is not: a machine
// that cannot draw has proved nothing about the package either way.
func picture(t *testing.T, size image.Point, draw func(o *op.Ops)) *image.RGBA {
	t.Helper()

	window, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no drawing surface: %v", err)
	}
	defer window.Release()

	var ops op.Ops
	paint.FillShape(&ops, ground, clip.Rect(image.Rectangle{Max: size}).Op())
	draw(&ops)

	if err := window.Frame(&ops); err != nil {
		t.Fatalf("the drawing was refused: %v", err)
	}

	got := image.NewRGBA(image.Rectangle{Max: size})
	if err := window.Screenshot(got); err != nil {
		t.Fatalf("the picture could not be read back: %v", err)
	}
	return got
}

// uniform answers the one colour covering area, and fails when area is not one
// colour.
//
// Every pixel is looked at rather than a few sampled, because the faults this
// file is about are the ones that leave most of an area right: a gradient that
// bands, a clip that is off by a pixel, an edge that was drawn with the
// previous brush.
func uniform(t *testing.T, got *image.RGBA, area image.Rectangle) color.RGBA {
	t.Helper()

	first := got.RGBAAt(area.Min.X, area.Min.Y)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if at := got.RGBAAt(x, y); at != first {
				t.Fatalf("(%d,%d) is %v but (%d,%d) is %v: the area is not one colour",
					area.Min.X, area.Min.Y, first, x, y, at)
			}
		}
	}
	return first
}

// TestAColourWithNoAlphaPaintsNothing fixes that a transparent brush is a
// brush that does not mark the picture.
//
// It is what lets a caller hold a colour it may not want to draw -- a variant
// with no fill, a row that is not the chosen one -- and draw it unconditionally
// instead of asking first. Every such caller is a place where a transparent
// colour that painted anything at all would put a block of the wrong shade on
// screen, and it would only be visible on the one ground nobody developed
// against.
func TestAColourWithNoAlphaPaintsNothing(t *testing.T) {
	size := image.Pt(64, 64)
	area := image.Rect(0, 0, 32, 32)
	invisible := color.NRGBA{B: 0xff}

	got := picture(t, size, func(o *op.Ops) {
		paint.FillShape(o, invisible, clip.Rect(area).Op())
	})
	if at := uniform(t, got, area); at != opaque(ground) {
		t.Errorf("a colour with no alpha painted %v, want the ground %v", at, opaque(ground))
	}

	// The control, and it is not ceremony: without it this test is passed by a
	// package that paints nothing under any circumstances, which is the
	// failure it is least able to notice.
	solid := invisible
	solid.A = 0xff

	got = picture(t, size, func(o *op.Ops) {
		paint.FillShape(o, solid, clip.Rect(area).Op())
	})
	if at := uniform(t, got, area); at != opaque(solid) {
		t.Errorf("the same colour, opaque, painted %v, want %v", at, opaque(solid))
	}
}

// TestAGradientBetweenOneStopAndItselfIsOneColour fixes what a gradient with no
// length draws.
//
// Two stops in the same place describe no direction and no distance, and the
// position of a pixel along that gradient is a division by nothing. A caller
// arrives here by arithmetic rather than by intent -- a gradient across a row
// that came out empty, an animation caught at the frame where its two ends
// meet -- so the answer has to be a colour and not whatever an infinity does
// after it reaches the hardware.
//
// The colour is the end one. Every point of a plane is at or past the end of a
// gradient that ends where it starts.
func TestAGradientBetweenOneStopAndItselfIsOneColour(t *testing.T) {
	area := image.Rect(0, 0, 32, 32)
	stop := f32.Pt(10, 10)
	end := color.NRGBA{G: 0xff, A: 0xff}

	got := picture(t, image.Pt(64, 64), func(o *op.Ops) {
		defer clip.Rect(area).Op().Push(o).Pop()
		paint.LinearGradientOp{
			Stop1:  stop,
			Color1: color.NRGBA{B: 0xff, A: 0xff},
			Stop2:  stop,
			Color2: end,
		}.Add(o)
		paint.PaintOp{}.Add(o)
	})

	if at := uniform(t, got, area); at != opaque(end) {
		t.Errorf("a gradient with no length painted %v, want the end colour %v", at, opaque(end))
	}
}

// TestAnImageWithNoPixelsIsNotAnImage fixes that an empty picture is carried
// rather than refused.
//
// It arrives from every direction at once: a decode that produced nothing, a
// thumbnail not fetched yet, a cropped region that came out empty, and the zero
// value of the type -- which is what a struct field holds before anybody sets
// it. A panic on any of those is a panic inside a frame, and a frame that
// panics takes the window with it.
//
// This half needs no drawing surface, so it runs on every machine.
func TestAnImageWithNoPixelsIsNotAnImage(t *testing.T) {
	for _, test := range []struct {
		name  string
		brush paint.ImageOp
	}{
		{"the zero value", paint.ImageOp{}},
		{"empty, already in the display format", paint.NewImageOp(image.NewRGBA(image.Rectangle{}))},
		{"empty, and converted on the way in", paint.NewImageOp(image.NewGray(image.Rectangle{}))},
	} {
		t.Run(test.name, func(t *testing.T) {
			if at := test.brush.Size(); at != (image.Point{}) {
				t.Errorf("an image with no pixels reports the size %v, want %v", at, image.Point{})
			}
			// Adding it is the part that has to survive. It is written without
			// a recover, because a panic here should name this line.
			var ops op.Ops
			test.brush.Add(&ops)
		})
	}
}

// TestAnImageWithNoPixelsPaintsNothing fixes the other half: an empty picture
// is not merely survivable, it changes nothing.
//
// The brush set before it has to still be the brush after it. An empty image
// that reached the rasteriser as a texture of no size is the fault this
// catches, and it does not look like a crash -- it looks like the area coming
// out transparent, or the frame being refused, on the one platform whose
// driver minds.
func TestAnImageWithNoPixelsPaintsNothing(t *testing.T) {
	area := image.Rect(0, 0, 32, 32)
	mark := color.NRGBA{B: 0xff, A: 0xff}
	empty := paint.NewImageOp(image.NewRGBA(image.Rectangle{}))

	got := picture(t, image.Pt(64, 64), func(o *op.Ops) {
		defer clip.Rect(area).Op().Push(o).Pop()
		paint.ColorOp{Color: mark}.Add(o)
		empty.Add(o)
		paint.PaintOp{}.Add(o)
	})

	if at := uniform(t, got, area); at != opaque(mark) {
		t.Errorf("an empty image left the area %v, want the brush that was already set, %v", at, opaque(mark))
	}
}

// TestPaintingStopsAtTheClip fixes the boundary the whole package is drawn
// against.
//
// Fill paints a plane with no edges of its own, so the clip is the only thing
// that gives it any. Nothing else in this repository decides where a colour
// ends: a button's shape, a row's background, the ground under a screen are all
// this operation with a different area pushed in front of it.
func TestPaintingStopsAtTheClip(t *testing.T) {
	size := image.Pt(64, 64)
	inside := image.Rect(8, 8, 40, 40)
	mark := color.NRGBA{B: 0xff, A: 0xff}

	got := picture(t, size, func(o *op.Ops) {
		defer clip.Rect(inside).Op().Push(o).Pop()
		paint.Fill(o, mark)
	})

	if at := uniform(t, got, inside); at != opaque(mark) {
		t.Errorf("inside the clip is %v, want the mark %v", at, opaque(mark))
	}

	// One pixel past each corner rather than a sample taken well outside: a
	// clip that is off by one is still wrong, and it is invisible to anything
	// that looks at the middle of the outside.
	for _, at := range []image.Point{
		{X: inside.Min.X - 1, Y: inside.Min.Y - 1},
		{X: inside.Max.X, Y: inside.Min.Y},
		{X: inside.Min.X, Y: inside.Max.Y},
		{X: inside.Max.X, Y: inside.Max.Y},
	} {
		if pixel := got.RGBAAt(at.X, at.Y); pixel != opaque(ground) {
			t.Errorf("(%d,%d) is outside the clip and is %v, want the ground %v",
				at.X, at.Y, pixel, opaque(ground))
		}
	}
}

// TestAnOpacityLayerBlendsWhatItHolds fixes the one operation here that is not
// a brush.
//
// It is a layer and not a colour: what it holds is drawn to an image of its
// own and then blended in, so an opacity of a half over two overlapping marks
// is one translucent picture rather than two translucent marks, one showing
// through the other. The ends are what can be asserted exactly -- nothing at
// nought, everything at one -- and the middle is asserted as being neither,
// because the blend is the hardware's arithmetic and not this package's.
func TestAnOpacityLayerBlendsWhatItHolds(t *testing.T) {
	size := image.Pt(64, 64)
	area := image.Rect(0, 0, 32, 32)
	mark := color.NRGBA{B: 0xff, A: 0xff}

	at := func(opacity float32) color.RGBA {
		got := picture(t, size, func(o *op.Ops) {
			defer paint.PushOpacity(o, opacity).Pop()
			paint.FillShape(o, mark, clip.Rect(area).Op())
		})
		return uniform(t, got, area)
	}

	if none := at(0); none != opaque(ground) {
		t.Errorf("an opacity of nought painted %v, want the ground %v", none, opaque(ground))
	}
	if full := at(1); full != opaque(mark) {
		t.Errorf("an opacity of one painted %v, want the mark %v", full, opaque(mark))
	}

	half := at(0.5)
	if half == opaque(ground) || half == opaque(mark) {
		t.Errorf("an opacity of a half painted %v, want a blend of the ground %v and the mark %v",
			half, opaque(ground), opaque(mark))
	}

	// Out of range is clamped rather than refused, so that a value computed by
	// an animation -- which overshoots by design -- draws the frame it is
	// closest to instead of drawing nothing.
	if over, full := at(4), at(1); over != full {
		t.Errorf("an opacity above one painted %v, want the same as an opacity of one, %v", over, full)
	}
	if under, none := at(-1), at(0); under != none {
		t.Errorf("an opacity below nought painted %v, want the same as an opacity of nought, %v", under, none)
	}
}
