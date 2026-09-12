package gesture

import (
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
)

// Hover reports whether a pointer is over an area.
//
// The answer is the state rather than an event, and that is the difference
// between this and every other gesture here: a control asks while it is
// drawing, and what it needs is where the pointer is now -- not a record of the
// frame it arrived in, which it would then have to keep itself.
//
// There are two states and both are named by what the pointer did to reach
// them. It is inside from the moment the area reports the crossing, and outside
// again when the pointer crosses back out or when the pointer is taken away
// entirely.
type Hover struct {
	// entered is whether the pointer is inside the area.
	entered bool
	// pid is the pointer the state belongs to.
	pid pointer.ID
}

// Add claims the current area, so that pointers crossing it reach this gesture.
func (h *Hover) Add(ops *op.Ops) {
	event.Op(ops, h)
}

// Update reads what has arrived and reports whether a pointer is inside.
//
// A cancellation ends the hover exactly as leaving does. A pointer can go away
// without ever crossing the edge -- the window loses it, the system claims it
// for a gesture of its own -- and a control that waited for the crossing stays
// lit with nothing over it.
func (h *Hover) Update(q input.Source) bool {
	for {
		ev, ok := q.Event(pointer.Filter{
			Target: h,
			Kinds:  pointer.Enter | pointer.Leave | pointer.Cancel,
		})
		if !ok {
			return h.entered
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Enter:
			h.pid = e.PointerID
			h.entered = true
		case pointer.Leave, pointer.Cancel:
			// Only the pointer that is inside can leave. With two fingers on
			// the screen, one of them crossing out says nothing about where
			// the other one is.
			if h.entered && h.pid == e.PointerID {
				h.entered = false
			}
		}
	}
}
