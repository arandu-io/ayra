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
	"github.com/arandu-io/ayra/engine/unit"
	giowidget "github.com/arandu-io/ayra/engine/widget"

	"github.com/arandu-io/ayra"
)

// Input is the state half of a text field: the string, the caret, the
// selection and the undo history.
//
// The caller holds it, and holds one per field. Two fields sharing a value is
// one field drawn twice, and typing in either changes both.
type Input struct {
	editor giowidget.Editor
}

// Text is what the field holds.
func (i *Input) Text() string { return i.editor.Text() }

// SetText replaces what the field holds, and moves the caret to the end.
func (i *Input) SetText(s string) { i.editor.SetText(s) }

// Submitted reports that the person pressed return on a field that accepts it,
// once, and consumes the event.
//
// Only a field whose Kind says so raises this. On a multi-line field return
// inserts a line, which is what return means there.
func (i *Input) Submitted(c ayra.Context) bool {
	for {
		event, ok := i.editor.Update(c.Context)
		if !ok {
			return false
		}
		if _, submitted := event.(giowidget.SubmitEvent); submitted {
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
	// Number is digits, a decimal separator and nothing else.
	Number
	// Telephone is digits and the two symbols a dial pad has.
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
}

// Layout draws the field and returns the room it took.
func (p InputProps) Layout(c ayra.Context, state *Input) ayra.Dimensions {
	t := c.Theme

	border := t.Colours.Input
	if p.Invalid {
		border = t.Colours.Destructive
	}
	if c.Focused(&state.editor) && !p.Invalid {
		border = t.Colours.Ring
	}

	ink := t.Colours.Foreground
	if p.Disabled {
		ink = fade(ink)
		border = fade(border)
		c.Context = c.Context.Disabled()
	}

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

	return p.surface(c, t.Colours.Background, border, func(inner ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(inner.Context,
			func(gtx layout.Context) layout.Dimensions {
				return p.content(inner.With(gtx), state, ink)
			})
	})
}

// content draws the editor, with the placeholder behind it while it is empty.
func (p InputProps) content(c ayra.Context, state *Input, ink color.NRGBA) ayra.Dimensions {
	size := unit.Sp(c.Theme.Type.Body)

	if state.editor.Len() == 0 && p.Placeholder != "" {
		hint := op.Record(c.Ops)
		label(c, p.Placeholder, size, c.Theme.Colours.MutedForeground)
		drawn := hint.Stop()
		drawn.Add(c.Ops)
	}

	inkOp := op.Record(c.Ops)
	paint.ColorOp{Color: ink}.Add(c.Ops)
	inkStop := inkOp.Stop()

	selection := op.Record(c.Ops)
	paint.ColorOp{Color: c.Theme.Colours.Accent}.Add(c.Ops)
	selectionStop := selection.Stop()

	return state.editor.Layout(c.Context, c.Shaper, font.Font{}, size, inkStop, selectionStop)
}

// surface paints the field's background and border behind its content.
func (p InputProps) surface(c ayra.Context, fill, border color.NRGBA, content ayra.Widget) ayra.Dimensions {
	measure := op.Record(c.Ops)
	dims := content(c)
	drawn := measure.Stop()

	// A field fills the room it is given rather than shrinking to its text,
	// because a row of fields that each ended where their content did would be
	// a form with a ragged edge.
	dims.Size.X = c.Constraints.Max.X

	radius := c.Dp(unit.Dp(c.Theme.Metrics.Radius))
	shape := clip.UniformRRect(image.Rectangle{Max: dims.Size}, radius)

	paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(c.Dp(unit.Dp(1)))}
	paint.FillShape(c.Ops, border, stroke.Op())

	drawn.Add(c.Ops)
	return dims
}
