package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/unit"
)

// SeparatorProps is what a rule between two things is drawn from.
//
// It carries no state, and it carries no content: a separator that could hold a
// label would be a heading with a line through it, which is a different thing
// and belongs to whoever draws the heading.
type SeparatorProps struct {
	// Vertical draws it down instead of across, for a rule between two columns.
	Vertical bool

	// Inset is the room left at each end, so a rule inside a card can stop
	// short of the card's own edge instead of touching it.
	Inset unit.Dp
}

// Layout draws the rule and returns the room it took.
//
// It fills the axis it runs along and takes one pixel across, which is the
// whole of what a separator is: the caller decides how much room it has, and a
// rule that chose its own length would be a rule that disagrees with the column
// it sits in.
func (p SeparatorProps) Layout(c ayra.Context) ayra.Dimensions {
	thickness := c.Dp(unit.Dp(1))
	if thickness < 1 {
		// At one pixel per point a hairline still has to be one pixel. Rounded
		// to zero it is not a thin rule, it is no rule.
		thickness = 1
	}
	inset := c.Dp(p.Inset)

	size := image.Point{X: c.Constraints.Max.X, Y: thickness}
	if p.Vertical {
		size = image.Point{X: thickness, Y: c.Constraints.Max.Y}
	}

	rule := image.Rectangle{Max: size}
	if p.Vertical {
		rule.Min.Y, rule.Max.Y = inset, max(size.Y-inset, inset)
	} else {
		rule.Min.X, rule.Max.X = inset, max(size.X-inset, inset)
	}

	paint.FillShape(c.Ops, c.Theme.Colours.Border, clip.Rect(rule).Op())
	return ayra.Dimensions{Size: size}
}
