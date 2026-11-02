package frame_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
)

// Translucent ink is blended in the space a picture is written in, as the
// browser half does. Blended in straightened light, white at 15% over black came
// out at 108 of 255 and the same colour in a browser is 38, so every border,
// wash, scrim and badge of the dark scheme was twice as bright in the app as on
// the site. The figures here are the ones the CSS rgba() composite gives, which
// were also read from a Chrome (white 15% over black 38; over the dark ground 47).

// blended is the sRGB composite of ink over an opaque ground: each channel the
// ink's times its alpha plus the ground's times what is left.
func blended(ink, ground color.NRGBA) color.NRGBA {
	a := float64(ink.A) / 255
	mix := func(i, g uint8) uint8 { return uint8(math.Round(float64(i)*a + float64(g)*(1-a))) }
	return color.NRGBA{R: mix(ink.R, ground.R), G: mix(ink.G, ground.G), B: mix(ink.B, ground.B), A: 255}
}

// drawn paints ground and then ink over it, and reads the middle of the result.
func drawn(t *testing.T, scheme theme.Scheme, ground, ink color.NRGBA) color.NRGBA {
	t.Helper()
	screen := func(c ayra.Context) ayra.Dimensions {
		area := clip.Rect(image.Rect(0, 0, 40, 40)).Op()
		paint.FillShape(c.Ops, ground, area)
		paint.FillShape(c.Ops, ink, area)
		return ayra.Dimensions{Size: image.Pt(40, 40)}
	}
	img, err := frame.Draw(frame.Options{Width: 40, Height: 40, Density: 1, Scheme: scheme, Fonts: fonts()}, screen)
	if err != nil {
		if noSurface(err) {
			t.Skipf("no drawing surface on this machine: %v", err)
		}
		t.Fatal(err)
	}
	p := img.RGBAAt(20, 20)
	return color.NRGBA{R: p.R, G: p.G, B: p.B, A: 255}
}

func within(a, b color.NRGBA, tolerance int) bool {
	diff := func(x, y uint8) int {
		d := int(x) - int(y)
		if d < 0 {
			d = -d
		}
		return d
	}
	return diff(a.R, b.R) <= tolerance && diff(a.G, b.G) <= tolerance && diff(a.B, b.B) <= tolerance
}

func TestTranslucentInkBlendsAsABrowserDoes(t *testing.T) {
	white15 := color.NRGBA{R: 255, G: 255, B: 255, A: 38}
	black := color.NRGBA{A: 255}

	// The figure in the report: 38, and not the 108 straightened light gave.
	if got := drawn(t, theme.Dark, black, white15); !within(got, color.NRGBA{R: 38, G: 38, B: 38, A: 255}, 1) {
		t.Errorf("white at 15%% over black drew %v, a browser draws 38", got)
	}

	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		th := theme.New(scheme)
		palette := th.Colours
		for name, test := range map[string]struct{ ink, ground color.NRGBA }{
			"white 15% over the ground":     {white15, palette.Background},
			"white 15% over the card":       {white15, palette.Card},
			"the border over the ground":    {palette.Border, palette.Background},
			"the border over the card":      {palette.Border, palette.Card},
			"the input over the card":       {palette.Input, palette.Card},
			"a scrim, black at 50%":         {color.NRGBA{A: 128}, palette.Background},
			"a disabled label, half opaque": {color.NRGBA{R: palette.Primary.R, G: palette.Primary.G, B: palette.Primary.B, A: 128}, palette.Background},
			"a destructive wash, 20%":       {color.NRGBA{R: palette.Destructive.R, G: palette.Destructive.G, B: palette.Destructive.B, A: 51}, palette.Background},
		} {
			want := blended(test.ink, test.ground)
			got := drawn(t, scheme, test.ground, test.ink)
			if !within(got, want, 1) {
				t.Errorf("%s, %s: drew %v, the sRGB composite is %v", scheme, name, got, want)
			}
		}
	}
}
