package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	engine "github.com/arandu-io/ayra/engine/widget"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
)

// Input is the state half of a text field: the string, the caret, the
// selection and the undo history.
//
// The caller holds it, and holds one per field. Two fields sharing a value is
// one field drawn twice, and typing in either changes both.
type Input struct {
	editor engine.Editor
}

// Text is what the field holds.
func (i *Input) Text() string { return i.editor.Text() }

// SetText replaces what the field holds, and puts the caret at the start of it.
func (i *Input) SetText(s string) { i.editor.SetText(s) }

// Submitted reports that the person pressed return on a field that accepts it,
// once, and consumes the event.
//
// Only a single-line field raises this. On a multi-line one return inserts a
// line, which is what return means there, and Kind decides nothing about it.
func (i *Input) Submitted(c ayra.Context) bool {
	for {
		event, ok := i.editor.Update(c.Context)
		if !ok {
			return false
		}
		if _, submitted := event.(engine.SubmitEvent); submitted {
			return true
		}
	}
}

// Kind is what a field expects, which decides the on-screen keyboard and
// whether the characters are shown.
//
// It is one field rather than two -- a type and a "secret" flag -- because the
// two are never independent: a password field that showed its characters would
// not be a password field, and a numeric field that hid them would be a PIN,
// which is the same choice made once.
type Kind uint8

const (
	// Text is prose: a name, a title, a note.
	Text Kind = iota
	// Email raises a keyboard with the characters an address needs.
	Email
	// Password hides what is typed and asks the platform not to correct it.
	Password
	// Number asks for the keyboard digits and a decimal separator are on. What
	// may be typed is not narrowed, so the value is still checked where it is
	// read: a hardware keyboard can send any character to any field.
	Number
	// Telephone asks for the keyboard a dial pad has. What may be typed is not
	// narrowed.
	Telephone
	// URL raises a keyboard with the characters an address needs.
	URL
)

// hint is what the platform is told to raise.
func (k Kind) hint() key.InputHint {
	switch k {
	case Email:
		return key.HintEmail
	case Password:
		return key.HintPassword
	case Number:
		return key.HintNumeric
	case Telephone:
		return key.HintTelephone
	case URL:
		return key.HintURL
	}
	return key.HintText
}

// String names the kind.
func (k Kind) String() string {
	switch k {
	case Email:
		return "email"
	case Password:
		return "password"
	case Number:
		return "number"
	case Telephone:
		return "tel"
	case URL:
		return "url"
	}
	return "text"
}

// InputProps is what a text field is drawn from.
type InputProps struct {
	// ComponentProps is what the caller changes on this one instance. A field
	// publishes "root", the box; "focus" and "invalid", which are laid over the
	// root while the field has the focus or is rejected; and "placeholder".
	ComponentProps
	// Placeholder is shown while the field is empty. It is not a label: a
	// field whose only label is its placeholder loses it the moment someone
	// types, and what is left is a box with no name.
	Placeholder string
	// Kind is what the field expects. Zero is prose.
	Kind Kind
	// Multiline lets the text wrap and return insert a line. A single-line
	// field turns return into a submission instead.
	Multiline bool
	// Disabled draws the field as unavailable and refuses input.
	Disabled bool
	// Invalid draws the field as rejected. What was wrong is said beside it,
	// by whoever knows -- this only carries the look.
	Invalid bool

	// widget is the name the field is restyled under: "input", or "textarea"
	// for the field that wraps it.
	widget string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p InputProps) PartNames() []string {
	return []string{"root", "focus", "invalid", "placeholder"}
}

// styles resolves the box, the ink and the placeholder of the field, with the
// state the field is in laid over the root.
func (p InputProps) styles(c ayra.Context, focused bool) (box, placeholder theme.Style) {
	t, name := c.Theme, p.widget
	if name == "" {
		name = "input"
	}
	variant := p.VariantName("default")

	root := t.Resolve(name, "root", variant, theme.Style{
		Background:  theme.Colour(t.Colours.Background),
		Foreground:  theme.Colour(t.Colours.Foreground),
		Border:      theme.Colour(t.Colours.Input),
		BorderWidth: theme.Pt(1),
		Radius:      theme.Pt(t.Metrics.Radius),
		Padding:     theme.InsetXY(12, 8),
		Font:        theme.Font{Size: theme.Pt(t.Type.Body)},
	}, p.ComponentProps)

	switch {
	case p.Invalid:
		root = t.Resolve(name, "invalid", variant, theme.Style{Border: theme.Colour(t.Colours.Destructive)}, theme.Component{Parts: p.Parts}).Or(root)
	case focused:
		root = t.Resolve(name, "focus", variant, theme.Style{Border: theme.Colour(t.Colours.Ring)}, theme.Component{Parts: p.Parts}).Or(root)
	}

	placeholder = t.Resolve(name, "placeholder", variant, theme.Style{
		Foreground: theme.Colour(t.Colours.MutedForeground),
	}, p.ComponentProps)
	return root, placeholder
}

// Layout draws the field and returns the room it took.
func (p InputProps) Layout(c ayra.Context, state *Input) ayra.Dimensions {
	box, hint := p.styles(c, c.Focused(&state.editor))
	border, ink := box.Border.NRGBA(), box.Foreground.NRGBA()
	if p.Disabled {
		ink = fade(ink)
		border = fade(border)
		c.Context = c.Context.Disabled()
	}
	box.Border, box.Foreground = theme.Colour(border), theme.Colour(ink)

	state.editor.SingleLine = !p.Multiline
	state.editor.Submit = !p.Multiline
	state.editor.ReadOnly = p.Disabled
	state.editor.InputHint = p.Kind.hint()
	if p.Kind == Password {
		// The rune the platform itself uses, rather than an asterisk: an
		// asterisk is narrower than most glyphs, so a masked field would be
		// visibly shorter than the same text unmasked.
		state.editor.Mask = '•'
	} else {
		state.editor.Mask = 0
	}

	return p.surface(c, box, func(inner ayra.Context) ayra.Dimensions {
		return insetOf(box.Padding).Layout(inner.Context,
			func(gtx layout.Context) layout.Dimensions {
				return p.content(inner.With(gtx), state, box, hint)
			})
	})
}

// content draws the editor, with the placeholder behind it while it is empty.
func (p InputProps) content(c ayra.Context, state *Input, box, hint theme.Style) ayra.Dimensions {
	size := unit.Sp(box.Font.Size.Or(theme.Pt(c.Theme.Type.Body)).Points())
	face := c.Theme.Type.Apply(font.Font{}, box.Font)

	if state.editor.Len() == 0 && p.Placeholder != "" {
		recorded := op.Record(c.Ops)
		placeholder(c, p.Placeholder, size, hint.Foreground.NRGBA(), face)
		drawn := recorded.Stop()
		drawn.Add(c.Ops)
	}

	inkOp := op.Record(c.Ops)
	paint.ColorOp{Color: box.Foreground.NRGBA()}.Add(c.Ops)
	inkStop := inkOp.Stop()

	selection := op.Record(c.Ops)
	paint.ColorOp{Color: c.Theme.Colours.Accent}.Add(c.Ops)
	selectionStop := selection.Stop()

	return state.editor.Layout(c.Context, c.Shaper, face, size, inkStop, selectionStop)
}

// placeholder draws the hint shown while the field is empty.
//
// It starts where the text will start, because it is standing in for that
// text: a hint centred in a field the caret types into from the left announces
// itself as something other than what the person is about to replace.
func placeholder(c ayra.Context, content string, size unit.Sp, ink color.NRGBA, face font.Font) ayra.Dimensions {
	return drawFace(c, content, size, ink, 1, text.Start, face)
}

// surface paints the field's background and border behind its content.
func (p InputProps) surface(c ayra.Context, box theme.Style, content ayra.Widget) ayra.Dimensions {
	measure := op.Record(c.Ops)
	dims := content(c)
	drawn := measure.Stop()

	// A field fills the room it is given rather than shrinking to its text,
	// because a row of fields that each ended where their content did would be
	// a form with a ragged edge.
	dims.Size.X = c.Constraints.Max.X

	radius := c.Dp(unit.Dp(box.Radius.Points()))
	shape := clip.UniformRRect(image.Rectangle{Max: dims.Size}, radius)

	paint.FillShape(c.Ops, box.Background.NRGBA(), shape.Op(c.Ops))
	stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(c.Dp(unit.Dp(box.BorderWidth.Points())))}
	paint.FillShape(c.Ops, box.Border.NRGBA(), stroke.Op())

	drawn.Add(c.Ops)
	return dims
}
