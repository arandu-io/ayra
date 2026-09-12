package pointer_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
)

// The tests are written from outside the package, through the router, because
// that is where the answers live. A hit area is not a function here: it is a
// clip pushed around a handler, and whether a press lands in it is decided by
// code in another package reading the same operation list a window replays.
// A test that recomputed the rule would agree with itself and with nothing
// else, which is the failure that matters -- the documentation in this package
// describes a boundary, and the point is that the boundary is the real one.

// TestAButtonSetContainsWhatWasPutIntoIt is the set operation the whole package
// leans on: every handler that cares which button is down asks Contain.
func TestAButtonSetContainsWhatWasPutIntoIt(t *testing.T) {
	named := []struct {
		name   string
		button pointer.Buttons
	}{
		{"ButtonPrimary", pointer.ButtonPrimary},
		{"ButtonSecondary", pointer.ButtonSecondary},
		{"ButtonTertiary", pointer.ButtonTertiary},
		{"ButtonQuaternary", pointer.ButtonQuaternary},
		{"ButtonQuinary", pointer.ButtonQuinary},
	}

	for _, held := range named {
		set := held.button
		if !set.Contain(held.button) {
			t.Errorf("a set made of %s does not contain %s", held.name, held.name)
		}
		for _, other := range named {
			if other.button == held.button {
				continue
			}
			if set.Contain(other.button) {
				t.Errorf("a set made of %s contains %s, which was never put in it", held.name, other.name)
			}
		}
	}

	// Two buttons at once is the ordinary case -- a chord, or a second button
	// pressed before the first is released -- and the set has to answer for
	// each of them and for the pair.
	var all pointer.Buttons
	for _, b := range named {
		all |= b.button
	}
	for _, b := range named {
		if !all.Contain(b.button) {
			t.Errorf("a set of every button does not contain %s, so two of the constants share a bit", b.name)
		}
	}
	if !all.Contain(all) {
		t.Error("a set does not contain itself")
	}

	pair := pointer.ButtonPrimary | pointer.ButtonSecondary
	if !pair.Contain(pointer.ButtonPrimary | pointer.ButtonSecondary) {
		t.Error("a pair does not contain the pair it was made of")
	}
	if pair.Contain(pointer.ButtonPrimary | pointer.ButtonTertiary) {
		t.Error("a pair contains a pair it holds only half of; Contain answers all-of, not any-of")
	}

	// Contain asks for every button of the argument, so nothing is asked of an
	// empty argument and every set answers yes. A caller reaching for any-of
	// has to say so, and this is the line that would break if Contain drifted
	// into meaning it.
	if !pointer.Buttons(0).Contain(0) {
		t.Error("the empty set does not contain the empty set")
	}
	if !all.Contain(0) {
		t.Error("a full set does not contain the empty set")
	}
	if pointer.Buttons(0).Contain(pointer.ButtonPrimary) {
		t.Error("the empty set contains a button, so a Move event would read as a press")
	}
}

// TestTheTextOfAButtonSetNamesWhatIsInIt covers the form the set is read in,
// which is a failing test or a log line.
func TestTheTextOfAButtonSetNamesWhatIsInIt(t *testing.T) {
	for _, tc := range []struct {
		buttons pointer.Buttons
		text    string
	}{
		{pointer.ButtonPrimary, "ButtonPrimary"},
		{pointer.ButtonSecondary, "ButtonSecondary"},
		{pointer.ButtonTertiary, "ButtonTertiary"},
		{pointer.ButtonQuaternary, "ButtonQuaternary"},
		{pointer.ButtonQuinary, "ButtonQuinary"},
		{pointer.ButtonPrimary | pointer.ButtonSecondary, "ButtonPrimary|ButtonSecondary"},
		{pointer.ButtonQuinary | pointer.ButtonPrimary, "ButtonPrimary|ButtonQuinary"},
	} {
		if got := tc.buttons.String(); got != tc.text {
			t.Errorf("got %q; want %q", got, tc.text)
		}
	}
}

// TestTheTextOfEveryKindIsStable fixes the spelling of every kind and of a
// combination of them.
//
// Kind is a bitmask, so a filter is written as a run of kinds or-ed together
// and the text of one is the text of its parts joined. The order is the order
// of the bits, not the order they were written in: two filters naming the same
// kinds read the same, which is what makes the text worth comparing at all.
func TestTheTextOfEveryKindIsStable(t *testing.T) {
	for _, tc := range []struct {
		kind pointer.Kind
		text string
	}{
		{pointer.Cancel, "Cancel"},
		{pointer.Press, "Press"},
		{pointer.Release, "Release"},
		{pointer.Move, "Move"},
		{pointer.Drag, "Drag"},
		{pointer.Enter, "Enter"},
		{pointer.Leave, "Leave"},
		{pointer.Scroll, "Scroll"},

		{pointer.Enter | pointer.Leave, "Enter|Leave"},
		{pointer.Press | pointer.Release, "Press|Release"},
		{pointer.Move | pointer.Scroll, "Move|Scroll"},
		{pointer.Cancel | pointer.Press, "Cancel|Press"},

		// Written high bit first, and read low bit first.
		{pointer.Leave | pointer.Enter | pointer.Release | pointer.Press, "Press|Release|Enter|Leave"},

		{
			pointer.Cancel | pointer.Press | pointer.Release | pointer.Move |
				pointer.Drag | pointer.Enter | pointer.Leave | pointer.Scroll,
			"Cancel|Press|Release|Move|Drag|Enter|Leave|Scroll",
		},
	} {
		t.Run(tc.text, func(t *testing.T) {
			if got := tc.kind.String(); got != tc.text {
				t.Errorf("got %q; want %q", got, tc.text)
			}
		})
	}
}

// TestEveryKindIsOneBitOfItsOwn is what lets the text above be a join.
func TestEveryKindIsOneBitOfItsOwn(t *testing.T) {
	kinds := map[string]pointer.Kind{
		"Cancel":  pointer.Cancel,
		"Press":   pointer.Press,
		"Release": pointer.Release,
		"Move":    pointer.Move,
		"Drag":    pointer.Drag,
		"Enter":   pointer.Enter,
		"Leave":   pointer.Leave,
		"Scroll":  pointer.Scroll,
	}
	seen := make(map[pointer.Kind]string, len(kinds))
	for name, kind := range kinds {
		if kind == 0 || kind&(kind-1) != 0 {
			t.Errorf("%s is %#x, which is not a single bit, so a filter naming it names something else too", name, uint(kind))
		}
		if other, ok := seen[kind]; ok {
			t.Errorf("%s and %s are the same bit, so a handler asking for one is given the other", name, other)
		}
		seen[kind] = name
	}
}

// TestTheTextOfAnUnnamedValueIsTheNumber keeps String total.
//
// The one moment any of these is read is the moment something is already
// wrong -- a failing comparison, a log line about an event that should not have
// arrived. A String that panicked there would replace the value that was wrong
// with a stack trace from the code printing it, which is the one thing the
// reader does not need.
func TestTheTextOfAnUnnamedValueIsTheNumber(t *testing.T) {
	for _, tc := range []struct {
		what string
		text string
	}{
		{"a kind of no bits", pointer.Kind(0).String()},
		{"a kind of an unnamed bit", pointer.Kind(1 << 20).String()},
		{"a named kind beside an unnamed bit", (pointer.Press | 1<<20).String()},
		{"an unnamed priority", pointer.Priority(9).String()},
		{"an unnamed source", pointer.Source(9).String()},
		{"an unnamed cursor", pointer.Cursor(200).String()},
		{"a button set of no buttons", pointer.Buttons(0).String()},
		{"a button set of an unnamed bit", pointer.Buttons(1 << 7).String()},
	} {
		if tc.text == "" {
			t.Errorf("%s reads as nothing, which in a failure message is indistinguishable from the value being absent", tc.what)
		}
	}

	if got, want := (pointer.Press | 1<<20).String(), "Press"; got == want {
		t.Errorf("a kind carrying an unnamed bit reads as %q, so the bit that does not belong is the one that is hidden", got)
	}
}

// TestEveryNamedCursorHasItsOwnText covers the widest of these sets, where a
// case left out of a switch is the likely mistake.
func TestEveryNamedCursorHasItsOwnText(t *testing.T) {
	seen := make(map[string]pointer.Cursor)
	for c := pointer.CursorDefault; c <= pointer.CursorNorthWestSouthEastResize; c++ {
		text := c.String()
		if text == "" {
			t.Errorf("cursor %d reads as nothing", c)
			continue
		}
		if other, ok := seen[text]; ok {
			t.Errorf("cursor %d and cursor %d both read as %q", c, other, text)
		}
		seen[text] = c
	}
	if len(seen) != 26 {
		t.Errorf("%d cursors are named and 26 were expected; a shape added or removed changes what a control can ask for", len(seen))
	}
}

// TestTheTextOfEveryPriorityAndSourceIsStable fixes the two smallest sets.
func TestTheTextOfEveryPriorityAndSourceIsStable(t *testing.T) {
	if got, want := pointer.Shared.String(), "Shared"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
	if got, want := pointer.Grabbed.String(), "Grabbed"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
	if got, want := pointer.Mouse.String(), "Mouse"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
	if got, want := pointer.Touch.String(), "Touch"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

// TestTheHitAreaOfARectangleHoldsItsLowEdgeAndNotItsHigh is the boundary the
// package documentation describes, asked of the code that decides it.
func TestTheHitAreaOfARectangleHoldsItsLowEdgeAndNotItsHigh(t *testing.T) {
	var ops op.Ops
	control := new(int)
	area := clip.Rect(image.Rect(10, 20, 50, 60)).Push(&ops)
	event.Op(&ops, control)
	area.Pop()

	for _, tc := range []struct {
		name   string
		at     f32.Point
		inside bool
	}{
		{"the low corner", f32.Pt(10, 20), true},
		{"the low edge in x", f32.Pt(10, 40), true},
		{"the low edge in y", f32.Pt(30, 20), true},
		{"just short of the low edge in x", f32.Pt(9, 40), false},
		{"just short of the low edge in y", f32.Pt(30, 19), false},

		{"the last whole pixel before the high corner", f32.Pt(49, 59), true},
		{"a hair short of the high corner", f32.Pt(49.99, 59.99), true},
		{"the high corner", f32.Pt(50, 60), false},
		{"the high edge in x", f32.Pt(50, 40), false},
		{"the high edge in y", f32.Pt(30, 60), false},

		{"the middle", f32.Pt(30, 40), true},
		{"well outside", f32.Pt(1000, 1000), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := pressed(&ops, tc.at, control)
			if want := tc.inside; got[0] != want {
				t.Errorf("a press at %v was inside=%t; want inside=%t", tc.at, got[0], want)
			}
		})
	}
}

// TestTwoAreasSharingAnEdgeGiveThePressToExactlyOne is why the boundary is
// half-open rather than closed.
//
// Controls are laid out edge to edge, so the high edge of one is the low edge
// of the next. A closed boundary would put that column of pixels in both areas
// and a press there would go to two controls at once; an open one would put it
// in neither, and a row of buttons would have a dead line between each pair.
// Holding the low edge and not the high is what makes the seam belong to one
// side, whichever seam it is.
func TestTwoAreasSharingAnEdgeGiveThePressToExactlyOne(t *testing.T) {
	var ops op.Ops
	left, right := new(int), new(int)

	l := clip.Rect(image.Rect(0, 0, 50, 100)).Push(&ops)
	event.Op(&ops, left)
	l.Pop()
	r := clip.Rect(image.Rect(50, 0, 100, 100)).Push(&ops)
	event.Op(&ops, right)
	r.Pop()

	for x := 48; x <= 52; x++ {
		at := f32.Pt(float32(x), 50)
		got := pressed(&ops, at, left, right)
		switch {
		case got[0] && got[1]:
			t.Errorf("a press at %v went to both controls, so the seam is in two hit areas", at)
		case !got[0] && !got[1]:
			t.Errorf("a press at %v went to neither control, so the seam is a dead column", at)
		case x < 50 && !got[0]:
			t.Errorf("a press at %v went to the right control", at)
		case x >= 50 && !got[1]:
			t.Errorf("a press at %v went to the left control", at)
		}
	}
}

// TestAPressOutsideEveryAreaReachesNoControl is the other half of a hit area:
// what it keeps out.
func TestAPressOutsideEveryAreaReachesNoControl(t *testing.T) {
	var ops op.Ops
	first, second := new(int), new(int)

	a := clip.Rect(image.Rect(0, 0, 20, 20)).Push(&ops)
	event.Op(&ops, first)
	a.Pop()
	b := clip.Rect(image.Rect(60, 60, 80, 80)).Push(&ops)
	event.Op(&ops, second)
	b.Pop()

	for _, at := range []f32.Point{
		f32.Pt(40, 40),
		f32.Pt(-1, -1),
		f32.Pt(20, 20),
		f32.Pt(80, 80),
		f32.Pt(0, 60),
	} {
		got := pressed(&ops, at, first, second)
		if got[0] || got[1] {
			t.Errorf("a press at %v reached a control: first=%t second=%t", at, got[0], got[1])
		}
	}
}

// TestWhereTwoAreasOverlapTheNearerOneTakesThePress fixes what happens when a
// control is drawn over another.
//
// The areas form a tree in the order they were pushed, and the search runs from
// the last one back. A control drawn after another is in front of it on the
// screen, and it is in front of it here too: the press stops at the first one
// that answers, so a dialog over a form does not press the form behind it.
func TestWhereTwoAreasOverlapTheNearerOneTakesThePress(t *testing.T) {
	var ops op.Ops
	behind, infront := new(int), new(int)

	b := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, behind)
	b.Pop()
	f := clip.Rect(image.Rect(50, 50, 150, 150)).Push(&ops)
	event.Op(&ops, infront)
	f.Pop()

	overlap := pressed(&ops, f32.Pt(75, 75), behind, infront)
	if overlap[0] {
		t.Error("a press where the two overlap reached the control behind")
	}
	if !overlap[1] {
		t.Error("a press where the two overlap did not reach the control in front")
	}

	// Outside the overlap each still answers for its own area, so what changed
	// is which one wins a contest, not which one is reachable.
	only := pressed(&ops, f32.Pt(25, 25), behind, infront)
	if !only[0] || only[1] {
		t.Errorf("a press inside the rear area alone: behind=%t infront=%t", only[0], only[1])
	}
}

// TestAControlInsideAnotherDoesNotTakeThePressFromIt is the half of the
// overlap rule that is easy to state backwards.
//
// Being in front is a relation between siblings. A control declared inside
// another overlaps it completely and still does not take the press from it: the
// search skips what is beside the match, and resumes at what encloses it. It is
// why a pressable row inside a pressable list leaves the list its press, and why
// a container that wanted the opposite has to be a sibling rather than a parent.
func TestAControlInsideAnotherDoesNotTakeThePressFromIt(t *testing.T) {
	var ops op.Ops
	container, inner := new(int), new(int)

	outer := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, container)
	in := clip.Rect(image.Rect(20, 20, 80, 80)).Push(&ops)
	event.Op(&ops, inner)
	in.Pop()
	outer.Pop()

	both := pressed(&ops, f32.Pt(50, 50), container, inner)
	if !both[1] {
		t.Error("a press inside the inner control did not reach it")
	}
	if !both[0] {
		t.Error("a press inside the inner control did not reach the container, so an enclosing area was treated as one behind")
	}

	// Inside the container and outside the inner one, only the container.
	outside := pressed(&ops, f32.Pt(10, 10), container, inner)
	if !outside[0] || outside[1] {
		t.Errorf("a press in the container alone: container=%t inner=%t", outside[0], outside[1])
	}
}

// TestAnAreaClipsTheOneInsideIt is the intersection rule.
func TestAnAreaClipsTheOneInsideIt(t *testing.T) {
	var ops op.Ops
	inner := new(int)

	// The inner area reaches to 200, the one enclosing it stops at 100, and
	// what answers is the overlap.
	outer := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	in := clip.Rect(image.Rect(50, 50, 200, 200)).Push(&ops)
	event.Op(&ops, inner)
	in.Pop()
	outer.Pop()

	for _, tc := range []struct {
		at     f32.Point
		inside bool
	}{
		{f32.Pt(75, 75), true},
		{f32.Pt(25, 25), false},
		{f32.Pt(150, 150), false},
		{f32.Pt(99, 99), true},
		{f32.Pt(100, 100), false},
	} {
		if got := pressed(&ops, tc.at, inner)[0]; got != tc.inside {
			t.Errorf("a press at %v was inside=%t; want inside=%t", tc.at, got, tc.inside)
		}
	}
}

// TestAPassThroughAreaLeavesThePressForTheOneBehindIt is the exception to the
// rule above, and the reason an overlay can be transparent to input.
func TestAPassThroughAreaLeavesThePressForTheOneBehindIt(t *testing.T) {
	var ops op.Ops
	behind, overlay := new(int), new(int)

	b := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, behind)
	b.Pop()

	o := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	pass := pointer.PassOp{}.Push(&ops)
	event.Op(&ops, overlay)
	pass.Pop()
	o.Pop()

	got := pressed(&ops, f32.Pt(50, 50), behind, overlay)
	if !got[1] {
		t.Error("the pass-through overlay did not receive the press")
	}
	if !got[0] {
		t.Error("the control behind a pass-through overlay did not receive the press")
	}
}

// pressed replays ops into a fresh router, queues one press at pos, and answers
// for each tag whether it received it.
//
// The router is new every call. A reused one remembers where the pointer was,
// and the second press would carry that history -- an Enter, a Leave, a set of
// handlers recorded at the previous press -- so the answer would be about the
// path taken and not about the position asked for.
func pressed(ops *op.Ops, pos f32.Point, tags ...event.Tag) []bool {
	var r input.Router

	// Each filter answers a reset event the first time it is offered, before
	// any frame exists. Drain it here so what is read after the press is the
	// press.
	for _, tag := range tags {
		drain(&r, tag)
	}

	r.Frame(ops)
	r.Queue(pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: pos,
	})

	got := make([]bool, len(tags))
	for i, tag := range tags {
		for _, e := range drain(&r, tag) {
			if e, ok := e.(pointer.Event); ok && e.Kind == pointer.Press {
				got[i] = true
			}
		}
	}
	return got
}

// drain reads every event waiting for tag.
func drain(r *input.Router, tag event.Tag) []event.Event {
	filter := pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Cancel}
	var events []event.Event
	for {
		e, ok := r.Event(filter)
		if !ok {
			return events
		}
		events = append(events, e)
	}
}
