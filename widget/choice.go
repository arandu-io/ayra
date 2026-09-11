package widget

import (
	"image"
	"strconv"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Select is the state half of a control that picks one of a list.
type Select struct {
	open     Disclosure
	options  []Button
	selected int
}

// Selected is the index chosen, or -1 when nothing has been.
func (s *Select) Selected() int {
	if s.selected == 0 && len(s.options) == 0 {
		return -1
	}
	return s.selected
}

// Choose picks one, for the state a screen arrives with.
func (s *Select) Choose(index int) { s.selected = index }

// Showing reports whether the list is open.
func (s *Select) Showing() bool { return s.open.Showing() }

// SelectProps is a control that picks one of a short list.
//
// Short, and the type says so: a list long enough to need searching is an
// [Autocomplete]. A select that grew a search field would be that control under
// this name, and a screen that needed it would find two controls doing one job.
type SelectProps struct {
	// Options are the choices, in order.
	Options []string
	// Placeholder is shown while nothing is chosen.
	Placeholder string
	// Disabled draws it as unavailable and refuses to open.
	Disabled bool
	// Invalid draws it as rejected.
	Invalid bool
}

// Layout draws the control and, while it is open, the list under it.
func (p SelectProps) Layout(c ayra.Context, state *Select) ayra.Dimensions {
	for len(state.options) < len(p.Options) {
		state.options = append(state.options, Button{})
	}

	if !p.Disabled {
		state.open.Changed(c)
	}
	for index := range p.Options {
		if state.options[index].Clicked(c) {
			state.selected = index
			state.open.Close()
		}
	}

	dims := p.closed(c, state)
	if state.open.Showing() && !p.Disabled {
		p.list(c, state, dims)
	}
	return dims
}

// closed draws the control itself: the chosen value and the marker.
func (p SelectProps) closed(c ayra.Context, state *Select) ayra.Dimensions {
	label, ink := p.Placeholder, c.Theme.Colours.MutedForeground
	if state.selected >= 0 && state.selected < len(p.Options) {
		label, ink = p.Options[state.selected], c.Theme.Colours.Foreground
	}
	if p.Disabled {
		ink = fade(ink)
	}

	border := c.Theme.Colours.Border
	if p.Invalid {
		border = c.Theme.Colours.Destructive
	}

	return state.open.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		return surface(inner, inner.Theme.Colours.Background, border, controlRadius(inner), func(c ayra.Context) ayra.Dimensions {
			return layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				row := c.With(gtx)
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(row.Context,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return drawText(row.With(gtx), label, unit.Sp(c.Theme.Type.Body), ink, 1, text.Start, plain())
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return marker(row.With(gtx), state.open.Showing())
					}),
				)
			})
		})
	})
}

// list draws the options under the control.
func (p SelectProps) list(c ayra.Context, state *Select, closed ayra.Dimensions) {
	children := make([]layout.FlexChild, 0, len(p.Options))
	for index, option := range p.Options {
		index, option := index, option
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return ItemProps{Title: option, Pressable: true, Selected: index == state.selected}.
				Layout(inner, &state.options[index], nil)
		}))
	}

	panel := op.Record(c.Ops)
	inner := c
	inner.Constraints.Min.X = closed.Size.X
	inner.Constraints.Max.X = closed.Size.X

	size := surface(inner, inner.Theme.Colours.Popover, inner.Theme.Colours.Border, controlRadius(inner), func(c ayra.Context) ayra.Dimensions {
		return layout.UniformInset(4).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(c.With(gtx).Context, children...)
		})
	}).Size
	drawn := panel.Stop()

	offset := op.Offset(image.Pt(0, closed.Size.Y+c.Dp(unit.Dp(4)))).Push(c.Ops)
	defer clip.Rect(image.Rectangle{Max: size}).Push(c.Ops).Pop()
	drawn.Add(c.Ops)
	offset.Pop()
}

// Stepper is the state half of a number field: the value, and the two steps.
type Stepper struct {
	value int
	down  Button
	up    Button
}

// Value is the number.
func (s *Stepper) Value() int { return s.value }

// SetValue writes it, for the state a screen arrives with.
func (s *Stepper) SetValue(value int) { s.value = value }

// NumberProps is a field for a whole number with two buttons.
//
// Buttons rather than a keyboard alone, because on a phone the alternative is
// the numeric keypad for a value most people change by one. It clamps rather
// than refusing: a control that let somebody type past its own maximum and
// complained afterwards is a control that wasted the typing.
type NumberProps struct {
	// Min and Max bound it. Both zero leaves it unbounded below and above.
	Min, Max int
	// Step is how much a press changes it. Zero is one.
	Step int
	// Disabled draws it as unavailable.
	Disabled bool
}

// clamp holds the value inside the bounds the props declare.
//
// A control that let somebody reach a value and complained afterwards wasted
// the press; one that kept counting past its own maximum is a form that submits
// a number the server rejects. Both bounds at zero means unbounded, which is
// what a caller who filled in neither meant.
func (p NumberProps) clamp(state *Stepper) {
	if p.Min == 0 && p.Max == 0 {
		return
	}
	state.value = min(max(state.value, p.Min), p.Max)
}

// Layout draws the value between its two buttons.
func (p NumberProps) Layout(c ayra.Context, state *Stepper) ayra.Dimensions {
	step := p.Step
	if step == 0 {
		step = 1
	}

	if !p.Disabled {
		if state.down.Clicked(c) {
			state.value -= step
		}
		if state.up.Clicked(c) {
			state.value += step
		}
	}
	p.clamp(state)

	atFloor := (p.Min != 0 || p.Max != 0) && state.value <= p.Min
	atCeiling := (p.Min != 0 || p.Max != 0) && state.value >= p.Max

	c.Constraints.Min = image.Point{}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return ButtonProps{Label: "-", Variant: Outline, Size: Small, Disabled: p.Disabled || atFloor}.Layout(inner, &state.down)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				ink := c.Theme.Colours.Foreground
				if p.Disabled {
					ink = fade(ink)
				}
				return drawText(inner.With(gtx), strconv.Itoa(state.value), unit.Sp(c.Theme.Type.Body), ink, 1, text.Middle, mono())
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return ButtonProps{Label: "+", Variant: Outline, Size: Small, Disabled: p.Disabled || atCeiling}.Layout(inner, &state.up)
		}),
	)
}
