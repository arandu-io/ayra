package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
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
	// ComponentProps is what the caller changes on this one instance. A badge
	// publishes one part, "root".
	ComponentProps
	// Label is the text inside it.
	Label string
	// Variant is what it is for. Zero is the ordinary one.
	Variant Variant
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p BadgeProps) PartNames() []string { return []string{"root"} }

// Layout draws the badge and returns the room it took.
//
// It takes the room its text needs and no more, which is the difference between
// this and a button: a badge stretched across a column is a banner, and a
// banner is an [AlertProps].
func (p BadgeProps) Layout(c ayra.Context) ayra.Dimensions {
	fill, ink, border := p.colours(c.Theme)
	style := c.Theme.Resolve("badge", "root", p.VariantName(p.Variant.String()), theme.Style{
		Background: theme.Colour(fill),
		Foreground: theme.Colour(ink),
		Border:     theme.Colour(border),
		Padding:    theme.InsetXY(8, 2),
		Font:       theme.Font{Size: theme.Pt(c.Theme.Type.Small)},
	}, p.ComponentProps)

	c.Constraints.Min = image.Point{}

	measure := op.Record(c.Ops)
	dims := insetOf(style.Padding).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return drawPart(c.With(gtx), p.Label, style, unit.Sp(c.Theme.Type.Small), 1, text.Start, semibold())
	})
	drawn := measure.Stop()

	// A pill rather than the control radius: a badge is round-ended at every
	// size, and reusing the radius makes a short one look like a small button.
	radius := dims.Size.Y / 2
	if style.Radius.IsSet() {
		radius = c.Dp(unit.Dp(style.Radius.Points()))
	}
	width := max(c.Dp(unit.Dp(1)), 1)
	if style.BorderWidth.IsSet() {
		width = c.Dp(unit.Dp(style.BorderWidth.Points()))
	}
	paintShape(c, dims.Size, style.Background.NRGBA(), style.Border.NRGBA(), radius, width)

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
