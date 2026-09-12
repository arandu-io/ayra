package gesture

import (
	"image"
	"testing"
	"time"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/unit"
)

// bounds is the area every script below presses inside of, and away is a point
// outside it. A gesture is a state machine over a region, so half of what these
// tests say is about which side of the edge the pointer was on.
var bounds = image.Rect(0, 0, 100, 100)

var (
	centre = f32.Pt(50, 50)
	away   = f32.Pt(150, 150)
)

// metric is one pixel per dp, which makes touchSlop exactly three pixels and
// lets a script say how far the pointer travelled in the numbers it writes.
var metric = unit.Metric{}

// epoch is the frame clock the scroll scripts start on. Only distances from it
// mean anything.
var epoch = time.Unix(0, 0)

// scrollRange is wider than anything a script scrolls, so nothing is clamped on
// the way in and a wrong total is the gesture's own arithmetic.
var scrollRange = pointer.ScrollRange{Min: -1000, Max: 1000}

// participant is one handler in a script: how it claims an area, and what it
// does with the events it is handed.
//
// Both halves happen every frame. An area with no filter behind it is not a
// handler and is never hit, and filters last exactly one frame -- a control that
// is not drawn is not listening.
type participant struct {
	add   func(*op.Ops)
	drain func(input.Source)
}

// stage is the frame loop the gestures live in.
//
// It exists because these are state machines across frames, and the frame
// boundary is where some of the transitions happen: a grab is requested during
// a frame and taken at the end of it, so what it cancels is cancelled in the
// next one. A script that queued events and read the answers without ever
// ending a frame would never see that half of the machine.
type stage struct {
	router  input.Router
	ops     op.Ops
	players []participant
}

func newStage(t *testing.T, players ...participant) *stage {
	t.Helper()
	s := &stage{players: players}
	s.frame()
	return s
}

// frame ends one frame and begins the next: every participant claims its area
// and declares its filters again, and takes whatever arrived since.
//
// The areas are pushed pass-through so that two gestures can share the same
// pixels. That is not convenience -- the router hands a lone handler the grab
// for free, so a threshold that exists to take the pointer away from somebody
// else is unreachable while the gesture is alone in its area.
func (s *stage) frame() {
	s.ops.Reset()
	pass := pointer.PassOp{}.Push(&s.ops)
	for _, p := range s.players {
		stack := clip.Rect(bounds).Push(&s.ops)
		p.add(&s.ops)
		stack.Pop()
	}
	pass.Pop()

	for _, p := range s.players {
		p.drain(s.router.Source())
	}
	s.router.Frame(&s.ops)
}

// queue delivers events and runs the frame that reads them.
func (s *stage) queue(evts ...event.Event) {
	s.router.Queue(evts...)
	s.frame()
}

// hoverer is a [Hover] that remembers its last answer, so a script can read it
// after the frame that produced it.
type hoverer struct {
	hover   Hover
	hovered bool
}

func (h *hoverer) player() participant {
	return participant{
		add:   h.hover.Add,
		drain: func(s input.Source) { h.hovered = h.hover.Update(s) },
	}
}

// clicker is a [Click] that keeps what it reported.
type clicker struct {
	click Click
	got   []ClickEvent
}

func (c *clicker) player() participant {
	return participant{
		add: c.click.Add,
		drain: func(s input.Source) {
			for {
				e, ok := c.click.Update(s)
				if !ok {
					return
				}
				c.got = append(c.got, e)
			}
		},
	}
}

// took returns and forgets everything reported since the last call.
func (c *clicker) took() []ClickEvent {
	out := c.got
	c.got = nil
	return out
}

// dragger is a [Drag] that keeps the events it passed on.
type dragger struct {
	drag Drag
	axis Axis
	got  []pointer.Event
}

func (d *dragger) player() participant {
	return participant{
		add: d.drag.Add,
		drain: func(s input.Source) {
			for {
				e, ok := d.drag.Update(metric, s, d.axis)
				if !ok {
					return
				}
				d.got = append(d.got, e)
			}
		},
	}
}

func (d *dragger) took() []pointer.Event {
	out := d.got
	d.got = nil
	return out
}

// scroller is a [Scroll] that adds up what it reported and carries the clock
// the fling is measured against.
type scroller struct {
	scroll Scroll
	axis   Axis
	now    time.Time
	total  int
}

func (s *scroller) player() participant {
	return participant{
		add: s.scroll.Add,
		drain: func(src input.Source) {
			s.total += s.scroll.Update(metric, src, s.now, s.axis, scrollRange, scrollRange)
		},
	}
}

func (s *scroller) took() int {
	out := s.total
	s.total = 0
	return out
}

// bystander is a handler that shares the area and does nothing with what it
// receives.
//
// It stops the router granting the gesture under test an implicit grab: with a
// single handler in the area every event already arrives Grabbed, and the
// distance a pointer must travel before it takes the grab is never consulted.
// Its filter names presses alone, so the cancellation a grab sends to the
// losers is not a message it is waiting for.
func bystander(tag event.Tag) participant {
	return participant{
		add: func(ops *op.Ops) { event.Op(ops, tag) },
		drain: func(s input.Source) {
			for {
				if _, ok := s.Event(pointer.Filter{Target: tag, Kinds: pointer.Press}); !ok {
					return
				}
			}
		},
	}
}

// mouse builds a mouse event. A press holds the primary button, because that is
// the only one a click answers to.
func mouse(kind pointer.Kind, p f32.Point) pointer.Event {
	e := pointer.Event{Kind: kind, Source: pointer.Mouse, Position: p}
	if kind == pointer.Press {
		e.Buttons = pointer.ButtonPrimary
	}
	return e
}

// touch builds a touch event at a moment. The moment is what the scroll paths
// need: the velocity behind a fling is estimated from positions and the times
// they arrived at.
func touch(kind pointer.Kind, p f32.Point, when time.Duration) pointer.Event {
	return pointer.Event{Kind: kind, Source: pointer.Touch, Position: p, Time: when}
}

// wheel builds a scroll event, which belongs to no pointer and is delivered
// wherever it happened.
func wheel(by f32.Point) pointer.Event {
	return pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: centre, Scroll: by}
}

// kinds reduces reported clicks to what a script usually asserts on.
func kinds(evts []ClickEvent) []ClickKind {
	out := make([]ClickKind, len(evts))
	for i, e := range evts {
		out[i] = e.Kind
	}
	return out
}

func wantKinds(t *testing.T, got []ClickEvent, want ...ClickKind) {
	t.Helper()
	have := kinds(got)
	if len(have) != len(want) {
		t.Fatalf("got %v, want %v", have, want)
	}
	for i := range have {
		if have[i] != want[i] {
			t.Fatalf("got %v, want %v", have, want)
		}
	}
}

func TestHoverFollowsThePointer(t *testing.T) {
	var h hoverer
	s := newStage(t, h.player())

	if h.hovered {
		t.Fatal("hovered before the pointer arrived")
	}

	s.queue(mouse(pointer.Move, centre))
	if !h.hovered {
		t.Fatal("not hovered with the pointer inside the area")
	}

	s.queue(mouse(pointer.Move, away))
	if h.hovered {
		t.Fatal("still hovered with the pointer outside the area")
	}
}

// A hover ends when the system takes the pointer away, and not only when it is
// moved out. Without this the pointer leaves with the window and the control is
// left lit with nothing over it.
func TestHoverEndsOnCancel(t *testing.T) {
	var h hoverer
	s := newStage(t, h.player())

	s.queue(mouse(pointer.Move, centre))
	if !h.hovered {
		t.Fatal("not hovered with the pointer inside the area")
	}

	s.queue(pointer.Event{Kind: pointer.Cancel})
	if h.hovered {
		t.Fatal("still hovered after the pointer was cancelled")
	}
}

// The cancellation carries no pointer number, and the hover has to end anyway.
// A finger is rarely the pointer numbered zero, and matching the number the way
// a crossing is matched would leave every control a second finger touched lit
// with nothing over it.
func TestHoverEndsOnCancelOfANumberedPointer(t *testing.T) {
	var h hoverer
	s := newStage(t, h.player())

	arrive := mouse(pointer.Move, centre)
	arrive.PointerID = 1
	s.queue(arrive)
	if !h.hovered {
		t.Fatal("not hovered with the pointer inside the area")
	}

	s.queue(pointer.Event{Kind: pointer.Cancel})
	if h.hovered {
		t.Fatal("still hovered after the pointer was cancelled")
	}
}

// The press is reported as it happens and the click when the pointer comes back
// up, which is the whole shape of the gesture: a control draws itself held down
// on the first and acts on the second.
func TestClickReportsPressThenClick(t *testing.T) {
	var c clicker
	s := newStage(t, c.player())

	s.queue(mouse(pointer.Move, centre), mouse(pointer.Press, centre))
	wantKinds(t, c.took(), KindPress)
	if !c.click.Pressed() {
		t.Error("not pressed while the pointer is down")
	}
	if !c.click.Hovered() {
		t.Error("not hovered with the pointer inside the area")
	}

	s.queue(mouse(pointer.Release, centre))
	wantKinds(t, c.took(), KindClick)
	if c.click.Pressed() {
		t.Error("still pressed after the pointer came up")
	}
}

// What the press carries is what a control reads to place a menu, to extend a
// selection, or to tell a plain click from a modified one.
func TestClickCarriesTheEventItCameFrom(t *testing.T) {
	var c clicker
	s := newStage(t, c.player())

	press := mouse(pointer.Press, f32.Pt(12, 34))
	press.Modifiers = key.ModShift
	s.queue(mouse(pointer.Move, f32.Pt(12, 34)), press)

	got := c.took()
	wantKinds(t, got, KindPress)
	if want := image.Pt(12, 34); got[0].Position != want {
		t.Errorf("got position %v, want %v", got[0].Position, want)
	}
	if got[0].Source != pointer.Mouse {
		t.Errorf("got source %v, want mouse", got[0].Source)
	}
	if got[0].Modifiers != key.ModShift {
		t.Errorf("got modifiers %v, want shift", got[0].Modifiers)
	}
	if got[0].NumClicks != 1 {
		t.Errorf("got %d clicks, want 1", got[0].NumClicks)
	}
}

// Pressing a control and letting go somewhere else is how a press is taken
// back, and it is the only way to take one back once it has begun.
func TestClickReleasedOutsideIsCancelled(t *testing.T) {
	var c clicker
	s := newStage(t, c.player())

	s.queue(mouse(pointer.Move, centre), mouse(pointer.Press, centre))
	wantKinds(t, c.took(), KindPress)

	s.queue(mouse(pointer.Move, away), mouse(pointer.Release, away))
	wantKinds(t, c.took(), KindCancel)
}

// Leaving the area and coming back does not end the press: the pointer is still
// down and still this gesture's, so the release lands as a click. A gesture that
// dropped the press at the edge would refuse every click whose pointer wandered
// a pixel out and back.
func TestClickReturnsToTheAreaAndStillClicks(t *testing.T) {
	var c clicker
	s := newStage(t, c.player())

	s.queue(mouse(pointer.Move, centre), mouse(pointer.Press, centre))
	wantKinds(t, c.took(), KindPress)

	s.queue(mouse(pointer.Move, away))
	if got := c.took(); len(got) != 0 {
		t.Fatalf("got %v leaving the area, want nothing", kinds(got))
	}
	if c.click.Hovered() {
		t.Error("hovered with the pointer outside the area")
	}
	if !c.click.Pressed() {
		t.Error("the press was dropped at the edge of the area")
	}

	s.queue(mouse(pointer.Move, centre), mouse(pointer.Release, centre))
	wantKinds(t, c.took(), KindClick)
}

// The system taking the pointer away mid-press cancels the click, and the
// control that was drawing itself held down stops.
func TestClickCancelledMidPress(t *testing.T) {
	var c clicker
	s := newStage(t, c.player())

	s.queue(mouse(pointer.Move, centre), mouse(pointer.Press, centre))
	wantKinds(t, c.took(), KindPress)

	s.queue(pointer.Event{Kind: pointer.Cancel})
	wantKinds(t, c.took(), KindCancel)
	if c.click.Pressed() || c.click.Hovered() {
		t.Error("still pressed or hovered after the pointer was cancelled")
	}
}

// A press that travels far enough is a drag, and a drag is not a click.
//
// The distance is not measured by the click: it is measured by whoever wants
// the pointer, and taking it cancels everybody else. This is what stops a row
// firing when the list it sits in was scrolled with a finger on top of it. The
// grab is requested during a frame and taken at the end of it, so the
// cancellation lands in the frame after the one that crossed the threshold.
func TestClickCancelledByAGrab(t *testing.T) {
	var c clicker
	var d dragger
	d.axis = Both
	s := newStage(t, c.player(), d.player())

	s.queue(touch(pointer.Press, f32.Pt(50, 50), 0))
	wantKinds(t, c.took(), KindPress)
	d.took()

	// Two pixels, inside the slop: nobody has claimed the pointer.
	s.queue(touch(pointer.Move, f32.Pt(50, 52), 10*time.Millisecond))
	d.took()
	if got := c.took(); len(got) != 0 {
		t.Fatalf("got %v moving within the slop, want nothing", kinds(got))
	}

	// Ten pixels, past it: the drag asks for the pointer.
	s.queue(touch(pointer.Move, f32.Pt(50, 62), 20*time.Millisecond))
	d.took()
	s.frame()
	wantKinds(t, c.took(), KindCancel)

	s.queue(touch(pointer.Release, f32.Pt(50, 62), 30*time.Millisecond))
	if got := c.took(); len(got) != 0 {
		t.Fatalf("got %v releasing after the grab, want nothing", kinds(got))
	}
}

// The secondary button is what opens a context menu, and a control that took it
// for a press would act twice on one gesture.
func TestClickIgnoresTheSecondaryButton(t *testing.T) {
	var c clicker
	s := newStage(t, c.player())

	press := mouse(pointer.Press, centre)
	press.Buttons = pointer.ButtonSecondary
	s.queue(mouse(pointer.Move, centre), press, mouse(pointer.Release, centre))

	if got := c.took(); len(got) != 0 {
		t.Fatalf("got %v for a secondary press, want nothing", kinds(got))
	}
}

// A touch has no buttons to inspect, so the button set is only consulted for a
// mouse. Read either way, it would refuse every finger.
func TestClickAcceptsATouchWithNoButtons(t *testing.T) {
	var c clicker
	s := newStage(t, c.player())

	s.queue(
		touch(pointer.Press, centre, 0),
		touch(pointer.Release, centre, 10*time.Millisecond),
	)
	wantKinds(t, c.took(), KindPress, KindClick)
}

// Successive presses are counted while they keep arriving inside the window,
// and the count is what tells a double click from two clicks. The window runs
// from press to press, so a slow release does not break the run.
func TestClickCountsRepeats(t *testing.T) {
	for _, tc := range []struct {
		label string
		times []time.Duration
		want  []int
	}{
		{
			label: "one press is one click",
			times: []time.Duration{200 * time.Millisecond},
			want:  []int{1},
		},
		{
			label: "a second press inside the window counts two",
			times: []time.Duration{0, doubleClickDuration - 1},
			want:  []int{1, 2},
		},
		{
			label: "a third inside the window counts three",
			times: []time.Duration{0, doubleClickDuration - 1, 2*doubleClickDuration - 2},
			want:  []int{1, 2, 3},
		},
		{
			label: "the window runs from the last press, not the first",
			times: []time.Duration{0, doubleClickDuration + 1},
			want:  []int{1, 1},
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			var c clicker
			s := newStage(t, c.player())

			evts := make([]event.Event, 0, 2*len(tc.times))
			for _, when := range tc.times {
				press := mouse(pointer.Press, centre)
				press.Time = when
				release := mouse(pointer.Release, centre)
				release.Time = when
				evts = append(evts, press, release)
			}
			s.queue(evts...)

			var got []int
			for _, e := range c.took() {
				if e.Kind == KindClick {
					got = append(got, e.NumClicks)
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v clicks, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v clicks, want %v", got, tc.want)
				}
			}
		})
	}
}

// The gesture follows whichever pointer pressed it, and adopts a new identifier
// at the press.
//
// A platform is free to renumber a pointer between one press and the next, and
// a gesture that answered only to the number it first saw would go deaf after
// the renumbering: the area is already entered under the new number, so no
// enter event arrives to correct it and the press is the only chance.
//
// The release is what the assertion turns on rather than the press. A press is
// reported whatever number it carries -- it is the release that is matched
// against the pointer being followed, so a gesture that never adopted the new
// number still reports the press and then drops the click.
func TestClickAdoptsTheNewPointerID(t *testing.T) {
	var c clicker
	s := newStage(t, c.player())

	numbered := func(kind pointer.Kind, id pointer.ID) pointer.Event {
		e := mouse(kind, centre)
		e.PointerID = id
		return e
	}

	s.queue(numbered(pointer.Move, 1), numbered(pointer.Press, 1))
	c.took()
	// The second pointer arrives while the first is down. The router records
	// the area as entered for it, so no enter event will follow.
	s.queue(numbered(pointer.Move, 2))
	c.took()
	s.queue(numbered(pointer.Release, 1))
	c.took()

	s.queue(numbered(pointer.Press, 2), numbered(pointer.Release, 2))
	wantKinds(t, c.took(), KindPress, KindClick)
}

// A drag passes on the events it recognises and says, between them, whether it
// is in use. What it passes on is the pointer event itself, because a drag is
// the positions: a caller given only the distance would have to keep the
// previous one to do anything with it.
func TestDragReportsTheSequence(t *testing.T) {
	var d dragger
	d.axis = Both
	s := newStage(t, d.player())

	if d.drag.Dragging() || d.drag.Pressed() {
		t.Fatal("in use before anything was pressed")
	}

	s.queue(touch(pointer.Press, f32.Pt(10, 10), 0))
	got := d.took()
	if len(got) != 1 || got[0].Kind != pointer.Press {
		t.Fatalf("got %d events at the press, want one press", len(got))
	}
	if !d.drag.Dragging() || !d.drag.Pressed() {
		t.Error("not in use while the pointer is down")
	}

	s.queue(touch(pointer.Move, f32.Pt(30, 40), 10*time.Millisecond))
	got = d.took()
	if len(got) != 1 || got[0].Kind != pointer.Drag {
		t.Fatalf("got %d events moving, want one drag", len(got))
	}
	if got[0].Position != f32.Pt(30, 40) {
		t.Errorf("got position %v, want (30, 40)", got[0].Position)
	}

	s.queue(touch(pointer.Release, f32.Pt(30, 40), 20*time.Millisecond))
	got = d.took()
	if len(got) != 1 || got[0].Kind != pointer.Release {
		t.Fatalf("got %d events at the release, want one release", len(got))
	}
	if d.drag.Dragging() || d.drag.Pressed() {
		t.Error("still in use after the pointer came up")
	}
}

// A drag along one axis reports the other coordinate as it was at the press. A
// control that reads both -- a slider, a scrollbar -- would otherwise follow the
// hand sideways off its own track.
func TestDragPinsTheOtherAxis(t *testing.T) {
	for _, tc := range []struct {
		axis Axis
		want f32.Point
	}{
		{axis: Horizontal, want: f32.Pt(30, 10)},
		{axis: Vertical, want: f32.Pt(10, 40)},
		{axis: Both, want: f32.Pt(30, 40)},
	} {
		t.Run(tc.axis.String(), func(t *testing.T) {
			var d dragger
			d.axis = tc.axis
			s := newStage(t, d.player())

			s.queue(touch(pointer.Press, f32.Pt(10, 10), 0))
			d.took()

			s.queue(touch(pointer.Move, f32.Pt(30, 40), 10*time.Millisecond))
			got := d.took()
			if len(got) != 1 {
				t.Fatalf("got %d events moving, want one", len(got))
			}
			if got[0].Position != tc.want {
				t.Errorf("got position %v, want %v", got[0].Position, tc.want)
			}
		})
	}
}

// A drag is begun by the primary button or by a finger, and by nothing else.
func TestDragIgnoresTheSecondaryButton(t *testing.T) {
	var d dragger
	d.axis = Both
	s := newStage(t, d.player())

	press := mouse(pointer.Press, centre)
	press.Buttons = pointer.ButtonSecondary
	s.queue(press)

	if got := d.took(); len(got) != 0 {
		t.Fatalf("got %d events for a secondary press, want none", len(got))
	}
	if d.drag.Dragging() || d.drag.Pressed() {
		t.Error("in use after a press it does not answer to")
	}
}

// A cancelled drag ends where a release would have ended it, and the event is
// passed on so that a control holding something can put it back.
func TestDragEndsOnCancel(t *testing.T) {
	var d dragger
	d.axis = Both
	s := newStage(t, d.player())

	s.queue(touch(pointer.Press, centre, 0))
	d.took()

	s.queue(pointer.Event{Kind: pointer.Cancel})
	got := d.took()
	if len(got) != 1 || got[0].Kind != pointer.Cancel {
		t.Fatalf("got %d events, want one cancel", len(got))
	}
	if d.drag.Dragging() || d.drag.Pressed() {
		t.Error("still in use after the pointer was cancelled")
	}
}

// A wheel reports whole pixels along the axis the caller asked about, and the
// axis is the caller's because the same wheel scrolls a page one way and a row
// of cards the other.
func TestScrollReadsTheAxisItWasAsked(t *testing.T) {
	for _, tc := range []struct {
		axis Axis
		want int
	}{
		{axis: Horizontal, want: 3},
		{axis: Vertical, want: 5},
		{axis: Both, want: 8},
	} {
		t.Run(tc.axis.String(), func(t *testing.T) {
			sc := &scroller{axis: tc.axis, now: epoch}
			s := newStage(t, sc.player())

			s.queue(wheel(f32.Pt(3, 5)))
			if got := sc.took(); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// What a wheel reports is whole pixels and what it receives is not, so the
// remainder is kept rather than dropped. Dropped, a trackpad that sends a third
// of a pixel per frame scrolls nothing at all, for ever.
func TestScrollKeepsTheFraction(t *testing.T) {
	sc := &scroller{axis: Vertical, now: epoch}
	s := newStage(t, sc.player())

	for range 3 {
		s.queue(wheel(f32.Pt(0, 0.5)))
	}
	if got := sc.took(); got != 1 {
		t.Errorf("got %d pixels from three half-pixel scrolls, want 1", got)
	}
}

// A finger scrolls nothing until it has travelled far enough to claim the
// pointer, and then reports the whole travel at once. That is what keeps the
// content under the finger: the slop is a delay before the scroll begins, not
// a distance the content is short by afterwards.
func TestScrollTouchDragNeedsTheSlop(t *testing.T) {
	sc := &scroller{axis: Vertical, now: epoch}
	s := newStage(t, sc.player(), bystander(new(int)))

	s.queue(touch(pointer.Press, f32.Pt(50, 50), 0))
	if got := sc.took(); got != 0 {
		t.Fatalf("got %d at the press, want 0", got)
	}
	if sc.scroll.State() != StateDragging {
		t.Fatalf("got %v after a touch press, want %v", sc.scroll.State(), StateDragging)
	}

	// Two pixels, inside the slop: not a scroll yet.
	s.queue(touch(pointer.Move, f32.Pt(50, 48), 10*time.Millisecond))
	if got := sc.took(); got != 0 {
		t.Fatalf("got %d moving within the slop, want 0", got)
	}

	// Past the slop: the pointer is claimed at the end of this frame.
	s.queue(touch(pointer.Move, f32.Pt(50, 40), 20*time.Millisecond))
	sc.took()

	s.queue(touch(pointer.Move, f32.Pt(50, 30), 30*time.Millisecond))
	if got := sc.took(); got != 20 {
		t.Errorf("got %d once the pointer was claimed, want the whole 20 travelled", got)
	}
}

// A mouse drag is not a scroll on the platforms this runs on: the wheel is how
// a mouse scrolls, and a mouse dragging inside a list is selecting in it.
func TestScrollIgnoresAMouseDrag(t *testing.T) {
	sc := &scroller{axis: Vertical, now: epoch}
	s := newStage(t, sc.player(), bystander(new(int)))

	s.queue(
		mouse(pointer.Press, f32.Pt(50, 50)),
		mouse(pointer.Move, f32.Pt(50, 20)),
	)
	if got := sc.took(); got != 0 {
		t.Errorf("got %d for a mouse drag, want 0", got)
	}
	if sc.scroll.State() != StateIdle {
		t.Errorf("got %v for a mouse drag, want %v", sc.scroll.State(), StateIdle)
	}
}

// A finger let go while still moving keeps the content moving, and the movement
// is reported frame by frame from the velocity it was let go at. Stopping it is
// what a fresh press does, and what the caller can do directly.
func TestScrollFlingsAfterTheRelease(t *testing.T) {
	sc := &scroller{axis: Vertical, now: epoch}
	s := newStage(t, sc.player(), bystander(new(int)))

	s.queue(touch(pointer.Press, f32.Pt(50, 90), 0))
	for i := 1; i <= 5; i++ {
		when := time.Duration(i) * 10 * time.Millisecond
		sc.now = epoch.Add(when)
		s.queue(touch(pointer.Move, f32.Pt(50, float32(90-15*i)), when))
	}

	release := 60 * time.Millisecond
	sc.now = epoch.Add(release)
	s.queue(touch(pointer.Release, f32.Pt(50, 15), release))
	sc.took()

	if sc.scroll.State() != StateFlinging {
		t.Fatalf("got %v after a release in motion, want %v", sc.scroll.State(), StateFlinging)
	}

	sc.now = epoch.Add(release + 16*time.Millisecond)
	s.frame()
	if got := sc.took(); got == 0 {
		t.Error("the fling reported no movement")
	}

	sc.scroll.Stop()
	if sc.scroll.State() != StateIdle {
		t.Errorf("got %v after stopping, want %v", sc.scroll.State(), StateIdle)
	}
}

func TestAxisString(t *testing.T) {
	for _, tc := range []struct {
		axis Axis
		want string
	}{
		{axis: Horizontal, want: "Horizontal"},
		{axis: Vertical, want: "Vertical"},
		{axis: Both, want: "Both"},
	} {
		if got := tc.axis.String(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestClickKindString(t *testing.T) {
	for _, tc := range []struct {
		kind ClickKind
		want string
	}{
		{kind: KindPress, want: "KindPress"},
		{kind: KindClick, want: "KindClick"},
		{kind: KindCancel, want: "KindCancel"},
	} {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestScrollStateString(t *testing.T) {
	for _, tc := range []struct {
		state ScrollState
		want  string
	}{
		{state: StateIdle, want: "StateIdle"},
		{state: StateDragging, want: "StateDragging"},
		{state: StateFlinging, want: "StateFlinging"},
	} {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

// A click is an event, and the queue is what decides that. Without it the
// gesture compiles and nothing it reports can be delivered.
func TestClickEventIsAnEvent(t *testing.T) {
	var _ event.Event = ClickEvent{}
}
