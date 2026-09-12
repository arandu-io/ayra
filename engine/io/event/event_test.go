package event_test

import (
	"testing"

	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/op"
)

// TestOpRecordsTheTagItself fixes that an identity survives the frame it was
// declared in.
//
// A tag is written into the operation list while drawing and read back when the
// list is routed, and the state kept for a handler is found under it. Anything
// that copies or rebuilds the value in between produces a key that no longer
// matches the one the handler holds, and the handler is then a new one every
// frame: never focused, never pressed, and impossible to debug from the
// symptom.
func TestOpRecordsTheTagItself(t *testing.T) {
	var handler int
	tag := &handler

	var o op.Ops
	event.Op(&o, tag)

	recorded := decode(&o)
	if len(recorded) != 1 {
		t.Fatalf("declaring one tag recorded %d operations, want 1", len(recorded))
	}
	if kind := ops.OpType(recorded[0].Data[0]); kind != ops.TypeInput {
		t.Errorf("declaring a tag recorded %v, want %v", kind, ops.TypeInput)
	}
	if refs := recorded[0].Refs; len(refs) != 1 || refs[0] != event.Tag(tag) {
		t.Errorf("the tag read back is %v, want the one that was declared", refs)
	}
}

// TestTwoTagsStayApart fixes that identity is not flattened on the way in.
//
// Two handlers are two tags and have to stay two. Were they to arrive as one --
// compared by what they point at rather than by where they point -- a press on
// either would be answered by both, which is the failure that looks like a
// control acting at a distance.
func TestTwoTagsStayApart(t *testing.T) {
	first, second := new(int), new(int)

	var o op.Ops
	event.Op(&o, first)
	event.Op(&o, second)

	recorded := decode(&o)
	if len(recorded) != 2 {
		t.Fatalf("declaring two tags recorded %d operations, want 2", len(recorded))
	}
	if recorded[0].Refs[0] == recorded[1].Refs[0] {
		t.Error("two handlers were recorded under one tag")
	}
	if recorded[0].Refs[0] != event.Tag(first) || recorded[1].Refs[0] != event.Tag(second) {
		t.Error("the tags were recorded in an order other than the one they were declared in")
	}
}

// TestOpRefusesANilTag fixes where that mistake is reported.
//
// Nothing can be keyed by nil, so the routing would fail on it as well -- but it
// would fail while walking a list that no longer says which call put the value
// there. Refusing at the call keeps the report on the line that can be fixed.
func TestOpRefusesANilTag(t *testing.T) {
	var o op.Ops

	func() {
		defer func() {
			if recover() == nil {
				t.Error("declaring a nil tag was accepted, want a panic")
			}
		}()
		event.Op(&o, nil)
	}()

	if recorded := decode(&o); len(recorded) != 0 {
		t.Errorf("a refused tag left %d operations behind, want 0", len(recorded))
	}
}

// decode reads back what was written into an operation list.
func decode(o *op.Ops) []ops.EncodedOp {
	var reader ops.Reader
	reader.Reset(&o.Internal)

	var recorded []ops.EncodedOp
	for {
		encoded, ok := reader.Decode()
		if !ok {
			return recorded
		}
		recorded = append(recorded, encoded)
	}
}
