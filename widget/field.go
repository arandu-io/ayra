package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	engine "github.com/arandu-io/ayra/engine/widget"
)

// FieldProps is a labelled control with its help and its error.
//
// It exists because those four pieces are always written together and are
// always written slightly differently: the label above or beside, the error in
// place of the help or under it, the spacing a point out. One type is what makes
// every form in a product agree, and it is the single most repeated shape in
// any application that has forms at all.
type FieldProps struct {
	// Label names the control.
	Label string
	// Help is the sentence under it, shown while there is no error.
	Help string
	// Error replaces the help when the value was rejected. Setting it is what
	// marks the control invalid: two flags would drift, and a field that looks
	// wrong with no message is worse than either.
	Error string
	// Required marks it as one that cannot be left blank.
	Required bool
	// Disabled draws the label as unavailable. The control below it is the
	// caller's, so the same flag has to reach that too.
	Disabled bool
}

// Layout draws the label, the control and whichever line belongs under it.
func (p FieldProps) Layout(c ayra.Context, control ayra.Widget) ayra.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.Label == "" {
				return layout.Dimensions{}
			}
			return LabelProps{Text: p.Label, Required: p.Required, Disabled: p.Disabled}.Layout(c.With(gtx))
		}),
		layout.Rigid(layout.Spacer{Height: 6}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return control(c.With(gtx))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			note, ink := p.Help, c.Theme.Colours.MutedForeground
			if p.Error != "" {
				note, ink = p.Error, c.Theme.Colours.Destructive
			}
			if note == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), note, unit.Sp(c.Theme.Type.Small), ink, 0, text.Start, plain())
			})
		}),
	)
}

// Invalid reports whether the control inside should be drawn as rejected.
//
// A caller passes this into the control rather than the control reading the
// field, because the control is the caller's: a field can hold anything, and
// what "invalid" looks like belongs to the thing being drawn.
func (p FieldProps) Invalid() bool { return p.Error != "" }

// PasswordProps is a field whose value can be revealed.
//
// A password field with no way to see what was typed is the commonest cause of
// a failed sign-in on a phone, where the keyboard hides half the letters and
// autocorrect touches the rest.
type PasswordProps struct {
	// Placeholder is shown while the field is empty.
	Placeholder string
	// Disabled draws the field as unavailable and refuses input.
	Disabled bool
	// Invalid draws the field as rejected. What was wrong is said beside it by
	// whoever knows; this carries the look and nothing else.
	Invalid bool
}

// Secret is the state half of a password field: what is typed, and whether it
// is shown.
//
// Named for what it holds rather than for the control, because Password is
// already the [Kind] that asks the platform to hide the typing, and one word
// for the two of them is a compile error today and a confusion afterwards.
type Secret struct {
	Input
	reveal engine.Clickable
	shown  bool
}

// Shown reports whether the value is currently visible.
func (p *Secret) Shown() bool { return p.shown }

// Layout draws the field with the control that reveals it.
func (p PasswordProps) Layout(c ayra.Context, state *Secret) ayra.Dimensions {
	if state.reveal.Clicked(c.Context) {
		state.shown = !state.shown
	}

	kind := Password
	if state.shown {
		// Shown, it is prose: a revealed password drawn in the password kind
		// still asks the platform for a password keyboard, which on a phone
		// means no autocorrect and no suggestion -- correct while hidden and
		// wrong once somebody is reading what they typed.
		kind = Text
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return InputProps{
				Placeholder: p.Placeholder,
				Kind:        kind,
				Disabled:    p.Disabled,
				Invalid:     p.Invalid,
			}.Layout(c.With(gtx), &state.Input)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}

			label := "Show"
			if state.shown {
				label = "Hide"
			}
			return layout.Inset{Left: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return state.reveal.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return drawText(c.With(gtx), label, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Middle, plain())
					})
				})
			})
		}),
	)
}
