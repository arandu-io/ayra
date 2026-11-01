package widget

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// FieldProps is a labelled control with its help and its error.
//
// It exists because those four pieces are always written together and are
// always written slightly differently: the label above or beside, the error in
// place of the help or under it, the spacing a point out. One type is what makes
// every form in a product agree, and it is the single most repeated shape in
// any application that has forms at all.
type FieldProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
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

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p FieldProps) PartNames() []string { return []string{"root"} }

// Layout draws the label, the control and whichever line belongs under it.
func (p FieldProps) Layout(c ayra.Context, control ayra.Widget) ayra.Dimensions {
	return withRoot(c, "field", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, control)
	})
}

// layout is Layout without the root part around it.
func (p FieldProps) layout(c ayra.Context, control ayra.Widget) ayra.Dimensions {
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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Placeholder is shown while the field is empty.
	Placeholder string
	// ShowLabel and HideLabel name the reveal control for a screen reader, which
	// is all that names it: it draws an eye. Empty says "Show" and "Hide", as
	// Kyse's does.
	ShowLabel, HideLabel string
	// Disabled draws the field as unavailable and refuses input.
	Disabled bool
	// Invalid draws the field as rejected. What was wrong is said beside it by
	// whoever knows; this carries the look and nothing else.
	Invalid bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p PasswordProps) PartNames() []string { return []string{"root", "group"} }

// Secret is the state half of a password field: what is typed, and whether it
// is shown.
//
// Named for what it holds rather than for the control, because Password is
// already the [Kind] that asks the platform to hide the typing, and one word
// for the two of them is a compile error today and a confusion afterwards.
type Secret struct {
	Input
	reveal Button
	shown  bool
}

// Shown reports whether the value is currently visible.
func (p *Secret) Shown() bool { return p.shown }

// Reveal shows the value or hides it, for a screen that arrives with it one way
// and for a test.
func (p *Secret) Reveal(shown bool) { p.shown = shown }

// Layout draws the field with the control that reveals it.
func (p PasswordProps) Layout(c ayra.Context, state *Secret) ayra.Dimensions {
	return withRoot(c, "password", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p PasswordProps) layout(c ayra.Context, state *Secret) ayra.Dimensions {
	if state.reveal.Clicked(c) {
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

	// The eye lives inside the field, at its right end, as Kyse's input group
	// has it: one border around the box and the control, drawn by the group, and
	// the box inside it drawn with none. The border says what the field is in --
	// focused, rejected -- because the box has none of its own to say it with.
	t := c.Theme
	border := t.Colours.Input
	switch {
	case p.Invalid:
		border = t.Colours.Destructive
	case c.Focused(&state.editor):
		border = t.Colours.Ring
	}
	if p.Disabled {
		border = fade(border)
	}
	bare := theme.Style{Border: theme.Transparent, Background: theme.Transparent, Radius: theme.Pt(0)}
	box := ComponentProps{Parts: theme.Parts{"root": bare, "focus": bare, "invalid": bare}}

	name, icon := orDefault(p.ShowLabel, "Show"), innerIcon("password", "Eye")
	if state.shown {
		name, icon = orDefault(p.HideLabel, "Hide"), innerIcon("password", "EyeSlash")
	}

	group := t.Resolve("password", "group", "default", theme.Style{
		Background: theme.Colour(t.Colours.Background),
		Border:     theme.Colour(border),
		Radius:     theme.Pt(t.Metrics.Radius),
	}, theme.Component{Parts: p.Parts})
	if !p.Parts["group"].Border.IsSet() {
		group.Border = theme.Colour(border)
	}

	return Surface(c, group, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return InputProps{
					ComponentProps: box,
					Placeholder:    p.Placeholder,
					Kind:           kind,
					Disabled:       p.Disabled,
					Invalid:        p.Invalid,
				}.Layout(c.With(gtx), &state.Input)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = layout.Constraints{}.Min
				return layout.Inset{Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ButtonProps{Label: name, Icon: icon, IconOnly: true, Size: Icon, Variant: Ghost, Disabled: p.Disabled}.Layout(inner.With(gtx), &state.reveal)
				})
			}),
		)
	})
}
