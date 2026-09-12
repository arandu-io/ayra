package byteslice_test

import (
	"testing"
	"unsafe"

	"github.com/arandu-io/ayra/engine/internal/byteslice"
)

// What a view has to be, fixed by test, because the failure here is not a red
// run.
//
// Everything in this package hands out a second name for memory somebody else
// owns. A view one byte too long reads past the value; a view that silently
// copied would take writes that never reach the original and draw a frame from
// stale data. Neither shows up as a panic at the point of the mistake -- the
// first corrupts whatever follows, the second produces a picture that is merely
// wrong -- so the properties are asserted here instead of trusted.

// A struct wide enough to have padding and more than one field, which is the
// shape the uniforms of a drawing program actually have.
type uniforms struct {
	scale  [2]float32
	offset [2]float32
	flag   uint8
}

func TestAnEmptySliceViewsNothing(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		view := byteslice.Slice[float32](nil)
		if len(view) != 0 {
			t.Errorf("a nil slice viewed %d bytes, want 0", len(view))
		}
	})

	t.Run("empty", func(t *testing.T) {
		view := byteslice.Slice(make([]float32, 0))
		if len(view) != 0 {
			t.Errorf("an empty slice viewed %d bytes, want 0", len(view))
		}
	})

	t.Run("empty with room", func(t *testing.T) {
		view := byteslice.Slice(make([]float32, 0, 8))
		if len(view) != 0 {
			t.Errorf("an empty slice viewed %d bytes, want 0", len(view))
		}
	})

	t.Run("truncated to empty", func(t *testing.T) {
		full := []uint16{1, 2, 3, 4}
		view := byteslice.Slice(full[:0])
		if len(view) != 0 {
			t.Errorf("a slice truncated to nothing viewed %d bytes, want 0", len(view))
		}
	})
}

func TestTheViewIsAsWideAsTheSliceIsLong(t *testing.T) {
	t.Run("byte", func(t *testing.T) { widthIsLengthTimesElement(t, []byte{1, 2, 3}) })
	t.Run("uint16", func(t *testing.T) { widthIsLengthTimesElement(t, []uint16{1, 2, 3, 4, 5}) })
	t.Run("float32", func(t *testing.T) { widthIsLengthTimesElement(t, []float32{1, 2, 3, 4}) })
	t.Run("uint64", func(t *testing.T) { widthIsLengthTimesElement(t, []uint64{1, 2}) })
	t.Run("struct", func(t *testing.T) { widthIsLengthTimesElement(t, make([]uniforms, 3)) })
	t.Run("array", func(t *testing.T) { widthIsLengthTimesElement(t, make([][4]float32, 6)) })
	t.Run("one element", func(t *testing.T) { widthIsLengthTimesElement(t, []float32{1}) })
	// Spare room is the case a literal never produces, and the one where a
	// width read off the capacity would hand a driver more bytes than the
	// caller has filled.
	t.Run("with spare room", func(t *testing.T) { widthIsLengthTimesElement(t, make([]float32, 3, 9)) })
}

// widthIsLengthTimesElement is the arithmetic every caller depends on: the
// count it passes to a driver is the length of this view, and a driver reads
// exactly that many bytes from the address it was given.
func widthIsLengthTimesElement[T any](t *testing.T, s []T) {
	t.Helper()

	var zero T
	want := int(unsafe.Sizeof(zero)) * len(s)

	view := byteslice.Slice(s)
	if len(view) != want {
		t.Errorf("%d elements of %d bytes viewed %d bytes, want %d", len(s), unsafe.Sizeof(zero), len(view), want)
	}
}

// TestAZeroWidthElementViewsNothing covers the element that has no bytes at
// all, because the size is a multiplier and a zero multiplier is where
// arithmetic written against the usual case gives an answer nobody meant.
func TestAZeroWidthElementViewsNothing(t *testing.T) {
	view := byteslice.Slice(make([]struct{}, 4))
	if len(view) != 0 {
		t.Errorf("four elements of no width viewed %d bytes, want 0", len(view))
	}
}

// TestWritingThroughTheViewChangesTheSlice is the whole reason this package
// exists: the view aliases the elements rather than copying them.
//
// Written in bytes of 0xFF and read back as the widest value the element can
// hold, so the assertion says nothing about which end the machine puts first.
func TestWritingThroughTheViewChangesTheSlice(t *testing.T) {
	s := make([]uint32, 4)

	view := byteslice.Slice(s)
	for i := range view {
		view[i] = 0xFF
	}

	for i, v := range s {
		if v != ^uint32(0) {
			t.Errorf("element %d is %#x after the view was filled, want %#x", i, v, ^uint32(0))
		}
	}
}

// TestWritingThroughTheSliceChangesTheView is the same aliasing read from the
// other side, which is the direction a driver uses: the caller fills a typed
// slice and the view it handed over earlier has to see it.
func TestWritingThroughTheSliceChangesTheView(t *testing.T) {
	s := make([]uint16, 3)
	view := byteslice.Slice(s)

	for i := range s {
		s[i] = ^uint16(0)
	}

	for i, b := range view {
		if b != 0xFF {
			t.Errorf("byte %d is %#x after the slice was filled, want 0xFF", i, b)
		}
	}
}

// TestTheViewStartsAtTheFirstElement fixes the address, which is the other half
// of what a driver is told. A view of the right length taken from the wrong
// place reads the right amount of the wrong memory.
func TestTheViewStartsAtTheFirstElement(t *testing.T) {
	s := []float32{1, 2, 3}

	view := byteslice.Slice(s)

	if unsafe.Pointer(&view[0]) != unsafe.Pointer(&s[0]) {
		t.Error("the view does not begin at the first element, so it is a copy or an offset")
	}
}

// TestTheViewKeepsTheSlicesSpareRoom fixes that a view describes the whole
// allocation and not only the part in use, which is what lets an appending
// caller reach the room the slice already owns.
func TestTheViewKeepsTheSlicesSpareRoom(t *testing.T) {
	s := make([]float32, 2, 8)

	view := byteslice.Slice(s)

	want := 8 * int(unsafe.Sizeof(float32(0)))
	if cap(view) != want {
		t.Errorf("the view has room for %d bytes, want %d", cap(view), want)
	}
}

func TestAViewOfAValueIsItsWholeWidth(t *testing.T) {
	var u uniforms

	view := byteslice.View(&u)

	if len(view) != int(unsafe.Sizeof(u)) {
		t.Errorf("a value of %d bytes viewed %d", unsafe.Sizeof(u), len(view))
	}
	if unsafe.Pointer(&view[0]) != unsafe.Pointer(&u) {
		t.Error("the view does not begin at the value")
	}
}

// TestWritingThroughAValuesViewChangesTheValue is the aliasing again, for the
// call that uniforms are handed over with.
func TestWritingThroughAValuesViewChangesTheValue(t *testing.T) {
	u := uniforms{flag: 1}

	view := byteslice.View(&u)
	for i := range view {
		view[i] = 0
	}

	if u.flag != 0 {
		t.Errorf("the field is %d after the view was cleared, want 0", u.flag)
	}
}

// TestAValueOfNoWidthViewsNothing covers the empty struct, which a caller can
// reach by instantiating a generic type with one and which must not be read as
// though it had a byte.
func TestAValueOfNoWidthViewsNothing(t *testing.T) {
	var nothing struct{}

	view := byteslice.View(&nothing)

	if len(view) != 0 {
		t.Errorf("a value of no width viewed %d bytes, want 0", len(view))
	}
}

// A nil pointer handed to View is not asserted here, and the reason is worth
// leaving behind. The runtime stops the program at the call, which is the
// behaviour the doc comment states, but how it stops depends on the build: with
// the race detector on, the check runs one frame deeper inside the runtime and
// the stop is fatal rather than recoverable, so a test that recovered from it
// would pass in one configuration and kill the suite in the other. What must
// not happen -- a view of address zero being handed back -- is the runtime's
// guarantee, not this package's.
