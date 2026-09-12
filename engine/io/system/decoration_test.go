package system_test

import (
	"testing"

	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/op"
)

// declared is every action the set names, in the order the bits run.
//
// It is written out rather than derived, because deriving it from the same
// switch the names come from would prove only that the switch agrees with
// itself.
var declared = []struct {
	action system.Action
	name   string
}{
	{system.ActionMinimize, "ActionMinimize"},
	{system.ActionMaximize, "ActionMaximize"},
	{system.ActionUnmaximize, "ActionUnmaximize"},
	{system.ActionFullscreen, "ActionFullscreen"},
	{system.ActionRaise, "ActionRaise"},
	{system.ActionCenter, "ActionCenter"},
	{system.ActionClose, "ActionClose"},
	{system.ActionMove, "ActionMove"},
}

// TestEveryActionNamesItself is the whole point of a name.
//
// An action with no name is not a smaller failure than a wrong one: it reaches
// a log, a test message or an error as an empty string, and what the reader is
// told is that the window was asked to do nothing.
func TestEveryActionNamesItself(t *testing.T) {
	for _, c := range declared {
		if got := c.action.String(); got != c.name {
			t.Errorf("the action %d names itself %q, want %q", uint(c.action), got, c.name)
		}
	}
}

// TestASetNamesEveryActionInIt checks the part that is a set rather than a
// value.
//
// The actions arrive together -- a titlebar reports everything pressed in one
// frame, and a window is told what it may do as everything at once -- so the
// interesting case is not one action but several, and a name missing from the
// middle of a set is the one that is easiest to miss.
func TestASetNamesEveryActionInIt(t *testing.T) {
	for _, c := range []struct {
		set  system.Action
		want string
	}{
		{system.ActionMinimize | system.ActionClose, "ActionMinimize|ActionClose"},
		{system.ActionMinimize | system.ActionFullscreen, "ActionMinimize|ActionFullscreen"},
		{system.ActionRaise | system.ActionCenter, "ActionRaise|ActionCenter"},
		{system.ActionMaximize | system.ActionUnmaximize | system.ActionMove, "ActionMaximize|ActionUnmaximize|ActionMove"},
	} {
		t.Run(c.want, func(t *testing.T) {
			if got := c.set.String(); got != c.want {
				t.Errorf("the set names itself %q, want %q", got, c.want)
			}
		})
	}
}

// TestAnUndeclaredBitLeavesNoEmptyNameBehind is the shape the failure takes
// when a name is missing.
//
// A bit with nothing to call it must be passed over, not written as its empty
// name: written, it puts a separator with nothing after it into the middle of
// the line, and what a reader sees is a set whose last member has no name
// rather than a set with a member nobody declared.
func TestAnUndeclaredBitLeavesNoEmptyNameBehind(t *testing.T) {
	undeclared := system.Action(1) << 20

	if got := undeclared.String(); got != "" {
		t.Errorf("a bit nothing declares named itself %q, want nothing", got)
	}
	if got := (system.ActionMinimize | undeclared).String(); got != "ActionMinimize" {
		t.Errorf("a set carrying an undeclared bit named itself %q, want %q", got, "ActionMinimize")
	}
	if got := (undeclared | system.ActionClose).String(); got != "ActionClose" {
		t.Errorf("a set opening on an undeclared bit named itself %q, want %q", got, "ActionClose")
	}
	if got := system.Action(0).String(); got != "" {
		t.Errorf("the empty set named itself %q, want nothing", got)
	}
}

// TestEveryActionSurvivesTheOpItIsWrittenInto is the constraint that bounds the
// set.
//
// The op carries the action in a single byte, so the eight declared bits are
// all there is room for. A ninth would be written as nought and the area would
// answer to nothing -- a titlebar button that draws, presses, and does not act,
// which looks like a fault in the window rather than in the set.
func TestEveryActionSurvivesTheOpItIsWrittenInto(t *testing.T) {
	for _, c := range declared {
		t.Run(c.name, func(t *testing.T) {
			var into op.Ops
			system.ActionInputOp(c.action).Add(&into)

			var reader ops.Reader
			reader.Reset(&into.Internal)
			encoded, ok := reader.Decode()
			if !ok {
				t.Fatal("nothing was written into the op list")
			}

			if got := ops.OpType(encoded.Data[0]); got != ops.TypeActionInput {
				t.Errorf("the op was written as type %d, want %d", got, ops.TypeActionInput)
			}
			if got := system.Action(encoded.Data[1]); got != c.action {
				t.Errorf("%s came back as %d, want %d", c.name, uint(got), uint(c.action))
			}
		})
	}
}
