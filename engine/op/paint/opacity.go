package paint

import (
	"encoding/binary"
	"math"

	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/op"
)

// OpacityStack is a layer begun by [PushOpacity] and ended by Pop.
//
// It holds where the layer began rather than the opacity itself, because what
// has to be checked is the pairing: a layer left open swallows the rest of the
// frame, and a Pop that belongs to a different layer would end the wrong one.
type OpacityStack struct {
	id      ops.StackID
	macroID uint32
	ops     *ops.Ops
}

// PushOpacity begins a layer drawn at the given opacity, in the range [0;1].
//
// Everything until [OpacityStack.Pop] is drawn into a picture of its own, and
// that picture is then blended in. The two steps are what makes this a layer
// rather than a translucent brush: two overlapping marks inside a layer are one
// translucent object, whereas the same two marks painted translucently show
// each through the other, and their overlap comes out darker than either. Where
// a control fades out as a whole -- disabled, dismissed, half way through
// appearing -- that difference is the difference between a faded control and a
// faded control with a bruise where its parts meet.
//
// The second picture is what it costs, so a single mark is better faded by its
// own colour's alpha.
//
// A value outside the range is clamped rather than refused, because a caller
// arrives outside it by arithmetic and not by intent: an animation that
// overshoots its end, a fraction of a duration that came out just past one.
// Clamping draws the frame nearest what was asked; refusing would drop a frame
// in the middle of a movement, which is the one place it would be seen.
func PushOpacity(o *op.Ops, opacity float32) OpacityStack {
	opacity = min(max(opacity, 0), 1)

	id, macroID := ops.PushOp(&o.Internal, ops.OpacityStack)
	data := ops.Write(&o.Internal, ops.TypePushOpacityLen)
	data[0] = byte(ops.TypePushOpacity)
	binary.LittleEndian.PutUint32(data[1:], math.Float32bits(opacity))

	return OpacityStack{ops: &o.Internal, id: id, macroID: macroID}
}

// Pop ends the layer and blends it into what was drawn before it.
func (t OpacityStack) Pop() {
	ops.PopOp(t.ops, ops.OpacityStack, t.id, t.macroID)
	data := ops.Write(t.ops, ops.TypePopOpacityLen)
	data[0] = byte(ops.TypePopOpacity)
}
