package widget

import (
	"image"
	"image/color"
	"testing"

	"github.com/arandu-io/ayra/theme"
)

// TestEveryVariantResolvesToPaletteColours reads what a variant answers with,
// which the exported surface deliberately does not expose.
//
// A variant is a name and a set of colours, and the failure that name hides is
// a set that is entirely transparent: the control takes room, paints nothing,
// and the screen reads as though the button were missing rather than as though
// it were wrong.
func TestEveryVariantResolvesToPaletteColours(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		th := theme.New(scheme)

		for _, v := range []Variant{Default, Secondary, Outline, Ghost, Destructive, Link} {
			t.Run(scheme.String()+"/"+v.String(), func(t *testing.T) {
				fill, ink, border := ButtonProps{Variant: v}.colours(th)

				if ink.A == 0 {
					t.Error("the label would be invisible: the ink is fully transparent")
				}
				// A ghost is transparent at rest on purpose, and it is the one
				// variant that is. Every other one has to put something on the
				// screen before it is pointed at.
				if v != Ghost && v != Link && fill.A == 0 && border.A == 0 {
					t.Error("nothing is drawn at rest: no fill and no border")
				}
			})
		}
	}
}

// TestTheVariantsAreDistinguishable keeps the set from collapsing.
//
// Two variants that resolve to the same three colours are one variant with two
// names, and the second name is a promise the drawing does not keep.
func TestTheVariantsAreDistinguishable(t *testing.T) {
	th := theme.New(theme.Light)

	type look struct{ fill, ink, border color.NRGBA }
	seen := map[look]Variant{}

	for _, v := range []Variant{Default, Secondary, Outline, Destructive} {
		fill, ink, border := ButtonProps{Variant: v}.colours(th)
		l := look{fill, ink, border}
		if other, repeated := seen[l]; repeated {
			t.Errorf("%s and %s draw identically", v, other)
		}
		seen[l] = v
	}
}

// TestADisabledLookIsDerivedRatherThanNamed fixes the decision: unavailable is
// half the alpha of whatever the variant already is, not a grey the palette
// would have to name six times -- once per variant, each a chance to be wrong.
func TestADisabledLookIsDerivedRatherThanNamed(t *testing.T) {
	opaque := color.NRGBA{R: 20, G: 30, B: 40, A: 255}

	got := fade(opaque)

	if got.A != 127 {
		t.Errorf("fade gave alpha %d, want half of 255", got.A)
	}
	if got.R != opaque.R || got.G != opaque.G || got.B != opaque.B {
		t.Errorf("fade changed the hue: %+v became %+v", opaque, got)
	}
}

// TestPressIsDarkerThanHover keeps the two states apart by eye. A control that
// looks the same hovered and held gives no feedback that the press landed,
// which reads as an application that did not respond.
func TestPressIsDarkerThanHover(t *testing.T) {
	base := color.NRGBA{R: 200, G: 200, B: 200, A: 255}

	hovered, held := press(base, false), press(base, true)

	if !(held.R < hovered.R && hovered.R < base.R) {
		t.Errorf("at rest %d, hovered %d, held %d: the three have to be told apart", base.R, hovered.R, held.R)
	}
}

// TestAnIconButtonIsSquare fixes what the size's own name promises.
//
// An inset is the same on four sides and the width still follows the label, so
// a mark one glyph wide came out narrower than it is tall and a mark of two
// came out wider. A row of them was a row of different shapes, which is the one
// thing a row of icons must not be.
func TestAnIconButtonIsSquare(t *testing.T) {
	for _, mark := range []string{"K", "+", "icon", "×"} {
		c, _ := field(t, theme.Light, 400)
		c.Constraints.Min = image.Point{}

		var state Button
		dims := ButtonProps{Label: mark, Size: Icon}.Layout(c, &state)

		if dims.Size.X != dims.Size.Y {
			t.Errorf("an icon button holding %q came out %dx%d", mark, dims.Size.X, dims.Size.Y)
		}
	}
}

// TestEveryToneIsTellableFromTheOthers keeps a closed set from carrying a word
// that changes nothing.
//
// Two of the four were the ordinary foreground and the primary colour, which
// are thirteen of two hundred and fifty-five apart in one scheme and twenty-one
// in the other. That is under the tolerance the picture comparison allows
// between two machines: a word marked accent and the word beside it were the
// same word, and the set had three members a reader could tell apart.
func TestEveryToneIsTellableFromTheOthers(t *testing.T) {
	const apart = 32

	for name, scheme := range map[string]theme.Scheme{"light": theme.Light, "dark": theme.Dark} {
		th := theme.New(scheme)
		tones := map[Tone]string{Normal: "normal", Muted: "muted", Danger: "danger", Accent: "accent"}

		for one, oneName := range tones {
			for other, otherName := range tones {
				if one >= other {
					continue
				}
				if distance(one.ink(th), other.ink(th)) < apart {
					t.Errorf("in the %s scheme %s and %s are %d apart, which nobody can see",
						name, oneName, otherName, distance(one.ink(th), other.ink(th)))
				}
			}
		}
	}
}

// distance answers how far apart two colours are, on the widest channel.
//
// The widest rather than the sum, because a pair that differs a little on every
// channel is a pair nobody can tell apart, and a sum would call that three
// times the difference it looks.
func distance(a, b color.NRGBA) int {
	widest := 0
	for _, pair := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		apart := int(pair[0]) - int(pair[1])
		if apart < 0 {
			apart = -apart
		}
		if apart > widest {
			widest = apart
		}
	}
	return widest
}
