package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// BadgeProps is what a small label beside something else is drawn from.
//
// It shares Variant with a button on purpose: the same word means the same
// thing on both, so a destructive badge and a destructive button are the same
// red, and somebody who learned one has learned the other.
type BadgeProps struct {
	// Label is the text inside it.
	Label string
	// Variant is what it is for. Zero is the ordinary one.
	Variant Variant
}

// Layout draws the badge and returns the room it took.
//
// It takes the room its text needs and no more, which is the difference between
// this and a button: a badge stretched across a column is a banner, and a
// banner is an [AlertProps].
func (p BadgeProps) Layout(c ayra.Context) ayra.Dimensions {
	fill, ink, border := p.colours(c.Theme)

	c.Constraints.Min = image.Point{}

	inset := layout.Inset{Top: 2, Bottom: 2, Left: 8, Right: 8}
	measure := op.Record(c.Ops)
	dims := inset.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return drawText(c.With(gtx), p.Label, unit.Sp(c.Theme.Type.Small), ink, 1, text.Start, semibold())
	})
	drawn := measure.Stop()

	// A pill rather than the control radius: a badge is round-ended at every
	// size, and reusing the radius makes a short one look like a small button.
	radius := dims.Size.Y / 2
	shape := clip.UniformRRect(image.Rectangle{Max: dims.Size}, radius)

	if fill.A > 0 {
		paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	}
	if border.A > 0 {
		stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(max(c.Dp(unit.Dp(1)), 1))}
		paint.FillShape(c.Ops, border, stroke.Op())
	}

	drawn.Add(c.Ops)
	return dims
}

// colours answers the fill, the ink and the border for this variant.
func (p BadgeProps) colours(t theme.Theme) (fill, ink, border color.NRGBA) {
	transparent := color.NRGBA{}

	switch p.Variant {
	case Secondary:
		return t.Colours.Secondary, t.Colours.SecondaryForeground, transparent
	case Outline, Ghost:
		return transparent, t.Colours.Foreground, t.Colours.Border
	case Destructive:
		return t.Colours.Destructive, t.Colours.PrimaryForeground, transparent
	case Link:
		return transparent, t.Colours.Primary, transparent
	}
	return t.Colours.Primary, t.Colours.PrimaryForeground, transparent
}
