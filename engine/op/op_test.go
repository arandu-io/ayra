package op

import (
	"image"
	"math"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
)

// What a list of operations has to promise, tested from the outside.
//
// Everything drawn by this library ends up here before it is a pixel, so the
// list is the one place where a defect is invisible in every screenshot and
// present in all of them. The properties below are the ones the rest of the
// engine relies on without asking: a recording replays as what was recorded, a
// stack that is pushed is popped, a reset list is a new list, and a transform
// undone leaves the plane where it found it.
//
// The tests read the encoded stream rather than the fields of a struct. The
// stream is what the rasterizer sees, and a test that agreed with the fields
// while the bytes said something else would pass on exactly the day it matters.

// recorded is one operation as the rasterizer receives it: what kind it is and
// the bytes that say the rest.
type recorded struct {
	kind ops.OpType
	data []byte
}

// stream reads a list the way the engine does and returns every operation it
// yields, in execution order.
//
// The bytes are copied because the reader hands out windows onto the list's own
// buffer, and a later write to the list would rewrite an answer already given.
func stream(o *Ops) []recorded {
	var reader ops.Reader
	reader.Reset(&o.Internal)

	var out []recorded
	for {
		encoded, more := reader.Decode()
		if !more {
			return out
		}
		out = append(out, recorded{
			kind: ops.OpType(encoded.Data[0]),
			data: append([]byte(nil), encoded.Data...),
		})
	}
}

// equal reports whether two streams are the same operations with the same
// bytes.
func equal(a, b []recorded) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].kind != b[i].kind || string(a[i].data) != string(b[i].data) {
			return false
		}
	}
	return true
}

// kinds names the operations of a stream, for a failure that says what was
// found instead of how many bytes differed.
func kinds(s []recorded) []ops.OpType {
	out := make([]ops.OpType, len(s))
	for i, op := range s {
		out[i] = op.kind
	}
	return out
}

// sample is a short drawing used wherever a test needs operations rather than
// particular ones: a pushed transform, one added inside it, and the pop.
func sample(o *Ops) {
	stack := Affine(f32.AffineId().Offset(f32.Pt(3, 5))).Push(o)
	Offset(image.Pt(7, 11)).Add(o)
	stack.Pop()
}

// TestARecordedMacroReplaysWhatWasRecorded is the property the whole facility
// rests on.
//
// Recording exists so that a widget can measure itself before deciding where to
// put what it measured -- a background cannot be painted until it is known how
// much there is to cover. That is only sound if replaying the recording is
// indistinguishable from having drawn it in place. If replay dropped an
// operation or reordered two, every control that measures itself would draw
// differently from one that does not, and nothing in the picture would say
// which half was wrong.
func TestARecordedMacroReplaysWhatWasRecorded(t *testing.T) {
	var direct Ops
	sample(&direct)

	var source Ops
	macro := Record(&source)
	sample(&source)
	call := macro.Stop()

	var replayed Ops
	call.Add(&replayed)

	want, got := stream(&direct), stream(&replayed)
	if !equal(want, got) {
		t.Errorf("a replayed recording yielded %v; drawing the same thing in place yields %v", kinds(got), kinds(want))
	}
}

// TestARecordedMacroReplaysOncePerCall is the reuse half of the same promise.
//
// A recording is replayed more than once on purpose -- the same measured
// content drawn at several offsets is how a row of anything is built -- and the
// second replay has to be the first one again. A recording that was consumed by
// reading it would draw the first item of a list and leave the rest blank,
// which reads as a data problem rather than as an operations one.
func TestARecordedMacroReplaysOncePerCall(t *testing.T) {
	var source Ops
	macro := Record(&source)
	sample(&source)
	call := macro.Stop()

	var once Ops
	call.Add(&once)

	var twice Ops
	call.Add(&twice)
	call.Add(&twice)

	first := stream(&once)
	both := stream(&twice)

	if len(both) != 2*len(first) {
		t.Fatalf("one replay yielded %d operations and two yielded %d; two replays are the first one again", len(first), len(both))
	}
	if !equal(first, both[:len(first)]) || !equal(first, both[len(first):]) {
		t.Errorf("the two replays differ: %v then %v", kinds(both[:len(first)]), kinds(both[len(first):]))
	}
}

// TestAReplayedMacroLeavesOutItsOwnHeader guards the arithmetic in Stop.
//
// The recording reserves room for a header and fills it in afterwards, and the
// call that replays it has to start past that header. Off by the header's
// length in either direction the replay begins inside an operation, and what
// the rasterizer decodes from there is whatever the bytes happen to spell.
func TestAReplayedMacroLeavesOutItsOwnHeader(t *testing.T) {
	var source Ops
	macro := Record(&source)
	Offset(image.Pt(1, 2)).Add(&source)
	call := macro.Stop()

	var replayed Ops
	call.Add(&replayed)

	got := stream(&replayed)
	if len(got) != 1 || got[0].kind != ops.TypeTransform {
		t.Fatalf("replaying a recording of one transform yielded %v", kinds(got))
	}
}

// TestAnUnclosedRecordingSwallowsWhatFollowsIt fixes what an unfinished macro
// does, because it has to do something.
//
// A recording that is never stopped has no end written into its header, and the
// reader treats it as reaching the end of the list -- so everything added after
// it is inside a recording nobody replays, and none of it is drawn. That is the
// safe direction: the alternative is executing operations that were meant to be
// held back, which would draw a measurement onto the screen.
func TestAnUnclosedRecordingSwallowsWhatFollowsIt(t *testing.T) {
	var o Ops
	Record(&o)
	sample(&o)

	if got := stream(&o); len(got) != 0 {
		t.Errorf("a list whose only recording was never stopped yielded %v; nothing in it has been replayed", kinds(got))
	}
}

// TestATransformIsPoppedInTheMacroThatPushedIt is the rule that keeps a
// recording self-contained.
//
// A recording is replayed somewhere other than where it was made. A push inside
// one and a pop outside it would mean the transform stack is unbalanced by an
// amount that depends on how many times the recording is replayed -- once for a
// button drawn once, forty times for a list -- and the drift lands on whatever
// is drawn next. It panics where the mistake is instead.
func TestATransformIsPoppedInTheMacroThatPushedIt(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("popping a transform in a different macro than the one that pushed it was allowed")
		}
	}()

	var o Ops
	stack := Offset(image.Point{}).Push(&o)
	Record(&o)
	stack.Pop()
}

// TestATransformIsPoppedOnce is the other half of the pairing.
//
// The stack is tracked by identity rather than by depth, so a second pop of the
// same push is caught rather than silently discarding whatever the enclosing
// widget pushed. Without it a control that popped twice would steal its
// parent's transform, and the drawing that moved would be its sibling's.
func TestATransformIsPoppedOnce(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("popping the same transform twice was allowed")
		}
	}()

	var o Ops
	stack := Offset(image.Pt(1, 1)).Push(&o)
	stack.Pop()
	stack.Pop()
}

// TestARecordingIsStoppedBeforeTheOneAroundIt keeps recordings nested rather
// than interleaved.
//
// Recordings nest because widgets nest: a control measures itself while its
// parent is measuring the whole row. Stopping the outer one first would write
// an end into the outer header that falls inside the inner recording, and the
// replay would start midway through an operation.
func TestARecordingIsStoppedBeforeTheOneAroundIt(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("stopping the outer recording while an inner one was open was allowed")
		}
	}()

	var o Ops
	outer := Record(&o)
	Record(&o)
	outer.Stop()
}

// TestEveryPushedTransformIsPoppedByTheEndOfAFrame is the balance stated as one
// count rather than as a rule each caller remembers.
//
// A frame that ends with more pushes than pops leaves the plane displaced, and
// because the list is reused between frames the displacement is not visible in
// the frame that caused it. Counting the two kinds in the finished stream is
// the cheapest way to see it.
func TestEveryPushedTransformIsPoppedByTheEndOfAFrame(t *testing.T) {
	var o Ops
	outer := Offset(image.Pt(4, 4)).Push(&o)
	sample(&o)
	inner := Affine(f32.AffineId().Scale(f32.Pt(0, 0), f32.Pt(2, 2))).Push(&o)
	Offset(image.Pt(1, 1)).Add(&o)
	inner.Pop()
	outer.Pop()

	pushes, pops := 0, 0
	for _, op := range stream(&o) {
		switch op.kind {
		case ops.TypeTransform:
			// The push flag is the byte after the kind: a transform that only
			// multiplies the current one does not touch the stack.
			if op.data[1] != 0 {
				pushes++
			}
		case ops.TypePopTransform:
			pops++
		}
	}

	if pushes != pops {
		t.Errorf("the frame pushed %d transforms and popped %d; a frame that ends displaced displaces the next one", pushes, pops)
	}
	if pushes == 0 {
		t.Error("the drawing pushed nothing, so the count proves nothing")
	}
}

// TestAddDoesNotTouchTheStack is the difference between the two ways to apply a
// transform.
//
// Push is for a transform that is taken back, Add is for one that stands until
// the enclosing push is popped. They encode as the same operation with one byte
// between them, so the byte is what this reads: a widget that pushed where it
// meant to add would hand back a transform its caller still needed.
func TestAddDoesNotTouchTheStack(t *testing.T) {
	var pushed Ops
	Offset(image.Pt(1, 2)).Push(&pushed)

	var added Ops
	Offset(image.Pt(1, 2)).Add(&added)

	push, add := stream(&pushed), stream(&added)
	if len(push) != 1 || len(add) != 1 {
		t.Fatalf("one transform each was expected; got %v and %v", kinds(push), kinds(add))
	}
	if push[0].data[1] == 0 {
		t.Error("Push did not mark the transform as pushed")
	}
	if add[0].data[1] != 0 {
		t.Error("Add marked the transform as pushed, so whoever added it owes a pop nobody will make")
	}
	if string(push[0].data[2:]) != string(add[0].data[2:]) {
		t.Error("Push and Add encoded different transformations from the same offset")
	}
}

// TestATransformComposedWithItsInverseIsTheIdentity is the property that makes
// nesting safe.
//
// Everything drawn inside a pushed transform is drawn in a frame the parent
// chose, and the parent gets its own frame back by composing the other way. If
// the composition were not the identity -- because the encoding lost precision,
// or because the elements were written in the wrong order -- the error would be
// tiny, it would accumulate with depth, and it would show as a deep widget
// drifting by a fraction of a pixel from a shallow one beside it.
func TestATransformComposedWithItsInverseIsTheIdentity(t *testing.T) {
	origin := f32.Pt(0, 0)
	corner := f32.Pt(120, 80)

	for _, c := range []struct {
		name string
		t    f32.Affine2D
	}{
		{"an offset", f32.AffineId().Offset(f32.Pt(13, -7))},
		{"a scale", f32.AffineId().Scale(origin, f32.Pt(2, 4))},
		{"a scale about a corner", f32.AffineId().Scale(corner, f32.Pt(0.5, 3))},
		{"a rotation", f32.AffineId().Rotate(origin, math.Pi/3)},
		{"a shear", f32.AffineId().Shear(origin, 0.2, 0.4)},
		{"all of them at once", f32.AffineId().Offset(f32.Pt(9, 2)).Rotate(corner, math.Pi/5).Scale(origin, f32.Pt(1.5, 0.75))},
	} {
		t.Run(c.name, func(t *testing.T) {
			var o Ops
			outer := Affine(c.t).Push(&o)
			inner := Affine(c.t.Invert()).Push(&o)
			inner.Pop()
			outer.Pop()

			var composed f32.Affine2D
			for _, op := range stream(&o) {
				if op.kind != ops.TypeTransform {
					continue
				}
				decoded, push := ops.DecodeTransform(op.data)
				if !push {
					t.Fatal("a pushed transform was not encoded as pushed")
				}
				composed = composed.Mul(decoded)
			}

			for _, p := range []f32.Point{origin, corner, f32.Pt(-31, 17)} {
				got := composed.Transform(p)
				if !near(got.X, p.X) || !near(got.Y, p.Y) {
					t.Errorf("the point %v came back as %v after a transformation and its inverse", p, got)
				}
			}
		})
	}
}

// TestAnOffsetIsATranslation ties the whole-pixel convenience to the
// transformation it stands for.
//
// Offset takes a point in pixels because that is what a layout has, and it is
// the one entry point most drawing goes through. A translation that scaled, or
// one that moved by the wrong sign, would move every widget in the product by
// the same wrong amount -- which looks like a layout that needs adjusting, and
// gets adjusted, everywhere.
func TestAnOffsetIsATranslation(t *testing.T) {
	by := image.Pt(17, -23)

	var o Ops
	Offset(by).Add(&o)

	got := stream(&o)
	if len(got) != 1 {
		t.Fatalf("one operation was expected; got %v", kinds(got))
	}

	decoded, _ := ops.DecodeTransform(got[0].data)
	moved := decoded.Transform(f32.Pt(5, 7))
	want := f32.Pt(5+float32(by.X), 7+float32(by.Y))
	if !near(moved.X, want.X) || !near(moved.Y, want.Y) {
		t.Errorf("an offset of %v moved a point to %v; %v was expected", by, moved, want)
	}
}

// TestResetLeavesTheListAsNew is what makes a list reusable across frames.
//
// The list is reset and refilled sixty times a second, and it exists in this
// form so that a frame costs no allocations. That is only correct if a reset
// list draws what a new one would: anything left behind -- a stack half unwound,
// a recording still open, a byte of the last frame -- would draw the previous
// frame's remains into this one, and it would take a frame or two to appear.
func TestResetLeavesTheListAsNew(t *testing.T) {
	var reused Ops
	sample(&reused)
	macro := Record(&reused)
	Offset(image.Pt(2, 2)).Add(&reused)
	macro.Stop()

	reused.Reset()

	if got := stream(&reused); len(got) != 0 {
		t.Fatalf("a reset list still yielded %v", kinds(got))
	}

	sample(&reused)

	var fresh Ops
	sample(&fresh)

	if !equal(stream(&fresh), stream(&reused)) {
		t.Error("a reset list and a new one drew the same thing differently")
	}
}

// TestResetClearsTheStacks is the part of a reset that no byte of the stream
// shows.
//
// A frame that panicked, or one that returned early, can leave a push
// unmatched. Reset is the only thing that runs afterwards, so if it did not
// clear the stacks the next frame would inherit the identities of the last
// one's and the first pop would be rejected against a push it never made --
// turning one bad frame into every frame after it.
func TestResetClearsTheStacks(t *testing.T) {
	var o Ops
	Offset(image.Pt(1, 1)).Push(&o)
	Record(&o)

	o.Reset()

	stack := Offset(image.Pt(2, 2)).Push(&o)
	stack.Pop()

	macro := Record(&o)
	Offset(image.Pt(3, 3)).Add(&o)
	call := macro.Stop()

	var replayed Ops
	call.Add(&replayed)
	if got := stream(&replayed); len(got) != 1 || got[0].kind != ops.TypeTransform {
		t.Errorf("a recording made after a reset replayed %v", kinds(got))
	}
}

// TestDeferredOperationsComeLast is what the facility is for.
//
// A widget that has to draw over everything -- a menu, a tooltip, anything that
// escapes the box it was laid out in -- cannot be moved later in the list,
// because the list is written in layout order and it does not know what comes
// after it. Deferring is how it says "after all of this" without knowing what
// all of this is.
func TestDeferredOperationsComeLast(t *testing.T) {
	var source Ops
	macro := Record(&source)
	Offset(image.Pt(99, 99)).Add(&source)
	call := macro.Stop()

	var o Ops
	Defer(&o, call)
	Offset(image.Pt(1, 1)).Add(&o)
	Offset(image.Pt(2, 2)).Add(&o)

	got := stream(&o)
	if len(got) == 0 {
		t.Fatal("nothing was drawn")
	}

	deferred, _ := ops.DecodeTransform(got[len(got)-1].data)
	moved := deferred.Transform(f32.Pt(0, 0))
	if !near(moved.X, 99) || !near(moved.Y, 99) {
		t.Errorf("the deferred operation was not last; the stream ended with a move to %v", moved)
	}
}

// TestDeferredOperationsRunInTheOrderTheyWereDeferred fixes the order among
// themselves.
//
// Two things that both escape their box still have to agree on which is on top,
// and the answer here is the order they were deferred in -- the same order they
// were laid out in. Reversing it, which is what Go's defer does, would put the
// first menu of a screen above the last one.
func TestDeferredOperationsRunInTheOrderTheyWereDeferred(t *testing.T) {
	var source Ops
	first := Record(&source)
	Offset(image.Pt(10, 0)).Add(&source)
	firstCall := first.Stop()

	second := Record(&source)
	Offset(image.Pt(20, 0)).Add(&source)
	secondCall := second.Stop()

	var o Ops
	Defer(&o, firstCall)
	Defer(&o, secondCall)

	got := stream(&o)
	if len(got) < 2 {
		t.Fatalf("two deferred operations were expected; got %v", kinds(got))
	}

	var moves []float32
	for _, op := range got {
		if op.kind != ops.TypeTransform {
			continue
		}
		decoded, _ := ops.DecodeTransform(op.data)
		if moved := decoded.Transform(f32.Pt(0, 0)); moved.X != 0 {
			moves = append(moves, moved.X)
		}
	}

	if len(moves) != 2 || !near(moves[0], 10) || !near(moves[1], 20) {
		t.Errorf("the deferred operations ran as %v; first deferred is first executed", moves)
	}
}

// TestDeferringRestoresTheTransformItWasDeferredIn is why Defer is not simply
// an append.
//
// The deferred drawing was written by a widget standing somewhere, and it is
// executed after everything that widget's parents pushed has been popped. Run
// as it stands it would be drawn in the window's own frame, at the top left,
// rather than where the widget is -- so the transformation in force is saved at
// the point of deferral and loaded again before it runs.
func TestDeferringRestoresTheTransformItWasDeferredIn(t *testing.T) {
	var source Ops
	macro := Record(&source)
	Offset(image.Pt(3, 3)).Add(&source)
	call := macro.Stop()

	var o Ops
	stack := Offset(image.Pt(50, 60)).Push(&o)
	Defer(&o, call)
	stack.Pop()

	var saved, loaded bool
	for _, op := range stream(&o) {
		switch op.kind {
		case ops.TypeSave:
			saved = true
		case ops.TypeLoad:
			loaded = true
		}
	}

	if !saved || !loaded {
		t.Error("a deferred operation was not wrapped in a saved and restored transformation, so it will be drawn where the window begins rather than where the widget is")
	}
}

// TestAnEmptyCallDrawsNothing covers the value a caller has before it has
// recorded anything.
//
// A zero call is what a struct field holds on the first frame, and a widget that
// draws its content only once it has some would otherwise have to guard every
// use. Adding one has to be allowed and has to do nothing -- including through
// Defer, which would otherwise wrap an empty recording and leave a save and a
// load in the stream for nothing.
func TestAnEmptyCallDrawsNothing(t *testing.T) {
	var o Ops
	var empty CallOp

	empty.Add(&o)
	Defer(&o, empty)

	if got := stream(&o); len(got) != 0 {
		t.Errorf("adding and deferring a call that recorded nothing yielded %v", kinds(got))
	}
}

// TestInvalidateCmdIsACommand keeps the redraw request on the path that carries
// commands.
//
// It is delivered by the same route as a focus change or a clipboard read, and
// the route accepts a command rather than anything. Losing the method would not
// fail here, it would fail at every call site at once, with an error about an
// interface rather than about a redraw.
func TestInvalidateCmdIsACommand(t *testing.T) {
	type command interface{ ImplementsCommand() }

	var cmd command = InvalidateCmd{}
	cmd.ImplementsCommand()
}

// near reports whether two coordinates are the same point as far as a screen is
// concerned.
//
// The tolerance is a thousandth of a pixel: the encoding is single precision and
// a composition of several transformations does not return the exact bits it
// started with, but anything that rounds to the same pixel is the same drawing.
func near(a, b float32) bool {
	return math.Abs(float64(a-b)) < 1e-3
}
