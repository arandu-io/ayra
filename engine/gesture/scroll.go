package gesture

import (
	"math"
	"runtime"
	"time"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/internal/fling"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
)

// ScrollState is what a [Scroll] is doing.
type ScrollState uint8

const (
	// StateIdle is nothing moving it. A wheel still scrolls in this state: a
	// wheel is an amount, not a gesture with a beginning and an end.
	StateIdle ScrollState = iota
	// StateDragging is a pointer down and following it. The content moves
	// exactly as far as the pointer does, because the pointer is on it.
	StateDragging
	// StateFlinging is the pointer gone and the content still moving, slowing
	// to a stop from the speed it was let go at.
	StateFlinging
)

// String names the state.
func (s ScrollState) String() string {
	switch s {
	case StateIdle:
		return "StateIdle"
	case StateDragging:
		return "StateDragging"
	case StateFlinging:
		return "StateFlinging"
	default:
		panic("invalid ScrollState")
	}
}

// Scroll turns wheels, dragging fingers and the movement after a finger lets go
// into one distance to scroll by.
//
// Three sources and one answer, and they belong together because a surface that
// scrolls has to treat them as one thing: they arrive mixed, they have to be
// added up in the same frame, and a surface that read them separately would
// have to decide which of them wins when two arrive at once.
//
// A wheel needs no state -- it is a distance already. The other two are the
// state machine. A finger that goes down begins a drag, but the drag scrolls
// nothing until the finger has travelled far enough to claim the pointer from
// whatever else is under it; from then on the content follows the finger
// exactly. When the finger lifts while still moving, the speed it was moving at
// becomes a fling, and the fling reports a decreasing distance every frame
// until it stops. A fresh press ends a fling, which is what lets a hand catch a
// moving list.
type Scroll struct {
	// dragging is whether a pointer is down and being followed.
	dragging bool
	// estimator collects the positions and their times, which is what a
	// velocity can be read out of at the release.
	estimator fling.Extrapolation
	// flinger is the movement after the release.
	flinger fling.Animation
	// pid is the pointer being followed.
	pid pointer.ID
	// last is the position last reported on, in whole pixels. It only moves
	// once the pointer has been claimed: until then nothing has been reported,
	// and the first distance reported afterwards is the whole travel since the
	// press.
	last int
	// scroll is the fraction of a pixel left over from what has been reported.
	scroll float32
}

// Add claims the current area, so that pointers over it reach this gesture.
func (s *Scroll) Add(ops *op.Ops) {
	event.Op(ops, s)
}

// Stop ends any movement left over from a release.
//
// It is what a surface calls when the scroll has to stop for a reason the
// gesture cannot see: the content was replaced, the list reached an end it is
// not allowed to pass, or something else took over the screen.
func (s *Scroll) Stop() {
	s.flinger = fling.Animation{}
}

// State reports what the gesture is doing, which is what a surface reads to
// decide whether it is being handled right now.
func (s *Scroll) State() ScrollState {
	switch {
	case s.flinger.Active():
		return StateFlinging
	case s.dragging:
		return StateDragging
	default:
		return StateIdle
	}
}

// Update reads what has arrived and returns how far to scroll along axis, in
// pixels, since the last call.
//
// The time is the frame's, not the events'. Movement after a release is a
// function of how long it has been since the release, so it advances once per
// frame whether or not anything arrived, and the gesture asks for another frame
// for as long as it is still moving.
//
// The two ranges are the room the surface has left to scroll in each direction.
// They are declared to the router rather than applied to the answer, so that a
// surface already at its end never takes the wheel event at all and the one
// behind it -- a page under a list that has run out -- gets it instead.
func (s *Scroll) Update(cfg unit.Metric, q input.Source, t time.Time, axis Axis, scrollx, scrolly pointer.ScrollRange) int {
	total := 0
	f := pointer.Filter{
		Target:  s,
		Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Scroll | pointer.Cancel,
		ScrollX: scrollx,
		ScrollY: scrolly,
	}
	for {
		evt, ok := q.Event(f)
		if !ok {
			break
		}
		e, ok := evt.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Scroll:
			switch axis {
			case Horizontal:
				s.scroll += e.Scroll.X
			case Vertical:
				s.scroll += e.Scroll.Y
			case Both:
				s.scroll += e.Scroll.X + e.Scroll.Y
			}
			// What is reported is whole pixels, and what arrives is not. The
			// remainder is kept rather than dropped: dropped, a trackpad
			// sending a third of a pixel per frame scrolls nothing at all, for
			// ever.
			whole := int(s.scroll)
			s.scroll -= float32(whole)
			total += whole
		case pointer.Press:
			if s.dragging {
				break
			}
			if e.Source != pointer.Touch && runtime.GOOS != "android" {
				// A mouse scrolls with its wheel. A mouse dragging inside a
				// list is selecting in it, not moving it -- except on Android,
				// where a mouse is expected to drag the content by convention.
				break
			}
			// The press catches whatever the last one left moving.
			s.Stop()
			s.estimator = fling.Extrapolation{}
			v := s.val(axis, e.Position)
			s.last = int(math.Round(float64(v)))
			s.estimator.Sample(e.Time, v)
			s.dragging = true
			s.pid = e.PointerID
		case pointer.Drag:
			if !s.dragging || s.pid != e.PointerID {
				break
			}
			val := s.val(axis, e.Position)
			s.estimator.Sample(e.Time, val)
			v := int(math.Round(float64(val)))
			dist := s.last - v
			if e.Priority < pointer.Grabbed {
				// Still shared with whatever else is under the pointer, so
				// nothing is reported yet. Once the travel since the press is
				// past the slop the pointer is asked for, and the whole travel
				// is reported in the frame after it is granted -- the content
				// ends up under the finger rather than short by the slop.
				if slop := cfg.Dp(touchSlop); dist >= slop || -slop >= dist {
					q.Execute(pointer.GrabCmd{Tag: s, ID: e.PointerID})
				}
				break
			}
			s.last = v
			total += dist
		case pointer.Release:
			if s.pid != e.PointerID {
				break
			}
			// A finger let go while still moving leaves the content moving. A
			// finger that stopped before letting go does not, and the distance
			// the estimate covers is what tells them apart: below the slop the
			// movement is a hand coming to rest, and starting a fling from it
			// would drift the content under a finger that meant to stop.
			estimate := s.estimator.Estimate()
			if slop, d := float32(cfg.Dp(touchSlop)), estimate.Distance; d < -slop || d > slop {
				s.flinger.Start(cfg, t, estimate.Velocity)
			}
			s.dragging = false
		case pointer.Cancel:
			s.dragging = false
		}
	}
	total += s.flinger.Tick(t)
	if s.flinger.Active() {
		// Nothing is touching the screen, so nothing else will ask for the
		// frame that draws the next step of the movement.
		q.Execute(op.InvalidateCmd{})
	}
	return total
}

// val reduces a position to the single number the axis reads it as.
func (s *Scroll) val(axis Axis, p f32.Point) float32 {
	switch axis {
	case Horizontal:
		return p.X
	case Vertical:
		return p.Y
	case Both:
		return p.X + p.Y
	default:
		return 0
	}
}
