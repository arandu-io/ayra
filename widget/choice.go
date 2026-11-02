package widget

import (
	"image"
	"strconv"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Select is the state half of a control that picks one of a list.
type Select struct {
	open    Disclosure
	options []Button
	// selected is the index picked, and chosen says whether one was.
	//
	// Two fields rather than a sentinel in one, because zero is an option: the
	// first. A control that stored "nothing picked" as zero answered with the
	// first option from the frame it was drawn, so the placeholder was never
	// reached and a form arrived already filled in with a value nobody chose --
	// which on a submit is the wrong answer sent without anybody being asked.
	selected int
	chosen   bool
	// scroll is where the list of options is, when there are more of them than
	// fit.
	scroll layout.List
	// floating places the list beside the control: over it when there is no room
	// under it, and inside the window.
	floating
}

// Selected is the index chosen, and -1 when nothing has been.
//
// A choice past the end of the options it was last drawn with is no choice,
// and answers -1 from that frame on.
func (s *Select) Selected() int {
	if !s.chosen {
		return -1
	}
	return s.selected
}

// Choose picks one, for the state a screen arrives with.
func (s *Select) Choose(index int) { s.selected, s.chosen = index, true }

// Open puts the list up, as a press on the control does, for a screen that
// restores it and for a picture of one open.
func (s *Select) Open() { s.open.Open() }

// Showing reports whether the list is open.
func (s *Select) Showing() bool { return s.open.Showing() }

// SelectProps is a control that picks one of a short list.
//
// Short, and the type says so: a list long enough to need searching is an
// [Autocomplete]. A select that grew a search field would be that control under
// this name, and a screen that needed it would find two controls doing one job.
type SelectProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Options are the choices, in order.
	Options []string
	// Placeholder is shown while nothing is chosen.
	Placeholder string
	// Disabled draws it as unavailable and refuses to open.
	Disabled bool
	// Invalid draws it as rejected.
	Invalid bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p SelectProps) PartNames() []string { return []string{"root"} }

// Layout draws the control and, while it is open, the list under it.
func (p SelectProps) Layout(c ayra.Context, state *Select) ayra.Dimensions {
	return withRoot(c, "select", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p SelectProps) layout(c ayra.Context, state *Select) ayra.Dimensions {
	for len(state.options) < len(p.Options) {
		state.options = append(state.options, Button{})
	}
	if state.chosen && (state.selected < 0 || state.selected >= len(p.Options)) {
		// A choice the options no longer reach -- the list got shorter under
		// it -- is no choice. The control draws the placeholder for it, and
		// Selected answering the old index would submit a value the screen
		// says nobody picked.
		state.selected, state.chosen = 0, false
	}

	if !p.Disabled {
		state.open.Changed(c)
		state.dismiss(c, &state.open, true, true)
	}
	for index := range p.Options {
		if state.options[index].Clicked(c) {
			state.selected, state.chosen = index, true
			state.open.Close()
		}
	}

	dims := p.closed(c, state)
	state.declareAnchor(c, dims)
	if state.open.Showing() && !p.Disabled {
		p.list(c, state, dims)
	}
	return dims
}

// closed draws the control itself: the chosen value and the caret.
func (p SelectProps) closed(c ayra.Context, state *Select) ayra.Dimensions {
	label, picked := p.label(state)
	ink := c.Theme.Colours.MutedForeground
	if picked {
		ink = c.Theme.Colours.Foreground
	}
	caret := c.Theme.Colours.MutedForeground
	if p.Disabled {
		ground := c.Theme.Colours.Background
		ink, caret = fadeOver(ink, ground), fadeOver(caret, ground)
	}

	// The border Kyse's select has is the input colour, as the text field's is, and
	// the ring colour on the keyboard focus, with the ring around it.
	border := c.Theme.Colours.Input
	if p.Invalid {
		border = c.Theme.Colours.Destructive
	}
	focused := !p.Disabled && c.Focused(&state.open.click)
	if focused {
		border = c.Theme.Colours.Ring
	}
	fill := fieldFill(c.Theme).NRGBA()
	if p.Disabled {
		fill = fadeOver(fill, c.Theme.Colours.Background)
		border = fadeOver(border, c.Theme.Colours.Background)
	}

	// The area that answers is grown to a finger; the field is drawn as it was.
	dims := TouchArea(c, func(c ayra.Context, w ayra.Widget) ayra.Dimensions {
		return state.open.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions { return w(c.With(gtx)) })
	}, func(inner ayra.Context) ayra.Dimensions {
		return surface(inner, fill, border, controlRadius(inner), func(c ayra.Context) ayra.Dimensions {
			return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				row := c.With(gtx)
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(row.Context,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return drawText(row.With(gtx), label, unit.Sp(c.Theme.Type.Body), ink, 1, text.Start, plain())
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// Kyse's native select draws the chevron from the stylesheet, and
						// the nearest in its icon set is CaretDown, which does not turn.
						return drawInner(row.With(gtx), innerIcon("select", "CaretDown"), innerPixels(row, innerSide), caret, 0)
					}),
				)
			})
		})
	})
	if focused {
		focusRing(c, dims.Size, controlRadius(c), c.Theme.Colours.Ring)
	}
	return dims
}

// label answers the text on the closed control, and whether it is a value
// somebody chose rather than the placeholder.
//
// It is a function of its own because it is the whole of one decision, and the
// decision was wrong: it asked only whether the index was in range, and zero is
// always in range -- so the placeholder was never reached and a form arrived
// filled in with the first option. A decision written inline is a decision no
// test can ask about without drawing the control and reading pixels.
func (p SelectProps) label(state *Select) (string, bool) {
	if state.chosen && state.selected >= 0 && state.selected < len(p.Options) {
		return p.Options[state.selected], true
	}
	return p.Placeholder, false
}

// listRoom is the most the list of options is tall, in dp: eight rows. More than
// that is a list that runs off the bottom of the window, whose last options
// nobody can reach, so it scrolls inside this height instead.
const listRoom = 296

// list draws the options beside the control.
func (p SelectProps) list(c ayra.Context, state *Select, closed ayra.Dimensions) {
	state.scroll.Axis = layout.Vertical

	inner := c
	inner.Constraints.Min.X = closed.Size.X
	inner.Constraints.Max.X = closed.Size.X
	if c.Overlay != nil {
		// The room the parent left under the control is not the list's: it floats
		// over what follows.
		inner.Constraints.Max.Y = c.Overlay.Safe.Dy()
	}

	build := func(maxHeight int) (op.CallOp, image.Point) {
		// At most eight rows, and less when the window has less room.
		most := c.Dp(unit.Dp(listRoom))
		if maxHeight > 0 {
			most = min(most, maxHeight)
		}
		recording := op.Record(c.Ops)
		size := surface(inner, inner.Theme.Colours.Popover, inner.Theme.Colours.Border, controlRadius(inner), func(c ayra.Context) ayra.Dimensions {
			return layout.UniformInset(4).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, most)
				return state.scroll.Layout(gtx, len(p.Options), func(gtx layout.Context, index int) layout.Dimensions {
					inner := c.With(gtx)
					return ItemProps{Title: p.Options[index], Pressable: true, Selected: state.chosen && index == state.selected}.
						Layout(inner, &state.options[index], nil)
				})
			})
		}).Size
		return recording.Stop(), size
	}

	// Put off to the end of the frame by float: drawn where the control is, the
	// list went under every control after it, and a select in the middle of a
	// form offered a list nobody could choose from.
	state.float(c, closed.Size, placement{gap: c.Dp(unit.Dp(4)), barrier: true}, build)
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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Min and Max bound it. Both zero leaves it unbounded below and above.
	Min, Max int
	// Step is how much a press changes it. Zero is one.
	Step int
	// DecrementLabel and IncrementLabel name the two buttons for a screen
	// reader, which is all that names them: they draw an icon. Empty says
	// "Decrease" and "Increase", as Kyse's do.
	DecrementLabel, IncrementLabel string
	// Disabled draws it as unavailable.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p NumberProps) PartNames() []string { return []string{"root"} }

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
	return withRoot(c, "number", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p NumberProps) layout(c ayra.Context, state *Stepper) ayra.Dimensions {
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
			return innerButton(inner, &state.down, innerIcon("number-input", "Minus"), orDefault(p.DecrementLabel, "Decrease"), Outline, p.Disabled || atFloor)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				ink := c.Theme.Colours.Foreground
				if p.Disabled {
					ink = fadeOver(ink, c.Theme.Colours.Background)
				}
				return drawText(inner.With(gtx), strconv.Itoa(state.value), unit.Sp(c.Theme.Type.Body), ink, 1, text.Middle, mono())
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return innerButton(inner, &state.up, innerIcon("number-input", "Plus"), orDefault(p.IncrementLabel, "Increase"), Outline, p.Disabled || atCeiling)
		}),
	)
}
