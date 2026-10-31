package widget

import (
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/unit"
)

// surface draws a filled, optionally bordered rounded rectangle behind a
// widget, and answers the room the widget took.
//
// It is here rather than in each control because every one of them does the
// same three things in the same order, and the order is not obvious: the
// content is laid out first without drawing, to learn how much there is to
// cover, and the background is painted underneath afterwards. Written per
// control, one of them eventually paints the fill over its own text.
func surface(c ayra.Context, fill, border color.NRGBA, radius int, content ayra.Widget) ayra.Dimensions {
	return surfaceWidth(c, fill, border, radius, hairline(c), content)
}

// hairline is one point, and never less than one pixel.
//
// At one pixel per point a rounded hairline is zero, and zero is not a thin
// line, it is no line -- which is a border that exists on a dense screen and
// disappears on an ordinary one.
func hairline(c ayra.Context) int {
	return max(c.Dp(unit.Dp(1)), 1)
}

// controlRadius is the corner every control shares, in pixels.
func controlRadius(c ayra.Context) int {
	return c.Dp(unit.Dp(c.Theme.Metrics.Radius))
}
