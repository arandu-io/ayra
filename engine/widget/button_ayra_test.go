package widget_test

import (
	"image"
	"testing"
	"time"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/widget"
)

// area is the square every script below presses inside of, and outside is a
// point beyond it. A press is only a click where it ends, so half of what these
// tests say is about which side of that edge the pointer was on.
var (
	area    = image.Pt(100, 100)
	centre  = f32.Pt(50, 50)
	outside = f32.Pt(500, 500)
)

// clickStage is the frame loop a Clickable lives in.
//
// It exists because a Clickable is a state machine across frames, and half of
// its transitions only happen at a frame boundary: the area it listens over and
// the key filters it declares last exactly one frame, so a script that queued
// events without ever drawing again would be queueing them at nothing.
type clickStage struct {
	router input.Router
	gtx    layout.Context
	draw   func(gtx layout.Context)
}

func newClickStage(t *testing.T, draw func(gtx layout.Context)) *clickStage {
	t.Helper()
	s := &clickStage{draw: draw}
	s.gtx = layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(area),
		Now:         time.Unix(0, 0),
		Source:      s.router.Source(),
	}
	s.frame()
	return s
}

// frame ends one frame and begins the next.
func (s *clickStage) frame() {
	s.gtx.Reset()
	s.gtx.Constraints = layout.Exact(area)
	s.draw(s.gtx)
	s.router.Frame(s.gtx.Ops)
}

// queue delivers events for the next read to take.
func (s *clickStage) queue(evts ...event.Event) {
	s.router.Queue(evts...)
}

// at builds a mouse event of the given kind at the given point.
func at(kind pointer.Kind, p f32.Point) pointer.Event {
	return pointer.Event{
		Kind:     kind,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: p,
	}
}

// fill is a widget that takes the whole square it is offered.
func fill(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: gtx.Constraints.Min}
}

// TestClickableAnswersOnceAndForgets fixes that a completed press is reported
// exactly once.
//
// The count is the whole contract: a caller asks once per frame and acts on a
// true, so a click left in the state would act again on every frame after it,
// and one press would open a dialog for as long as the dialog was open.
func TestClickableAnswersOnceAndForgets(t *testing.T) {
	var b widget.Clickable
	s := newClickStage(t, func(gtx layout.Context) { b.Layout(gtx, fill) })

	s.queue(at(pointer.Press, centre), at(pointer.Release, centre))
	if !b.Clicked(s.gtx) {
		t.Fatal("a press and a release inside the area were not a click")
	}
	if b.Clicked(s.gtx) {
		t.Error("the same click was reported twice")
	}
	s.frame()
	if b.Clicked(s.gtx) {
		t.Error("the click came back in the next frame")
	}
}

// TestClickableReleaseOutsideIsNotAClick fixes the only way a press is taken
// back by hand.
//
// Letting go outside is how somebody who pressed the wrong thing says so, and
// a control that acted anyway would make the decision unrecallable.
func TestClickableReleaseOutsideIsNotAClick(t *testing.T) {
	var b widget.Clickable
	s := newClickStage(t, func(gtx layout.Context) { b.Layout(gtx, fill) })

	s.queue(at(pointer.Press, centre))
	if b.Clicked(s.gtx) {
		t.Fatal("a press alone was reported as a click")
	}
	if !b.Pressed() {
		t.Fatal("the control was not held down after the press")
	}

	s.frame()
	s.queue(at(pointer.Move, outside), at(pointer.Release, outside))
	if b.Clicked(s.gtx) {
		t.Error("a release outside the area was reported as a click")
	}
	if b.Pressed() {
		t.Error("the control stayed held down after the release")
	}
}

// TestClickableCancelClearsTheHeldState fixes what happens when the press is
// taken away rather than given up.
//
// It is what a list does to the row under the finger that scrolled it. The row
// must stop drawing itself held and must not act, and the press it recorded is
// marked as the one that did not count.
func TestClickableCancelClearsTheHeldState(t *testing.T) {
	var b widget.Clickable
	s := newClickStage(t, func(gtx layout.Context) { b.Layout(gtx, fill) })

	s.queue(at(pointer.Press, centre))
	if b.Clicked(s.gtx) {
		t.Fatal("a press alone was reported as a click")
	}
	if got := b.History(); len(got) != 1 || !got[0].End.IsZero() || got[0].Cancelled {
		t.Fatalf("a press in progress was recorded as %#v", got)
	}

	s.frame()
	s.queue(pointer.Event{Kind: pointer.Cancel})
	if b.Clicked(s.gtx) {
		t.Error("a cancelled press was reported as a click")
	}
	if b.Pressed() {
		t.Error("the control stayed held down after the cancel")
	}
	got := b.History()
	if len(got) != 1 {
		t.Fatalf("expected the cancelled press to be kept, got %#v", got)
	}
	if !got[0].Cancelled {
		t.Error("the press was not marked cancelled")
	}
	if got[0].End.IsZero() {
		t.Error("the cancelled press was left without an end")
	}
}

// TestClickableCancelLeavesCompletedHistoryAlone fixes that cancelling the
// current press cannot rewrite the outcome of an earlier click. Consumers use
// the history to animate each press independently, including when one
// Clickable is drawn in more than one place.
func TestClickableCancelLeavesCompletedHistoryAlone(t *testing.T) {
	var b widget.Clickable
	s := newClickStage(t, func(gtx layout.Context) { b.Layout(gtx, fill) })

	s.queue(at(pointer.Press, centre), at(pointer.Release, centre))
	if !b.Clicked(s.gtx) {
		t.Fatal("the completed press was not reported as a click")
	}
	s.frame()
	s.queue(at(pointer.Press, centre))
	if b.Clicked(s.gtx) {
		t.Fatal("the second press completed before it was cancelled")
	}
	s.frame()
	s.queue(pointer.Event{Kind: pointer.Cancel})
	if b.Clicked(s.gtx) {
		t.Fatal("the cancelled press was reported as a click")
	}

	history := b.History()
	if len(history) != 2 {
		t.Fatalf("history has %d presses, want 2: %#v", len(history), history)
	}
	if history[0].Cancelled {
		t.Error("cancelling the current press retroactively cancelled the completed click")
	}
	if !history[1].Cancelled {
		t.Error("the current press was not marked cancelled")
	}
}

// TestClickableDrawnTwiceIsOneControlInTwoPlaces fixes the consequence of the
// state being the caller's.
//
// One value drawn in two places is one control with two areas, and it answers
// for both. This is a defect where two buttons were meant, and it is here so
// that it is a decision rather than a surprise: two controls are two values.
func TestClickableDrawnTwiceIsOneControlInTwoPlaces(t *testing.T) {
	var b widget.Clickable
	s := newClickStage(t, func(gtx layout.Context) {
		b.Layout(gtx, fill)
		defer op.Offset(image.Pt(200, 0)).Push(gtx.Ops).Pop()
		b.Layout(gtx, fill)
	})

	s.queue(at(pointer.Press, f32.Pt(250, 50)), at(pointer.Release, f32.Pt(250, 50)))
	if !b.Clicked(s.gtx) {
		t.Error("the second place the control was drawn in did not answer")
	}
}

// TestClickableProgrammaticClickIsReportedOnce fixes that a click asked for in
// code arrives the same way as one made by hand.
//
// It is how a control is driven from a keyboard shortcut or a test, and it is
// consumed like any other: a request that stayed set would fire every frame.
func TestClickableProgrammaticClickIsReportedOnce(t *testing.T) {
	var b widget.Clickable
	s := newClickStage(t, func(gtx layout.Context) { b.Layout(gtx, fill) })

	b.Click()
	if !b.Clicked(s.gtx) {
		t.Fatal("a requested click was not reported")
	}
	if b.Clicked(s.gtx) {
		t.Error("a requested click was reported twice")
	}
}

// TestClickableKeyboardNeedsPressAndRelease fixes the keyboard half.
//
// A control reachable only by pointer is a control somebody cannot reach at
// all, and the pair is what makes the key a press rather than a repeat: a key
// held down sends presses until it is let go.
func TestClickableKeyboardNeedsPressAndRelease(t *testing.T) {
	var b widget.Clickable
	s := newClickStage(t, func(gtx layout.Context) { b.Layout(gtx, fill) })
	s.gtx.Execute(key.FocusCmd{Tag: &b})
	s.frame()
	if !s.gtx.Focused(&b) {
		t.Fatal("the control did not take focus")
	}

	s.queue(key.Event{Name: key.NameSpace, State: key.Press})
	if b.Clicked(s.gtx) {
		t.Error("a key held down was reported as a click")
	}
	s.queue(key.Event{Name: key.NameSpace, State: key.Release})
	if !b.Clicked(s.gtx) {
		t.Error("a key pressed and released was not a click")
	}
}

// TestClickableKeyboardIgnoresAReleaseItDidNotSee fixes the case where the key
// went down somewhere else.
//
// Focus can move while a key is held -- that is what a shortcut that moves it
// does -- and the control that inherits the release never saw the press. Acting
// on it would be a control pressing itself.
func TestClickableKeyboardIgnoresAReleaseItDidNotSee(t *testing.T) {
	var b widget.Clickable
	s := newClickStage(t, func(gtx layout.Context) { b.Layout(gtx, fill) })
	s.gtx.Execute(key.FocusCmd{Tag: &b})
	s.frame()

	s.queue(key.Event{Name: key.NameReturn, State: key.Release})
	if b.Clicked(s.gtx) {
		t.Error("a release with no press before it was reported as a click")
	}
}

// TestClickableHoverFollowsThePointer fixes that crossing the edge is reported
// while nothing is pressed, which is the state a control draws itself lit from.
func TestClickableHoverFollowsThePointer(t *testing.T) {
	var b widget.Clickable
	s := newClickStage(t, func(gtx layout.Context) { b.Layout(gtx, fill) })

	s.queue(at(pointer.Move, centre))
	b.Clicked(s.gtx)
	if !b.Hovered() {
		t.Fatal("the pointer inside the area did not register as hovering")
	}

	s.frame()
	s.queue(at(pointer.Move, outside))
	b.Clicked(s.gtx)
	if b.Hovered() {
		t.Error("the pointer outside the area still registered as hovering")
	}
}
