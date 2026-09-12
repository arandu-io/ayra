package widget

import (
	"image"
	"testing"
	"time"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// The tests here drive whole frames rather than calling methods on the text
// view directly. What a person does to a selectable is press, move and let go,
// and every one of those arrives as a routed event addressed to a tag that only
// exists because a frame registered it. A test that reaches past the router
// proves the text view can select, which was never in doubt, and says nothing
// about whether the presses reach it.

// selectableLocale is the writing direction these tests read in. Direction is
// read when a key moves the caret, so it cannot be left at its zero value and
// be honest about what it means.
var selectableLocale = system.Locale{
	Language:  "EN",
	Direction: system.LTR,
}

// selectableFixture is a Selectable wired to a router, with room to lay out in
// and a clock that advances between gestures.
type selectableFixture struct {
	t      *testing.T
	router *input.Router
	gtx    layout.Context
	shaper *text.Shaper
	s      *Selectable
	now    time.Duration
}

// newSelectableFixture stands up a selectable holding content and lays it out
// once, so that the shaped text exists and the coordinates of a rune can be
// asked for.
func newSelectableFixture(t *testing.T, content string) *selectableFixture {
	t.Helper()
	r := new(input.Router)
	f := &selectableFixture{
		t:      t,
		router: r,
		shaper: text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection())),
		s:      new(Selectable),
		now:    time.Second,
	}
	f.gtx = layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(400, 200)),
		Locale:      selectableLocale,
		Source:      r.Source(),
	}
	f.s.SetText(content)
	f.frame()
	return f
}

// frame lays the selectable out and completes the frame, which is what makes
// the events queued since the last one reach it.
func (f *selectableFixture) frame() {
	f.t.Helper()
	f.gtx.Ops.Reset()
	f.gtx.Source = f.router.Source()
	f.s.Layout(f.gtx, f.shaper, font.Font{}, unit.Sp(10), op.CallOp{}, op.CallOp{})
	f.router.Frame(f.gtx.Ops)
}

// focus gives the selectable the keyboard, which the copy shortcut needs and
// which painting the selection also waits for.
func (f *selectableFixture) focus() {
	f.t.Helper()
	f.gtx.Execute(key.FocusCmd{Tag: f.s})
	f.frame()
}

// at answers the screen position of a rune offset, on the baseline of the line
// that offset falls on.
func (f *selectableFixture) at(runeOffset int) f32.Point {
	f.t.Helper()
	p := f.s.text.closestToRune(runeOffset)
	return f32.Pt(float32(p.x.Round()), float32(p.y))
}

// press and release are one gesture split in two, so that a test can put a
// move between them.
func (f *selectableFixture) press(at f32.Point) {
	f.t.Helper()
	f.router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Time:     f.now,
		Position: at,
	})
}

func (f *selectableFixture) drag(to f32.Point) {
	f.t.Helper()
	f.router.Queue(pointer.Event{
		Kind:     pointer.Move,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Time:     f.now,
		Position: to,
	})
}

func (f *selectableFixture) release(at f32.Point) {
	f.t.Helper()
	f.router.Queue(pointer.Event{
		Kind:     pointer.Release,
		Source:   pointer.Mouse,
		Time:     f.now,
		Position: at,
	})
}

// dragBetween is the whole gesture: down at one point, across, and up at the
// other. The clock is advanced afterwards so the next gesture is not read as
// the second half of this one.
func (f *selectableFixture) dragBetween(from, to f32.Point) {
	f.t.Helper()
	f.press(from)
	f.drag(to)
	f.release(to)
	f.frame()
	f.now += time.Second
}

// clicks performs n presses in one place quickly enough to count as a run of
// that length, then advances the clock past the run.
func (f *selectableFixture) clicks(n int, at f32.Point) {
	f.t.Helper()
	for range n {
		f.press(at)
		f.release(at)
		f.now += 10 * time.Millisecond
	}
	f.frame()
	f.now += time.Second
}

// span answers the selection in the order a reader would name it, which is the
// order the text view does not keep.
func (f *selectableFixture) span() (lo, hi int) {
	f.t.Helper()
	start, end := f.s.Selection()
	return min(start, end), max(start, end)
}

// A drag is a stretch of text, and a stretch has no direction. Pulling from the
// end back to the start is how anyone who overshot corrects it, and it has to
// land on the same words as pulling forward.
func TestSelectableDragIsTheSameBothWays(t *testing.T) {
	const content = "alpha beta gamma delta"

	forward := newSelectableFixture(t, content)
	forward.dragBetween(forward.at(2), forward.at(14))

	backward := newSelectableFixture(t, content)
	backward.dragBetween(backward.at(14), backward.at(2))

	fLo, fHi := forward.span()
	bLo, bHi := backward.span()
	if fLo != 2 || fHi != 14 {
		t.Errorf("dragging between rune 2 and rune 14 selected [%d,%d)", fLo, fHi)
	}
	if fLo != bLo || fHi != bHi {
		t.Errorf("dragging forward selected [%d,%d), backward selected [%d,%d)", fLo, fHi, bLo, bHi)
	}
	if got, want := forward.s.SelectedText(), backward.s.SelectedText(); got != want {
		t.Errorf("dragging forward selected %q, backward selected %q", got, want)
	}
	if got := forward.s.SelectedText(); got == "" {
		t.Error("dragging across the text selected nothing")
	}
}

// A double click takes the word under the pointer, and a word ends where the
// word ends. The comma after it is the next thing along, not part of it: a
// person double clicking a word to copy it wants the word.
func TestSelectableDoubleClickTakesTheWord(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		// offset is a rune inside the word being aimed at.
		offset int
		want   string
	}{
		{"plain word", "alpha beta gamma", 7, "beta"},
		{"word before a comma", "alpha beta, gamma", 7, "beta"},
		{"word after an opening bracket", "alpha (beta) gamma", 8, "beta"},
		{"word before a full stop", "alpha beta. gamma", 7, "beta"},
		{"punctuation itself", "alpha beta, gamma", 10, ","},
		{"first word of the line", "alpha beta gamma", 2, "alpha"},
		{"last word of the line", "alpha beta gamma", 13, "gamma"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSelectableFixture(t, tc.content)
			f.clicks(2, f.at(tc.offset))
			if got := f.s.SelectedText(); got != tc.want {
				t.Errorf("double click at rune %d of %q selected %q, want %q", tc.offset, tc.content, got, tc.want)
			}
		})
	}
}

// A triple click takes the line, and only the line: the newline that ends it
// belongs to no line a person can see, and taking it would paste a break
// nobody selected.
func TestSelectableTripleClickTakesTheLine(t *testing.T) {
	const content = "first line\nsecond line\nthird line"
	for _, tc := range []struct {
		offset int
		want   string
	}{
		{2, "first line"},
		{14, "second line"},
		{26, "third line"},
	} {
		f := newSelectableFixture(t, content)
		f.clicks(3, f.at(tc.offset))
		if got := f.s.SelectedText(); got != tc.want {
			t.Errorf("triple click at rune %d selected %q, want %q", tc.offset, got, tc.want)
		}
	}
}

// Copying nothing writes nothing. A clipboard is shared with every other
// program on the machine, and a widget that clears it because a key was pressed
// with no selection destroys something it never owned.
func TestSelectableCopyingAnEmptySelectionWritesNothing(t *testing.T) {
	f := newSelectableFixture(t, "alpha beta gamma")
	f.focus()

	f.s.ClearSelection()
	f.router.Queue(key.Event{State: key.Press, Name: "C", Modifiers: key.ModShortcut})
	f.frame()
	if mime, content, ok := f.router.WriteClipboard(); ok {
		t.Errorf("copying an empty selection wrote %q as %q", content, mime)
	}

	f.s.SetCaret(0, 5)
	f.router.Queue(key.Event{State: key.Press, Name: "C", Modifiers: key.ModShortcut})
	if _, _, ok := f.router.WriteClipboard(); ok {
		t.Error("the clipboard write completed before the frame processed its command")
	}
	f.frame()
	mime, content, ok := f.router.WriteClipboard()
	if !ok {
		t.Fatal("copying a selection wrote nothing")
	}
	if string(content) != "alpha" {
		t.Errorf("copied %q, want %q", content, "alpha")
	}
	if mime == "" {
		t.Error("copied content arrived with no media type")
	}
}

// Pressing somewhere else puts the caret there and drops what was selected.
// A selection that survived the next press would still be highlighted after the
// person had plainly moved on, and the next copy would take it.
func TestSelectablePressElsewhereClearsTheSelection(t *testing.T) {
	f := newSelectableFixture(t, "alpha beta gamma")
	f.dragBetween(f.at(0), f.at(5))
	if f.s.SelectedText() == "" {
		t.Fatal("the drag selected nothing, so there is nothing to clear")
	}

	// Rune 12 is inside the widget but outside the selected range.
	f.clicks(1, f.at(12))
	if got := f.s.SelectedText(); got != "" {
		t.Errorf("a press elsewhere left %q selected", got)
	}
	if n := f.s.SelectionLen(); n != 0 {
		t.Errorf("a press elsewhere left a selection %d runes long", n)
	}
}

// Nothing the outside world can do puts either end of the selection off the
// text. Offsets are handed to callers and used to slice content, so one past
// the end is not a cosmetic fault.
func TestSelectableSelectionStaysInsideTheContent(t *testing.T) {
	const content = "alpha beta gamma"
	length := len([]rune(content))

	f := newSelectableFixture(t, content)
	inside := func(what string) {
		t.Helper()
		start, end := f.s.Selection()
		if start < 0 || start > length || end < 0 || end > length {
			t.Errorf("%s left the selection at [%d,%d), outside [0,%d]", what, start, end, length)
		}
	}

	f.s.SetCaret(-100, 100)
	inside("SetCaret past both ends")

	f.dragBetween(f32.Pt(-500, -500), f32.Pt(5000, 5000))
	inside("a drag beginning and ending off the text")

	f.clicks(2, f32.Pt(5000, 5000))
	inside("a double click past the end")

	f.clicks(3, f32.Pt(-500, -500))
	inside("a triple click before the start")

	f.focus()
	f.router.Queue(key.Event{State: key.Press, Name: "A", Modifiers: key.ModShortcut})
	f.frame()
	inside("select all")
	if got := f.s.SelectedText(); got != content {
		t.Errorf("select all selected %q, want %q", got, content)
	}

	// Shortening the text under a selection that reached the old end is the
	// one way an offset outlives the content it pointed into.
	f.s.SetText("ab")
	f.frame()
	start, end := f.s.Selection()
	if start > 2 || end > 2 {
		t.Errorf("replacing the text left the selection at [%d,%d), past the new length 2", start, end)
	}
}

// Text is what was set, and setting the same text again is not a change: a
// selectable laid out from a value that is recomputed every frame would lose
// the selection on every frame.
func TestSelectableSetTextKeepsASelectionOnlyWhenTheTextIsTheSame(t *testing.T) {
	const content = "alpha beta gamma"
	f := newSelectableFixture(t, content)
	f.s.SetCaret(0, 5)

	f.s.SetText(content)
	if got := f.s.SelectedText(); got != "alpha" {
		t.Errorf("setting the same text left %q selected, want %q", got, "alpha")
	}

	f.s.SetText("different text entirely")
	if got := f.s.SelectedText(); got != "" {
		t.Errorf("setting different text left %q selected", got)
	}
	if got := f.s.Text(); got != "different text entirely" {
		t.Errorf("Text answered %q", got)
	}
}

// Update reports a change when the selection moved and not otherwise. A caller
// redraws on the strength of that answer, so one that is always true costs a
// frame every frame and one that is always false leaves the highlight stale.
func TestSelectableUpdateReportsOnlyRealChanges(t *testing.T) {
	f := newSelectableFixture(t, "alpha beta gamma")
	if f.s.Update(f.gtx) {
		t.Error("a frame with no events reported a selection change")
	}

	f.press(f.at(0))
	f.drag(f.at(5))
	f.release(f.at(5))
	f.gtx.Ops.Reset()
	f.gtx.Source = f.router.Source()
	if !f.s.Update(f.gtx) {
		t.Error("a drag across the text reported no selection change")
	}
	f.s.Layout(f.gtx, f.shaper, font.Font{}, unit.Sp(10), op.CallOp{}, op.CallOp{})
	f.router.Frame(f.gtx.Ops)

	if f.s.Update(f.gtx) {
		t.Error("the frame after the drag reported a selection change of its own")
	}
}

// Regions cover the selected runes and nothing outside the widget. They are
// what a caller draws the highlight from, so an empty answer for a real
// selection is a selection nobody can see.
func TestSelectableRegionsCoverTheSelection(t *testing.T) {
	f := newSelectableFixture(t, "alpha beta gamma")
	if regions := f.s.Regions(0, 0, nil); len(regions) != 0 {
		t.Errorf("an empty range produced %d regions", len(regions))
	}
	regions := f.s.Regions(0, 5, nil)
	if len(regions) == 0 {
		t.Fatal("a five rune range produced no regions")
	}
	for _, r := range regions {
		if r.Bounds.Dx() <= 0 || r.Bounds.Dy() <= 0 {
			t.Errorf("region %v has no area", r.Bounds)
		}
	}
}

// The zero value answers questions without being set up first. A widget is
// commonly declared and laid out in the same frame, and one that needs a
// constructor has no zero value worth the name.
func TestSelectableZeroValueAnswers(t *testing.T) {
	var s Selectable
	if got := s.Text(); got != "" {
		t.Errorf("the zero value holds %q", got)
	}
	if got := s.SelectedText(); got != "" {
		t.Errorf("the zero value has %q selected", got)
	}
	if s.Focused() {
		t.Error("the zero value reports the keyboard")
	}
	if s.Truncated() {
		t.Error("the zero value reports truncated text")
	}
	if n := s.SelectionLen(); n != 0 {
		t.Errorf("the zero value has a selection %d runes long", n)
	}
	s.SetCaret(5, 9)
	if start, end := s.Selection(); start != 0 || end != 0 {
		t.Errorf("the zero value took a caret at [%d,%d)", start, end)
	}
	if regions := s.Regions(0, 5, nil); len(regions) != 0 {
		t.Errorf("the zero value produced %d regions", len(regions))
	}
}
