package semantic

import (
	"testing"

	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/op"
)

// What a control says is checked on the list, never on the call.
//
// Every operation here ends as bytes that something else walks later, and the
// walk is driven by the tag in the first byte and the length that tag declares.
// A test that only called Add would pass on an operation that wrote the wrong
// tag, wrote the wrong number of bytes, or wrote nothing -- and the first two
// do not fail where they happen. They move the next operation off its boundary,
// so the failure surfaces as a different operation arriving mangled, which is
// the hardest kind of fault to trace back to the line that caused it.
//
// So these write, read back, and compare.

// written is one operation as it came off the list.
type written struct {
	kind ops.OpType
	data []byte
	refs []any
}

// list answers every operation add wrote, in the order it wrote them.
func list(t *testing.T, add func(o *op.Ops)) []written {
	t.Helper()

	o := new(op.Ops)
	add(o)

	var reader ops.Reader
	reader.Reset(&o.Internal)

	var found []written
	for {
		encoded, more := reader.Decode()
		if !more {
			return found
		}
		found = append(found, written{
			kind: ops.OpType(encoded.Data[0]),
			data: encoded.Data,
			refs: encoded.Refs,
		})
	}
}

// only answers the one operation add wrote, and fails if it wrote any other
// number of them.
func only(t *testing.T, add func(o *op.Ops)) written {
	t.Helper()

	found := list(t, add)
	if len(found) != 1 {
		t.Fatalf("%d operations reached the list, want exactly one", len(found))
	}
	return found[0]
}

// words answers the text an operation carries beside the list.
func words(t *testing.T, w written) string {
	t.Helper()

	if len(w.refs) != 1 {
		t.Fatalf("operation %d carries %d references, want one", w.kind, len(w.refs))
	}
	held, ok := w.refs[0].(*string)
	if !ok {
		t.Fatalf("operation %d carries a %T, want text", w.kind, w.refs[0])
	}
	return *held
}

// TestEveryOperationReachesTheList is the floor everything else stands on.
//
// An operation that writes nothing is silent in exactly the way a missing
// description is: the screen looks right, the control works, and what reads it
// aloud has nothing to say about it.
func TestEveryOperationReachesTheList(t *testing.T) {
	for _, c := range []struct {
		name string
		add  func(o *op.Ops)
		kind ops.OpType
	}{
		{"a label", func(o *op.Ops) { LabelOp("Save").Add(o) }, ops.TypeSemanticLabel},
		{"a description", func(o *op.Ops) { DescriptionOp("Writes the file").Add(o) }, ops.TypeSemanticDesc},
		{"a class", func(o *op.Ops) { Button.Add(o) }, ops.TypeSemanticClass},
		{"a selected state", func(o *op.Ops) { SelectedOp(true).Add(o) }, ops.TypeSemanticSelected},
		{"an enabled state", func(o *op.Ops) { EnabledOp(true).Add(o) }, ops.TypeSemanticEnabled},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := only(t, c.add)

			if w.kind != c.kind {
				t.Fatalf("%s was tagged %d, want %d", c.name, w.kind, c.kind)
			}
			if len(w.data) != int(c.kind.Size()) {
				t.Errorf("%s took %d bytes and its tag declares %d; everything written after it is read off its boundary",
					c.name, len(w.data), c.kind.Size())
			}
		})
	}
}

// TestWhatWasWrittenReadsBack checks the payload rather than the envelope.
func TestWhatWasWrittenReadsBack(t *testing.T) {
	t.Run("a label keeps its words", func(t *testing.T) {
		w := only(t, func(o *op.Ops) { LabelOp("Save").Add(o) })
		if said := words(t, w); said != "Save" {
			t.Errorf("the label reads back as %q", said)
		}
	})

	t.Run("a description keeps its words", func(t *testing.T) {
		w := only(t, func(o *op.Ops) { DescriptionOp("Writes the file").Add(o) })
		if said := words(t, w); said != "Writes the file" {
			t.Errorf("the description reads back as %q", said)
		}
	})

	t.Run("a class keeps its kind", func(t *testing.T) {
		for _, class := range []ClassOp{Unknown, Button, CheckBox, Editor, RadioButton, Switch} {
			w := only(t, func(o *op.Ops) { class.Add(o) })
			if got := ClassOp(w.data[1]); got != class {
				t.Errorf("%v reads back as %v", class, got)
			}
		}
	})

	t.Run("both states travel in both directions", func(t *testing.T) {
		for _, on := range []bool{true, false} {
			selected := only(t, func(o *op.Ops) { SelectedOp(on).Add(o) })
			if got := selected.data[1] != 0; got != on {
				t.Errorf("a selected state of %t reads back as %t", on, got)
			}

			enabled := only(t, func(o *op.Ops) { EnabledOp(on).Add(o) })
			if got := enabled.data[1] != 0; got != on {
				t.Errorf("an enabled state of %t reads back as %t", on, got)
			}
		}
	})
}

// TestOperationsDoNotDisturbEachOther writes the whole vocabulary at once.
//
// A control writes several of these in a row against one node, so the list they
// share is the ordinary case and not an unusual one. Each operation declares
// its own length, and the one after it is read from where that length ends: an
// operation that wrote a byte too few or too many would pass every test above
// on its own and take the rest of the node with it here.
func TestOperationsDoNotDisturbEachOther(t *testing.T) {
	found := list(t, func(o *op.Ops) {
		LabelOp("Remember me").Add(o)
		CheckBox.Add(o)
		DescriptionOp("Stays signed in on this device").Add(o)
		SelectedOp(true).Add(o)
		EnabledOp(false).Add(o)
	})

	want := []ops.OpType{
		ops.TypeSemanticLabel,
		ops.TypeSemanticClass,
		ops.TypeSemanticDesc,
		ops.TypeSemanticSelected,
		ops.TypeSemanticEnabled,
	}
	if len(found) != len(want) {
		t.Fatalf("%d operations reached the list, want %d", len(found), len(want))
	}
	for i, kind := range want {
		if found[i].kind != kind {
			t.Fatalf("operation %d of the node is tagged %d, want %d; the ones after it are read off the wrong boundary",
				i, found[i].kind, kind)
		}
	}

	if said := words(t, found[0]); said != "Remember me" {
		t.Errorf("the label reads back as %q", said)
	}
	if said := words(t, found[2]); said != "Stays signed in on this device" {
		t.Errorf("the description reads back as %q", said)
	}
	if got := ClassOp(found[1].data[1]); got != CheckBox {
		t.Errorf("the class reads back as %v", got)
	}
	if found[3].data[1] == 0 {
		t.Error("a ticked box reads back as clear")
	}
	if found[4].data[1] != 0 {
		t.Error("an unavailable control reads back as available")
	}
}

// TestTheZeroClassIsUnknown pins which class a node gets for free.
//
// A node is described by whatever was written under it, and a class nobody
// wrote is the zero value. If that zero were the first kind of control in the
// set rather than the absence of one, every node that never said what it was
// would announce as that kind -- confidently, and for a reason no caller could
// see in its own code.
func TestTheZeroClassIsUnknown(t *testing.T) {
	var unwritten ClassOp

	if unwritten != Unknown {
		t.Errorf("the zero class is %v", unwritten)
	}
	if Unknown == Button {
		t.Error("the absence of a class and the first class in the set are the same value")
	}

	w := only(t, func(o *op.Ops) { unwritten.Add(o) })
	if got := ClassOp(w.data[1]); got != Unknown {
		t.Errorf("a node written from the zero class announces as %v", got)
	}
}

// TestAnEmptyLabelIsWrittenAsItIs pins where the decision to stay quiet lives.
//
// A control with no name has nothing worth stopping on, but this package is not
// where that is decided: it carries what it was handed, and the caller that
// built an empty name is the one that knows whether there was supposed to be
// one. What has to hold here is that the empty case is ordinary -- one
// operation, the right tag, and the operations after it still on their
// boundaries.
func TestAnEmptyLabelIsWrittenAsItIs(t *testing.T) {
	found := list(t, func(o *op.Ops) {
		LabelOp("").Add(o)
		Button.Add(o)
	})

	if len(found) != 2 {
		t.Fatalf("%d operations reached the list, want two", len(found))
	}
	if found[0].kind != ops.TypeSemanticLabel {
		t.Fatalf("the empty label is tagged %d, want %d", found[0].kind, ops.TypeSemanticLabel)
	}
	if said := words(t, found[0]); said != "" {
		t.Errorf("the empty label reads back as %q", said)
	}
	if found[1].kind != ops.TypeSemanticClass {
		t.Errorf("the operation after an empty label is tagged %d, want %d", found[1].kind, ops.TypeSemanticClass)
	}
}

// TestEveryClassNamesItself covers the set, and covers additions to it.
//
// The name is what a failing test prints when it reports the wrong kind of
// control, so a class without one turns that report into a number the reader
// has to go and look up.
func TestEveryClassNamesItself(t *testing.T) {
	for _, c := range []struct {
		class ClassOp
		name  string
	}{
		{Unknown, "Unknown"},
		{Button, "Button"},
		{CheckBox, "CheckBox"},
		{Editor, "Editor"},
		{RadioButton, "RadioButton"},
		{Switch, "Switch"},
	} {
		if got := c.class.String(); got != c.name {
			t.Errorf("the class %d names itself %q, want %q", int(c.class), got, c.name)
		}
	}

	// Switch is the last of the set, and this is what notices when it stops
	// being: a class declared after it and left unnamed prints as a number.
	if int(Switch) != len(classNames)-1 {
		t.Errorf("the set runs to %d and %d classes are named; a class was added without a name",
			int(Switch), len(classNames))
	}
}

// TestAClassFromOutsideTheSetStillPrints keeps a diagnostic readable.
//
// A class arrives back off the list as one byte, so a value the set does not
// name is reachable without any caller having written one. This method is also
// what a printer calls while reporting something else -- the failure message
// that says which kind of control turned up is exactly where an unnamed value
// appears -- and a method that panicked there would replace the message
// somebody was reading with a note about a second failure.
func TestAClassFromOutsideTheSetStillPrints(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("naming a class from outside the set panicked: %v", err)
		}
	}()

	outside := ClassOp(len(classNames) + 7)
	if got := outside.String(); got == "" {
		t.Error("a class from outside the set names itself with nothing")
	}
	if got := ClassOp(-1).String(); got == "" {
		t.Error("a negative class names itself with nothing")
	}
}
