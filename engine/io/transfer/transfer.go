// Package transfer is dragging something onto something else, and pasting.
//
// Two halves describe themselves and the system decides whether they meet. One
// declares the kind of data it can produce, the other declares the kinds it
// understands, and when a drag crosses from the first to the second the two
// declarations are compared. Neither half is ever told the other exists, and
// neither is asked to name it: a screen that had to list what it could be
// dropped on would have to be edited every time something new was written.
//
// A source declares a [SourceFilter] for each kind it can produce. It hears an
// [InitiateEvent] when a drag begins, a [RequestEvent] when that drag is
// dropped on something that wants what it has, and a [CancelEvent] when the
// gesture is over however it ended. It answers a request with an [OfferCmd].
//
// A target declares a [TargetFilter] for each kind it accepts, and receives a
// [DataEvent] when data reaches it.
//
// A kind is a media type, and two of them meet only when they are the same
// string from end to end. There is no prefix, no wildcard and no family: a
// target asking for "image/" is asking for a type nothing produces, and a
// source offering "image/png" does not answer it. The permissive version reads
// as the kinder one and is not -- "text/" would gather markup, source, prose and
// comma-separated rows into one promise, and what a handler does with bytes in a
// shape it did not expect is worse than what it does with bytes it never got.
//
// # Who closes what arrives
//
// The data is a reader, and it is handed over rather than copied. Nothing here
// buffers a transfer: what a source puts in [OfferCmd] is the live handle, and
// executing the command gives it away. From that moment the source must keep it
// readable and must not close it -- it is no longer the source that knows when
// the reading is done.
//
// The reader reaches the target as [DataEvent.Open], and calling Open takes it
// over. Whoever calls it owes it a Close, whenever they are finished: reading
// may run on a goroutine of its own and finish long after the frame that
// delivered the event is drawn and gone, which is the point of passing a reader
// and not bytes -- a file of any size crosses without being held in memory
// first.
//
// A target may also decline, and declining is silent: it ignores the event, and
// the system closes the reader when the frame ends. That is why Open answers
// only within the frame the event arrived in. Afterwards there is nothing left
// to open, and a drop refused by every handler still costs nothing, which it
// would not if the last word belonged to a target that had already walked away.
//
// Open is called once. It answers the same reader every time, so a second call
// is a second owner for one handle, and the close that ends it happens twice.
package transfer

import (
	"io"

	"github.com/arandu-io/ayra/engine/io/event"
)

// SourceFilter is a source saying what it can produce.
//
// One filter carries one type, and a source that can produce two declares two.
// A list inside a single filter would need an order, and that order would
// quietly decide which type a target that accepts several ends up with -- a
// decision worth making out loud, in the order the target declares, where
// [RequestEvent] makes it.
type SourceFilter struct {
	// Target is this source's own handler, the tag it registered for input.
	// It is not the other end of the transfer: a source names itself here, and
	// never names who it is dragging towards.
	Target event.Tag
	// Type is the media type this source can produce.
	Type string
}

func (SourceFilter) ImplementsFilter() {}

// TargetFilter is a target saying what it accepts.
//
// One filter carries one type here too, and the order they are declared in is
// the order they are preferred: when a source can produce several of them, the
// one a target named first is the one it is given.
type TargetFilter struct {
	// Target is this target's own handler, the tag it registered for input.
	Target event.Tag
	// Type is the media type this target accepts.
	Type string
}

func (TargetFilter) ImplementsFilter() {}

// InitiateEvent says a drag has begun and this handler is part of it.
//
// The source hears it, and so does every target with a type in common with the
// source -- before the pointer is anywhere near any of them. That is what lets
// a screen show where the thing in hand could land while it is still in hand,
// which is the only warning a person gets that letting go will do nothing.
type InitiateEvent struct{}

func (InitiateEvent) ImplementsEvent() {}

// RequestEvent asks a source for the data, and it means the drop has already
// happened.
//
// It arrives when the gesture has landed, not while it is in flight, so a
// source opens whatever it is offering once and only for a target that is
// certain. A request sent on the way past would open a file for every handler
// the pointer crossed.
type RequestEvent struct {
	// Type is the type both sides named: the first one this target accepts that
	// this source can also produce. It is the type the answering [OfferCmd] must
	// carry, and the only one the target is prepared to read.
	Type string
}

func (RequestEvent) ImplementsEvent() {}

// OfferCmd is how a source answers a [RequestEvent].
//
// Executing it hands the data over and ends the source's part. What follows is
// not reported back: the source does not learn whether the target read what it
// gave, and does not wait to find out.
type OfferCmd struct {
	// Tag is the source answering, and it is the tag from its [SourceFilter].
	// An offer under any other tag answers nobody -- there is no request
	// outstanding in that name -- and the drop it was meant for ends as a
	// cancellation with the data untouched.
	Tag event.Tag
	// Type is the media type of Data, and it must be the type the request
	// named. The request already narrowed the choice to one the target said it
	// accepts; a different type here is data offered to somebody who never
	// asked for it.
	Type string
	// Data is the data itself, handed over and not copied.
	//
	// The source keeps it readable and stops being the one who closes it: the
	// target closes it if it opens it, and the system closes it if the target
	// does not. It may be read from a goroutine other than the one drawing the
	// frame, so it must stay valid until it is closed rather than until the
	// frame ends.
	Data io.ReadCloser
}

func (OfferCmd) ImplementsCommand() {}

// DataEvent hands a target the data, and it is the only event here that carries
// any.
//
// It arrives from a drop and from a paste alike. A target that accepts a type
// accepts it from either, and is not told which it was -- the two are the same
// act to whatever receives them, and a handler written to tell them apart would
// be a handler that treats the same bytes differently for a reason the person
// who sent them cannot see.
type DataEvent struct {
	// Type is the media type of what Open answers, and it is one of the types
	// this target declared.
	Type string
	// Open answers the data and takes it over.
	//
	// Call it in the frame this event arrived in or not at all: until somebody
	// takes it the reader belongs to the system, which closes it when the frame
	// ends. Call it once -- it answers the same reader every time, and a second
	// caller is a second owner closing a handle the first one already closed.
	//
	// What it answers is the caller's to close, whenever the caller is done.
	Open func() io.ReadCloser
}

func (DataEvent) ImplementsEvent() {}

// CancelEvent ends what an [InitiateEvent] began, and it always arrives.
//
// Every way a gesture can end ends here: dropped on a target that took the
// data, dropped on one that could not, let go over nothing at all. A handler
// that marked itself as a candidate on initiation therefore has one place to
// unmark itself rather than a condition for each ending, and cannot be left
// marked by the ending nobody thought of.
//
// It reaches the source and every target that was a candidate -- the same
// handlers the initiation reached, so that what was told the drag started is
// told it is over.
type CancelEvent struct{}

func (CancelEvent) ImplementsEvent() {}
