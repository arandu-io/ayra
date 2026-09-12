package widget

import (
	"image"
	"time"

	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/semantic"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
)

// Clickable is the persistent input state of one or more clickable areas.
type Clickable struct {
	click   gesture.Click
	history []Press

	requestClicks int
	pressedKey    key.Name
}

// Click describes a completed pointer, keyboard, or programmatic activation.
type Click struct {
	// Modifiers are the keys held when the activation completed.
	Modifiers key.Modifiers
	// NumClicks is the number of activations represented by this event.
	NumClicks int
}

// Press records a pointer press for transient visual feedback.
type Press struct {
	// Position is where the pointer went down.
	Position image.Point
	// Start is when the press began.
	Start time.Time
	// End is when the press ended. Zero means it is still active.
	End time.Time
	// Cancelled reports that the gesture ended without activating the control.
	Cancelled bool
}

// Click queues one programmatic activation for the next update.
func (b *Clickable) Click() {
	b.requestClicks++
}

// Clicked updates the control and consumes one pending click report.
func (b *Clickable) Clicked(gtx layout.Context) bool {
	return b.clicked(b, gtx)
}

func (b *Clickable) clicked(t event.Tag, gtx layout.Context) bool {
	_, clicked := b.update(t, gtx)
	return clicked
}

// Hovered reports whether a pointer is over any registered area.
func (b *Clickable) Hovered() bool {
	return b.click.Hovered()
}

// Pressed reports whether a pointer is holding a registered area.
func (b *Clickable) Pressed() bool {
	return b.click.Pressed()
}

// History returns recent pointer presses for drawing transient feedback.
func (b *Clickable) History() []Press {
	return b.history
}

// Layout updates the state, lays out w, and registers its dimensions as an
// input area for the following frame.
func (b *Clickable) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return b.layout(b, gtx, w)
}

func (b *Clickable) layout(target event.Tag, gtx layout.Context, widget layout.Widget) layout.Dimensions {
	for {
		_, ok := b.update(target, gtx)
		if !ok {
			break
		}
	}
	recording := op.Record(gtx.Ops)
	dims := widget(gtx)
	drawing := recording.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	semantic.EnabledOp(gtx.Enabled()).Add(gtx.Ops)
	b.click.Add(gtx.Ops)
	event.Op(gtx.Ops, target)
	drawing.Add(gtx.Ops)
	return dims
}

// Update processes input and returns the next activation, if any.
func (b *Clickable) Update(gtx layout.Context) (Click, bool) {
	return b.update(b, gtx)
}

func (b *Clickable) update(target event.Tag, gtx layout.Context) (Click, bool) {
	b.expireHistory(gtx.Now)
	if requested, ok := b.takeRequestedClicks(); ok {
		return requested, true
	}
	if clicked, ok := b.pointerClick(gtx); ok {
		return clicked, true
	}
	return b.keyClick(target, gtx)
}

func (b *Clickable) expireHistory(now time.Time) {
	for len(b.history) > 0 {
		oldest := b.history[0]
		if oldest.End.IsZero() || now.Sub(oldest.End) < time.Second {
			return
		}
		copy(b.history, b.history[1:])
		b.history = b.history[:len(b.history)-1]
	}
}

func (b *Clickable) takeRequestedClicks() (Click, bool) {
	count := b.requestClicks
	if count == 0 {
		return Click{}, false
	}
	b.requestClicks = 0
	return Click{NumClicks: count}, true
}

func (b *Clickable) pointerClick(gtx layout.Context) (Click, bool) {
	for {
		incoming, ok := b.click.Update(gtx.Source)
		if !ok {
			return Click{}, false
		}
		switch incoming.Kind {
		case gesture.KindClick:
			b.finishLastPress(gtx.Now)
			return Click{
				Modifiers: incoming.Modifiers,
				NumClicks: incoming.NumClicks,
			}, true
		case gesture.KindCancel:
			b.cancelPresses(gtx.Now)
		case gesture.KindPress:
			b.history = append(b.history, Press{
				Position: incoming.Position,
				Start:    gtx.Now,
			})
		}
	}
}

func (b *Clickable) finishLastPress(now time.Time) {
	if last := len(b.history) - 1; last >= 0 {
		b.history[last].End = now
	}
}

func (b *Clickable) cancelPresses(now time.Time) {
	for i := range b.history {
		if !b.history[i].End.IsZero() {
			continue
		}
		b.history[i].Cancelled = true
		b.history[i].End = now
	}
}

func (b *Clickable) keyClick(target event.Tag, gtx layout.Context) (Click, bool) {
	for {
		incoming, ok := gtx.Event(
			key.FocusFilter{Target: target},
			key.Filter{Focus: target, Name: key.NameReturn},
			key.Filter{Focus: target, Name: key.NameSpace},
		)
		if !ok {
			return Click{}, false
		}
		switch incoming := incoming.(type) {
		case key.FocusEvent:
			if incoming.Focus {
				b.pressedKey = ""
			}
		case key.Event:
			if !gtx.Focused(target) || !isClickKey(incoming.Name) {
				continue
			}
			switch incoming.State {
			case key.Press:
				b.pressedKey = incoming.Name
			case key.Release:
				if b.pressedKey != incoming.Name {
					continue
				}
				b.pressedKey = ""
				return Click{
					Modifiers: incoming.Modifiers,
					NumClicks: 1,
				}, true
			}
		}
	}
}

func isClickKey(name key.Name) bool {
	return name == key.NameReturn || name == key.NameSpace
}
