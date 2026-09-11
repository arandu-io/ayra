// Package frame draws a screen without a window and hands back the picture.
//
// It exists because a screen that compiles is not a screen that draws. Between
// the two are the parts nothing else exercises: the shaper finding a face, the
// GPU accepting the operations, a control computing a size that is not zero.
// A build says nothing about any of them, and neither does a test that calls a
// layout function and reads the dimensions it returns -- that path never
// reaches a rasteriser.
//
// What it produces is an image, which is the only artefact that can be looked
// at by a person and compared by a machine.
package frame

import (
	"fmt"
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/app"
	"github.com/arandu-io/ayra/engine/gpu/headless"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"

	"github.com/arandu-io/ayra/theme"
)

// Options are what a drawing is made under.
type Options struct {
	// Width and Height are the picture's size in points, before density.
	Width, Height int

	// Density is how many pixels there are per point: 1 for an ordinary
	// screen, 2 for a dense one. Both are drawn because a control can look
	// right at one and wrong at the other -- a hairline border rounds to zero
	// pixels at 1 and to two at 2.
	Density float32

	// Scheme is the palette to draw with.
	Scheme theme.Scheme

	// Fonts are the faces text is shaped from. Empty takes whatever the system
	// offers, which is nothing on a machine with no fonts installed -- so a
	// caller that needs the same picture everywhere carries its own.
	Fonts []text.FontFace
}

// withDefaults fills in what was not asked for.
func (o Options) withDefaults() Options {
	if o.Width <= 0 {
		o.Width = 360
	}
	if o.Height <= 0 {
		o.Height = 640
	}
	if o.Density <= 0 {
		o.Density = 1
	}
	return o
}

// Draw renders one screen and returns the picture.
//
// The screen is drawn twice and only the second picture is kept. The first is
// what a control returns before it has ever been laid out: a field does not
// know where its caret is, a button has not been told its own size, and
// anything positioned relative to a sibling has nothing to be relative to.
// Keeping that frame would fix a picture of an application's first instant
// rather than of an application.
func Draw(o Options, screen ayra.Widget) (*image.RGBA, error) {
	o = o.withDefaults()

	pixels := image.Pt(int(float32(o.Width)*o.Density), int(float32(o.Height)*o.Density))

	window, err := headless.NewWindow(pixels.X, pixels.Y)
	if err != nil {
		return nil, fmt.Errorf("ayra/frame: no drawing surface: %w", err)
	}
	defer window.Release()

	shaper := text.NewShaper(text.WithCollection(o.Fonts))
	palette := theme.New(o.Scheme)
	var source input.Source

	var ops op.Ops
	for pass := 0; pass < 2; pass++ {
		ops.Reset()

		gtx := app.NewContext(&ops, frameEvent(o, pixels, source))
		c := ayra.Context{
			Context:    gtx,
			Theme:      palette,
			Shaper:     shaper,
			Invalidate: func() {},
		}

		// The same ground the window paints. Without it the picture is drawn
		// over whatever the surface was cleared to, which is transparent --
		// and a dark screen photographed against transparency is a picture of
		// light text on white.
		c.Fill(palette.Colours.Background)

		screen(c)
		source = gtx.Source

		if err := window.Frame(&ops); err != nil {
			return nil, fmt.Errorf("ayra/frame: the drawing was refused: %w", err)
		}
	}

	picture := image.NewRGBA(image.Rectangle{Max: pixels})
	if err := window.Screenshot(picture); err != nil {
		return nil, fmt.Errorf("ayra/frame: the picture could not be read back: %w", err)
	}
	return picture, nil
}

// frameEvent builds the event a window would have delivered.
func frameEvent(o Options, pixels image.Point, source input.Source) app.FrameEvent {
	return app.FrameEvent{
		Size:   pixels,
		Source: source,
		Metric: unit.Metric{PxPerDp: o.Density, PxPerSp: o.Density},
	}
}
