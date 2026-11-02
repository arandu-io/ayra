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
	engine "github.com/arandu-io/ayra/engine/widget"
	"github.com/arandu-io/ayra/icons"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
)

// Button is the state half: what the control remembers between frames.
//
// The caller holds it, because a control redrawn thirty times a second cannot
// remember anything by itself. Two buttons are two values; one value drawn
// twice is one button in two places, and it answers Pressed for both.
type Button struct {
	click engine.Clickable
}

// Clicked reports a completed press, once, and consumes it.
//
// It is asked before the control is drawn, so that what the press changes is
// on the screen in the same frame. Asking afterwards costs a frame, and a
// frame of lag on a button is what reads as an unresponsive application.
//
// A button drawn disabled, or inside a disabled context, is no target for the
// pointer or the keyboard, so nothing done to it while it was drawn that way
// is reported here.
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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each part. The button publishes
	// one part, "root", which is the button itself.
	ComponentProps
	// Label is the text on the button.
	Label string
	// Icon is drawn before the label, and alone when there is none -- the
	// icon-only button, which wants a Size of Icon. The zero Icon draws nothing.
	Icon icons.Icon
	// IconOnly draws the icon alone and keeps Label as the name a screen reader
	// says, as Kyse's IconOnly does: an icon-only button with no name is
	// announced as "button" and nothing on the screen says which one.
	IconOnly bool
	// Trailing draws the icon after the label instead of before it, where Kyse
	// puts the caret of a menu and the arrow of a sortable heading.
	Trailing bool
	// Variant is what the button is for. Zero is the ordinary one.
	Variant Variant
	// Size is how much room it takes. Zero is the one a form is built from.
	Size Size
	// Disabled draws it as unavailable and stops it answering.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p ButtonProps) PartNames() []string { return []string{"root"} }

// Layout draws the button and returns the room it took.
//
// What a button takes and what it answers to are not the same room. It is
// drawn at the size its variant says -- 24 points tall for the smallest, 34 for
// the ordinary one -- and the area that answers the press is grown to
// [TouchTarget] by [TouchArea], while the room reported to the layout stays
// what was drawn.
func (p ButtonProps) Layout(c ayra.Context, state *Button) ayra.Dimensions {
	style := p.style(c.Theme)
	fill, ink, border := style.Background.NRGBA(), style.Foreground.NRGBA(), style.Border.NRGBA()

	// Asked before the context is made disabled: a control that cannot be
	// pressed cannot hold the focus either, so a disabled one never shows it.
	focused := !p.Disabled && c.Focused(&state.click)

	if p.Disabled {
		// Half opacity, as the browser half draws it, rather than a grey of its
		// own: a grey is a colour the palette would have to name for every
		// variant, and six greys that mean "not now" is six chances for one of
		// them to be wrong.
		ground := c.Theme.Colours.Background
		fill = fadeOver(fill, ground)
		ink = fadeOver(ink, ground)
		border = fadeOver(border, ground)
		c.Context = c.Context.Disabled()
	}

	if !p.Disabled && (state.Pressed() || state.Hovered()) {
		switch {
		case state.Pressed() && style.PressedBackground.IsSet():
			fill = style.PressedBackground.NRGBA()
		case !state.Pressed() && style.HoverBackground.IsSet():
			fill = style.HoverBackground.NRGBA()
		default:
			fill = press(fill, state.Pressed())
		}
	}
	if focused {
		// The border takes the ring's colour, as Kyse's focus-visible does; the
		// ring around the shape is drawn below.
		border = c.Theme.Colours.Ring
	}
	style.Background, style.Foreground, style.Border = theme.Colour(fill), theme.Colour(ink), theme.Colour(border)

	if p.Size == Icon {
		// Square, which the size's own name promises and the inset alone does
		// not give: an inset is the same on four sides and the width still
		// follows the label, so a mark one glyph wide came out narrower than it
		// is tall and a mark of two came out wider. A row of them was a row of
		// different shapes.
		c.Constraints.Min.X = 0
	}

	drawn := TouchArea(c, func(c ayra.Context, content ayra.Widget) ayra.Dimensions {
		return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions { return content(c.With(gtx)) })
	}, func(inner ayra.Context) ayra.Dimensions {
		return p.surface(inner, style, func(inner ayra.Context) ayra.Dimensions {
			return insetOf(style.Padding).Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
				content := c.With(gtx)
				shown := p.Label
				if p.IconOnly && !p.Icon.IsZero() {
					shown = ""
				}
				dims := buttonContent(content, shown, p.Icon, p.Trailing, style)
				if p.Variant == Link && p.Custom == "" && p.Icon.IsZero() {
					underline(content, dims, p.Label, style)
				}
				return dims
			})
		})
	})

	if focused {
		focusRing(c, drawn.Size, c.Dp(unit.Dp(style.Radius.Points())), c.Theme.Colours.Ring)
	}
	return drawn
}

// focusRing draws the ring that says a control holds the keyboard focus: three
// points around the shape, outside it, at half the strength of the ring colour --
// Kyse's focus-visible:ring-ring/50 ring-[3px].
//
// It is drawn after the press area rather than inside it, because the area
// clips to the shape it answers for and a ring outside the shape would be
// cut off exactly where it is wanted.
func focusRing(c ayra.Context, size image.Point, radius int, ring color.NRGBA) {
	width := max(c.Dp(unit.Dp(3)), 1)
	ring.A /= 2

	half := width / 2
	rect := image.Rect(-half, -half, size.X+half, size.Y+half)
	shape := clip.UniformRRect(rect, radius+half)
	stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(width)}
	paint.FillShape(c.Ops, ring, stroke.Op())
}

// style answers how the button looks: what the variant and the size say on
// their own, with the theme's look, the registered variant and the instance's
// own style laid over it.
func (p ButtonProps) style(t theme.Theme) theme.Style {
	fill, ink, border := p.colours(t)
	radius := t.Metrics.Radius
	if p.Variant == Link {
		radius = 0
	}
	base := theme.Style{
		Background: theme.Colour(fill),
		Foreground: theme.Colour(ink),
		Border:     theme.Colour(border),
		Radius:     theme.Pt(radius),
		Padding:    p.inset(),
		Font:       theme.Font{Size: theme.Pt(p.textSize(t))},
	}
	return t.Resolve("button", "root", p.VariantName(p.Variant.String()), base, p.ComponentProps)
}

// colours answers the fill, the ink and the border for this variant.
//
// Every one comes from the palette by name. A variant that needed a colour the
// palette does not name is registered in the theme under a name of its own,
// which is the open half of what used to be a closed set.
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
func (p ButtonProps) inset() theme.Insets {
	switch p.Size {
	case ExtraSmall:
		return theme.InsetXY(8, 4)
	case Small:
		return theme.InsetXY(10, 6)
	case Large:
		return theme.InsetXY(20, 12)
	case Icon:
		return theme.Uniform(8)
	}
	return theme.InsetXY(14, 8)
}

// textSize is the label's size in points, by control size.
func (p ButtonProps) textSize(t theme.Theme) float32 {
	switch p.Size {
	case ExtraSmall, Small:
		return t.Type.Small
	}
	return t.Type.Body
}

// surface paints the fill and the border behind the content.
//
// The content is laid out first, without drawing, to learn how big the surface
// has to be -- there is no other way round: a background cannot be painted
// before it is known how much there is to cover, and painting it afterwards
// would put it over the label.
func (p ButtonProps) surface(c ayra.Context, style theme.Style, content ayra.Widget) ayra.Dimensions {
	fill, border := style.Background.NRGBA(), style.Border.NRGBA()
	measure := op.Record(c.Ops)
	dims := content(c)
	drawn := measure.Stop()

	if p.Size == Icon {
		// Taken from the taller side, because a mark has to fit and a square
		// that shrank to the narrower one would clip it.
		side := max(dims.Size.X, dims.Size.Y)
		dims.Size = image.Pt(side, side)
	}

	radius := c.Dp(unit.Dp(style.Radius.Points()))
	rect := image.Rectangle{Max: dims.Size}
	shape := clip.UniformRRect(rect, radius)

	// Announced against the button's own shape rather than inside the press
	// target. A disabled control registers no target -- there is nothing to
	// press -- so an announcement made in there disappears exactly when it
	// matters most: what reached the tree was the label with no class and
	// nothing saying it was unavailable, which reads as ordinary text.
	close := announce(c, dims.Size, Announcement{Label: p.Label, Class: Pressable, Disabled: p.Disabled})

	if fill.A > 0 {
		paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	}
	if border.A > 0 {
		width := c.Dp(unit.Dp(1))
		if style.BorderWidth.IsSet() {
			width = c.Dp(unit.Dp(style.BorderWidth.Points()))
		}
		stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(width)}
		paint.FillShape(c.Ops, border, stroke.Op())
	}

	drawn.Add(c.Ops)
	close()
	return dims
}

// buttonContent draws what is on a button: the label, and the icon before it. An
// icon and a label are centred together as one, and an icon alone is centred in
// the room it is given.
func buttonContent(c ayra.Context, label string, icon icons.Icon, trailing bool, style theme.Style) ayra.Dimensions {
	if icon.IsZero() {
		return buttonLabel(c, label, style)
	}

	side := c.Dp(unit.Dp(style.Font.Size.Or(theme.Pt(c.Theme.Type.Body)).Points()))
	gap := 0
	if label != "" {
		gap = c.Dp(unit.Dp(6))
	}

	// Measured first, so that the pair can be centred as one.
	measure := op.Record(c.Ops)
	text := ayra.Dimensions{}
	if label != "" {
		measuring := c
		measuring.Constraints.Min = image.Point{}
		text = buttonLabel(measuring, label, style)
	}
	measure.Stop()

	width := side + gap + text.Size.X
	height := max(side, text.Size.Y)
	left := max((c.Constraints.Min.X-width)/2, 0)

	iconAt, labelAt := left, left+side+gap
	if trailing {
		iconAt, labelAt = left+text.Size.X+gap, left
	}
	icon0 := op.Offset(image.Pt(iconAt, (height-side)/2)).Push(c.Ops)
	DrawIcon(c, icon, side, style.Foreground.NRGBA())
	icon0.Pop()

	if label != "" {
		shifted := op.Offset(image.Pt(labelAt, 0)).Push(c.Ops)
		measuring := c
		measuring.Constraints.Min = image.Point{}
		buttonLabel(measuring, label, style)
		shifted.Pop()
	}
	return ayra.Dimensions{Size: image.Pt(max(width+left*2, c.Constraints.Min.X), height), Baseline: text.Baseline}
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
//
// It is centred in the height as well, when the button was given more than the
// line needs. A calendar's day is told to be tall enough to press, and the
// label sat against the top of it -- the number high in a square that was meant
// to hold it in the middle.
func buttonLabel(c ayra.Context, content string, style theme.Style) ayra.Dimensions {
	face := font.Font{Weight: font.Medium}
	room := c.Constraints.Min.Y
	if room <= 0 {
		return drawPart(c, content, style, unit.Sp(c.Theme.Type.Body), 1, text.Middle, face)
	}

	natural := c
	natural.Constraints.Min.Y = 0
	measured := op.Record(c.Ops)
	line := drawPart(natural, content, style, unit.Sp(c.Theme.Type.Body), 1, text.Middle, face)
	measured.Stop()
	if line.Size.Y >= room {
		return drawPart(c, content, style, unit.Sp(c.Theme.Type.Body), 1, text.Middle, face)
	}

	down := (room - line.Size.Y) / 2
	shift := op.Offset(image.Pt(0, down)).Push(c.Ops)
	dims := drawPart(natural, content, style, unit.Sp(c.Theme.Type.Body), 1, text.Middle, face)
	shift.Pop()
	dims.Size.Y = room
	dims.Baseline += down
	return dims
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
func underline(c ayra.Context, dims ayra.Dimensions, content string, style theme.Style) {
	measuring := c
	measuring.Constraints.Min.X = 0
	measuring.Constraints.Max.X = dims.Size.X

	measured := op.Record(c.Ops)
	text := buttonLabel(measuring, content, style)
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
	paint.FillShape(c.Ops, style.Foreground.NRGBA(), clip.Rect(rule).Op())
}

// fade halves the alpha, which is how a control says "not now" without the
// palette naming a colour for it.
func fade(c color.NRGBA) color.NRGBA {
	c.A /= 2
	return c
}

// fadeOver is [fade] drawn the way a browser draws opacity: the colour mixed
// half and half with the ground it sits on, in the colours' own space.
//
// Halving the alpha is not the same picture. The compositor blends in linear
// light, where half of white over black is a light grey and not the middle one,
// so a disabled control in the dark scheme came out at four fifths of the
// strength of an enabled one and the two could not be told apart: the secondary
// and ghost buttons looked the same pressed and unavailable. Mixing the two
// colours directly lands on the grey a person expects, in both schemes.
//
// A colour with nothing to mix -- the fill of a ghost -- stays as it is.
func fadeOver(c, ground color.NRGBA) color.NRGBA {
	if c.A == 0 {
		return c
	}
	mix := func(a, b uint8) uint8 { return uint8((uint16(a) + uint16(b) + 1) / 2) }
	return color.NRGBA{R: mix(c.R, ground.R), G: mix(c.G, ground.G), B: mix(c.B, ground.B), A: c.A}
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
