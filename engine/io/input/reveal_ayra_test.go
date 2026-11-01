package input

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
)

// The on-screen keyboard takes the bottom of the window. The screen is laid out
// again in what is left, and a list in that room draws only the rows that fit,
// so the field a person tapped can be one it leaves out of the frame. These
// tests are that frame: they redraw a screen in a smaller room the way a window
// does, and ask what the router did with the focus, which is the thing that, lost,
// closes the keyboard and leaves the person typing into nothing.

// held is the control a screen draws while it is on the screen.
func drawField(f *frame, tag event.Tag, r image.Rectangle) {
	defer f.area(r)()
	f.listen(tag, key.FocusFilter{Target: tag})
}

// TestAHeldFocusSurvivesFramesWithoutItsControl fixes that a focus the window
// holds is not given up, and the keyboard is not closed, in the frames that omit
// the control -- and that the control is not told it lost the focus when it comes
// back.
func TestAHeldFocusSurvivesFramesWithoutItsControl(t *testing.T) {
	var s screen
	field := new(int)
	rect := image.Rect(0, 300, 100, 350)

	s.frame(func(f *frame) { drawField(f, field, rect) })
	s.r.Source().Execute(key.FocusCmd{Tag: field})
	s.r.Source().Execute(key.SoftKeyboardCmd{Show: true})
	s.frame(func(f *frame) { drawField(f, field, rect) })
	if !s.r.Source().Focused(field) {
		t.Fatal("the control asked for the focus and did not get it")
	}
	s.r.TextInputState() // the request to raise the keyboard is read once

	s.r.HoldFocus(3)
	for i := range 3 {
		s.frame(func(f *frame) {})
		if !s.r.Source().Focused(field) {
			t.Fatalf("frame %d without the control: the held focus was given up", i+1)
		}
		if got := s.r.TextInputState(); got == TextInputClose {
			t.Fatalf("frame %d without the control: the keyboard was closed", i+1)
		}
	}

	// It comes back, still the focus, and was never told otherwise.
	var told []event.Event
	s.frame(func(f *frame) {
		defer f.area(rect)()
		told = f.listen(field, key.FocusFilter{Target: field})
	})
	for _, e := range told {
		if fe, ok := e.(key.FocusEvent); ok && !fe.Focus {
			t.Errorf("the control was told it lost the focus on coming back: %v", e)
		}
	}
	if !s.r.Source().Focused(field) {
		t.Error("the focus was gone when the control came back")
	}
}

// TestAHeldFocusEndsWhenTheHoldDoes fixes that a control that is really gone
// still loses the focus, a few frames later than it used to.
func TestAHeldFocusEndsWhenTheHoldDoes(t *testing.T) {
	var s screen
	field := new(int)
	rect := image.Rect(0, 0, 100, 50)

	s.frame(func(f *frame) { drawField(f, field, rect) })
	s.r.Source().Execute(key.FocusCmd{Tag: field})
	s.frame(func(f *frame) { drawField(f, field, rect) })

	s.r.HoldFocus(2)
	s.frame(func(f *frame) {})
	s.frame(func(f *frame) {})
	if !s.r.Source().Focused(field) {
		t.Fatal("the focus was given up before the hold ended")
	}
	s.frame(func(f *frame) {})
	if s.r.Source().Focused(field) {
		t.Error("the focus stayed on a control that never came back")
	}
	if got := s.r.TextInputState(); got != TextInputClose {
		t.Errorf("the keyboard was left %v, want %v", got, TextInputClose)
	}
}

// TestReleaseFocusEndsTheHold fixes that a window that has brought the field into
// view does not go on holding a focus the screen may drop.
func TestReleaseFocusEndsTheHold(t *testing.T) {
	var s screen
	field := new(int)
	rect := image.Rect(0, 0, 100, 50)
	s.frame(func(f *frame) { drawField(f, field, rect) })
	s.r.Source().Execute(key.FocusCmd{Tag: field})
	s.frame(func(f *frame) { drawField(f, field, rect) })

	s.r.HoldFocus(8)
	s.r.ReleaseFocus()
	s.frame(func(f *frame) {})
	if s.r.Source().Focused(field) {
		t.Error("the focus was still held after it was released")
	}
}

// TestTabFromAHeldFocusDoesNotPanic fixes that moving the focus while the focused
// control is not drawn walks from the start rather than from an index that
// belongs to a frame that is gone.
func TestTabFromAHeldFocusDoesNotPanic(t *testing.T) {
	var s screen
	a, b := new(int), new(int)
	draw := func(f *frame, withA bool) {
		if withA {
			drawField(f, a, image.Rect(0, 0, 100, 50))
		}
		drawField(f, b, image.Rect(0, 100, 100, 150))
	}
	s.frame(func(f *frame) { draw(f, true) })
	s.r.Source().Execute(key.FocusCmd{Tag: a})
	s.frame(func(f *frame) { draw(f, true) })

	s.r.HoldFocus(3)
	s.frame(func(f *frame) { draw(f, false) })
	s.r.MoveFocus(key.FocusForward)
	s.r.MoveFocus(key.FocusBackward)
	s.r.MoveFocus(key.FocusDown)
	s.r.MoveFocus(key.FocusRight)
}

// scrolling is a list as the router sees one: an area that takes scroll, whose
// rows are drawn only while they are inside it, and whose range -- how far it can
// scroll -- is the one it measured in the frame before. That last part is the one
// that matters: a list that fitted its room last frame offers nothing to scroll by
// in the first frame in a smaller one.
type scrolling struct {
	tag      *int
	offset   int
	extent   int // the content's height
	measured int // how far it could scroll when it was last laid out
}

// frame draws the list in a room of the given height, with one field at y.
func (l *scrolling) frame(f *frame, room int, field event.Tag, y int) (drawn bool) {
	defer f.area(image.Rect(0, 0, 100, room))()
	for _, e := range f.listen(l.tag, pointer.Filter{
		Target:  l.tag,
		Kinds:   pointer.Scroll,
		ScrollY: pointer.ScrollRange{Min: -l.offset, Max: l.measured - l.offset},
	}) {
		if pe, ok := e.(pointer.Event); ok {
			l.offset += int(pe.Scroll.Y)
		}
	}
	top := y - l.offset
	if top >= 0 && top+50 <= room {
		drawField(f, field, image.Rect(0, top, 100, top+50))
		drawn = true
	}
	l.measured = max(0, l.extent-room)
	l.offset = min(l.offset, l.measured)
	return drawn
}

// TestRevealBringsAFieldThatTheSmallerRoomLeftOut walks the frames of a keyboard
// coming up under a field at the bottom of a screen that fitted: the first frame
// in the smaller room leaves the field out and cannot be scrolled by, and the
// keyboard has to stay up while the list learns how far it can go.
func TestRevealBringsAFieldThatTheSmallerRoomLeftOut(t *testing.T) {
	var s screen
	list := &scrolling{tag: new(int), extent: 500}
	field := new(int)
	const y = 400 // the field is at 400..450 of a screen 500 tall

	s.frame(func(f *frame) { list.frame(f, 500, field, y) })
	s.r.Source().Execute(key.FocusCmd{Tag: field})
	s.r.Source().Execute(key.SoftKeyboardCmd{Show: true})
	s.frame(func(f *frame) { list.frame(f, 500, field, y) })
	if !s.r.Source().Focused(field) {
		t.Fatal("the field did not take the focus")
	}
	s.r.TextInputState()

	// The keyboard comes up: the room is 200 tall.
	room := image.Rect(0, 0, 100, 200)
	s.r.BeginReveal(room, true)
	for i := 0; i < RevealFrames; i++ {
		var drawn bool
		s.frame(func(f *frame) { drawn = list.frame(f, 200, field, y) })
		if got := s.r.TextInputState(); got == TextInputClose {
			t.Fatalf("frame %d in the smaller room closed the keyboard", i+1)
		}
		if !s.r.Source().Focused(field) {
			t.Fatalf("frame %d in the smaller room (field drawn: %v) lost the focus", i+1, drawn)
		}
		if !s.r.ContinueReveal(room) && drawn {
			break
		}
	}

	var drawn bool
	s.frame(func(f *frame) { drawn = list.frame(f, 200, field, y) })
	if !drawn {
		t.Fatalf("the field was not brought into the room: list offset %d", list.offset)
	}
	if top := y - list.offset; top < 0 || top+50 > 200 {
		t.Errorf("the field is at %d..%d, outside the room of 200", top, top+50)
	}
	if !s.r.Source().Focused(field) {
		t.Error("the field lost the focus on the way")
	}
}
