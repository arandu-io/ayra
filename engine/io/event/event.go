// Package event is the vocabulary every other input package is written in:
// what a handler is called, what reaches it, and how it asks.
//
// Nothing here routes anything and nothing here keeps state. Three declarations
// and one operation are the whole package, because its job is to be agreed on
// rather than to do something.
//
// It is a package of its own for a reason about imports. Keys, pointers and
// transfers each declare their own events and their own filters, and the router
// accepts all of them. Were the two interfaces declared where the routing
// lives, every one of those packages would have to import the router, which
// already imports them. Nothing here imports any of them, and that is what
// keeps the arrangement from being a cycle.
package event

import (
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/op"
)

// Tag is the identity a handler answers to, and all the router knows it by.
//
// It holds any value, and what belongs in it is an address the handler owns --
// for a handler h, that is &h. The state kept for a handler between frames is
// stored under this key, which asks two things of whatever is put here.
//
// It has to be the same value on the next frame. A tag built while drawing is a
// new key every frame, so the lookup finds nothing and the handler starts over
// as one that was never pressed, never focused and never dragged from. An
// address is the same for as long as the thing it points at is.
//
// And it has to be one handler's alone. Two handlers sharing a tag are one
// entry, so what happens to either happens to both: two values have two
// addresses, and one value drawn twice has one -- which is a control in two
// places that cannot tell them apart.
//
// The type is any, so neither is checked here. A tag that cannot be a key at
// all -- anything holding a slice, a map or a function -- is accepted by this
// package and fails when the router first looks it up, on a frame, rather than
// at the line that wrote it.
type Tag any

// Event is what reaches a handler: a key pressed, a pointer moved, a transfer
// finished, a window closing.
//
// The method says nothing, returns nothing and is never called. It is here so
// that the set is joined deliberately: a type becomes an event by declaring one
// line, and a value arriving at a handler is therefore one that some package
// meant to send. Declared as any it would accept everything, and every mistake
// would surface at the type switch on the other end, where the only thing left
// to do with an unrecognised value is drop it.
//
// There is no behaviour to require instead. A key and a dropped file have
// nothing in common worth a method; what they share is that they happened and
// that something is waiting for them.
type Event interface {
	ImplementsEvent()
}

// Filter is a handler asking for one kind of event, and it names the tag it is
// asking for.
//
// Carrying the tag is what makes asking the whole of the subscription. The
// router reads the target out of each filter it is handed, so the call that
// collects events is also the call that says who wants them: there is no
// register step to forget and none to take back, and what a handler is
// listening for is exactly what it asked for on the frame it asked.
//
// Events are asked for rather than delivered because of what a control is. It
// is a function that runs once a frame and remembers nothing between runs
// except what its caller holds for it. A callback would arrive outside that
// run, with nowhere to put what it changed and nothing drawing the result until
// something unrelated caused the next frame. Asked for at the top of the
// function instead, an event changes the state and the same frame draws it --
// which is why a control asks before it draws, and why asking afterwards costs
// the frame that is read as lag.
//
// One call answers one event and reports false when there are none left, so a
// control drains what it asked for in a loop and leaves nothing queued behind
// it.
type Filter interface {
	ImplementsFilter()
}

// Op declares a tag at this point in the operation list, which is what makes it
// a handler at all.
//
// Where the call sits is the answer to where the handler is. Replaying the list
// gives the router the transformation and the clip area in force at this point,
// and those are the shape a pointer has to be inside to reach this tag. Put
// above the clip that describes a control, it hands over the parent's area
// instead, and the control then answers presses that landed outside it.
//
// It is also how long the handler lives. State belonging to a tag that went a
// whole frame unmentioned -- neither declared here nor named in a filter -- is
// dropped, so a control that is not drawn is not a handler, and one that comes
// back comes back remembering nothing. That is the right answer for a control
// that was gone, and a surprise to anything that expected to hold the focus
// across the frames it skipped.
//
// A nil tag panics here instead of being recorded. It cannot key anything, so
// the router would fail on it too -- but it would fail mid-frame, with nothing
// left pointing at the call that wrote it.
func Op(o *op.Ops, tag Tag) {
	if tag == nil {
		panic("Tag must be non-nil")
	}
	data := ops.Write1(&o.Internal, ops.TypeInputLen, tag)
	data[0] = byte(ops.TypeInput)
}
