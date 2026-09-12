package input

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
)

// The tests below drive whole frames rather than the hit tree directly: the
// areas are declared again on every frame, the way an interface declares them,
// and the pointer events go in through the router. A test that reached into
// the tree would still pass with a router that never consulted it.

// ayraProbe is one handler with an area, driven over real frames.
type ayraProbe struct {
	tag event.Tag
	f   pointer.Filter
}

// ayraNewProbe registers a probe and swallows the reset event a handler is
// given before its first frame.
func ayraNewProbe(r *Router, kinds pointer.Kind) *ayraProbe {
	p := &ayraProbe{tag: new(int)}
	p.f = pointer.Filter{Target: p.tag, Kinds: kinds | pointer.Cancel}
	ayraDrain(r, p.f)
	return p
}

// ayraDrain takes every event waiting for the filters, in the order they were
// delivered.
func ayraDrain(r *Router, filters ...event.Filter) []pointer.Event {
	var out []pointer.Event
	for {
		e, ok := r.Event(filters...)
		if !ok {
			return out
		}
		if pe, ok := e.(pointer.Event); ok {
			out = append(out, pe)
		}
	}
}

// ayraKinds reduces a run of events to what kind each one was.
func ayraKinds(evts []pointer.Event) []pointer.Kind {
	kinds := make([]pointer.Kind, len(evts))
	for i, e := range evts {
		kinds[i] = e.Kind
	}
	return kinds
}

// ayraPositions reduces a run of events to where each one landed. The position
// is in the receiving handler's own coordinates, so two handlers under the
// same point report two different numbers -- which is what tells them apart in
// a stream that carries no tag.
func ayraPositions(evts []pointer.Event) []f32.Point {
	pos := make([]f32.Point, len(evts))
	for i, e := range evts {
		pos[i] = e.Position
	}
	return pos
}

func ayraSameKinds(got []pointer.Kind, want ...pointer.Kind) bool {
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

// ayraPressLands reports whether a press at pos reaches a handler whose area
// is declared by shape.
//
// A router and a frame per question, because a press that misses still changes
// what the router tracks, and a second question asked of the same router would
// be answering about that state instead of about the shape.
func ayraPressLands(shape func(*op.Ops) clip.Stack, pos f32.Point) bool {
	var r Router
	p := ayraNewProbe(&r, pointer.Press)

	var ops op.Ops
	area := shape(&ops)
	event.Op(&ops, p.tag)
	area.Pop()
	r.Frame(&ops)

	r.Queue(pointer.Event{Kind: pointer.Press, Position: pos})
	for _, e := range ayraDrain(&r, p.f) {
		if e.Kind == pointer.Press {
			return true
		}
	}
	return false
}

// TestPointerRectangleIsHalfOpen fixes which side of a rectangle belongs to it.
//
// The low edge is inside and the high edge is outside, on both axes. The rule
// matters more than the choice: two controls laid edge to edge share a line of
// pixels, and a rule that included both edges would give that line to both --
// a press there reaching two controls, or the wrong one, depending on which
// was declared last.
func TestPointerRectangleIsHalfOpen(t *testing.T) {
	rect := image.Rect(10, 20, 60, 80)
	shape := func(ops *op.Ops) clip.Stack { return clip.Rect(rect).Push(ops) }

	for _, tc := range []struct {
		name string
		pos  f32.Point
		want bool
	}{
		{"low corner is inside", f32.Pt(10, 20), true},
		{"low horizontal edge is inside", f32.Pt(10, 50), true},
		{"low vertical edge is inside", f32.Pt(30, 20), true},
		{"just inside the high corner", f32.Pt(59.5, 79.5), true},
		{"high horizontal edge is outside", f32.Pt(60, 50), false},
		{"high vertical edge is outside", f32.Pt(30, 80), false},
		{"high corner is outside", f32.Pt(60, 80), false},
		{"left of the area", f32.Pt(9.5, 50), false},
		{"above the area", f32.Pt(30, 19.5), false},
		{"far outside", f32.Pt(500, 500), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ayraPressLands(shape, tc.pos); got != tc.want {
				t.Errorf("press at %v on area %v landed=%t, want %t (the low edge belongs to the area, the high edge to whatever is next to it)", tc.pos, rect, got, tc.want)
			}
		})
	}
}

// TestPointerRectangleEdgesSurviveAnOffset asks the same question of an area
// that was moved, because the transform is undone before the shape is
// consulted and an error there shifts the whole boundary by the offset.
func TestPointerRectangleEdgesSurviveAnOffset(t *testing.T) {
	shape := func(ops *op.Ops) clip.Stack {
		defer op.Offset(image.Pt(100, 40)).Push(ops).Pop()
		return clip.Rect(image.Rect(0, 0, 50, 50)).Push(ops)
	}

	for _, tc := range []struct {
		name string
		pos  f32.Point
		want bool
	}{
		{"low corner of the moved area", f32.Pt(100, 40), true},
		{"just inside its high corner", f32.Pt(149.5, 89.5), true},
		{"its high edge is outside", f32.Pt(150, 60), false},
		{"where the area would have been unmoved", f32.Pt(25, 25), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ayraPressLands(shape, tc.pos); got != tc.want {
				t.Errorf("press at %v landed=%t, want %t (an offset moves the area, it does not widen it)", tc.pos, got, tc.want)
			}
		})
	}
}

// TestPointerEllipseIsTheShapeThatWasClipped is the one this file exists for.
//
// The clip carries a shape, not just a rectangle, and the hit test has to read
// the same shape. If it read the bounding rectangle instead, an elliptical
// control would answer a press in its corners -- outside everything that was
// ever painted. Nothing drawn on the screen would look wrong, so no picture
// can catch it: the fault is entirely in the part that is invisible.
func TestPointerEllipseIsTheShapeThatWasClipped(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	ellipse := func(ops *op.Ops) clip.Stack { return clip.Ellipse(bounds).Push(ops) }
	rect := func(ops *op.Ops) clip.Stack { return clip.Rect(bounds).Push(ops) }

	corners := []f32.Point{
		f32.Pt(5, 5),
		f32.Pt(95, 5),
		f32.Pt(5, 95),
		f32.Pt(95, 95),
	}
	for _, c := range corners {
		if ayraPressLands(ellipse, c) {
			t.Errorf("press at %v reached an ellipse clipped to %v; the corner of the enclosing rectangle is outside the ellipse", c, bounds)
		}
		// The same point under a rectangular clip does land. Without this the
		// test above would also pass for a hit test that answers nothing at
		// all.
		if !ayraPressLands(rect, c) {
			t.Errorf("press at %v missed a rectangle clipped to %v; the corner is inside the rectangle", c, bounds)
		}
	}

	inside := []f32.Point{
		f32.Pt(50, 50),
		f32.Pt(50, 1),
		f32.Pt(50, 99),
		f32.Pt(1, 50),
		f32.Pt(99, 50),
	}
	for _, p := range inside {
		if !ayraPressLands(ellipse, p) {
			t.Errorf("press at %v missed an ellipse clipped to %v; the point is inside it", p, bounds)
		}
	}
}

// TestPointerOverlappingSiblingsGoToTheTopOne fixes the order two controls
// drawn over each other are asked in.
//
// The one declared last is the one drawn last, which is the one the eye sees
// on top, and it is the one that answers. The one underneath hears nothing --
// not a press, not even an enter -- because a control that reacts through
// another one is a control reacting to a press the user aimed somewhere else.
func TestPointerOverlappingSiblingsGoToTheTopOne(t *testing.T) {
	var r Router
	under := ayraNewProbe(&r, pointer.Press|pointer.Enter|pointer.Leave)
	over := ayraNewProbe(&r, pointer.Press|pointer.Enter|pointer.Leave)

	var ops op.Ops
	lower := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, under.tag)
	lower.Pop()
	upper := clip.Rect(image.Rect(50, 0, 150, 100)).Push(&ops)
	event.Op(&ops, over.tag)
	upper.Pop()
	r.Frame(&ops)

	r.Queue(pointer.Event{Kind: pointer.Press, Position: f32.Pt(75, 50)})

	if got := ayraKinds(ayraDrain(&r, over.f)); !ayraSameKinds(got, pointer.Enter, pointer.Press) {
		t.Errorf("the control on top got %v, want [Enter Press]", got)
	}
	if got := ayraKinds(ayraDrain(&r, under.f)); len(got) != 0 {
		t.Errorf("the control underneath got %v, want nothing: the press landed on the one drawn over it", got)
	}
}

// TestPointerNestedAreasGoInsideOut fixes the other overlap, which is not the
// same one: a control inside another is not a control on top of it. Both hear
// the press, the inner one first, so that an inner control may act and an
// outer one may still count the press it contains.
//
// The two areas are offset differently, which is what makes the receiver of
// each event identifiable: the delivered position is in the receiver's own
// coordinates, and the same screen point is two different numbers there.
func TestPointerNestedAreasGoInsideOut(t *testing.T) {
	var r Router
	outer := ayraNewProbe(&r, pointer.Press)
	inner := ayraNewProbe(&r, pointer.Press)

	var ops op.Ops
	outerArea := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, outer.tag)
	shift := op.Offset(image.Pt(20, 20)).Push(&ops)
	innerArea := clip.Rect(image.Rect(0, 0, 50, 50)).Push(&ops)
	event.Op(&ops, inner.tag)
	innerArea.Pop()
	shift.Pop()
	outerArea.Pop()
	r.Frame(&ops)

	r.Queue(pointer.Event{Kind: pointer.Press, Position: f32.Pt(30, 30)})

	got := ayraPositions(ayraDrain(&r, inner.f, outer.f))
	want := []f32.Point{f32.Pt(10, 10), f32.Pt(30, 30)}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("press reached %v, want %v: the inner control first, each in its own coordinates", got, want)
	}
}

// TestPointerAreasAreRedeclaredEachFrame fixes that a frame replaces the hit
// tree rather than adding to it.
//
// A control that moved is gone from where it was. Keeping the old area would
// leave a press target behind on every frame a control moves in, which is
// every frame of anything that scrolls.
func TestPointerAreasAreRedeclaredEachFrame(t *testing.T) {
	var r Router
	p := ayraNewProbe(&r, pointer.Press)

	declare := func(at image.Rectangle) {
		var ops op.Ops
		area := clip.Rect(at).Push(&ops)
		event.Op(&ops, p.tag)
		area.Pop()
		r.Frame(&ops)
	}
	pressed := func(pos f32.Point) bool {
		r.Queue(
			pointer.Event{Kind: pointer.Press, Position: pos},
			pointer.Event{Kind: pointer.Release, Position: pos},
		)
		for _, e := range ayraDrain(&r, p.f) {
			if e.Kind == pointer.Press {
				return true
			}
		}
		return false
	}

	first := image.Rect(0, 0, 50, 50)
	second := image.Rect(100, 100, 150, 150)

	declare(first)
	if !pressed(f32.Pt(25, 25)) {
		t.Fatalf("press at the declared area %v missed", first)
	}

	declare(second)
	if pressed(f32.Pt(25, 25)) {
		t.Errorf("press at %v reached a control that this frame declares at %v; the previous frame's area outlived its frame", f32.Pt(25, 25), second)
	}
	if !pressed(f32.Pt(125, 125)) {
		t.Errorf("press at %v missed the control this frame declares at %v", f32.Pt(125, 125), second)
	}
}

// TestPointerHeldPointerKeepsItsOwner fixes what a press owns: a pointer that
// went down on a control keeps reporting to that control until it comes up,
// wherever it travels.
//
// A drag that stopped being delivered the moment it left the control would
// make every slider end at its own edge, and every other control it passed
// over would light up under a finger that was never aimed at it.
func TestPointerHeldPointerKeepsItsOwner(t *testing.T) {
	var r Router
	held := ayraNewProbe(&r, pointer.Press|pointer.Drag|pointer.Release|pointer.Enter|pointer.Leave)
	passed := ayraNewProbe(&r, pointer.Press|pointer.Drag|pointer.Release|pointer.Enter|pointer.Leave)

	var ops op.Ops
	a := clip.Rect(image.Rect(0, 0, 50, 50)).Push(&ops)
	event.Op(&ops, held.tag)
	a.Pop()
	b := clip.Rect(image.Rect(100, 100, 200, 200)).Push(&ops)
	event.Op(&ops, passed.tag)
	b.Pop()
	r.Frame(&ops)

	r.Queue(pointer.Event{Kind: pointer.Press, Position: f32.Pt(25, 25)})
	if got := ayraKinds(ayraDrain(&r, held.f)); !ayraSameKinds(got, pointer.Enter, pointer.Press) {
		t.Fatalf("press on the held control got %v, want [Enter Press]", got)
	}

	// Claim the pointer, which is what a control does once it has decided the
	// gesture is its own.
	r.Source().Execute(pointer.GrabCmd{Tag: held.tag})

	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(150, 150)})
	if got := ayraKinds(ayraDrain(&r, held.f)); !ayraSameKinds(got, pointer.Leave, pointer.Drag) {
		t.Errorf("the held control got %v, want [Leave Drag]: the pointer leaves its area but not its keeping", got)
	}
	if got := ayraKinds(ayraDrain(&r, passed.f)); len(got) != 0 {
		t.Errorf("the control the pointer was dragged over got %v, want nothing while another control holds the pointer", got)
	}

	r.Queue(pointer.Event{Kind: pointer.Release, Position: f32.Pt(150, 150)})
	if got := ayraKinds(ayraDrain(&r, held.f)); !ayraSameKinds(got, pointer.Release) {
		t.Errorf("the held control got %v on release, want [Release]: the press ends where it started, not where the finger was", got)
	}
	// Only once the pointer is no longer held does the control it now sits
	// over hear anything at all, and what it hears is an enter -- never the
	// end of somebody else's press.
	if got := ayraKinds(ayraDrain(&r, passed.f)); !ayraSameKinds(got, pointer.Enter) {
		t.Errorf("the control under the released pointer got %v, want [Enter]", got)
	}
}
