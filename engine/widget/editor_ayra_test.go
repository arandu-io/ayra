package widget

import (
	"image"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// ayraEnglish is a left to right locale, which is what the properties below
// are stated in. Direction is not incidental here: which arrow key moves the
// caret forward depends on it.
var ayraEnglish = system.Locale{
	Language:  "EN",
	Direction: system.LTR,
}

// ayraField is an editor with just enough of a window around it to receive
// input method events.
//
// There is no window and no driver. An input method on a real platform is a
// process of its own that talks to the window, and what reaches the editor at
// the end of that is a handful of events any test can queue. Everything below
// that does not need events uses the editor on its own, which is how it is
// meant to work: an editor that needed a frame before it could hold a string
// could not be filled in before it is first shown.
type ayraField struct {
	t      *testing.T
	editor *Editor
	router *input.Router
	gtx    layout.Context
	shaper *text.Shaper
}

// newAyraField returns a focused editor and the router feeding it.
func newAyraField(t *testing.T, singleLine bool) *ayraField {
	t.Helper()
	f := &ayraField{
		t:      t,
		editor: new(Editor),
		router: new(input.Router),
		shaper: text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection())),
	}
	f.editor.SingleLine = singleLine
	f.gtx = layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(200, 100)),
		Locale:      ayraEnglish,
		Source:      f.router.Source(),
	}
	f.gtx.Execute(key.FocusCmd{Tag: f.editor})
	f.frame()
	return f
}

// frame draws the editor once and hands the frame to the router, which is what
// makes the editor reachable by the events queued for the next one.
func (f *ayraField) frame() {
	f.t.Helper()
	f.gtx.Ops.Reset()
	f.gtx.Execute(key.FocusCmd{Tag: f.editor})
	f.editor.Layout(f.gtx, f.shaper, font.Font{}, unit.Sp(10), op.CallOp{}, op.CallOp{})
	f.router.Frame(f.gtx.Ops)
}

// send queues events and draws the frame that consumes them.
func (f *ayraField) send(events ...event.Event) {
	f.t.Helper()
	f.router.Queue(events...)
	f.frame()
}

// state is what the input method is told about the field.
func (f *ayraField) state() input.EditorState {
	return f.router.EditorState()
}

// ayraSelection reports the selection with its ends in order, which is what
// bounds are checked against. The editor keeps them unordered on purpose, since
// which end moved last is which end a selection grows from.
func ayraSelection(e *Editor) (lo, hi int) {
	start, end := e.Selection()
	if start > end {
		start, end = end, start
	}
	return start, end
}

// ayraCheckSelectionInside fails if either end of the selection is outside the
// content. An end past it reads text that is not there; an end below zero is a
// negative index into the buffer.
func ayraCheckSelectionInside(t *testing.T, e *Editor, what string) {
	t.Helper()
	start, end := e.Selection()
	length := e.Len()
	for _, end := range []struct {
		name string
		at   int
	}{{"start", start}, {"end", end}} {
		if end.at < 0 || end.at > length {
			t.Errorf("%s: selection %s is %d, outside content of %d runes", what, end.name, end.at, length)
		}
	}
}

// ayraClusters counts grapheme clusters the way the editor moves over them: by
// asking it to move and seeing where it stops.
func ayraClusters(e *Editor, s string) int {
	probe := new(Editor)
	probe.SetText(s)
	probe.SetCaret(0, 0)
	n := 0
	for {
		caret, _ := probe.Selection()
		if caret >= probe.Len() {
			return n
		}
		probe.MoveCaret(1, 1)
		next, _ := probe.Selection()
		if next == caret {
			return n
		}
		n++
	}
}

// TestAyraEditorInsertThenDeleteRestoresContent states the roundtrip: text
// typed and then rubbed out again leaves the field as it was found, caret
// included.
//
// The cases insert at the end of content that cannot fuse with them, because a
// mark that joins the character before it is one character afterwards, and
// erasing that character is not the reverse of typing the mark.
func TestAyraEditorInsertThenDeleteRestoresContent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initial string
		typed   string
	}{
		{"ascii", "hello ", "world"},
		{"multi byte", "안П你 ", "안П你"},
		{"combining", "abc ", "éé"},
		{"emoji", "note ", "🤬🤬"},
		{"newline", "first\n", "second"},
		{"whole content", "", "everything"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := new(Editor)
			e.SetText(tc.initial)
			before := e.Len()
			e.SetCaret(before, before)

			if got := e.Insert(tc.typed); got != utf8.RuneCountInString(tc.typed) {
				t.Fatalf("inserted %d runes, wanted %d", got, utf8.RuneCountInString(tc.typed))
			}
			if got, want := e.Text(), tc.initial+tc.typed; got != want {
				t.Fatalf("after insert content is %q, wanted %q", got, want)
			}

			clusters := ayraClusters(e, tc.typed)
			for range clusters {
				if e.Delete(-1) == 0 {
					t.Fatalf("backspace deleted nothing with %d runes left", e.Len())
				}
			}
			if got := e.Text(); got != tc.initial {
				t.Errorf("after %d backspaces content is %q, wanted %q", clusters, got, tc.initial)
			}
			start, end := e.Selection()
			if start != before || end != before {
				t.Errorf("caret is at (%d, %d), wanted (%d, %d)", start, end, before, before)
			}
		})
	}
}

// TestAyraEditorSelectionNeverLeavesContent states that no caret movement and
// no selection put either end outside the text, however far it is asked to go
// and however the content changed underneath it.
func TestAyraEditorSelectionNeverLeavesContent(t *testing.T) {
	const huge = 1 << 30
	contents := []string{"", "a", "hello world", "안П你 hello 안П你", "ééé", "one\ntwo\nthree", "🤬 x 🤬"}
	offsets := []int{-huge, -7, -1, 0, 1, 3, 7, huge}

	for _, content := range contents {
		e := new(Editor)
		e.SetText(content)
		ayraCheckSelectionInside(t, e, "after SetText "+content)

		for _, start := range offsets {
			for _, end := range offsets {
				e.SetCaret(start, end)
				ayraCheckSelectionInside(t, e, "after SetCaret")

				e.MoveCaret(start, end)
				ayraCheckSelectionInside(t, e, "after MoveCaret")

				// Shrinking the content under a selection that reached the old
				// end is where an end left behind would show.
				e.Insert("")
				ayraCheckSelectionInside(t, e, "after deleting the selection")
				e.SetText(content)
			}
		}
	}
}

// TestAyraEditorCaretStopsAtTheEnds states that a caret asked past either end
// of the content stops there, rather than running on and being clamped by
// whoever reads it next.
func TestAyraEditorCaretStopsAtTheEnds(t *testing.T) {
	for _, content := range []string{"", "a", "hello world", "éé", "one\ntwo"} {
		e := new(Editor)
		e.SetText(content)
		length := e.Len()

		e.SetCaret(0, 0)
		e.MoveCaret(1<<30, 1<<30)
		if start, end := e.Selection(); start != length || end != length {
			t.Errorf("%q: caret run forward is at (%d, %d), wanted (%d, %d)", content, start, end, length, length)
		}
		// Already at the end, another push must not move it.
		e.MoveCaret(1<<30, 1<<30)
		if start, end := e.Selection(); start != length || end != length {
			t.Errorf("%q: caret pushed past the end is at (%d, %d), wanted (%d, %d)", content, start, end, length, length)
		}

		e.MoveCaret(-(1 << 30), -(1 << 30))
		if start, end := e.Selection(); start != 0 || end != 0 {
			t.Errorf("%q: caret run backward is at (%d, %d), wanted (0, 0)", content, start, end)
		}
		e.MoveCaret(-(1 << 30), -(1 << 30))
		if start, end := e.Selection(); start != 0 || end != 0 {
			t.Errorf("%q: caret pushed past the start is at (%d, %d), wanted (0, 0)", content, start, end)
		}
	}
}

// TestAyraEditorUndoThenRedoIsIdentity states that undo followed by redo
// leaves the field exactly as it was, for every step of an editing session and
// for the session unwound whole and replayed.
func TestAyraEditorUndoThenRedoIsIdentity(t *testing.T) {
	type step struct {
		caret       [2]int
		insert      string
		deleteWords int
	}
	steps := []step{
		{caret: [2]int{0, 0}, insert: "안П你 hello 안П你"},
		{caret: [2]int{1, 5}, insert: "П"},
		{caret: [2]int{3, 4}, insert: "é"},
		{caret: [2]int{0, 0}, insert: "🤬 "},
		{caret: [2]int{2, 2}, deleteWords: 1},
		{caret: [2]int{4, 4}, insert: ""},
		{caret: [2]int{0, 3}, insert: "replaced"},
	}

	e := new(Editor)
	type snapshot struct {
		content    string
		start, end int
	}
	var seen []snapshot
	take := func() snapshot {
		start, end := e.Selection()
		return snapshot{e.Text(), start, end}
	}
	seen = append(seen, take())

	for i, s := range steps {
		e.SetCaret(s.caret[0], s.caret[1])
		if s.deleteWords != 0 {
			e.deleteWord(s.deleteWords)
		} else {
			e.Insert(s.insert)
		}
		after := take()

		// One step back and forward again, at every step.
		if _, ok := e.undo(); !ok {
			t.Fatalf("step %d: nothing to undo", i)
		}
		if _, ok := e.redo(); !ok {
			t.Fatalf("step %d: nothing to redo", i)
		}
		if got := take(); got != after {
			t.Fatalf("step %d: undo then redo gave %+v, wanted %+v", i, got, after)
		}
		seen = append(seen, after)
	}

	// The whole session unwound, then replayed. Undo restores content; where
	// the caret lands after an undo is the editor's own answer and is not
	// claimed to be where it was, so only the content is compared going back.
	for i := len(seen) - 1; i > 0; i-- {
		if got := e.Text(); got != seen[i].content {
			t.Fatalf("unwinding to step %d: content is %q, wanted %q", i, got, seen[i].content)
		}
		if _, ok := e.undo(); !ok {
			t.Fatalf("unwinding to step %d: nothing to undo", i)
		}
	}
	if got := e.Text(); got != seen[0].content {
		t.Fatalf("fully unwound content is %q, wanted %q", got, seen[0].content)
	}
	for i := 1; i < len(seen); i++ {
		if _, ok := e.redo(); !ok {
			t.Fatalf("replaying step %d: nothing to redo", i)
		}
		if got := e.Text(); got != seen[i].content {
			t.Fatalf("replaying step %d: content is %q, wanted %q", i, got, seen[i].content)
		}
	}
	if _, ok := e.redo(); ok {
		t.Error("redo past the end of the history did something")
	}
}

// TestAyraEditorInputMethodReplacementPlacesCaret states the contract a
// composing input method depends on: the field replaces the range it was
// given, and then puts the caret exactly where the method asked, not where the
// field would have put it on its own.
//
// This is the whole of composing a character. Every keystroke of one rewrites
// the same stretch of text and then says where the caret belongs inside the
// result; a field that answered with its own idea would push the caret off the
// character being written.
func TestAyraEditorInputMethodReplacementPlacesCaret(t *testing.T) {
	for _, tc := range []struct {
		name     string
		initial  string
		replace  key.Range
		text     string
		want     key.Range
		expected string
	}{
		{
			name:     "composing in place",
			initial:  "ni",
			replace:  key.Range{Start: 0, End: 2},
			text:     "你",
			want:     key.Range{Start: 1, End: 1},
			expected: "你",
		},
		{
			name:     "caret left inside the replacement",
			initial:  "hello world",
			replace:  key.Range{Start: 6, End: 11},
			text:     "안П你",
			want:     key.Range{Start: 7, End: 7},
			expected: "hello 안П你",
		},
		{
			name:     "caret before the replacement",
			initial:  "abc",
			replace:  key.Range{Start: 1, End: 2},
			text:     "XYZ",
			want:     key.Range{Start: 1, End: 1},
			expected: "aXYZc",
		},
		{
			name:     "selection over the replacement",
			initial:  "abc",
			replace:  key.Range{Start: 0, End: 1},
			text:     "é",
			want:     key.Range{Start: 0, End: 2},
			expected: "ébc",
		},
		{
			name:     "replacement is an insertion",
			initial:  "ac",
			replace:  key.Range{Start: 1, End: 1},
			text:     "b",
			want:     key.Range{Start: 2, End: 2},
			expected: "abc",
		},
		{
			name:     "replacement is a deletion",
			initial:  "abc",
			replace:  key.Range{Start: 1, End: 2},
			text:     "",
			want:     key.Range{Start: 1, End: 1},
			expected: "ac",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAyraField(t, true)
			f.editor.SetText(tc.initial)
			f.frame()

			f.send(key.EditEvent{Range: tc.replace, Text: tc.text})
			if got := f.editor.Text(); got != tc.expected {
				t.Fatalf("content is %q, wanted %q", got, tc.expected)
			}

			f.send(key.SelectionEvent(tc.want))
			start, end := f.editor.Selection()
			if start != tc.want.Start || end != tc.want.End {
				t.Errorf("caret is at (%d, %d), the input method asked for (%d, %d)", start, end, tc.want.Start, tc.want.End)
			}
			ayraCheckSelectionInside(t, f.editor, "after an input method replacement")

			// What the field reports back has to agree, or the next keystroke
			// of the same character is composed against a stale position.
			if got := f.state().Selection.Range; got != tc.want {
				t.Errorf("the input method was told the selection is %+v, wanted %+v", got, tc.want)
			}
		})
	}
}

// TestAyraEditorInputMethodSelectionSurvivesShrinkingContent states that a
// selection the input method set, and then had the ground cut from under it,
// is still inside the content.
//
// It is the case a field gets wrong in one direction only: the method asks for
// a range in the text it last saw, the application replaces the text in the
// same frame, and the range now names runes that are not there.
func TestAyraEditorInputMethodSelectionSurvivesShrinkingContent(t *testing.T) {
	f := newAyraField(t, false)
	f.editor.SetText("a long enough line to select the end of")
	f.frame()

	f.send(key.SelectionEvent(key.Range{Start: 30, End: 38}))
	ayraCheckSelectionInside(t, f.editor, "after the input method selected")

	f.editor.SetText("short")
	f.frame()
	ayraCheckSelectionInside(t, f.editor, "after the content shrank")

	f.send(key.SelectionEvent(key.Range{Start: 400, End: 900}))
	ayraCheckSelectionInside(t, f.editor, "after a selection past the end")
	if start, end := ayraSelection(f.editor); start != f.editor.Len() || end != f.editor.Len() {
		t.Errorf("a selection past the end landed at (%d, %d), wanted both at %d", start, end, f.editor.Len())
	}
}

// TestAyraEditorSnippetMatchesContent states that the piece of text handed to
// the input method is the piece the field actually holds at that range.
//
// The snippet is the method's only view of the text around the caret. One that
// is stale, or one whose range no longer fits the content, is what makes a
// method delete the wrong characters when it backs over what it composed.
func TestAyraEditorSnippetMatchesContent(t *testing.T) {
	f := newAyraField(t, false)
	f.editor.SetText("the quick brown fox")
	f.frame()

	check := func(what string) {
		t.Helper()
		snippet := f.state().Snippet
		content := []rune(f.editor.Text())
		if snippet.Start < 0 || snippet.End < snippet.Start || snippet.End > len(content) {
			t.Fatalf("%s: snippet range %+v does not fit %d runes", what, snippet.Range, len(content))
		}
		if want := string(content[snippet.Start:snippet.End]); snippet.Text != want {
			t.Errorf("%s: snippet holds %q, the content at that range is %q", what, snippet.Text, want)
		}
	}

	f.send(key.SnippetEvent(key.Range{Start: 4, End: 9}))
	check("after the input method asked for a snippet")

	// The application replaces the text under the snippet.
	f.editor.SetText("ox")
	f.frame()
	check("after the content shrank under the snippet")

	f.send(key.SnippetEvent(key.Range{Start: 0, End: 500}))
	check("after a snippet asked for past the end")

	f.editor.SetText("안П你 hello 안П你")
	f.frame()
	f.send(key.SnippetEvent(key.Range{Start: 1, End: 4}))
	check("after a snippet over multi byte runes")
}

// TestAyraEditorFocusResetsInputMethodState states that a field losing focus
// forgets what was being composed in it.
//
// Composition state belongs to the method that owns the focus. Kept across a
// change of focus, it is a half written character waiting to be finished by
// whatever types next.
func TestAyraEditorFocusResetsInputMethodState(t *testing.T) {
	f := newAyraField(t, true)
	f.editor.SetText("hello")
	f.frame()

	f.send(key.EditEvent{Range: key.Range{Start: 5, End: 5}, Text: "ni"})
	f.send(key.CompositionEvent{Start: 3, End: 5})
	if got := f.editor.ime.composition; got != (key.Range{Start: 3, End: 5}) {
		t.Fatalf("composition is %+v, wanted {3 5}", got)
	}
	if f.editor.compositionBounds().Empty() {
		t.Error("a composition over visible text has no bounds on the screen")
	}

	other := new(Editor)
	f.gtx.Execute(key.FocusCmd{Tag: other})
	f.gtx.Ops.Reset()
	f.editor.Layout(f.gtx, f.shaper, font.Font{}, unit.Sp(10), op.CallOp{}, op.CallOp{})
	f.router.Frame(f.gtx.Ops)

	if got := f.editor.ime.composition; got != (key.Range{Start: -1, End: -1}) {
		t.Errorf("after losing focus the composition is %+v, wanted the empty one", got)
	}
	if got := f.editor.compositionBounds(); !got.Empty() {
		t.Errorf("after losing focus the composition still covers %v", got)
	}
}

// TestAyraEditorCompositionOutsideContentHasNoBounds states that a composition
// range naming runes the field does not have draws nothing and reports
// nothing, rather than reaching past the end of the text to find out.
func TestAyraEditorCompositionOutsideContentHasNoBounds(t *testing.T) {
	f := newAyraField(t, true)
	f.editor.SetText("hi")
	f.frame()

	for _, r := range []key.Range{
		{Start: -1, End: -1},
		{Start: 0, End: 0},
		{Start: 2, End: 2},
	} {
		f.editor.ime.composition = r
		if got := f.editor.compositionBounds(); !got.Empty() {
			t.Errorf("composition %+v reported bounds %v, wanted none", r, got)
		}
	}
}

// TestAyraEditorCombiningMarksAreNotCutInHalf states that a mark and the
// character it sits on are one thing to everything the person does: one
// backspace takes both, one arrow key steps over both.
//
// The rune is the wrong unit here and the editor does not use it. Half of a
// cluster left behind is a mark with nothing to attach to, which the shaper
// draws on a dotted circle of its own.
func TestAyraEditorCombiningMarksAreNotCutInHalf(t *testing.T) {
	const acute = "́"
	content := "e" + acute + "a" + acute + "o" + acute

	t.Run("backspace takes the cluster", func(t *testing.T) {
		e := new(Editor)
		e.SetText(content)
		e.SetCaret(e.Len(), e.Len())
		if got := e.Delete(-1); abs(got) != 2 {
			t.Fatalf("backspace deleted %d runes, wanted a 2-rune cluster", got)
		}
		if got, want := e.Text(), "e"+acute+"a"+acute; got != want {
			t.Fatalf("content is %q, wanted %q", got, want)
		}
		ayraCheckNoLooseMarks(t, e.Text())
	})

	t.Run("forward delete takes the cluster", func(t *testing.T) {
		e := new(Editor)
		e.SetText(content)
		e.SetCaret(0, 0)
		if got := e.Delete(1); got != 2 {
			t.Fatalf("delete removed %d runes, wanted the 2 of the cluster", got)
		}
		if got, want := e.Text(), "a"+acute+"o"+acute; got != want {
			t.Fatalf("content is %q, wanted %q", got, want)
		}
		ayraCheckNoLooseMarks(t, e.Text())
	})

	t.Run("the caret steps over the cluster", func(t *testing.T) {
		e := new(Editor)
		e.SetText(content)
		e.SetCaret(0, 0)
		for step, want := range []int{2, 4, 6, 6} {
			e.MoveCaret(1, 1)
			if start, end := e.Selection(); start != want || end != want {
				t.Fatalf("step %d: caret is at (%d, %d), wanted both at %d", step, start, end, want)
			}
		}
		for step, want := range []int{4, 2, 0, 0} {
			e.MoveCaret(-1, -1)
			if start, end := e.Selection(); start != want || end != want {
				t.Fatalf("back step %d: caret is at (%d, %d), wanted both at %d", step, start, end, want)
			}
		}
	})

	t.Run("erasing the whole content one cluster at a time", func(t *testing.T) {
		e := new(Editor)
		e.SetText(content)
		e.SetCaret(e.Len(), e.Len())
		for e.Len() > 0 {
			if e.Delete(-1) == 0 {
				t.Fatalf("backspace deleted nothing with %q left", e.Text())
			}
			ayraCheckNoLooseMarks(t, e.Text())
		}
	})

	t.Run("an input method replacement over a cluster", func(t *testing.T) {
		f := newAyraField(t, true)
		f.editor.SetText(content)
		f.frame()
		f.send(key.EditEvent{Range: key.Range{Start: 0, End: 2}, Text: "ü"})
		if got, want := f.editor.Text(), "üa"+acute+"o"+acute; got != want {
			t.Fatalf("content is %q, wanted %q", got, want)
		}
		ayraCheckNoLooseMarks(t, f.editor.Text())
	})
}

// ayraCheckNoLooseMarks fails if the content begins with a combining mark, or
// holds one whose base character was taken out from under it.
func ayraCheckNoLooseMarks(t *testing.T, s string) {
	t.Helper()
	hasBase := false
	for i, r := range s {
		if !unicode.Is(unicode.Mn, r) {
			hasBase = true
			continue
		}
		if !hasBase {
			t.Errorf("content %q has a combining mark at %d with nothing under it", s, i)
		}
	}
}

// TestAyraEditorSingleLineHasNoNewlines states that a field declared as one
// line holds one line, whatever it is filled from: typing, pasting, an input
// method, or the application setting the text.
//
// The alternative is a field one line tall holding text that is taller, where
// what is out of view cannot be scrolled to because there is nowhere to
// scroll.
func TestAyraEditorSingleLineHasNoNewlines(t *testing.T) {
	e := new(Editor)
	e.SingleLine = true

	e.SetText("one\ntwo\nthree")
	if strings.Contains(e.Text(), "\n") {
		t.Errorf("SetText left a newline in %q", e.Text())
	}
	e.SetCaret(e.Len(), e.Len())
	e.Insert("\nfour\n")
	if strings.Contains(e.Text(), "\n") {
		t.Errorf("Insert left a newline in %q", e.Text())
	}

	f := newAyraField(t, true)
	f.editor.SetText("start")
	f.frame()
	f.send(key.EditEvent{Range: key.Range{Start: 5, End: 5}, Text: "\nmore"})
	if strings.Contains(f.editor.Text(), "\n") {
		t.Errorf("an input method left a newline in %q", f.editor.Text())
	}
}

// TestAyraEditorMaxLenHoldsAgainstEveryWayIn states that the limit is on the
// content and not on one way of reaching it: what the limit refuses when typed
// it refuses when pasted, when set, and when written by an input method.
func TestAyraEditorMaxLenHoldsAgainstEveryWayIn(t *testing.T) {
	const limit = 8
	newLimited := func() *Editor {
		e := new(Editor)
		e.MaxLen = limit
		e.SingleLine = true
		return e
	}

	e := newLimited()
	e.SetText(strings.Repeat("x", 40))
	if got := e.Len(); got != limit {
		t.Errorf("SetText past the limit left %d runes, wanted %d", got, limit)
	}

	e = newLimited()
	e.SetText("abc")
	e.SetCaret(3, 3)
	e.Insert(strings.Repeat("y", 40))
	if got := e.Len(); got != limit {
		t.Errorf("Insert past the limit left %d runes, wanted %d", got, limit)
	}

	f := newAyraField(t, true)
	f.editor.MaxLen = limit
	f.editor.SetText("abc")
	f.frame()
	f.send(key.EditEvent{Range: key.Range{Start: 3, End: 3}, Text: strings.Repeat("z", 40)})
	if got := f.editor.Len(); got != limit {
		t.Errorf("an input method past the limit left %d runes, wanted %d", got, limit)
	}
	ayraCheckSelectionInside(t, f.editor, "after an input method hit the limit")
}

// TestAyraEditorReadOnlyRefusesEveryWayIn states that a field that cannot be
// edited cannot be edited by an input method either, which is the way in that
// does not go through the keyboard handling where the flag is easy to
// remember.
func TestAyraEditorReadOnlyRefusesEveryWayIn(t *testing.T) {
	f := newAyraField(t, true)
	f.editor.SetText("locked")
	f.editor.ReadOnly = true
	f.frame()

	f.send(key.EditEvent{Range: key.Range{Start: 0, End: 6}, Text: "opened"})
	if got := f.editor.Text(); got != "locked" {
		t.Errorf("an input method wrote into a read only field: %q", got)
	}

	// Selecting is still allowed: a field nobody may edit is still a field
	// whose contents someone may want to copy.
	f.send(key.SelectionEvent(key.Range{Start: 0, End: 6}))
	if got := f.editor.SelectedText(); got != "locked" {
		t.Errorf("a read only field refused a selection: %q", got)
	}
}
