package input

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
)

// The tests here drive the router the way a window does, in frames.
//
// That is not a stylistic preference. Half of what the router does happens at
// the frame boundary and nowhere else: a claim on a pointer is granted there, a
// handler that stopped being declared is forgotten there, and the focus is
// taken away from a control that is no longer on the screen there. A test that
// queues a pile of events and drains them once never crosses that boundary, so
// it exercises the delivery half of the machine and none of the bookkeeping
// half -- and the bookkeeping half is the one whose faults reach a person, as a
// button that stays lit or a caret on a field that is gone.
//
// So every test below redeclares its areas and its filters on every frame,
// which is what a screen does, and asks what arrived after each one.

// screen is a router plus the operation list a frame is drawn into.
type screen struct {
	r   Router
	ops op.Ops
}

// frame draws one frame and closes it.
//
// The body is the screen: it pushes areas, declares what it listens for and
// reads what arrived since the last frame. Closing the frame is this
// function's, not the body's, because a frame that a test forgets to close is
// a test that passes for the wrong reason.
func (s *screen) frame(body func(f *frame)) {
	s.ops.Reset()
	body(&frame{s: s})
	s.r.Frame(&s.ops)
}

// frame is one pass over a screen.
type frame struct {
	s *screen
}

// area pushes a rectangle that the controls declared inside it listen over, and
// returns what closes it.
func (f *frame) area(r image.Rectangle) func() {
	return clip.Rect(r).Push(&f.s.ops).Pop
}

// listen declares a control: it takes everything waiting for the given filters
// and registers the tag over the area in force.
//
// Reading before registering is the order a control uses -- what it draws this
// frame depends on what happened last frame -- and the order matters to the
// router, because the filters a call declares are the ones that route the next
// frame's events.
func (f *frame) listen(tag event.Tag, filters ...event.Filter) []event.Event {
	var got []event.Event
	for {
		e, ok := f.s.r.Event(filters...)
		if !ok {
			break
		}
		got = append(got, e)
	}
	event.Op(&f.s.ops, tag)
	return got
}

// pointerKinds names the kinds of a run of pointer events, for a failure
// message that says what arrived rather than how many.
func pointerKinds(evts []event.Event) []pointer.Kind {
	kinds := make([]pointer.Kind, 0, len(evts))
	for _, e := range evts {
		if pe, ok := e.(pointer.Event); ok {
			kinds = append(kinds, pe.Kind)
		}
	}
	return kinds
}

func sameKinds(got []pointer.Kind, want ...pointer.Kind) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// press queues a press and a release at one point, as a finger does.
func press(r *Router, at f32.Point) {
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at},
		pointer.Event{Kind: pointer.Release, Position: at},
	)
}

// TestRouterKeyboardReachesOneControl fixes that what an input method sends
// goes to the focused control and to no other.
//
// The event carries no target of its own -- it says "replace this text", and
// which field is meant is whatever has the keyboard. A second delivery is
// therefore not a duplicate notification but a second edit, applied to a field
// the person is not typing into.
func TestRouterKeyboardReachesOneControl(t *testing.T) {
	var s screen
	first, second := new(int), new(int)

	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 50))()
		f.listen(first, key.FocusFilter{Target: first})
		f.listen(second, key.FocusFilter{Target: second})
	})
	s.r.Source().Execute(key.FocusCmd{Tag: first})

	s.r.Queue(key.EditEvent{Text: "a"})

	var firstGot, secondGot []event.Event
	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 50))()
		firstGot = f.listen(first, key.FocusFilter{Target: first})
		secondGot = f.listen(second, key.FocusFilter{Target: second})
	})

	edits := 0
	for _, e := range firstGot {
		if _, ok := e.(key.EditEvent); ok {
			edits++
		}
	}
	if edits != 1 {
		t.Errorf("the focused control received %d edits, want exactly 1; got %#v", edits, firstGot)
	}
	for _, e := range secondGot {
		if _, ok := e.(key.EditEvent); ok {
			t.Errorf("an unfocused control received an edit: %#v", e)
		}
	}
}

// TestRouterFocusEndsWithTheControl fixes that the focus goes back to nothing
// when the control holding it stops being drawn.
//
// A focus left pointing at a control that is no longer on the screen is a
// keyboard that types into nothing: the platform is still told to keep the soft
// keyboard up, and every key pressed is routed to a handler the router is about
// to forget.
func TestRouterFocusEndsWithTheControl(t *testing.T) {
	var s screen
	field := new(int)

	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 50))()
		f.listen(field, key.FocusFilter{Target: field})
	})
	s.r.Source().Execute(key.FocusCmd{Tag: field})

	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 50))()
		f.listen(field, key.FocusFilter{Target: field})
	})
	if !s.r.Source().Focused(field) {
		t.Fatal("the control asked for the focus and did not get it")
	}

	// A frame where the control is not drawn at all.
	s.frame(func(f *frame) {})

	if s.r.Source().Focused(field) {
		t.Error("the focus stayed on a control that left the screen")
	}
	if got, want := s.r.TextInputState(), TextInputClose; got != want {
		t.Errorf("the keyboard was left %v, want %v: a focus that ends has to close it", got, want)
	}
}

// TestRouterForgetsAControlThatMissedAFrame fixes that a tag absent for one
// frame loses everything the router remembered about it.
//
// The state a control keeps here is what the router knows and the control does
// not: whether it has been told to reset. A control returning after an absence
// is a new control as far as the screen is concerned -- it may be a different
// row of a list reusing the same tag -- so it is told to drop whatever it
// thought it was in the middle of, exactly as it was on its first frame.
func TestRouterForgetsAControlThatMissedAFrame(t *testing.T) {
	var s screen
	tag := new(int)
	filter := pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Cancel}

	var first, again []event.Event
	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 50))()
		first = f.listen(tag, filter)
	})
	if !sameKinds(pointerKinds(first), pointer.Cancel) {
		t.Fatalf("a control's first frame got %v, want one Cancel to start from", pointerKinds(first))
	}

	// A frame in which the control is not declared. The router has nothing to
	// keep the state alive for.
	s.frame(func(f *frame) {})

	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 50))()
		again = f.listen(tag, filter)
	})
	if !sameKinds(pointerKinds(again), pointer.Cancel) {
		t.Errorf("a control that missed a frame got %v on its return, want the Cancel a first frame gets", pointerKinds(again))
	}

	// And a control that was there all along is told once and not again.
	var third []event.Event
	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 50))()
		third = f.listen(tag, filter)
	})
	if kinds := pointerKinds(third); len(kinds) != 0 {
		t.Errorf("a control present on consecutive frames got %v, want nothing", kinds)
	}
}

// TestRouterHitSearchResumesAtTheParent fixes the shape of the search that
// decides who is under the pointer.
//
// Two things are being fixed at once, and they are the two halves of one walk.
// Siblings are tried in turn, so a press that misses the one on top is offered
// to the one beneath it. And a sibling that does match ends the search among
// siblings and resumes at its parent, so the container is told about a press on
// what it contains -- which is what lets a list scroll under a row -- while the
// sibling the press was not over hears nothing.
//
// Without the resumption a container would only ever hear about presses on its
// own bare ground, and a row would have to tell its list by hand.
func TestRouterHitSearchResumesAtTheParent(t *testing.T) {
	var s screen
	container, left, right := new(int), new(int), new(int)

	kinds := pointer.Press | pointer.Release | pointer.Cancel
	draw := func(f *frame) (c, l, r []event.Event) {
		defer f.area(image.Rect(0, 0, 100, 100))()
		c = f.listen(container, pointer.Filter{Target: container, Kinds: kinds})
		func() {
			defer f.area(image.Rect(0, 0, 60, 100))()
			l = f.listen(left, pointer.Filter{Target: left, Kinds: kinds})
		}()
		func() {
			defer f.area(image.Rect(40, 0, 100, 100))()
			r = f.listen(right, pointer.Filter{Target: right, Kinds: kinds})
		}()
		return c, l, r
	}

	s.frame(func(f *frame) { draw(f) })

	// A point the two children overlap on. The one drawn last is on top.
	press(&s.r, f32.Pt(50, 50))

	var gotContainer, gotLeft, gotRight []event.Event
	s.frame(func(f *frame) { gotContainer, gotLeft, gotRight = draw(f) })

	if !sameKinds(pointerKinds(gotRight), pointer.Press, pointer.Release) {
		t.Errorf("the control on top got %v, want a press and a release", pointerKinds(gotRight))
	}
	if !sameKinds(pointerKinds(gotContainer), pointer.Press, pointer.Release) {
		t.Errorf("the container got %v, want the press its child got: the search resumes at the parent", pointerKinds(gotContainer))
	}
	if kinds := pointerKinds(gotLeft); len(kinds) != 0 {
		t.Errorf("the covered sibling got %v, want nothing: a match ends the search among siblings", kinds)
	}

	// A point only the lower sibling covers. The search has to walk past the
	// one on top to find it.
	press(&s.r, f32.Pt(20, 50))
	s.frame(func(f *frame) { gotContainer, gotLeft, gotRight = draw(f) })

	if !sameKinds(pointerKinds(gotLeft), pointer.Press, pointer.Release) {
		t.Errorf("the sibling under the point got %v, want a press and a release", pointerKinds(gotLeft))
	}
	if !sameKinds(pointerKinds(gotContainer), pointer.Press, pointer.Release) {
		t.Errorf("the container got %v on a press over its other child, want the press", pointerKinds(gotContainer))
	}
	if kinds := pointerKinds(gotRight); len(kinds) != 0 {
		t.Errorf("a sibling the point is not over got %v, want nothing", kinds)
	}
}

// TestRouterGrabCancelsTheOthersOnTheNextFrame fixes when a claim on a pointer
// takes effect, and it is the half of the grab that a drained queue never
// shows.
//
// The claim is asked for while a frame is being drawn and settled as part of
// it. What the losing handlers are told is a Cancel, and they are told on the
// frame after the one the claim was made in -- because they have already read
// their events for the frame in which it happened. A gesture that undid its
// drawing on the strength of seeing the Cancel immediately would be a gesture
// that never undoes it.
func TestRouterGrabCancelsTheOthersOnTheNextFrame(t *testing.T) {
	var s screen
	container, inner := new(int), new(int)

	kinds := pointer.Press | pointer.Release | pointer.Drag | pointer.Cancel
	draw := func(f *frame) (c, i []event.Event) {
		defer f.area(image.Rect(0, 0, 100, 100))()
		c = f.listen(container, pointer.Filter{Target: container, Kinds: kinds})
		func() {
			defer f.area(image.Rect(20, 20, 80, 80))()
			i = f.listen(inner, pointer.Filter{Target: inner, Kinds: kinds})
		}()
		return c, i
	}

	s.frame(func(f *frame) { draw(f) })

	s.r.Queue(pointer.Event{Kind: pointer.Press, Position: f32.Pt(50, 50)})

	var gotContainer, gotInner []event.Event
	s.frame(func(f *frame) {
		gotContainer, gotInner = draw(f)
		// The claim is made inside the frame, after the events of the frame
		// have been read -- which is where a gesture makes it, once the press
		// has travelled far enough to be a drag.
		s.r.Source().Execute(pointer.GrabCmd{Tag: inner})
	})
	if !sameKinds(pointerKinds(gotInner), pointer.Press) {
		t.Fatalf("the inner control got %v, want the press it grabbed on", pointerKinds(gotInner))
	}
	if !sameKinds(pointerKinds(gotContainer), pointer.Press) {
		t.Fatalf("the container got %v, want the press: it is what makes the grab worth testing", pointerKinds(gotContainer))
	}

	s.frame(func(f *frame) { gotContainer, gotInner = draw(f) })
	if !sameKinds(pointerKinds(gotContainer), pointer.Cancel) {
		t.Errorf("the handler that lost the pointer got %v on the frame after the claim, want a Cancel", pointerKinds(gotContainer))
	}
	if kinds := pointerKinds(gotInner); len(kinds) != 0 {
		t.Errorf("the control that grabbed got %v, want nothing: a claim is not cancelled by its own success", kinds)
	}

	// From here the pointer belongs to one handler, and moving it says so.
	s.r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(55, 55)})
	s.frame(func(f *frame) { gotContainer, gotInner = draw(f) })
	if !sameKinds(pointerKinds(gotInner), pointer.Drag) {
		t.Errorf("the grabbing control got %v while holding the pointer, want the drag", pointerKinds(gotInner))
	}
	if kinds := pointerKinds(gotContainer); len(kinds) != 0 {
		t.Errorf("a cancelled handler got %v, want nothing more from a pointer it lost", kinds)
	}
	for _, e := range gotInner {
		if pe, ok := e.(pointer.Event); ok && pe.Priority != pointer.Grabbed {
			t.Errorf("the grabbing control was handed an event at priority %v, want Grabbed", pe.Priority)
		}
	}
}

// TestRouterRefusesAGrabFromOutside fixes that a claim on a pointer is refused
// unless the claimant is among the handlers that pointer matched.
//
// A granted claim cancels every other handler, so granting one to a control the
// pointer was never over is a way for anything on the screen to silence
// everything else.
func TestRouterRefusesAGrabFromOutside(t *testing.T) {
	var s screen
	near, far := new(int), new(int)

	kinds := pointer.Press | pointer.Release | pointer.Cancel
	draw := func(f *frame) (n, o []event.Event) {
		func() {
			defer f.area(image.Rect(0, 0, 50, 50))()
			n = f.listen(near, pointer.Filter{Target: near, Kinds: kinds})
		}()
		func() {
			defer f.area(image.Rect(100, 100, 150, 150))()
			o = f.listen(far, pointer.Filter{Target: far, Kinds: kinds})
		}()
		return n, o
	}

	s.frame(func(f *frame) { draw(f) })
	s.r.Queue(pointer.Event{Kind: pointer.Press, Position: f32.Pt(25, 25)})

	var gotNear, gotFar []event.Event
	s.frame(func(f *frame) {
		gotNear, gotFar = draw(f)
		s.r.Source().Execute(pointer.GrabCmd{Tag: far})
	})
	if !sameKinds(pointerKinds(gotNear), pointer.Press) {
		t.Fatalf("the control under the press got %v, want the press", pointerKinds(gotNear))
	}

	s.frame(func(f *frame) { gotNear, gotFar = draw(f) })
	if kinds := pointerKinds(gotNear); len(kinds) != 0 {
		t.Errorf("a control lost its pointer to a claim from outside: got %v", kinds)
	}
	if kinds := pointerKinds(gotFar); len(kinds) != 0 {
		t.Errorf("the refused claimant got %v, want nothing", kinds)
	}
}

// TestRouterDisabledSourceDeliversNothing fixes that a disabled source is inert
// in both directions: no event comes out of it, and no command goes in.
//
// It is what a control drawn as unavailable is handed, and the promise has to
// hold for commands as well -- a disabled field that could still take the focus
// would open the keyboard on a control that refuses to be typed into.
func TestRouterDisabledSourceDeliversNothing(t *testing.T) {
	var s screen
	tag := new(int)

	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 100))()
		f.listen(tag, key.FocusFilter{Target: tag}, pointer.Filter{Target: tag, Kinds: pointer.Press})
	})
	press(&s.r, f32.Pt(50, 50))

	off := s.r.Source().Disabled()
	if off.Enabled() {
		t.Fatal("a disabled source reports itself enabled")
	}
	if _, ok := off.Event(pointer.Filter{Target: tag, Kinds: pointer.Press}); ok {
		t.Error("a disabled source delivered an event")
	}
	off.Execute(key.FocusCmd{Tag: tag})
	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 100))()
		f.listen(tag, key.FocusFilter{Target: tag})
	})
	if s.r.Source().Focused(tag) {
		t.Error("a command from a disabled source was carried out")
	}

	var zero Source
	if zero.Enabled() {
		t.Error("the zero Source reports itself enabled")
	}
	if _, ok := zero.Event(pointer.Filter{Target: tag}); ok {
		t.Error("the zero Source delivered an event")
	}
	if zero.Focused(tag) {
		t.Error("the zero Source reports a focus")
	}
}

// TestRouterMovesFocusBetweenFrames fixes that moving the focus by keyboard
// walks the controls in the order they were declared, and wraps.
//
// The order is the frame's, which is why this needs frames: it is rebuilt from
// the operation list every time, so a control that moved on the screen moves in
// the tab order with it, and a control that left the screen leaves it.
func TestRouterMovesFocusBetweenFrames(t *testing.T) {
	var s screen
	first, second := new(int), new(int)

	draw := func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 20))()
		f.listen(first, key.FocusFilter{Target: first})
		func() {
			defer f.area(image.Rect(0, 20, 100, 40))()
			f.listen(second, key.FocusFilter{Target: second})
		}()
	}

	s.frame(draw)
	s.r.MoveFocus(key.FocusForward)
	s.frame(draw)
	if !s.r.Source().Focused(first) {
		t.Fatal("moving the focus forward from nothing did not reach the first control")
	}

	s.r.MoveFocus(key.FocusForward)
	s.frame(draw)
	if !s.r.Source().Focused(second) {
		t.Fatal("moving the focus forward did not reach the second control")
	}

	s.r.MoveFocus(key.FocusForward)
	s.frame(draw)
	if !s.r.Source().Focused(first) {
		t.Error("the focus did not wrap round to the first control")
	}

	s.r.MoveFocus(key.FocusBackward)
	s.frame(draw)
	if !s.r.Source().Focused(second) {
		t.Error("moving the focus backward from the first control did not wrap to the last")
	}
}

// TestRouterWakesForPendingEvents fixes that an event nobody has read keeps the
// window awake.
//
// A frame is drawn in response to something happening. An event that arrived
// and was not delivered -- because the control it is for has not asked yet --
// would otherwise sit in the router until something unrelated woke the window,
// which is a press that appears to do nothing until the pointer is moved.
func TestRouterWakesForPendingEvents(t *testing.T) {
	var s screen
	tag := new(int)

	s.frame(func(f *frame) {
		defer f.area(image.Rect(0, 0, 100, 100))()
		f.listen(tag, pointer.Filter{Target: tag, Kinds: pointer.Press})
	})
	if _, wake := s.r.WakeupTime(); wake {
		t.Fatal("a frame that changed nothing asked for another")
	}

	s.r.Queue(pointer.Event{Kind: pointer.Press, Position: f32.Pt(50, 50)})
	if _, wake := s.r.WakeupTime(); !wake {
		t.Error("an undelivered event did not ask for a frame")
	}
}
