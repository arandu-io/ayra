package widget_test

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font/gofont"
	ayraframe "github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
)

// drawn renders a screen without a window, at one pixel to the point, and hands
// back the picture. A test that reads pixels says what a person sees; one that
// reads a size says what a layout function answered, and the two have parted
// before.
func drawn(t *testing.T, width, height int, scheme theme.Scheme, screen ayra.Widget) *image.RGBA {
	t.Helper()

	got, err := ayraframe.Draw(ayraframe.Options{
		Width: width, Height: height, Density: 1, Scheme: scheme, Fonts: gofont.Collection(),
	}, func(c ayra.Context) ayra.Dimensions {
		// The window hands a screen the whole of its size as the least it may
		// take, and a control drawn straight into it stretches to fill that. A
		// screen sits in the top corner and takes what it needs.
		c.Constraints.Min = image.Point{}
		return screen(c)
	})
	if err != nil {
		if strings.Contains(err.Error(), "no drawing surface") {
			t.Skipf("no drawing surface on this machine: %v", err)
		}
		t.Fatalf("the screen did not draw: %v", err)
	}
	return got
}

// luminance is how bright a pixel is, from 0 to 255.
func luminance(c color.RGBA) int {
	return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
}

// inked reports whether anything in the rectangle is darker than the page it is
// drawn on, in the light scheme.
func inked(picture *image.RGBA, area image.Rectangle) bool {
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if luminance(picture.RGBAAt(x, y)) < 200 {
				return true
			}
		}
	}
	return false
}
