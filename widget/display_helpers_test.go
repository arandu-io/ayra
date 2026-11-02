package widget_test

import (
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/arandu-io/ayra/engine/gpu/headless"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/unit"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
)

// Found on 02/11/2026 by drawing the display widgets under the Alpr.sh theme at
// the width of a phone. Each test below fails on the code that was there.

// at draws one frame of the screen at a given moment and scheme.
func (p *panel) at(scheme theme.Scheme, now time.Time, screen func(ayra.Context) ayra.Dimensions) ayra.Dimensions {
	return p.under(theme.New(scheme), now, screen)
}

// under is at with a theme of the caller's.
func (p *panel) under(th theme.Theme, now time.Time, screen func(ayra.Context) ayra.Dimensions) ayra.Dimensions {
	p.ops.Reset()
	gtx := layout.Context{
		Ops:         p.ops,
		Source:      p.router.Source(),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: p.size},
	}
	dims := screen(ayra.Context{Context: gtx, Theme: th, Shaper: p.shaper})
	p.router.Frame(p.ops)
	return dims
}

// raster draws what the panel last built, or skips the test on a machine with no
// drawing surface.
func (p *panel) raster(t *testing.T) *image.RGBA {
	t.Helper()
	window, err := headless.NewWindow(p.size.X, p.size.Y)
	if err != nil {
		t.Skipf("no drawing surface on this machine: %v", err)
	}
	defer window.Release()
	if err := window.Frame(p.ops); err != nil {
		t.Fatalf("the drawing was refused: %v", err)
	}
	picture := image.NewRGBA(image.Rectangle{Max: p.size})
	if err := window.Screenshot(picture); err != nil {
		t.Fatalf("the picture could not be read back: %v", err)
	}
	return picture
}

// contrast is the WCAG ratio between two opaque colours.
func contrast(a, b color.NRGBA) float64 {
	l := func(c color.NRGBA) float64 {
		f := func(v uint8) float64 {
			x := float64(v) / 255
			if x <= 0.03928 {
				return x / 12.92
			}
			return pow((x+0.055)/1.055, 2.4)
		}
		return 0.2126*f(c.R) + 0.7152*f(c.G) + 0.0722*f(c.B)
	}
	hi, lo := l(a), l(b)
	if lo > hi {
		hi, lo = lo, hi
	}
	return (hi + 0.05) / (lo + 0.05)
}

func pow(x, y float64) float64 { return math.Pow(x, y) }
