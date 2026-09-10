package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	giowidget "github.com/arandu-io/ayra/engine/widget"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
)

// Button is the state half: what the control remembers between frames.
//
// The caller holds it, because a control redrawn thirty times a second cannot
// remember anything by itself. Two buttons are two values; one value drawn
// twice is one button in two places, and it answers Pressed for both.
type Button struct {
	click giowidget.Clickable
}

// Clicked reports a completed press, once, and consumes it.
//
// It is asked before the control is drawn, so that what the press changes is
// on the screen in the same frame. Asking afterwards costs a frame, and a
// frame of lag on a button is what reads as an unresponsive application.
func (b *Button) Clicked(c ayra.Context) bool { return b.click.Clicked(c.Context) }

// Pressed reports whether the control is held down right now.
func (b *Button) Pressed() bool { return b.click.Pressed() }

// Hovered reports whether the pointer is over the control.
func (b *Button) Hovered() bool { return b.click.Hovered() }

// ButtonProps is what a button is drawn from.
//
// The fields are the ones the browser half of the product names, so a control
// described on one side is described the same way on the other. What is absent
// is what has no meaning here: there is no class, no attribute and no request
// -- a press calls Go, and the screen decides what that means.
type ButtonProps struct {
	// Label is the text on the button.
	Label string
	// Variant is what the button is for. Zero is the ordinary one.
	Variant Variant
	// Size is how much room it takes. Zero is the one a form is built from.
	Size Size
	// Disabled draws it as unavailable and stops it answering.
	Disabled bool
}

// Layout draws the button and returns the room it took.
func (p ButtonProps) Layout(c ayra.Context, state *Button) ayra.Dimensions {
	fill, ink, border := p.colours(c.Theme)

	if p.Disabled {
		// Half opacity rather than a grey of its own: a grey is a colour the
		// palette would have to name for every variant, and six greys that
		// mean "not now" is six chances for one of them to be wrong.
		fill = fade(fill)
		ink = fade(ink)
		border = fade(border)
		c.Context = c.Context.Disabled()
	}

	if !p.Disabled && (state.Pressed() || state.Hovered()) {
		fill = press(fill, state.Pressed())
	}

	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		return p.surface(inner, fill, border, func(inner ayra.Context) ayra.Dimensions {
			return p.inset().Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
				content := c.With(gtx)
				dims := buttonLabel(content, p.Label, p.textSize(c.Theme), ink)
				if p.Variant == Link {
					underline(content, dims, p.Label, p.textSize(c.Theme), ink)
				}
				return dims
			})
		})
	})
}

// colours answers the fill, the ink and the border for this variant.
//
// Every one comes from the palette by name. A variant that needed a colour the
// palette does not name would be a variant that does not belong to this design
// system, which is the whole reason the set is closed.
func (p ButtonProps) colours(t theme.Theme) (fill, ink, border color.NRGBA) {
	transparent := color.NRGBA{}

	switch p.Variant {
	case Secondary:
		return t.Colours.Secondary, t.Colours.SecondaryForeground, transparent
	case Outline:
		return transparent, t.Colours.Foreground, t.Colours.Border
	case Ghost:
		return transparent, t.Colours.Foreground, transparent
	case Destructive:
		return t.Colours.Destructive, t.Colours.PrimaryForeground, transparent
	case Link:
		return transparent, t.Colours.Primary, transparent
	}
	return t.Colours.Primary, t.Colours.PrimaryForeground, transparent
}

// inset is the room around the label, by size.
func (p ButtonProps) inset() layout.Inset {
	switch p.Size {
	case ExtraSmall:
		return layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8}
	case Small:
		return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}
	case Large:
		return layout.Inset{Top: 12, Bottom: 12, Left: 20, Right: 20}
	case Icon:
		return layout.UniformInset(8)
	}
	return layout.Inset{Top: 8, Bottom: 8, Left: 14, Right: 14}
}

// textSize is the label's size, by control size.
func (p ButtonProps) textSize(t theme.Theme) unit.Sp {
	switch p.Size {
	case ExtraSmall, Small:
		return unit.Sp(t.Type.Small)
	case Large:
		return unit.Sp(t.Type.Heading)
	}
	return unit.Sp(t.Type.Body)
}

// surface paints the fill and the border behind the content.
//
// The content is laid out first, without drawing, to learn how big the surface
// has to be -- there is no other way round: a background cannot be painted
// before it is known how much there is to cover, and painting it afterwards
// would put it over the label.
func (p ButtonProps) surface(c ayra.Context, fill, border color.NRGBA, content ayra.Widget) ayra.Dimensions {
	measure := op.Record(c.Ops)
	dims := content(c)
	drawn := measure.Stop()

	radius := c.Dp(unit.Dp(c.Theme.Metrics.Radius))
	if p.Variant == Link {
		radius = 0
	}
	rect := image.Rectangle{Max: dims.Size}
	shape := clip.UniformRRect(rect, radius)

	if fill.A > 0 {
		paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	}
	if border.A > 0 {
		stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(c.Dp(unit.Dp(1)))}
		paint.FillShape(c.Ops, border, stroke.Op())
	}

	drawn.Add(c.Ops)
	return dims
}

// buttonLabel draws a button's text: one line, centred in whatever width the
// button was given.
//
// Centred rather than left-aligned, because a button takes the width it is
// offered and a form offers it the whole column. Left-aligned text in a
// full-width button sits against the corner and reads as a heading with a box
// around it -- which is what this looked like before anybody drew one.
//
// The name says which control it belongs to, and that is the fix for a real
// mistake: it was called "label", the text field reused it, and centring it
// here centred every placeholder in the product.
func buttonLabel(c ayra.Context, content string, size unit.Sp, ink color.NRGBA) ayra.Dimensions {
	return drawText(c, content, size, ink, 1, text.Middle, font.Font{Weight: font.Medium})
}

// underline draws the rule beneath a link's text.
//
// A link is the one variant with no fill and no border, and without this it is
// drawn exactly like a ghost: the palette gives both of them the foreground
// colour, so two variants produced one picture and the difference existed only
// in the name. The rule is what a reader recognises as a link, and it is the
// only thing that can carry that here -- there is no cursor change to rely on
// and no hover at all on a touch screen.
//
// It is measured rather than drawn across the control, because a link takes
// the width it is offered like any other button, and a rule under the whole
// column is a divider.
func underline(c ayra.Context, dims ayra.Dimensions, content string, size unit.Sp, ink color.NRGBA) {
	measuring := c
	measuring.Constraints.Min.X = 0
	measuring.Constraints.Max.X = dims.Size.X

	measured := op.Record(c.Ops)
	text := buttonLabel(measuring, content, size, ink)
	measured.Stop()

	width := text.Size.X
	if width <= 0 || width > dims.Size.X {
		width = dims.Size.X
	}

	left := (dims.Size.X - width) / 2
	thickness := c.Dp(unit.Dp(1))
	if thickness < 1 {
		// At one pixel per point a hairline still has to be one pixel. Rounded
		// to zero it is not a thin rule, it is no rule.
		thickness = 1
	}

	// Below the baseline rather than at the bottom of the line: a rule at the
	// bottom sits a descender's depth away from the word on a line with no
	// descenders, and touches the box on a line that has them.
	baseline := dims.Size.Y - dims.Baseline
	top := baseline + c.Dp(unit.Dp(2))
	if top+thickness > dims.Size.Y {
		top = dims.Size.Y - thickness
	}

	rule := image.Rect(left, top, left+width, top+thickness)
	paint.FillShape(c.Ops, ink, clip.Rect(rule).Op())
}

// fade halves the alpha, which is how a control says "not now" without the
// palette naming a colour for it.
func fade(c color.NRGBA) color.NRGBA {
	c.A /= 2
	return c
}

// press darkens a fill while the pointer is over it or on it.
//
// Two steps rather than one, because hover and press have to be told apart by
// eye: a control that looks the same hovered and held gives no feedback that
// the press landed.
func press(c color.NRGBA, held bool) color.NRGBA {
	shade := uint8(12)
	if held {
		shade = 28
	}
	if c.A == 0 {
		// Nothing to darken: an outline or a ghost gets a wash instead, taken
		// from the ink it will draw with rather than from a colour of its own.
		return color.NRGBA{A: shade}
	}
	return color.NRGBA{R: darken(c.R, shade), G: darken(c.G, shade), B: darken(c.B, shade), A: c.A}
}

func darken(v, by uint8) uint8 {
	if v < by {
		return 0
	}
	return v - by
}
