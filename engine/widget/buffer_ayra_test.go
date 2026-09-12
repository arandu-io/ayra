package widget

import (
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

// bufferText reads the whole buffer back.
//
// It is a helper rather than a method because reading everything is not what
// the buffer is for: the text is kept in two pieces with a hole between them,
// and joining them costs a copy of the document. An editor never needs that --
// it reads the few bytes around the caret -- so only a test asks.
func bufferText(t *testing.T, e *editBuffer) string {
	t.Helper()
	got := make([]byte, e.Size())
	n, err := e.ReadAt(got, 0)
	if err != nil && err != io.EOF {
		t.Fatalf("reading the buffer back: %v", err)
	}
	if int64(n) != e.Size() {
		t.Fatalf("read %d of %d bytes back", n, e.Size())
	}
	return string(got[:n])
}

// bufferType inserts a string at a byte offset, which is what a keystroke is.
func bufferType(e *editBuffer, at int64, s string) {
	e.ReplaceRunes(at, 0, s)
}

// TestTypingAndDeletingLeavesTheBufferAsItStarted is the property the gap
// costs its complexity for.
//
// The gap moves to wherever the caret is and the text is copied around it, so
// every edit rearranges the storage. Content that survives that is content the
// rearrangement did not corrupt, and an off-by-one in the copy shows up here as
// a duplicated or swallowed run of bytes rather than as a crash -- which is the
// worst kind of corruption, because the document still reads as text.
func TestTypingAndDeletingLeavesTheBufferAsItStarted(t *testing.T) {
	var e editBuffer

	bufferType(&e, 0, "the quick brown fox")
	if got := bufferText(t, &e); got != "the quick brown fox" {
		t.Fatalf("after typing the buffer holds %q", got)
	}

	// Into the middle, which is the case the gap has to move for. Appending at
	// the end never moves it and would prove nothing.
	bufferType(&e, 9, " dark")
	if got := bufferText(t, &e); got != "the quick dark brown fox" {
		t.Fatalf("after an insertion in the middle the buffer holds %q", got)
	}

	e.ReplaceRunes(9, 5, "")
	if got := bufferText(t, &e); got != "the quick brown fox" {
		t.Fatalf("after deleting what was typed the buffer holds %q", got)
	}

	// Backwards, which is what the backspace key is: the same call with a
	// negative count, deleting the runes before the offset rather than after
	// it.
	e.ReplaceRunes(int64(len("the quick brown fox")), -4, "")
	if got := bufferText(t, &e); got != "the quick brown" {
		t.Fatalf("after a backwards delete the buffer holds %q", got)
	}

	e.ReplaceRunes(0, int64(utf8.RuneCountInString("the quick brown")), "")
	if got := bufferText(t, &e); got != "" {
		t.Fatalf("after deleting everything the buffer holds %q", got)
	}
	if e.Size() != 0 {
		t.Fatalf("the empty buffer reports %d bytes", e.Size())
	}
}

// TestChangedIsReportedOnceForEachEdit is what a redraw is decided by.
//
// The editor asks once per frame and draws if the answer is yes, so an answer
// that stays yes repaints for ever and an answer that never comes leaves the
// typed character invisible until something else asks for a frame.
func TestChangedIsReportedOnceForEachEdit(t *testing.T) {
	var e editBuffer

	if e.Changed() {
		t.Error("an untouched buffer reports a change")
	}

	bufferType(&e, 0, "a")
	if !e.Changed() {
		t.Error("typing a character is not reported as a change")
	}
	if e.Changed() {
		t.Error("the same change is reported twice")
	}

	// An edit that replaces nothing with nothing is not an edit. Reporting it
	// would repaint on every arrow key, because moving the caret goes through
	// the same call with an empty replacement.
	e.ReplaceRunes(0, 0, "")
	if e.Changed() {
		t.Error("an edit that changes no bytes is reported as a change")
	}
}

// TestReadingPastTheEndAnswersRatherThanCrashing fixes the boundary the
// storage is most often asked about.
//
// The caller is a text view walking the document, and it reads a fixed-size
// window around a byte offset without knowing how much is left -- the last
// window of every document runs off the end. An offset beyond the end is the
// same request one byte later, so answering one and faulting on the other
// would make a crash out of a scroll that went one line too far.
func TestReadingPastTheEndAnswersRatherThanCrashing(t *testing.T) {
	var e editBuffer
	bufferType(&e, 0, "abc")

	for _, c := range []struct {
		name   string
		offset int64
		want   string
	}{
		{"at the end", 3, ""},
		{"one past the end", 4, ""},
		{"far past the end", 4096, ""},
		{"the last byte", 2, "c"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := make([]byte, 8)
			n, err := e.ReadAt(got, c.offset)
			if err != io.EOF {
				t.Errorf("reading %d bytes from offset %d answered %v, want EOF", len(got), c.offset, err)
			}
			if string(got[:n]) != c.want {
				t.Errorf("reading from offset %d answered %q, want %q", c.offset, got[:n], c.want)
			}
		})
	}

	// A negative offset is a caret arithmetic mistake upstream. It has no
	// answer, and the one thing it must not do is index backwards into the
	// storage and hand back whatever is there.
	if _, err := e.ReadAt(make([]byte, 4), -1); err == nil {
		t.Error("reading from a negative offset answered no error")
	}

	// An empty destination is a real call: it is what a caller with nothing
	// left to fill makes, and it asks for no bytes rather than for the end.
	if n, err := e.ReadAt(nil, 0); n != 0 || err != nil {
		t.Errorf("reading into no space answered %d, %v; want 0, nil", n, err)
	}
}

// TestReadingSpansTheGapWhereverItSits is the other half of the same contract.
//
// The gap sits at the last caret position, so the bytes a reader asks for are
// routinely split across it. Reading is the only operation that has to pretend
// the hole is not there, and it is the operation that is asked for the
// document's own text.
func TestReadingSpansTheGapWhereverItSits(t *testing.T) {
	const text = "one two three four"
	for at := 0; at <= len(text); at++ {
		var e editBuffer
		bufferType(&e, 0, text)
		// Move the gap by making an edit there and undoing it, which is what
		// a caret arriving and a character being typed and erased does.
		bufferType(&e, int64(at), "x")
		e.ReplaceRunes(int64(at), 1, "")

		if got := bufferText(t, &e); got != text {
			t.Fatalf("with the gap at %d the buffer reads %q, want %q", at, got, text)
		}
		for offset := 0; offset <= len(text); offset++ {
			got := make([]byte, len(text))
			n, _ := e.ReadAt(got, int64(offset))
			if want := text[offset:]; string(got[:n]) != want {
				t.Fatalf("with the gap at %d, reading from %d gave %q, want %q", at, offset, got[:n], want)
			}
		}
	}
}

// TestAByteOffsetIsNotARuneCount is the confusion this type is built to hold.
//
// One call carries both: where to edit is a byte offset and how much to delete
// is a rune count, because the caret is positioned in bytes and the keyboard
// deletes characters. Text where the two agree -- which is all the text
// anybody types while writing this -- proves nothing about either.
func TestAByteOffsetIsNotARuneCount(t *testing.T) {
	// Five runes, seven bytes: the accents are two bytes each.
	const text = "héllö"
	if len(text) == utf8.RuneCountInString(text) {
		t.Fatalf("the fixture %q has one byte per rune and tests nothing", text)
	}

	t.Run("deleting forwards counts runes", func(t *testing.T) {
		var e editBuffer
		bufferType(&e, 0, text)
		// Byte 1 is where the two-byte é begins. Deleting one rune there has
		// to take both of its bytes; taking one would leave half a character.
		e.ReplaceRunes(1, 1, "")
		if got := bufferText(t, &e); got != "hllö" {
			t.Fatalf("deleting one rune at byte 1 left %q, want %q", got, "hllö")
		}
		if !utf8.ValidString(bufferText(t, &e)) {
			t.Error("the buffer holds a partial rune")
		}
	})

	t.Run("deleting backwards counts runes", func(t *testing.T) {
		var e editBuffer
		bufferType(&e, 0, text)
		// Backspace at the end of the text, which sits at byte 7 and not at
		// byte 5 where the fifth rune would be if runes were bytes.
		e.ReplaceRunes(int64(len(text)), -1, "")
		if got := bufferText(t, &e); got != "héll" {
			t.Fatalf("backspacing one rune left %q, want %q", got, "héll")
		}
	})

	t.Run("a backwards replacement starts where deletion started", func(t *testing.T) {
		var e editBuffer
		bufferType(&e, 0, "aébcd")
		// The suffix makes the old caret different from the start of the
		// deletion. Inserting at the old byte offset would put X after c.
		e.ReplaceRunes(int64(len("aéb")), -1, "X")
		if got := bufferText(t, &e); got != "aéXcd" {
			t.Fatalf("replacing backwards left %q, want %q", got, "aéXcd")
		}
	})

	t.Run("size is in bytes", func(t *testing.T) {
		var e editBuffer
		bufferType(&e, 0, text)
		if e.Size() != int64(len(text)) {
			t.Fatalf("the buffer reports %d, want %d bytes for %q", e.Size(), len(text), text)
		}
	})

	t.Run("deleting more runes than there are stops at the end", func(t *testing.T) {
		var e editBuffer
		bufferType(&e, 0, text)
		e.ReplaceRunes(0, 500, "")
		if got := bufferText(t, &e); got != "" {
			t.Fatalf("deleting past the end left %q", got)
		}
		bufferType(&e, 0, text)
		e.ReplaceRunes(int64(len(text)), -500, "")
		if got := bufferText(t, &e); got != "" {
			t.Fatalf("backspacing past the start left %q", got)
		}
	})
}

// TestIllFormedTextIsRepairedOnTheWayIn keeps the invariant every reader
// depends on.
//
// What arrives is a paste or a platform's idea of what the keyboard produced,
// and neither is guaranteed to be well formed. Every reader here decodes runes,
// so a broken sequence stored as it came would be decoded differently by each
// of them, and the caret arithmetic that walks it would stop agreeing with the
// text that is drawn.
func TestIllFormedTextIsRepairedOnTheWayIn(t *testing.T) {
	var e editBuffer
	bufferType(&e, 0, "a\xffb")

	got := bufferText(t, &e)
	if !utf8.ValidString(got) {
		t.Fatalf("the buffer stored ill-formed text: %q", got)
	}
	if !strings.HasPrefix(got, "a") || !strings.HasSuffix(got, "b") {
		t.Fatalf("repairing the text lost the good bytes around it: %q", got)
	}
}
