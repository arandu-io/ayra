// Package gesture turns pointer events into the actions a control acts on.
//
// What a window delivers is a stream of presses, moves and releases with no
// meaning attached. The same three events are a click, the beginning of a drag,
// or a scroll that has not started yet, and which one they are depends on where
// the pointer went next, how far, and how long it took. Deciding that is the
// whole of this package.
//
// Every gesture here is a state machine that outlives the frame, and the caller
// holds it. A control is drawn from nothing many times a second and remembers
// nothing by itself, so the same value has to be handed back every frame: a
// gesture copied per frame never sees the release of its own press, and the
// control it belongs to is held down for ever.
//
// Using one is two calls, both every frame. Add claims the area it listens
// over, and Update takes what has arrived since the last frame and says what it
// means. Neither works alone -- an area with no Update behind it declares no
// filters and is handed nothing, and an Update with no area is never under the
// pointer. What Update reports is reported once: a caller that reads it after
// drawing acts a frame late, and a frame of lag on a press is what reads as an
// application that has stopped answering.
package gesture

import "github.com/arandu-io/ayra/engine/unit"

// Axis is the direction a gesture reads movement along.
//
// It is the caller's choice rather than the gesture's, because the same
// hardware means different things to different surfaces: one wheel scrolls a
// page down and a row of cards sideways, and one finger moves a slider along
// its own track and nothing across it.
type Axis uint8

const (
	// Horizontal reads movement across and ignores movement up and down.
	Horizontal Axis = iota
	// Vertical reads movement up and down and ignores movement across.
	Vertical
	// Both reads movement in either direction. Where a single distance is
	// wanted it is the sum of the two, which is what a diagonal flick means to
	// a surface that can go either way.
	Both
)

// String names the axis.
func (a Axis) String() string {
	switch a {
	case Horizontal:
		return "Horizontal"
	case Vertical:
		return "Vertical"
	case Both:
		return "Both"
	default:
		panic("invalid Axis")
	}
}

// touchSlop is how far a pointer may travel before what it is doing stops being
// a press and becomes a drag.
//
// A pointer held still never is: the contact patch of a finger moves by a pixel
// or two from the pressure of holding it there, and a hand resting on a mouse
// does the same. Under this distance the movement is that noise and the press
// stands. Over it, the gesture asks the router for the pointer, and the router
// cancels every other handler under it -- which is what stops a row firing when
// the list it sits in was scrolled with a finger on top of it.
//
// It is in device independent pixels because a hand is the same size on every
// screen. Three pixels of a dense one is a distance no thumb notices, and three
// of a coarse one is still a deliberate movement.
const touchSlop = unit.Dp(3)
