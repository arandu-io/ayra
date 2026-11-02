package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/icons"

	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// Found on 02/11/2026 by drawing the display widgets under the Alpr.sh theme at
// the width of a phone. Each test below fails on the code that was there.

// TestAnIconSitsOnTheLineOfTheWordBesideIt reads where an icon says its baseline
// is. A row aligned by baseline puts that line on the word's own, so the icon
// has to report a line near its bottom edge. It reported one at its top edge, and
// the whole square hung below the line of the word it stands beside.
func TestAnIconSitsOnTheLineOfTheWordBesideIt(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)

	for _, size := range []unit.Dp{12, 14, 20, 32} {
		dims := widget.IconProps{Icon: icons.Trash(), Size: size}.Layout(c)

		// Baseline is the distance up from the bottom edge. A quarter of the side
		// at most: the square rests on the line, a little below it, as an icon
		// does beside a word on the web.
		if dims.Baseline < 0 || dims.Baseline*4 > dims.Size.Y {
			t.Errorf("a %v point icon puts its baseline %d up from its bottom edge, in a square %d tall: it does not sit on the word's line", size, dims.Baseline, dims.Size.Y)
		}
	}
}
