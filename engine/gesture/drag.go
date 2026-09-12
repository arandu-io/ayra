package gesture

import (
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
)

// Drag detects a pointer being dragged across an area.
//
// What it reports is the pointer events themselves rather than a distance,
// because a drag is the positions: a caller handed only the step since the last
// frame would have to keep the previous one to do anything with it, and every
// caller would keep it differently. What the gesture does instead is the two
// things a caller should not have to repeat.
//
// The first is following one pointer. It takes the pointer that pressed and
// answers to nothing else until that pointer is released or cancelled, so a
// second hand on the screen does not move what the first one is holding.
//
// The second is asking for the pointer once the movement has gone far enough to
// be a drag rather than a press that has not let go yet. Until then the pointer
// is still shared with whatever else is under it, and the press could still
// turn out to be a click; afterwards it is this gesture's alone and everything
// else under it has been cancelled. See [touchSlop].
type Drag struct {
	// dragging is whether a pointer is being followed.
	dragging bool
	// pressed is whether a pointer is down.
	pressed bool
	// pid is the pointer being followed.
	pid pointer.ID
	// start is where that pointer went down, which is the coordinate the
	// pinned axis reports for the whole gesture.
	start f32.Point
}

// Add claims the current area, so that pointers over it reach this gesture.
func (d *Drag) Add(ops *op.Ops) {
	event.Op(ops, d)
}

// Dragging reports whether a pointer is being followed.
func (d *Drag) Dragging() bool { return d.dragging }

// Pressed reports whether a pointer is down on it.
func (d *Drag) Pressed() bool { return d.pressed }

// Update reads what has arrived and returns the next pointer event of the drag.
//
// Along a single axis the other coordinate is reported as it was at the press,
// rather than as it is. A control that reads both -- a slider, a scrollbar, a
// row being reordered -- would otherwise follow the hand sideways off its own
// track, and the hand always drifts sideways.
func (d *Drag) Update(cfg unit.Metric, q input.Source, axis Axis) (pointer.Event, bool) {
	for {
		ev, ok := q.Event(pointer.Filter{
			Target: d,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			return pointer.Event{}, false
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}

		switch e.Kind {
		case pointer.Press:
			if !(e.Buttons == pointer.ButtonPrimary || e.Source == pointer.Touch) {
				// A drag is begun by the primary button or by a finger. A
				// finger reports no buttons at all, which is why the two are
				// asked separately.
				continue
			}
			d.pressed = true
			if d.dragging {
				// Already following a pointer. The new one is another hand,
				// and taking it would move what the first one is holding.
				continue
			}
			d.dragging = true
			d.pid = e.PointerID
			d.start = e.Position
		case pointer.Drag:
			if !d.dragging || e.PointerID != d.pid {
				continue
			}
			switch axis {
			case Horizontal:
				e.Position.Y = d.start.Y
			case Vertical:
				e.Position.X = d.start.X
			case Both:
				// Both coordinates are the pointer's own.
			}
			if e.Priority < pointer.Grabbed {
				// Still shared. The distance is measured from the press and
				// not from the last frame, because a hand that creeps a pixel
				// per frame never crosses a per-frame threshold and would drag
				// the whole screen without ever claiming the pointer.
				diff := e.Position.Sub(d.start)
				slop := cfg.Dp(touchSlop)
				if diff.X*diff.X+diff.Y*diff.Y > float32(slop*slop) {
					q.Execute(pointer.GrabCmd{Tag: d, ID: e.PointerID})
				}
			}
		case pointer.Release, pointer.Cancel:
			d.pressed = false
			if !d.dragging || e.PointerID != d.pid {
				continue
			}
			d.dragging = false
		}

		return e, true
	}
}
