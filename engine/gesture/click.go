package gesture

import (
	"image"
	"time"

	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
)

// doubleClickDuration is how soon a second press has to arrive to count as part
// of the same click.
//
// It is measured press to press rather than release to press, so a first click
// held down a moment longer does not break the run. Shorter than this and a
// double click that was not quite quick enough arrives as two separate ones;
// longer, and two deliberate clicks on the same spot are read as one double.
const doubleClickDuration = 200 * time.Millisecond

// ClickKind is what a [ClickEvent] reports.
type ClickKind uint8

const (
	// KindPress is the pointer going down inside the area.
	//
	// It is reported as it happens rather than held back until the outcome is
	// known, because a control draws itself held from this moment. A press
	// with no feedback until the release is a control that looks broken for as
	// long as a finger rests on it.
	KindPress ClickKind = iota
	// KindClick is a press that completed: the pointer came back up with the
	// gesture still holding it, over the area it started on. This is the one a
	// control acts on.
	KindClick
	// KindCancel is a press that ended without becoming a click -- let go
	// outside the area, or taken away by whoever claimed the pointer. A
	// control that drew itself held has to stop, and must not act.
	KindCancel
)

// String names the kind.
func (ct ClickKind) String() string {
	switch ct {
	case KindPress:
		return "KindPress"
	case KindClick:
		return "KindClick"
	case KindCancel:
		return "KindCancel"
	default:
		panic("invalid ClickKind")
	}
}

// ClickEvent is one thing that happened to a [Click].
type ClickEvent struct {
	// Kind is what happened.
	Kind ClickKind
	// Position is where it happened, in the area's own coordinates.
	Position image.Point
	// Source is whether a mouse or a finger did it. A control that puts
	// something beside the pointer reads it: on a screen being touched there
	// is no cursor to put anything beside, and whatever is put there is under
	// the hand.
	Source pointer.Source
	// Modifiers are the keys held down at the press. They are carried from
	// the press to the click, so a control acting on the click can still tell
	// a plain one from a modified one.
	Modifiers key.Modifiers
	// NumClicks is the length of the run this press ends: one for a single
	// click, two for a double. It counts up while presses keep arriving within
	// [doubleClickDuration] of each other and starts again at one when they
	// stop.
	NumClicks int
}

// ImplementsEvent marks the type as something the input queue can deliver.
func (ClickEvent) ImplementsEvent() {}

// Click detects clicks over an area.
//
// It is the machine under every control that answers a press, and almost all of
// it is about the ways a press ends without being a click. A press begins when
// a pointer goes down inside the area and ends when that same pointer comes
// back up. Whether the end is a click depends on where the pointer was by then
// and on whether it was still this gesture's to read.
//
// The pointer stays this gesture's while it is down, even outside the area. The
// press is not dropped at the edge -- it is reported as no longer hovering, and
// crossing back in restores it, so a click whose pointer wandered a pixel out
// and back is still a click. Letting go outside is how a press is taken back by
// hand, and it is the only way to take one back by hand.
//
// The other way out is not the gesture's own doing: a pointer that travels far
// enough is claimed by whatever it is really dragging, and the claim cancels
// everything else under it. That is how a press on a row of a list stops being
// a press when the list is scrolled instead. See [touchSlop].
type Click struct {
	// clickedAt is when the last press arrived, and what the next one is
	// measured against.
	clickedAt time.Duration
	// clicks is how long the current run of presses is.
	clicks int
	// pressed is whether the pointer being followed is down.
	pressed bool
	// hovered is whether that pointer is inside the area.
	hovered bool
	// entered is whether this gesture has ever been told a pointer was
	// inside.
	//
	// It is not the same question as hovered and it is never cleared by a
	// crossing. It asks whether crossings are reported here at all: a gesture
	// that has never been told a pointer was inside cannot tell where a
	// release happened, and refusing the click would refuse every one of them.
	entered bool
	// pid is the pointer being followed.
	pid pointer.ID
}

// Add claims the current area, so that pointers over it reach this gesture.
func (c *Click) Add(ops *op.Ops) {
	event.Op(ops, c)
}

// Hovered reports whether the pointer is inside the area.
func (c *Click) Hovered() bool { return c.hovered }

// Pressed reports whether the pointer is down on it right now.
func (c *Click) Pressed() bool { return c.pressed }

// Update reads what has arrived and returns the next thing that happened.
//
// Call it until it answers false. One frame can hold a whole press and release,
// and a caller that takes a single event per frame falls a frame behind its own
// pointer for every event it left in the queue.
func (c *Click) Update(q input.Source) (ClickEvent, bool) {
	for {
		evt, ok := q.Event(pointer.Filter{
			Target: c,
			Kinds:  pointer.Press | pointer.Release | pointer.Enter | pointer.Leave | pointer.Cancel,
		})
		if !ok {
			return ClickEvent{}, false
		}
		e, ok := evt.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			if c.pressed {
				// A second pointer going down during a press is somebody
				// else's gesture. This one is already spoken for.
				break
			}
			if e.Source == pointer.Mouse && e.Buttons != pointer.ButtonPrimary {
				// The button set is only read for a mouse, because a finger
				// has none and reading it either way would refuse every touch.
				// The secondary button belongs to the context menu: a control
				// that took it for a press would act twice on one gesture.
				break
			}
			// The pointer is adopted at the press and not only at the
			// crossing. A platform may renumber a pointer between one press
			// and the next, and by then the area is already entered under the
			// new number, so no crossing follows to correct it. A gesture
			// still answering to the old number would go deaf.
			c.pid = e.PointerID
			c.pressed = true
			if e.Time-c.clickedAt < doubleClickDuration {
				c.clicks++
			} else {
				c.clicks = 1
			}
			c.clickedAt = e.Time
			return ClickEvent{
				Kind:      KindPress,
				Position:  e.Position.Round(),
				Source:    e.Source,
				Modifiers: e.Modifiers,
				NumClicks: c.clicks,
			}, true
		case pointer.Release:
			if !c.pressed || c.pid != e.PointerID {
				break
			}
			c.pressed = false
			if c.entered && !c.hovered {
				return ClickEvent{Kind: KindCancel}, true
			}
			return ClickEvent{
				Kind:      KindClick,
				Position:  e.Position.Round(),
				Source:    e.Source,
				Modifiers: e.Modifiers,
				NumClicks: c.clicks,
			}, true
		case pointer.Enter:
			if !c.pressed {
				c.pid = e.PointerID
			}
			if c.pid == e.PointerID {
				c.hovered = true
				c.entered = true
			}
		case pointer.Leave:
			// A crossing by another pointer while this one is down is another
			// hand on the screen, and says nothing about the press in
			// progress. Only when nothing is pressed does a crossing decide
			// which pointer this gesture is following.
			if !c.pressed {
				c.pid = e.PointerID
			}
			if c.pid == e.PointerID {
				c.hovered = false
			}
		case pointer.Cancel:
			// Whoever claimed the pointer took the press with it, and took the
			// crossings too: the gesture no longer knows where the pointer is,
			// so it goes back to knowing nothing at all.
			wasPressed := c.pressed
			c.pressed = false
			c.hovered = false
			c.entered = false
			if wasPressed {
				return ClickEvent{Kind: KindCancel}, true
			}
		}
	}
}
