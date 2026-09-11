package widget

import (
	"image/color"
	"testing"

	"github.com/arandu-io/ayra/theme"
)

// A row that is not the current one paints nothing.
//
// It painted the page's own background, which is invisible on the page and a
// block of the wrong colour anywhere else. Both schemes made that visible, in
// opposite directions, and both were shipping -- it was found by the first
// thing that drew every control side by side rather than one to a screen.

// TestAnUnchosenRowPaintsNothing is the fix.
func TestAnUnchosenRowPaintsNothing(t *testing.T) {
	for name, scheme := range map[string]theme.Scheme{"light": theme.Light, "dark": theme.Dark} {
		c, _ := field(t, scheme, 300)

		if fill := (ItemProps{Title: "Members"}).fill(c); fill != (color.NRGBA{}) {
			t.Errorf("in the %s scheme an unchosen row paints %v rather than taking what is behind it", name, fill)
		}
		if fill := (ItemProps{Title: "Members", Selected: true}).fill(c); fill == (color.NRGBA{}) {
			t.Errorf("in the %s scheme a chosen row paints nothing, so nothing marks it", name)
		}
	}
}

// TestTheSchemesAreWhyThisMatters keeps the reason checkable rather than
// remembered.
//
// The fix reads as fussy until these two facts are in front of somebody: a
// panel is not the page's colour, and in one scheme a muted surface and a
// chosen row are the same value. Each is one half of the fault, and each would
// come back as an argument for painting the background again.
func TestTheSchemesAreWhyThisMatters(t *testing.T) {
	dark := theme.New(theme.Dark).Colours
	if dark.Background == dark.Popover {
		t.Error("a panel is the page's own colour in the dark scheme, which is not what was measured")
	}

	light := theme.New(theme.Light).Colours
	if light.Muted != light.Accent {
		t.Log("a muted surface and a chosen row are no longer one value in the light scheme; half of this fault is gone from the palette")
	}
}

// TestAChosenRowIsVisibleOnEverySurfaceItIsDrawnOn is the half the palette
// cannot promise.
//
// A chosen row marks itself with one colour, and that colour has to differ from
// whatever it sits on. Where it does not, the mark is invisible -- which is the
// same failure as painting nothing, arrived at from the other side.
func TestAChosenRowIsVisibleOnEverySurfaceItIsDrawnOn(t *testing.T) {
	for name, scheme := range map[string]theme.Scheme{"light": theme.Light, "dark": theme.Dark} {
		c, _ := field(t, scheme, 300)
		colours := theme.New(scheme).Colours

		chosen := ItemProps{Title: "Members", Selected: true}
		fill, edge := chosen.fill(c), chosen.edge(c)

		for surface, ground := range map[string]color.NRGBA{
			"the page":     colours.Background,
			"a panel":      colours.Popover,
			"a card":       colours.Card,
			"a muted area": colours.Muted,
		} {
			if fill != ground {
				continue
			}
			// The fill is the ground. Something else has to mark the row, and
			// the line around it is that something.
			if edge == (color.NRGBA{}) || edge == ground {
				t.Errorf("in the %s scheme a chosen row on %s is the colour it sits on and draws no line, so nothing marks it", name, surface)
			}
		}
	}
}
