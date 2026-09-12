/*
Package op is the list a frame is drawn into.

Nothing here paints when it is called. A control appends operations -- set this
colour, move here, cover that rectangle -- and the finished list is handed to
the window, read once, and turned into pixels. The indirection is what makes a
frame cheap enough to draw sixty times a second: the operations are serialized
into a buffer that is reused between frames, so redrawing a screen costs no
allocations and produces no garbage for the collector to find later, in the
middle of an animation.

A list is filled, handed over, and reset:

	var ops op.Ops

	for {
		switch e := window.Event().(type) {
		case app.FrameEvent:
			ops.Reset()
			paint.ColorOp{Color: ...}.Add(&ops)
			paint.PaintOp{}.Add(&ops)
			e.Frame(&ops)
		}
	}

# State

Reading the list is running a very small machine. It has state -- a colour, a
transformation, a clipping shape -- and each operation either sets that state or
draws with it.

The state comes in two kinds, and the difference is most of this API. A colour
is simply set: the last one added wins and there is nothing to undo. A
transformation composes with whatever is already in force, so it is pushed
instead, and what is pushed has to come off again:

	stack := op.Offset(image.Pt(8, 8)).Push(&ops)
	// Everything added here is drawn eight pixels down and across.
	stack.Pop()

# Recording

Operations can be recorded rather than executed, and replayed afterwards:

	macro := op.Record(&ops)
	// Anything added here is recorded instead of drawn.
	call := macro.Stop()

	// Draw the recording, here:
	call.Add(&ops)

That is how a control draws a background. The size of a background is not known
until the content has been laid out, and painting it after the content puts it
over the content -- so the content is recorded, its size is read, the background
is painted, and the recording is replayed on top of it.
*/
package op

import (
	"encoding/binary"
	"image"
	"math"
	"time"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
)

// Ops is a list of drawing operations.
//
// It is filled by the controls of one frame and handed to the window at the end
// of it. The same value is reused for the next frame, which is what [Ops.Reset]
// is for: a list allocated per frame would hand the collector a fresh buffer
// sixty times a second, and the pause that eventually collects them lands in
// the middle of whatever is moving.
type Ops struct {
	// Internal is the serialized list, and it is exported for the engine rather
	// than for the caller.
	//
	// The operations are bytes instead of values so that appending one costs no
	// allocation and holds no pointer the collector has to trace. Nothing
	// outside this module has any reason to read it, and its shape is not part
	// of what this package promises.
	Internal ops.Ops
}

// MacroOp is a recording in progress: operations are being appended to the list
// but held back from execution until the recording is replayed.
//
// It is the value returned by [Record] and consumed by [MacroOp.Stop]. It
// carries the position the recording started at, because the recording's own
// header cannot be written until its length is known -- which is not until it
// ends.
type MacroOp struct {
	ops *ops.Ops
	// id is the recording's place on the stack of open recordings. Recordings
	// nest, because the controls that make them nest, and the identity is what
	// catches an inner one being left open.
	id ops.StackID
	// pc is where the recording began, and where its header is filled in.
	pc ops.PC
}

// CallOp replays the operations of a recording.
//
// It is a range of an existing list rather than a copy of it, so replaying a
// recording twice costs two calls and no second copy of what was recorded --
// which is what makes it usable for the repeated content of a list or a table.
//
// The zero value replays nothing. That is deliberate: it is what a control
// holds on the frame before it has recorded anything, and a zero value that had
// to be guarded at every use would be guarded at all but one.
type CallOp struct {
	// ops is the list the recording lives in, which is not necessarily the list
	// it is replayed into.
	ops *ops.Ops
	// start and end bracket the recorded operations, excluding the recording's
	// own header.
	start ops.PC
	end   ops.PC
}

// InvalidateCmd asks for another frame, at a time of the caller's choosing.
//
// The zero value asks for one immediately, and a time in the future asks for
// one then -- which is how anything that animates keeps itself running: a
// control that is part way through a transition requests the next frame while
// drawing the current one.
//
// It is a command rather than a call because it comes from inside a frame, and
// a frame is being drawn when it is issued. Requesting the redraw directly from
// there would ask the window to start a frame while it is finishing one.
type InvalidateCmd struct {
	// At is when the frame is wanted. The zero time means now.
	At time.Time
}

// TransformOp moves, scales, rotates or shears everything drawn after it.
//
// It is a transformation rather than a position because those are the same
// thing once things nest: a control is drawn in the frame its parent chose, and
// it chooses a frame for its own children by composing onto that one. Absolute
// coordinates would require every control to know where it had been put.
type TransformOp struct {
	t f32.Affine2D
}

// TransformStack is a pushed [TransformOp], and the right to take it back off.
//
// It is a value the caller has to hold rather than a depth the list counts,
// because pushes and pops are written by different functions at different
// depths of the layout. Holding the identity is what lets an unbalanced pop be
// caught where it was made, instead of showing up as a drawing that is offset
// by however much a caller further up forgot.
type TransformStack struct {
	id ops.StackID
	// macroID is the recording that was open when the push was made. A push and
	// a pop that straddle a recording boundary are rejected against it.
	macroID uint32
	ops     *ops.Ops
}

// Defer executes c after everything else in the list, including anything
// deferred before it.
//
// It is how a control draws outside the box it was laid out in -- a menu, a
// tooltip, anything that has to cover what comes after it. Such a control
// cannot simply draw later, because the list is written in layout order and it
// does not know what is still to come.
//
// The transformation in force is saved here and restored before c runs, so the
// deferred drawing appears where the control is. Everything else -- the colour,
// the clip -- is reset, because it belonged to whatever was being drawn at the
// time and that is long finished by then.
//
// Deferred operations run in the order they were deferred, which is the order
// they were laid out in. That is the opposite of Go's defer, and it is the
// order that puts the later of two overlapping menus on top.
func Defer(o *Ops, c CallOp) {
	if c.ops == nil {
		return
	}
	// The transformation is saved now, while it is still in force, and loaded
	// from inside the wrapper -- which is the only place that runs at the right
	// moment, since the wrapper is what execution is postponed to.
	state := ops.Save(&o.Internal)
	macro := Record(o)
	state.Load()
	c.Add(o)
	wrapped := macro.Stop()

	// A deferral is the marker followed by the wrapped recording: the reader
	// takes the call that comes after the marker and sets it aside.
	data := ops.Write(&o.Internal, ops.TypeDeferLen)
	data[0] = byte(ops.TypeDefer)
	wrapped.Add(o)
}

// Reset empties the list for the next frame.
//
// It clears the stacks along with the operations, which matters after a frame
// that ended badly: a push left unmatched by a panic or an early return would
// otherwise be inherited by the next frame, and the first honest pop of that
// frame would be rejected against a push it never made -- turning one bad frame
// into every frame after it.
//
// Any recording made from this list stops being replayable. Adding one
// afterwards is not caught, and the list it was added to fails when it is read.
func (o *Ops) Reset() {
	ops.Reset(&o.Internal)
}

// Record starts a recording: operations added from here on are held back until
// the recording is replayed.
func Record(o *Ops) MacroOp {
	m := MacroOp{
		ops: &o.Internal,
		id:  ops.PushMacro(&o.Internal),
		pc:  ops.PCFor(&o.Internal),
	}
	// The header says where the recording ends, which is not known yet. The
	// room for it is taken now so that the recorded operations begin at a fixed
	// distance from the start, and Stop fills it in.
	data := ops.Write(m.ops, ops.TypeMacroLen)
	data[0] = byte(ops.TypeMacro)
	return m
}

// Stop ends the recording and returns the call that replays it.
//
// A recording that is never stopped has no end written into its header, and the
// reader then treats it as running to the end of the list -- so everything
// added after it is held back with it and none of it is drawn. That is the safe
// direction to fail in: the other one would execute operations that were meant
// to be measured, drawing a measurement onto the screen.
func (m MacroOp) Stop() CallOp {
	ops.PopMacro(m.ops, m.id)
	ops.FillMacro(m.ops, m.pc)
	return CallOp{
		ops: m.ops,
		// Past the header: it describes the recording and is not part of it.
		start: m.pc.Add(ops.TypeMacro),
		end:   ops.PCFor(m.ops),
	}
}

// Add replays the recorded operations into o.
//
// The recording may live in another list, and it may be replayed any number of
// times. Adding a call whose list has since been [Ops.Reset] is not rejected
// here, and fails when o is read.
func (c CallOp) Add(o *Ops) {
	if c.ops == nil {
		return
	}
	ops.AddCall(&o.Internal, c.ops, c.start, c.end)
}

// Offset is the transformation that moves everything by off.
//
// It takes whole pixels because that is what a layout has finished with, and it
// is the transformation nearly all drawing goes through: a control is placed by
// moving to where it goes and drawing at the origin.
func Offset(off image.Point) TransformOp {
	offf := f32.Pt(float32(off.X), float32(off.Y))
	return Affine(f32.AffineId().Offset(offf))
}

// Affine is the transformation a, for the cases an offset cannot express --
// scaling, rotation, shear, or any composition of them.
func Affine(a f32.Affine2D) TransformOp {
	return TransformOp{t: a}
}

// Push applies the transformation and returns the stack entry that takes it
// back off.
//
// This is the one to reach for. A transformation that is not taken back stays
// in force for everything drawn afterwards, including everything drawn by the
// caller's caller, so the pop belongs next to the push where it can be seen.
func (t TransformOp) Push(o *Ops) TransformStack {
	id, macroID := ops.PushOp(&o.Internal, ops.TransStack)
	t.add(o, true)
	return TransformStack{ops: &o.Internal, id: id, macroID: macroID}
}

// Add applies the transformation without pushing it, leaving it in force until
// the enclosing push is popped.
//
// It is for the caller that has already pushed and wants to move again within
// the same frame of reference -- a row placing one item after another, rather
// than a control giving a child its own space.
func (t TransformOp) Add(o *Ops) {
	t.add(o, false)
}

// add writes the transformation, marked as pushed or not.
//
// The six elements go out in row-major order at a fixed distance from the
// start, so that the reader can take them without decoding anything before
// them. Single precision is what the rasterizer works in; nothing is gained by
// carrying more of the number than the thing reading it can use.
func (t TransformOp) add(o *Ops, push bool) {
	data := ops.Write(&o.Internal, ops.TypeTransformLen)
	data[0] = byte(ops.TypeTransform)
	if push {
		data[1] = 1
	}

	elems := data[2:]
	bo := binary.LittleEndian
	a, b, c, d, e, f := t.t.Elems()
	bo.PutUint32(elems[4*0:], math.Float32bits(a))
	bo.PutUint32(elems[4*1:], math.Float32bits(b))
	bo.PutUint32(elems[4*2:], math.Float32bits(c))
	bo.PutUint32(elems[4*3:], math.Float32bits(d))
	bo.PutUint32(elems[4*4:], math.Float32bits(e))
	bo.PutUint32(elems[4*5:], math.Float32bits(f))
}

// Pop takes the transformation back off, restoring the one that was in force
// before the push.
//
// It panics if this is not the transformation on top, or if the push was made
// inside a different recording. Both are the same mistake seen from two sides,
// and both are worth a panic: an unbalanced stack does not fail where it
// happened, it moves something drawn later by an amount that depends on how
// many times the recording around it is replayed.
func (t TransformStack) Pop() {
	ops.PopOp(t.ops, ops.TransStack, t.id, t.macroID)
	data := ops.Write(t.ops, ops.TypePopTransformLen)
	data[0] = byte(ops.TypePopTransform)
}

// ImplementsCommand marks the request as one the window accepts.
func (InvalidateCmd) ImplementsCommand() {}
