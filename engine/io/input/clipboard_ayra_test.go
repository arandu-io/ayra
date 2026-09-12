package input

import (
	"io"
	"strings"
	"testing"

	"github.com/arandu-io/ayra/engine/io/clipboard"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/transfer"
	"github.com/arandu-io/ayra/engine/op"
)

const clipboardMIME = "application/text"

// clipboardAnswer is what the platform hands back, shaped the way the runtime
// hands it on: a type and a reader opened on demand.
func clipboardAnswer(content string) transfer.DataEvent {
	return transfer.DataEvent{
		Type: clipboardMIME,
		Open: func() io.ReadCloser {
			return io.NopCloser(strings.NewReader(content))
		},
	}
}

// trackedData is a WriteCmd payload that remembers whether it was drained and
// closed, which is the only way to tell from the outside.
type trackedData struct {
	reader *strings.Reader
	closed bool
}

func newTrackedData(content string) *trackedData {
	return &trackedData{reader: strings.NewReader(content)}
}

func (d *trackedData) Read(p []byte) (int, error) { return d.reader.Read(p) }

func (d *trackedData) Close() error {
	d.closed = true
	return nil
}

// TestClipboardReadSurvivesFrames drives real frames between the command and
// the answer, which is the whole difficulty of reading a clipboard: the frame
// that asked is finished and gone by the time anything arrives.
func TestClipboardReadSurvivesFrames(t *testing.T) {
	ops, r := new(op.Ops), new(Router)
	h := new(int)
	target := transfer.TargetFilter{Target: h, Type: clipboardMIME}

	r.Source().Execute(clipboard.ReadCmd{Tag: h})
	if !r.ClipboardRequested() {
		t.Fatal("the platform was not asked in the frame the command was executed")
	}

	for range 3 {
		events(r, -1, target)
		r.Frame(ops)
		if r.ClipboardRequested() {
			t.Error("the platform was asked again while the first question still stood")
		}
		if got := len(r.state().receivers); got != 1 {
			t.Fatalf("the request did not survive the frame: %d receivers waiting, want 1", got)
		}
	}

	r.Queue(clipboardAnswer("late"))
	assertClipboardAnswers(t, events(r, -1, target), "late")
	if got := len(r.state().receivers); got != 0 {
		t.Errorf("%d receivers still waiting after the answer arrived, want 0", got)
	}
}

// TestClipboardTargetFilterIsAskedEveryFrame pins the trap the command's
// documentation warns about: the answer is offered in the frame it arrives in,
// and a handler that is not filtering right then loses it for good.
func TestClipboardTargetFilterIsAskedEveryFrame(t *testing.T) {
	ops, r := new(op.Ops), new(Router)
	h := new(int)
	target := transfer.TargetFilter{Target: h, Type: clipboardMIME}

	r.Source().Execute(clipboard.ReadCmd{Tag: h})
	events(r, -1, target)
	r.Frame(ops)

	// A frame in which the handler asks for nothing, and the answer arrives.
	r.Queue(clipboardAnswer("lost"))
	r.Frame(ops)

	// Now it asks, one frame too late.
	assertClipboardAnswers(t, events(r, -1, target))
	if got := len(r.state().receivers); got != 0 {
		t.Errorf("%d receivers still waiting, want 0: the answer was spent on the frame that dropped it", got)
	}

	// The same sequence with the filter in place delivers.
	r.Source().Execute(clipboard.ReadCmd{Tag: h})
	events(r, -1, target)
	r.Frame(ops)
	events(r, -1, target)
	r.Queue(clipboardAnswer("kept"))
	assertClipboardAnswers(t, events(r, -1, target), "kept")
}

// TestClipboardOneCommandBuysOneAnswer asks twice with the same tag while the
// first question stands. The second buys nothing: the platform is one thing
// asked once, and asking it again would answer twice.
func TestClipboardOneCommandBuysOneAnswer(t *testing.T) {
	r := new(Router)
	h := new(int)

	r.Source().Execute(clipboard.ReadCmd{Tag: h})
	if got := len(r.state().receivers); got != 1 {
		t.Fatalf("got %d receivers waiting, want 1", got)
	}
	if !r.ClipboardRequested() {
		t.Fatal("the platform was not asked")
	}

	r.Source().Execute(clipboard.ReadCmd{Tag: h})
	if got := len(r.state().receivers); got != 1 {
		t.Errorf("got %d receivers waiting, want 1: the same tag was enrolled twice", got)
	}
	if r.ClipboardRequested() {
		t.Error("the platform was asked a second time for a question already standing")
	}

	// A second tag is a second handler waiting, not a second question: both are
	// answered by the one answer that arrives.
	other := new(int)
	r.Source().Execute(clipboard.ReadCmd{Tag: other})
	if got := len(r.state().receivers); got != 2 {
		t.Fatalf("got %d receivers waiting, want 2", got)
	}
	r.Queue(clipboardAnswer("shared"))
	for _, tag := range []event.Tag{h, other} {
		assertClipboardAnswers(t, events(r, -1, transfer.TargetFilter{Target: tag, Type: clipboardMIME}), "shared")
	}
}

// TestClipboardWriteIsDrainedAndClosedWhileExecuting fixes what the caller must
// not do afterwards, by fixing what has already happened by the time Execute
// returns.
func TestClipboardWriteIsDrainedAndClosedWhileExecuting(t *testing.T) {
	r := new(Router)
	data := newTrackedData("copied")

	r.Source().Execute(clipboard.WriteCmd{Type: clipboardMIME, Data: data})

	if !data.closed {
		t.Error("the payload was not closed as the command was executed")
	}
	if left := data.reader.Len(); left != 0 {
		t.Errorf("%d bytes left unread: the payload is drained to the end", left)
	}

	mime, content, ok := r.WriteClipboard()
	if !ok || mime != clipboardMIME || string(content) != "copied" {
		t.Errorf("got %q %q %v, want %q %q true", mime, content, ok, clipboardMIME, "copied")
	}
}

// TestClipboardSecondWriteInAFrameWins holds the asymmetry a caller is most
// likely to get wrong: a clipboard holds one thing, and the loser of the frame
// is dropped without an error anywhere.
func TestClipboardSecondWriteInAFrameWins(t *testing.T) {
	r := new(Router)
	first, second := newTrackedData("first"), newTrackedData("second")

	r.Source().Execute(clipboard.WriteCmd{Type: clipboardMIME, Data: first})
	r.Source().Execute(clipboard.WriteCmd{Type: clipboardMIME, Data: second})

	if !first.closed || !second.closed {
		t.Error("a payload was left open: both are closed, including the one that is dropped")
	}

	mime, content, ok := r.WriteClipboard()
	if !ok || mime != clipboardMIME || string(content) != "second" {
		t.Errorf("got %q %q %v, want %q %q true", mime, content, ok, clipboardMIME, "second")
	}
	if _, _, ok := r.WriteClipboard(); ok {
		t.Error("the content was handed over twice: it is taken, not read")
	}
}

// assertClipboardAnswers reads the content out of the delivered data events and
// compares it with what the platform was made to answer.
func assertClipboardAnswers(t *testing.T, got []event.Event, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i, e := range got {
		de, ok := e.(transfer.DataEvent)
		if !ok {
			t.Fatalf("got %T, want transfer.DataEvent", e)
		}
		content, err := io.ReadAll(de.Open())
		if err != nil {
			t.Fatalf("reading the answer: %v", err)
		}
		if string(content) != want[i] {
			t.Errorf("got answer %q, want %q", content, want[i])
		}
	}
}
