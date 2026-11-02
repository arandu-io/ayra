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
	held   bool
}

// Hovering reports whether the pointer is resting on it, or it is held.
func (h *Hover) Hovering() bool { return h.inside || h.held }

// Hold keeps what appears on hover showing without a pointer, and Hold(false)
// lets go. It is for a screen that shows the tooltip on a keyboard focus or on a
// long press, and for a picture of one: a pointer cannot be drawn.
func (h *Hover) Hold(held bool) { h.held = held }

// TooltipProps is a short line that appears beside a control.
type TooltipProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Text is the line. One line: a tooltip long enough to wrap is help, and
	// help belongs under the control where it can be read without hovering.
	Text string
	// Below draws it under rather than over, for a control at the top of a
	// window where above is off the screen.
	Below bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p TooltipProps) PartNames() []string { return []string{"root"} }

// Layout draws the control, and the tooltip over it while the pointer rests.
func (p TooltipProps) Layout(c ayra.Context, state *Hover, control ayra.Widget) ayra.Dimensions {
	return withRoot(c, "tooltip", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state, control)
	})
}

// layout is Layout without the root part around it.
func (p TooltipProps) layout(c ayra.Context, state *Hover, control ayra.Widget) ayra.Dimensions {
	dims := control(c)

	// The area is registered after the control is drawn and over exactly its
	// size, so a tooltip belongs to what it describes rather than to the row
	// that happens to contain it.
	//
	// The clip is closed before the bubble is drawn. The bubble sits outside the
	// control's rectangle, above or below it, and drawn inside the clip it was
	// cut away entirely: a tooltip that never showed.
	area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(c.Ops)
	state.hover.Add(c.Ops)
	event.Op(c.Ops, state)
	area.Pop()
	state.inside = state.hover.Update(c.Source)

	if !state.Hovering() || p.Text == "" {
		return dims
	}

	// Deferred to the end of the frame, so the bubble is over what comes after the
	// control -- the next button of a row drew over it -- and placed with the
	// window in view: kept inside its sides, and under the control when there is
	// no room over it.
	bubble := op.Record(c.Ops)
	size := p.draw(c)
	drawn := bubble.Stop()

	gap := c.Dp(unit.Dp(6))
	left := 0
	below := p.Below
	if c.Overlay != nil {
		if bounds, ok := c.Source.Bounds(state); ok {
			bounds = bounds.Add(c.Overlay.Window.Min)
			safe := c.Overlay.Safe
			edge := c.Dp(unit.Dp(8))
			if !below && bounds.Min.Y-gap-size.Y < safe.Min.Y {
				below = true
			}
			if bounds.Min.X+left+size.X > safe.Max.X-edge {
				left = safe.Max.X - edge - size.X - bounds.Min.X
			}
			if bounds.Min.X+left < safe.Min.X+edge {
				left = safe.Min.X + edge - bounds.Min.X
			}
		}
	}
	top := -size.Y - gap
	if below {
		top = dims.Size.Y + gap
	}
	shown := op.Record(c.Ops)
	offset := op.Offset(image.Pt(left, top)).Push(c.Ops)
	drawn.Add(c.Ops)
	offset.Pop()
	op.Defer(c.Ops, shown.Stop())

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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Width is how wide a message may get. Zero takes one that fits a sentence.
	Width unit.Dp
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p ToastProps) PartNames() []string { return []string{"root"} }

// Layout draws the stack in the corner of the room it was given.
func (p ToastProps) Layout(c ayra.Context, state *Toasts) ayra.Dimensions {
	return withRoot(c, "toast", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p ToastProps) layout(c ayra.Context, state *Toasts) ayra.Dimensions {
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
			dims := AlertProps{Title: message.Title, Severity: message.Severity, endRoom: innerSide + 8}.Layout(inner)
			// Kyse's close button is an X in the corner, and pressing anywhere on
			// the message closes it: the mark says so.
			side := innerPixels(inner, innerSide)
			margin := inner.Dp(unit.Dp(12))
			at := op.Offset(image.Pt(dims.Size.X-margin-side, margin)).Push(inner.Ops)
			drawInner(inner, innerIcon("toast", "X"), side, inner.Theme.Colours.MutedForeground, 0)
			at.Pop()
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
