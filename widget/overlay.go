package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Hover is the state half of something that appears when the pointer rests on
// it.
//
// It reports hovering and nothing else, which is the honest surface: a touch
// screen has no hover at all, so every control built on this has to work
// without it. What that means in practice is that a tooltip may add
// information and must never be the only place something is said.
type Hover struct {
	hover  gesture.Hover
	inside bool
}

// Hovering reports whether the pointer is resting on it.
func (h *Hover) Hovering() bool { return h.inside }

// TooltipProps is a short line that appears beside a control.
type TooltipProps struct {
	// Text is the line. One line: a tooltip long enough to wrap is help, and
	// help belongs under the control where it can be read without hovering.
	Text string
	// Below draws it under rather than over, for a control at the top of a
	// window where above is off the screen.
	Below bool
}

// Layout draws the control, and the tooltip over it while the pointer rests.
func (p TooltipProps) Layout(c ayra.Context, state *Hover, control ayra.Widget) ayra.Dimensions {
	dims := control(c)

	// The area is registered after the control is drawn and over exactly its
	// size, so a tooltip belongs to what it describes rather than to the row
	// that happens to contain it.
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(c.Ops).Pop()
	state.hover.Add(c.Ops)
	event.Op(c.Ops, state)
	state.inside = state.hover.Update(c.Source)

	if !state.inside || p.Text == "" {
		return dims
	}

	// Recorded and replayed at the end of the frame would be better, and this
	// package has no such pass: drawn here, the tooltip is over everything
	// already drawn and under everything after it, which is right for a control
	// in a row and wrong for one behind a later panel.
	bubble := op.Record(c.Ops)
	size := p.draw(c)
	drawn := bubble.Stop()

	top := -size.Y - c.Dp(unit.Dp(6))
	if p.Below {
		top = dims.Size.Y + c.Dp(unit.Dp(6))
	}
	offset := op.Offset(image.Pt(0, top)).Push(c.Ops)
	drawn.Add(c.Ops)
	offset.Pop()

	return dims
}

// draw paints the bubble and answers its size.
func (p TooltipProps) draw(c ayra.Context) image.Point {
	inner := c
	inner.Constraints.Min = image.Point{}

	return surface(inner, inner.Theme.Colours.Foreground, inner.Theme.Colours.Foreground, c.Dp(unit.Dp(6)), func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return drawText(c.With(gtx), p.Text, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.Background, 1, text.Start, plain())
		})
	}).Size
}

// Toasts is the state half of a stack of messages: which ones are showing, and
// the press that dismisses each.
//
// The queue is held here rather than by the screen because the rule about how
// many are visible at once belongs to the control: a screen that pushed six
// would otherwise draw six, and the sixth would be off the bottom of the
// window.
type Toasts struct {
	messages []Toast
	dismiss  []gesture.Click
}

// Toast is one message.
type Toast struct {
	// Title is the line.
	Title string
	// Severity is how it should read.
	Severity Severity
}

// Show adds a message.
//
// The oldest is dropped once there are more than three, and three is the point
// at which a stack stops being readable: a person who is shown four at once
// reads none of them.
func (t *Toasts) Show(message Toast) {
	t.messages = append(t.messages, message)
	for len(t.messages) > 3 {
		t.messages = t.messages[1:]
	}
	for len(t.dismiss) < len(t.messages) {
		t.dismiss = append(t.dismiss, gesture.Click{})
	}
}

// Showing is how many are visible.
func (t *Toasts) Showing() int { return len(t.messages) }

// ToastProps is where the stack sits.
type ToastProps struct {
	// Width is how wide a message may get. Zero takes one that fits a sentence.
	Width unit.Dp
}

// Layout draws the stack in the corner of the room it was given.
func (p ToastProps) Layout(c ayra.Context, state *Toasts) ayra.Dimensions {
	if len(state.messages) == 0 {
		return ayra.Dimensions{}
	}

	// A press on one removes it. Read before drawing, so a message dismissed
	// this frame does not appear again in it.
	for index := len(state.messages) - 1; index >= 0; index-- {
		if index >= len(state.dismiss) {
			continue
		}
		for {
			if _, ok := state.dismiss[index].Update(c.Source); !ok {
				break
			}
			state.messages = append(state.messages[:index], state.messages[index+1:]...)
			break
		}
	}
	if len(state.messages) == 0 {
		return ayra.Dimensions{}
	}

	width := p.Width
	if width == 0 {
		width = 320
	}

	children := make([]layout.FlexChild, 0, len(state.messages)*2)
	for index, message := range state.messages {
		index, message := index, message
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: 8}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			if maximum := inner.Dp(width); inner.Constraints.Max.X > maximum {
				inner.Constraints.Max.X = maximum
			}
			inner.Constraints.Min.X = inner.Constraints.Max.X

			measure := op.Record(inner.Ops)
			dims := AlertProps{Title: message.Title, Severity: message.Severity}.Layout(inner)
			drawn := measure.Stop()

			defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(inner.Ops).Pop()
			if index < len(state.dismiss) {
				state.dismiss[index].Add(inner.Ops)
				event.Op(inner.Ops, &state.dismiss[index])
			}
			drawn.Add(inner.Ops)
			return dims
		}))
	}

	return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(c.Context, children...)
}
