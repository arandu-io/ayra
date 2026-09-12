package input

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
)

// TestFocusGoesToWhoeverAsked is the plainest thing the focus has to do, and
// the one every other test here is measured against.
func TestFocusGoesToWhoeverAsked(t *testing.T) {
	r := new(Router)
	h := new(int)
	focus := key.FocusFilter{Target: h}

	assertEventSequence(t, events(r, 1, focus), key.FocusEvent{Focus: false})

	r.Source().Execute(key.FocusCmd{Tag: h})

	assertEventSequence(t, events(r, -1, focus), key.FocusEvent{Focus: true})
	assertFocus(t, r, h)
}

// TestFocusReturnsToNothingWhenTheOwnerGoes drives two real frames: one that
// draws the focused handler and one that does not. The handler still asks for
// focus events in both, so what removes the focus is its absence from the
// frame and nothing else.
func TestFocusReturnsToNothingWhenTheOwnerGoes(t *testing.T) {
	r := new(Router)
	h := new(int)
	focus := key.FocusFilter{Target: h}

	events(r, -1, focus)
	drawn := new(op.Ops)
	event.Op(drawn, h)
	r.Frame(drawn)

	r.Source().Execute(key.FocusCmd{Tag: h})
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})
	assertEventSequence(t, events(r, -1, focus), key.FocusEvent{Focus: true})

	// A frame that draws the handler keeps the focus where it is.
	events(r, -1, focus)
	drawn.Reset()
	event.Op(drawn, h)
	r.Frame(drawn)
	assertFocus(t, r, h)

	// A frame without it takes the focus back, and shuts the keyboard that was
	// opened for it -- a keyboard standing open for a control that is no longer
	// on the screen covers half of what replaced it.
	events(r, -1, focus)
	r.Frame(new(op.Ops))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)
}

// TestOnlyOneHandlerHoldsFocus checks the losing side as well as the winning
// one. A handler that is never told it lost keeps drawing a caret.
func TestOnlyOneHandlerHoldsFocus(t *testing.T) {
	r := new(Router)
	first, second := new(int), new(int)
	firstFocus := key.FocusFilter{Target: first}
	secondFocus := key.FocusFilter{Target: second}

	assertEventSequence(t, events(r, 1, firstFocus), key.FocusEvent{Focus: false})
	assertEventSequence(t, events(r, 1, secondFocus), key.FocusEvent{Focus: false})

	r.Source().Execute(key.FocusCmd{Tag: first})
	assertEventSequence(t, events(r, 1, firstFocus), key.FocusEvent{Focus: true})
	assertFocus(t, r, first)

	r.Source().Execute(key.FocusCmd{Tag: second})
	assertEventSequence(t, events(r, 1, firstFocus), key.FocusEvent{Focus: false})
	assertEventSequence(t, events(r, 1, secondFocus), key.FocusEvent{Focus: true})
	assertFocus(t, r, second)
	if r.Source().Focused(first) {
		t.Error("the handler that lost the focus still reports holding it")
	}
}

// TestFocusOrderFollowsDeclarationOrder puts the handlers on the screen in the
// reverse of the order they are declared in, so that an implementation walking
// the screen instead of the frame gets a different answer from the right one.
//
// Forward and backward are the order the frame was written in: that is the
// order the author of the screen can see, and the only one they control.
func TestFocusOrderFollowsDeclarationOrder(t *testing.T) {
	r := new(Router)
	ops := new(op.Ops)
	handlers := []*int{new(int), new(int), new(int)}
	bounds := []image.Rectangle{
		image.Rect(0, 200, 100, 280),
		image.Rect(0, 100, 100, 180),
		image.Rect(0, 0, 100, 80),
	}

	for i, h := range handlers {
		area := clip.Rect(bounds[i]).Push(ops)
		event.Op(ops, h)
		area.Pop()
		events(r, -1, key.FocusFilter{Target: h})
	}
	r.Frame(ops)

	for _, want := range []*int{handlers[0], handlers[1], handlers[2], handlers[0]} {
		r.MoveFocus(key.FocusForward)
		assertFocus(t, r, want)
	}
	for _, want := range []*int{handlers[2], handlers[1], handlers[0]} {
		r.MoveFocus(key.FocusBackward)
		assertFocus(t, r, want)
	}
}
