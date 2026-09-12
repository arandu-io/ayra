package input

import (
	"image"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/arandu-io/ayra/engine/f32"
	f32internal "github.com/arandu-io/ayra/engine/internal/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/io/clipboard"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/io/semantic"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/io/transfer"
	"github.com/arandu-io/ayra/engine/op"
)

// Router decides which control hears what.
//
// It sits between a window, which knows only that something was pressed at a
// coordinate or that a key went down, and the controls, which know only
// themselves. Nothing on the screen has an address: a control is a tag, an area
// it claimed while it was being drawn, and a list of the kinds of event it
// wants. What the router does is turn a stream of hardware into deliveries to
// those tags, and remember, between frames, the few things a control cannot
// remember for itself -- which one has the keyboard, which one has claimed a
// pointer, and which ones are still on the screen at all.
//
// The frame is the unit of everything here. A screen is drawn from nothing many
// times a second, and on every pass every control redeclares its area and its
// filters; what the previous pass declared is what this pass's events are
// routed by. So a control that stops being drawn stops existing here after one
// frame, with its focus and its claims released, and a control that appears
// starts from nothing -- which is the right answer when the same tag is a
// different row of a list than it was.
//
// A router is not safe for concurrent use. It belongs to the frame loop, and
// work finishing elsewhere reaches the screen by asking for another frame, not
// by reaching in here.
//
// [Source] is the half of this that controls are given. They never hold a
// Router.
type Router struct {
	// savedTrans and transStack hold the transformations the operation list
	// saved and pushed, so that an area declared under one is hit-tested under
	// the same one.
	savedTrans []f32.Affine2D
	transStack []f32.Affine2D
	// handlers is everything the router remembers per tag. Its keys are the
	// tags declared in the most recent frame, and nothing else: a handler whose
	// tag went missing is deleted at the end of the frame that missed it.
	handlers map[event.Tag]*handler
	pointer  struct {
		queue     pointerQueue
		collector pointerCollector
	}
	key struct {
		queue keyQueue
		// filter, nextFilter and scratchFilter are to key events what the
		// fields of the same name on handler are to everything else. Key events
		// are not addressed to a tag -- what they are addressed to is whatever
		// has the focus -- so their filters cannot be kept per tag.
		filter        keyFilter
		nextFilter    keyFilter
		scratchFilter keyFilter
	}
	cqueue clipboardQueue
	// changes is the pending state, newest last. The first element holds the
	// state and the undelivered events of the current frame; the rest are the
	// steps that got there.
	//
	// The history is kept rather than collapsed because a command can rewrite
	// it: a control that moves the focus halfway through a frame changes where
	// the events after that point should have gone, and replaying them from the
	// step the command applied to is what makes the frame come out as though
	// the focus had moved first.
	changes []stateChange
	reader  ops.Reader
	// wakeup and wakeupTime are the earliest time the window has been asked to
	// draw again, if it has been asked at all.
	wakeup     bool
	wakeupTime time.Time
	// commands are the ones held back for the next frame, in order.
	commands []Command
	// transfers holds the readers handed out with a transfer.DataEvent, so that
	// the ones nobody opened can be closed. A drop whose data is never read
	// would otherwise keep a file or a pipe open until the process ends.
	transfers []io.ReadCloser
	// deferring is set once a command can no longer be applied where it was
	// asked for, and everything after it waits for the next frame.
	deferring bool
	// scratchFilters is reused between calls so that asking for events
	// allocates nothing. A control asks once per frame and there are hundreds
	// of them.
	scratchFilters []taggedFilter
}

// Source is what a control is handed, and the whole of what it may do.
//
// It carries no state of its own beyond which router it belongs to and whether
// it is switched off, so passing it by value down a draw call costs nothing and
// a control cannot accidentally keep one alive past the frame it was given in.
//
// The zero value is disabled, and that is deliberate rather than a convenience:
// a control drawn outside a window -- in a test, in a tool that renders one
// frame -- gets a Source that answers nothing rather than a nil pointer that
// panics on the first press.
type Source struct {
	r        *Router
	disabled bool
}

// Command is a request a control makes of the router rather than a fact the
// router reports: move the focus, claim a pointer, read the clipboard, draw
// again at a given time.
//
// It is an interface with an unexported-in-spirit marker rather than a set of
// methods on Source, because the set is open at the edges -- each io package
// names the commands that belong to it -- and closed where it matters: nothing
// outside those packages can invent one the router would accept.
type Command interface {
	// ImplementsCommand marks a type as a command.
	ImplementsCommand()
}

// SemanticNode is one entry of the tree that describes a frame to whatever
// reads a screen aloud.
type SemanticNode struct {
	// ID identifies this node, and is kept stable across frames for as long as
	// the description it belongs to does not change. A tree whose identifiers
	// were fresh every frame would be read out again from the top thirty times
	// a second.
	ID SemanticID
	// ParentID is the node this one hangs from. The root's is the zero value.
	ParentID SemanticID
	// Children are the nodes below this one, in the order they were drawn.
	Children []SemanticNode
	// Desc is what this node says about itself.
	Desc SemanticDesc

	// areaIdx is where the node's area lives while the tree is being built.
	areaIdx int
}

// SemanticDesc is what one control tells a screen reader about itself.
type SemanticDesc struct {
	// Class is what kind of control it is.
	Class semantic.ClassOp
	// Description is what the label does not say and a person would need.
	Description string
	// Label is the text a person sees on it.
	Label string
	// Selected says a control that can be chosen is the chosen one.
	Selected bool
	// Disabled says it is drawn but does not answer.
	Disabled bool
	// Gestures are the ways it can be acted on, which is how a reader offers
	// its actions rather than guessing them from the class.
	Gestures SemanticGestures
	// Bounds is where it is, in window coordinates.
	Bounds image.Rectangle
}

// SemanticGestures is the set of ways a control can be acted on.
//
// It is a set rather than one value because a control is often both: a row of a
// list is pressed to open it and scrolled to move the list it is in, and a
// reader that could only offer one of the two would hide the other.
type SemanticGestures int

const (
	// ClickGesture is a control that answers a press.
	ClickGesture SemanticGestures = 1 << iota
	// ScrollGesture is a control that answers a scroll.
	ScrollGesture
)

// SemanticID identifies one semantic description.
//
// By convention the zero value is no identifier at all, which is what a node
// that describes nothing carries and what a parent lookup answers at the root.
type SemanticID uint

// SystemEvent wraps an event the platform has a use for if no control takes it.
//
// A catch-all filter never matches one. Only a filter that names the event --
// a [github.com/arandu-io/ayra/engine/io/key.Filter] with a Name -- receives
// it, and if none does, the window is free to act on it instead. That is how
// Tab moves the focus: a field that wants to insert a tab character asks for it
// by name and gets it, and every other frame Tab reaches nobody and the window
// moves the focus. A control that matched everything would silently take the
// only key that navigates the interface.
type SystemEvent struct {
	// Event is what was wrapped.
	Event event.Event
}

// handler is everything the router remembers about one tag.
type handler struct {
	// active says the tag was seen during the current frame, either because it
	// asked for events or because the operation list declared it. Whatever is
	// not active when the frame closes is forgotten.
	active  bool
	pointer pointerHandler
	key     keyHandler
	// filter is what the tag asked for during the previous frame, and it is
	// what this frame's events are routed by. Routing by what is asked for
	// during the frame would mean a control's first ask decided whether it
	// heard about something that happened before it was drawn.
	filter filter
	// nextFilter is what it is asking for during this frame, which becomes
	// filter when the frame closes.
	nextFilter filter
	// processedFilter is what it has already exhausted this frame -- the
	// filters it asked with and got nothing more for. A command arriving after
	// that cannot be applied where it was asked for, because the control has
	// already acted on a state the command would rewrite.
	processedFilter filter
}

// filter is the union of everything one tag asked for.
//
// Focus is a single flag rather than a set because
// [github.com/arandu-io/ayra/engine/io/key.FocusFilter] is a single
// subscription: it carries the focus changing and everything an input method
// sends to whatever is focused, and a control that took the second without the
// first would go on composing into a field the person has left.
type filter struct {
	pointer   pointerFilter
	focusable bool
}

// taggedFilter is a filter together with the tag that asked for it.
type taggedFilter struct {
	tag    event.Tag
	filter filter
}

// stateChange is one step: what arrived, the state it produced, and what that
// owes to controls.
type stateChange struct {
	// event is what caused the step, and is nil for a step the router made of
	// its own accord -- closing a frame, carrying out a command.
	event  event.Event
	state  inputState
	events []taggedEvent
}

// inputState is an immutable snapshot of everything routing depends on.
//
// Immutable, and copied rather than mutated, because the router replays: a
// command can require the frame to come out as though it had been carried out
// before events that have already been processed, and replaying onto a state
// that had been edited in place would replay onto the wrong one.
type inputState struct {
	clipboardState
	keyState
	pointerState
}

// taggedEvent is an event and the tag it is for. A nil tag means the event is
// for whatever matches it, which is how key events travel: they are addressed
// to the focus, and the focus is a property of the state rather than of the
// event.
type taggedEvent struct {
	event event.Event
	tag   event.Tag
}

// Source returns the interface to hand to controls.
func (q *Router) Source() Source {
	return Source{r: q}
}

// Execute carries out a command, or does nothing if the source is disabled.
func (s Source) Execute(c Command) {
	if !s.Enabled() {
		return
	}
	s.r.execute(c)
}

// Disabled returns a copy of this source that delivers nothing and carries out
// nothing.
//
// It is what a control drawn as unavailable is handed, and it has to refuse
// commands as well as events: a disabled field that could still take the focus
// would open the keyboard on a control that refuses to be typed into.
func (s Source) Disabled() Source {
	s2 := s
	s2.disabled = true
	return s2
}

// Enabled reports whether this source delivers anything. The zero value does
// not.
func (s Source) Enabled() bool {
	return s.r != nil && !s.disabled
}

// Focused reports whether tag has the keyboard, as of the most recent
// [github.com/arandu-io/ayra/engine/io/key.FocusEvent] delivered.
//
// It answers from the delivered state rather than the pending one so that what
// a control draws agrees with what it was told. A control reading a focus it
// has not yet been notified of would draw a caret one frame before it starts
// receiving the typing.
func (s Source) Focused(tag event.Tag) bool {
	if !s.Enabled() {
		return false
	}
	return s.r.state().keyState.focus == tag
}

// Event returns the next event matching any of the filters, and whether there
// was one.
//
// A control calls it in a loop until it answers false, once per frame, before
// it draws. The filters are both the question and the declaration: what they
// match is handed over now, and the same set is what the next frame's events
// are routed by.
func (s Source) Event(filters ...event.Filter) (event.Event, bool) {
	if !s.Enabled() {
		return nil, false
	}
	return s.r.Event(filters...)
}

// Event returns the next event matching any of the filters, and whether there
// was one. It is what [Source.Event] calls, and a window with its own frame
// loop may call it directly.
//
// Three things happen here, in this order, and the order is the contract.
// First the filters are recorded, so that a control that asks is routed to from
// the next frame on. Then a control that the router has never spoken to is told
// to reset, before it is told anything else. Only then is the pending queue
// searched.
func (q *Router) Event(filters ...event.Filter) (event.Event, bool) {
	q.declare(filters)
	if e, ok := q.resetEvent(filters); ok {
		return e, true
	}
	if e, ok := q.deliver(filters); ok {
		return e, true
	}
	// Nothing left for these filters this frame. Remember that, because a
	// command arriving afterwards cannot pretend to have happened before a
	// control that has already finished reading.
	for _, tf := range q.scratchFilters {
		h := q.stateFor(tf.tag)
		h.processedFilter.Merge(tf.filter)
	}
	return nil, false
}

// declare records what the caller is asking for, against the tags that asked.
//
// The filters are gathered into scratch space first and merged per tag
// afterwards, rather than merged as they are seen, because one call commonly
// carries several filters for the same tag and a control asks once per frame in
// a screen with hundreds of them. The scratch space is reused between calls so
// that the whole of this allocates nothing.
func (q *Router) declare(filters []event.Filter) {
	q.scratchFilters = q.scratchFilters[:0]
	q.key.scratchFilter = q.key.scratchFilter[:0]
	for _, f := range filters {
		var t event.Tag
		switch f := f.(type) {
		case key.Filter:
			// Key filters are not per tag: what they match is decided by the
			// focus, so they are kept in one list for the whole frame.
			q.key.scratchFilter = append(q.key.scratchFilter, f)
			continue
		case transfer.SourceFilter:
			t = f.Target
		case transfer.TargetFilter:
			t = f.Target
		case key.FocusFilter:
			t = f.Target
		case pointer.Filter:
			t = f.Target
		}
		if t == nil {
			// A filter with no target addresses nothing. It is not an error --
			// a control that has not been given its tag yet declares one -- but
			// there is nowhere to record it.
			continue
		}
		q.scratchFilterFor(t).Add(f)
	}
	for _, tf := range q.scratchFilters {
		h := q.stateFor(tf.tag)
		h.filter.Merge(tf.filter)
		h.nextFilter.Merge(tf.filter)
	}
	q.key.filter = append(q.key.filter, q.key.scratchFilter...)
	q.key.nextFilter = append(q.key.nextFilter, q.key.scratchFilter...)
}

// scratchFilterFor returns the scratch filter for tag, adding one if this call
// has not seen the tag yet.
//
// Growing the slice by hand rather than with append is what keeps the reuse:
// append past the length but within the capacity would hand back the previous
// call's filter with the previous call's contents still in it, so an entry
// taken from spare capacity is reset, and only a genuinely new one is appended.
func (q *Router) scratchFilterFor(t event.Tag) *filter {
	for i := range q.scratchFilters {
		if s := &q.scratchFilters[i]; s.tag == t {
			return &s.filter
		}
	}
	n := len(q.scratchFilters)
	if n < cap(q.scratchFilters) {
		q.scratchFilters = q.scratchFilters[:n+1]
		tf := &q.scratchFilters[n]
		tf.tag = t
		tf.filter.Reset()
		return &tf.filter
	}
	q.scratchFilters = append(q.scratchFilters, taggedFilter{tag: t})
	return &q.scratchFilters[n].filter
}

// resetEvent returns the event that tells a control to start from nothing, if
// one of these filters is owed it.
//
// Every control is told once, on the first frame the router hears from it, and
// told again if it ever disappears for a frame and comes back. It is the answer
// to a question a control cannot ask: a control is a value the caller holds, and
// a router has no way of knowing whether the value it is being asked for is the
// one from last frame or a fresh one in the same place. Being told to reset is
// what lets it be either.
func (q *Router) resetEvent(filters []event.Filter) (event.Event, bool) {
	for _, f := range filters {
		switch f := f.(type) {
		case key.FocusFilter:
			if f.Target == nil {
				continue
			}
			h := q.stateFor(f.Target)
			if reset, ok := h.key.ResetEvent(); ok {
				return reset, true
			}
		case pointer.Filter:
			if f.Target == nil {
				continue
			}
			h := q.stateFor(f.Target)
			// Matched against the filter as well, because the reset is a
			// Cancel and a control that never asked for Cancel has no use for
			// one -- and would be handed an event of a kind it does not
			// understand as the first thing it ever heard.
			if reset, ok := h.pointer.ResetEvent(); ok && h.filter.pointer.Matches(reset) {
				return reset, true
			}
		}
	}
	return nil, false
}

// deliver hands over the first pending event these filters match, and removes
// it from the queue.
//
// The search walks the history oldest first, so events reach a control in the
// order they happened. It stops at the first step while a command is deferred:
// everything past that step is waiting to be replayed once the command has been
// applied, and handing it over now would deliver it twice.
func (q *Router) deliver(filters []event.Filter) (event.Event, bool) {
	for i := range q.changes {
		if q.deferring && i > 0 {
			break
		}
		change := &q.changes[i]
		for j, evt := range change.events {
			match := false
			switch e := evt.event.(type) {
			case key.Event:
				// Addressed to the focus rather than to a tag, and the focus
				// meant is the one in force at the step the key arrived -- not
				// the one in force now. A key pressed before the focus moved
				// belongs to the control that had it.
				match = q.key.scratchFilter.Matches(change.state.keyState.focus, e, false)
			default:
				for _, tf := range q.scratchFilters {
					if evt.tag == tf.tag && tf.filter.Matches(evt.event) {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
			change.events = slices.Delete(change.events, j, j+1)
			// The caller is about to act on this event, so the state it
			// happened in is now the current one. Everything before it is
			// settled and cannot be replayed any more.
			q.collapseState(i)
			return evt.event, true
		}
	}
	return nil, false
}

// collapseState folds the steps in [1;idx] into the first one, which becomes
// the current state and keeps their undelivered events.
func (q *Router) collapseState(idx int) {
	if idx == 0 {
		return
	}
	first := &q.changes[0]
	first.state = q.changes[idx].state
	for _, ch := range q.changes[1 : idx+1] {
		first.events = append(first.events, ch.events...)
	}
	q.changes = append(q.changes[:1], q.changes[idx+1:]...)
}

// Frame closes the current frame and opens the next one over the operations in
// frame.
//
// This is where the bookkeeping happens, and most of it cannot happen anywhere
// else. What each control asked for during the frame becomes what it is routed
// by; every control is told to reset what it tracked per frame; the operation
// list is read to learn the areas and the tags that exist now; whatever was not
// declared is forgotten, with its focus and its pointers released; and the
// commands that could not be carried out where they were asked for are carried
// out here.
//
// Events nobody took are discarded, because a frame has been drawn in answer to
// them and holding them would deliver them to whatever is drawn next. The
// exception is the events held back by a command: those were never offered.
func (q *Router) Frame(frame *op.Ops) {
	remaining := q.closeChanges()

	// A transfer nobody opened is closed here. The event carried the reader and
	// the target was free to ignore it, so this is the only place that knows
	// nothing will read it.
	for _, rc := range q.transfers {
		if rc != nil {
			rc.Close()
		}
	}
	q.transfers = nil
	q.deferring = false

	for _, h := range q.handlers {
		h.filter, h.nextFilter = h.nextFilter, h.filter
		h.nextFilter.Reset()
		h.processedFilter.Reset()
		h.pointer.Reset()
		h.key.Reset()
	}
	q.key.filter, q.key.nextFilter = q.key.nextFilter, q.key.filter
	q.key.nextFilter = q.key.nextFilter[:0]

	var opsList *ops.Ops
	if frame != nil {
		opsList = &frame.Internal
	}
	q.reader.Reset(opsList)
	q.collect()

	for k, h := range q.handlers {
		if !h.active {
			delete(q.handlers, k)
		} else {
			h.active = false
		}
	}

	q.executeCommands()
	q.Queue(remaining...)

	// The queues get the new frame last, once the handler set is what the frame
	// declared. This is where a pointer that is now over something else is told
	// so, and where a focus whose control has gone is given up.
	st := q.lastState()
	pst, evts := q.pointer.queue.Frame(q.handlers, st.pointerState)
	st.pointerState = pst
	st.keyState = q.key.queue.Frame(q.handlers, q.lastState().keyState)
	q.changeState(nil, st, evts)

	q.collapseState(len(q.changes) - 1)
}

// closeChanges ends the frame's history and returns the events to replay.
//
// There is something to replay only when a command was deferred: those events
// arrived after it and were never offered to anyone, so they are queued again
// against the state the command produced. Otherwise the history collapses to
// its last state and the undelivered events go, which is what makes a frame the
// boundary it is.
func (q *Router) closeChanges() []event.Event {
	n := len(q.changes)
	if n == 0 {
		return nil
	}
	if !q.deferring {
		state := q.changes[n-1].state
		q.changes = append(q.changes[:0], stateChange{state: state})
		return nil
	}
	var remaining []event.Event
	for _, ch := range q.changes[1:] {
		remaining = append(remaining, ch.event)
	}
	q.changes = append(q.changes[:0], stateChange{state: q.changes[0].state})
	return remaining
}

// Queue routes events. They are delivered to controls that ask for them, in
// order, on this frame or the next.
func (q *Router) Queue(events ...event.Event) {
	for _, e := range events {
		se, system := e.(SystemEvent)
		if system {
			e = se.Event
		}
		q.processEvent(e, system)
	}
}

// Add records one filter against this union.
//
// Transfer filters are kept with the pointer ones because that is what a
// transfer is routed by: a drag begins under a pointer, and what may receive
// the drop is decided by where the pointer is when it is released.
func (f *filter) Add(flt event.Filter) {
	switch flt := flt.(type) {
	case key.FocusFilter:
		f.focusable = true
	case pointer.Filter:
		f.pointer.Add(flt)
	case transfer.SourceFilter, transfer.TargetFilter:
		f.pointer.Add(flt)
	}
}

// Merge folds f2 into f.
func (f *filter) Merge(f2 filter) {
	f.focusable = f.focusable || f2.focusable
	f.pointer.Merge(f2.pointer)
}

// Matches reports whether this union would take e.
func (f *filter) Matches(e event.Event) bool {
	switch e.(type) {
	case key.FocusEvent, key.SnippetEvent, key.EditEvent, key.SelectionEvent, key.CompositionEvent:
		return f.focusable
	default:
		return f.pointer.Matches(e)
	}
}

// Reset empties the union, keeping the space its type lists were using. The
// same control asks for the same types every frame, so the allocation is made
// once and the frames after the first make none.
func (f *filter) Reset() {
	*f = filter{
		pointer: pointerFilter{
			sourceMimes: f.pointer.sourceMimes[:0],
			targetMimes: f.pointer.targetMimes[:0],
		},
	}
}

// processEvent turns one incoming event into a state change.
//
// Each kind knows where it is addressed. A pointer event is addressed by
// position and the pointer queue works out to whom; a key event is addressed to
// whatever matches it, with the focus deciding; everything an input method
// sends is addressed to the focus itself and goes nowhere if there is none.
func (q *Router) processEvent(e event.Event, system bool) {
	state := q.lastState()
	switch e := e.(type) {
	case pointer.Event:
		pstate, evts := q.pointer.queue.Push(q.handlers, state.pointerState, e)
		state.pointerState = pstate
		q.changeState(e, state, evts)
	case key.Event:
		var evts []taggedEvent
		if q.key.filter.Matches(state.keyState.focus, e, system) {
			// No tag: which control this reaches is decided when it is
			// delivered, against the focus as it stood here.
			evts = append(evts, taggedEvent{event: e})
		}
		q.changeState(e, state, evts)
	case key.SnippetEvent:
		// Widened to cover whatever the field already had, because the
		// platform asks for the text around the caret and a narrower answer
		// than last time would shrink what it holds rather than refresh it.
		if r := state.content.Snippet.Range; rangeOverlaps(r, key.Range(e)) {
			e.Start = min(e.Start, r.Start)
			e.End = max(e.End, r.End)
		}
		q.changeState(e, state, q.toFocus(state, e))
	case key.CompositionEvent:
		// Normalised so that a field is never handed a range that runs
		// backwards. The platform is free to report the two ends in either
		// order, and every field would otherwise have to sort them itself.
		e = key.CompositionEvent(rangeNorm(key.Range(e)))
		q.changeState(e, state, q.toFocus(state, e))
	case key.EditEvent, key.FocusEvent, key.SelectionEvent:
		q.changeState(e, state, q.toFocus(state, e))
	case transfer.DataEvent:
		cstate, evts := q.cqueue.Push(state.clipboardState, e)
		state.clipboardState = cstate
		q.changeState(e, state, evts)
	default:
		panic("input: unknown event type")
	}
}

// toFocus addresses an event to whatever has the keyboard, and to nothing at
// all when nothing has.
func (q *Router) toFocus(state inputState, e event.Event) []taggedEvent {
	f := state.focus
	if f == nil {
		return nil
	}
	return []taggedEvent{{tag: f, event: e}}
}

// execute carries out a command, now if it still can be and next frame if it
// cannot.
//
// It can be carried out now when no control that the command's own events
// concern has finished reading. If one has, the frame has already been drawn as
// though the command had not happened, and applying it here would produce a
// frame where half the controls acted on one state and half on another. So it
// waits, and everything after it waits with it -- which is why the deferral is
// a flag on the router rather than a property of the command.
//
// Applying it now means replaying: the events that have arrived since the state
// the command applies to are queued again on top of the state it produced, so
// the frame comes out as though the command had been the first thing to happen.
func (q *Router) execute(c Command) {
	if !q.deferring {
		ch := q.executeCommand(c)
		immediate := true
		for _, e := range ch.events {
			h, ok := q.handlers[e.tag]
			immediate = immediate && (!ok || !h.processedFilter.Matches(e.event))
		}
		if immediate {
			var evts []event.Event
			for _, ch := range q.changes {
				if ch.event != nil {
					evts = append(evts, ch.event)
				}
			}
			if len(q.changes) > 1 {
				q.changes = q.changes[:1]
			}
			q.changeState(nil, ch.state, ch.events)
			q.Queue(evts...)
			return
		}
	}
	q.deferring = true
	q.commands = append(q.commands, c)
}

// state is the state as delivered: what the controls have been told.
func (q *Router) state() inputState {
	if len(q.changes) > 0 {
		return q.changes[0].state
	}
	return inputState{}
}

// lastState is the state as it stands, including steps nobody has been told
// about yet.
func (q *Router) lastState() inputState {
	if n := len(q.changes); n > 0 {
		return q.changes[n-1].state
	}
	return inputState{}
}

// executeCommands carries out everything that was held back, in the order it
// was asked for.
func (q *Router) executeCommands() {
	for _, c := range q.commands {
		ch := q.executeCommand(c)
		q.changeState(nil, ch.state, ch.events)
	}
	q.commands = nil
}

// executeCommand applies one command to the delivered state and returns the
// step it produces, without committing it.
//
// It works from the delivered state rather than the latest one because that is
// what the control asking has seen. The caller decides whether the step can be
// committed here or has to wait.
func (q *Router) executeCommand(c Command) stateChange {
	state := q.state()
	var evts []taggedEvent
	switch req := c.(type) {
	case key.SelectionCmd:
		state.keyState = q.key.queue.setSelection(state.keyState, req)
	case key.FocusCmd:
		state.keyState, evts = q.key.queue.Focus(q.handlers, state.keyState, req.Tag)
	case key.SoftKeyboardCmd:
		state.keyState = state.keyState.softKeyboard(req.Show)
	case key.SnippetCmd:
		state.keyState = q.key.queue.setSnippet(state.keyState, req)
	case transfer.OfferCmd:
		state.pointerState, evts = q.pointer.queue.offerData(q.handlers, state.pointerState, req)
	case clipboard.WriteCmd:
		q.cqueue.ProcessWriteClipboard(req)
	case clipboard.ReadCmd:
		state.clipboardState = q.cqueue.ProcessReadClipboard(state.clipboardState, req.Tag)
	case pointer.GrabCmd:
		state.pointerState, evts = q.pointer.queue.grab(state.pointerState, req)
	case op.InvalidateCmd:
		// The earliest request wins. Two controls asking to be drawn again at
		// different times both want to be drawn by theirs, and the later one is
		// satisfied by the earlier frame.
		if !q.wakeup || req.At.Before(q.wakeupTime) {
			q.wakeup = true
			q.wakeupTime = req.At
		}
	}
	return stateChange{state: state, events: evts}
}

// changeState appends a step, or merges it into the last one.
//
// A step is kept separate only when an incoming event produced something for a
// control, because that is the only case a command can need to replay from. A
// step that owes nobody anything cannot change what anybody saw, so it is
// merged and the history stays short.
func (q *Router) changeState(e event.Event, state inputState, evts []taggedEvent) {
	// A transfer hands over a reader, and whether the target opened it is not
	// otherwise knowable. Wrapping Open so that it strikes the reader off the
	// list is what lets the frame close the ones nobody took.
	for i := range evts {
		e := &evts[i]
		if de, ok := e.event.(transfer.DataEvent); ok {
			transferIdx := len(q.transfers)
			data := de.Open()
			q.transfers = append(q.transfers, data)
			de.Open = func() io.ReadCloser {
				q.transfers[transferIdx] = nil
				return data
			}
			e.event = de
		}
	}
	if len(q.changes) == 0 {
		q.changes = append(q.changes, stateChange{})
	}
	if e != nil && len(evts) > 0 {
		q.changes = append(q.changes, stateChange{event: e, state: state, events: evts})
		return
	}
	prev := &q.changes[len(q.changes)-1]
	prev.state = state
	prev.events = append(prev.events, evts...)
}

// rangeOverlaps reports whether two ranges share any position.
func rangeOverlaps(r1, r2 key.Range) bool {
	r1 = rangeNorm(r1)
	r2 = rangeNorm(r2)
	return r1.Start <= r2.Start && r2.Start < r1.End ||
		r1.Start <= r2.End && r2.End < r1.End
}

// rangeNorm puts a range's ends in order.
func rangeNorm(r key.Range) key.Range {
	if r.End < r.Start {
		r.End, r.Start = r.Start, r.End
	}
	return r
}

// MoveFocus moves the keyboard focus one step in dir, among the controls the
// most recent frame declared.
//
// The order is the frame's, rebuilt every time: a control that moved on the
// screen moves with it, and one that left the screen leaves it. Forward and
// backward wrap; the directional moves do not, because there is no sensible
// control to the right of the rightmost one and jumping to the far left of the
// next row is not what an arrow key means.
func (q *Router) MoveFocus(dir key.FocusDirection) {
	state := q.lastState()
	kstate, evts := q.key.queue.MoveFocus(q.handlers, state.keyState, dir)
	state.keyState = kstate
	q.changeState(nil, state, evts)
}

// RevealFocus scrolls the focused control into viewport, if anything above it
// can scroll.
//
// It is what makes keyboard navigation usable in a long form: the focus can
// move to a control that is off the screen, and without this it would be taking
// the typing from somewhere nobody can see.
func (q *Router) RevealFocus(viewport image.Rectangle) {
	kh, ok := q.focusedKeyHandler()
	if !ok {
		return
	}
	bounds := q.key.queue.BoundsFor(kh)
	area := q.key.queue.AreaFor(kh)
	viewport = q.pointer.queue.ClipFor(area, viewport)

	// How far the control is outside the viewport, on each side, taking
	// whichever side it overflows. Clamped to nought at both ends so that a
	// control already inside is not scrolled at all, and one taller than the
	// viewport is brought to its top edge rather than swung past it.
	topleft := bounds.Min.Sub(viewport.Min)
	topleft = maxPoint(topleft, bounds.Max.Sub(viewport.Max))
	topleft = minPoint(image.Pt(0, 0), topleft)
	bottomright := bounds.Max.Sub(viewport.Max)
	bottomright = minPoint(bottomright, bounds.Min.Sub(viewport.Min))
	bottomright = maxPoint(image.Pt(0, 0), bottomright)
	s := topleft
	if s.X == 0 {
		s.X = bottomright.X
	}
	if s.Y == 0 {
		s.Y = bottomright.Y
	}
	q.ScrollFocus(s)
}

// ScrollFocus scrolls whatever can scroll above the focused control by dist.
func (q *Router) ScrollFocus(dist image.Point) {
	kh, ok := q.focusedKeyHandler()
	if !ok {
		return
	}
	area := q.key.queue.AreaFor(kh)
	q.changeState(nil, q.lastState(), q.pointer.queue.Deliver(q.handlers, area, pointer.Event{
		Kind:   pointer.Scroll,
		Source: pointer.Touch,
		Scroll: f32internal.FPt(dist),
	}))
}

// ClickFocus presses and releases the focused control, at its centre.
//
// It is how a platform's accessibility layer activates what it has read out:
// there is no pointer involved, so one is invented over the control the focus
// is on. The two events go to the same state rather than one after the other,
// because they are one action and a control that saw a frame between them would
// draw itself held down for it.
func (q *Router) ClickFocus() {
	kh, ok := q.focusedKeyHandler()
	if !ok {
		return
	}
	bounds := q.key.queue.BoundsFor(kh)
	center := bounds.Max.Add(bounds.Min).Div(2)
	e := pointer.Event{
		Position: f32.Pt(float32(center.X), float32(center.Y)),
		Source:   pointer.Touch,
	}
	area := q.key.queue.AreaFor(kh)
	state := q.lastState()
	e.Kind = pointer.Press
	q.changeState(nil, state, q.pointer.queue.Deliver(q.handlers, area, e))
	e.Kind = pointer.Release
	q.changeState(nil, state, q.pointer.queue.Deliver(q.handlers, area, e))
}

// focusedKeyHandler answers the focused control's key state, and whether the
// focus currently names a control that is on the screen.
//
// The second answer is not a formality. A focus command is granted against the
// state a control has seen, and it names a tag: the tag need not be on the
// screen at all, and it need not be one that takes the keyboard even if it is.
// The frame that follows drops such a focus, but between the command and that
// frame the router holds a focus with nothing behind it -- and everything here
// that works from the focus reads the frame's layout by index. Asked in that
// window, all three of these used to bring the process down, which is the worst
// possible answer to a request that arrived from the accessibility layer.
func (q *Router) focusedKeyHandler() (*keyHandler, bool) {
	focus := q.lastState().focus
	if focus == nil {
		return nil, false
	}
	h, ok := q.handlers[focus]
	if !ok {
		return nil, false
	}
	// orderPlusOne is set while the frame is read, for the handlers that took
	// their place in the focus order. Anything else has no bounds and no area
	// this frame, and its index into the order is last frame's or nothing.
	if h.key.orderPlusOne == 0 {
		return nil, false
	}
	return &h.key, true
}

// maxPoint takes the larger of each axis.
func maxPoint(p1, p2 image.Point) image.Point {
	return image.Pt(max(p1.X, p2.X), max(p1.Y, p2.Y))
}

// minPoint takes the smaller of each axis.
func minPoint(p1, p2 image.Point) image.Point {
	return image.Pt(min(p1.X, p2.X), min(p1.Y, p2.Y))
}

// ActionAt returns the window action declared under p, if any.
//
// It is what makes a region of the interface drag the window or resize it: the
// platform asks before it decides whether a press belongs to the application at
// all.
func (q *Router) ActionAt(p f32.Point) (system.Action, bool) {
	return q.pointer.queue.ActionAt(p)
}

// TextInputState returns whether the soft keyboard should be opened, closed or
// left alone, and clears the request.
//
// It is a request rather than a state because that is what the platform takes:
// asking for a keyboard that is already up has to be distinguishable from not
// asking, or every frame would reopen it.
func (q *Router) TextInputState() TextInputState {
	state := q.state()
	kstate, s := state.InputState()
	state.keyState = kstate
	q.changeState(nil, state, nil)
	return s
}

// TextInputHint returns what kind of text the focused control expects, and
// whether that has changed since the last call.
func (q *Router) TextInputHint() (key.InputHint, bool) {
	return q.key.queue.InputHint(q.handlers, q.state().keyState)
}

// WriteClipboard returns the content most recently asked to be copied, and
// clears it. It answers once per request.
func (q *Router) WriteClipboard() (mime string, content []byte, ok bool) {
	return q.cqueue.WriteClipboard()
}

// ClipboardRequested reports whether a control is waiting to be handed the
// clipboard.
//
// Reading a clipboard costs a round trip to the platform, and on some of them a
// visible permission prompt, so it happens when a control has asked and not on
// the chance that one will.
func (q *Router) ClipboardRequested() bool {
	return q.cqueue.ClipboardRequested(q.lastState().clipboardState)
}

// Cursor returns the shape the pointer should take, as of the most recent
// delivered state.
func (q *Router) Cursor() pointer.Cursor {
	return q.state().cursor
}

// SemanticAt returns the first semantic description under pos, if any.
func (q *Router) SemanticAt(pos f32.Point) (SemanticID, bool) {
	return q.pointer.queue.SemanticAt(pos)
}

// AppendSemantics appends the frame's semantic tree to nodes and returns the
// result. The root is the first node added.
func (q *Router) AppendSemantics(nodes []SemanticNode) []SemanticNode {
	q.pointer.collector.q = &q.pointer.queue
	// A root is ensured even for a frame that drew nothing, so that the tree
	// handed to the platform always has somewhere to hang from. A reader given
	// an empty tree reports the window as having no content, which is not the
	// same thing as a window whose content is empty.
	q.pointer.collector.ensureRoot()
	return q.pointer.queue.AppendSemantics(nodes)
}

// EditorState returns what the focused control has told the router about its
// text, or the zero value if nothing is focused.
//
// It is what an input method needs: where the caret is, what is selected, and
// the stretch of text around it.
func (q *Router) EditorState() EditorState {
	return q.key.queue.editorState(q.handlers, q.state().keyState)
}

// stateFor returns the state kept for tag, creating it if this is the first the
// router has heard of it, and marks the tag as present in this frame.
func (q *Router) stateFor(tag event.Tag) *handler {
	if tag == nil {
		panic("input: nil tag")
	}
	s, ok := q.handlers[tag]
	if !ok {
		s = new(handler)
		if q.handlers == nil {
			q.handlers = make(map[event.Tag]*handler)
		}
		q.handlers[tag] = s
	}
	s.active = true
	return s
}

// collect reads the frame's operations and rebuilds what routing needs from
// them: the tree of areas, the transformation each was declared under, where
// each tag sits in it, and what every area says about itself.
//
// It is a second pass over the same list the renderer draws from, and that is
// the point: an area is a clip, so the shape a control is drawn inside is the
// shape it is hit inside, and the two cannot drift apart. A control that
// declared its hit area separately would be a control whose ink and whose
// pressable region are two facts that have to be kept in agreement by hand.
func (q *Router) collect() {
	q.transStack = q.transStack[:0]
	pc := &q.pointer.collector
	pc.q = &q.pointer.queue
	pc.Reset()
	kq := &q.key.queue
	kq.Reset()
	t := f32.AffineId()
	for encOp, ok := q.reader.Decode(); ok; encOp, ok = q.reader.Decode() {
		switch ops.OpType(encOp.Data[0]) {
		case ops.TypeSave:
			id := ops.DecodeSave(encOp.Data)
			if extra := id - len(q.savedTrans) + 1; extra > 0 {
				for range extra {
					q.savedTrans = append(q.savedTrans, f32.AffineId())
				}
			}
			q.savedTrans[id] = t
		case ops.TypeLoad:
			id := ops.DecodeLoad(encOp.Data)
			t = q.savedTrans[id]
			// A load jumps to a state recorded elsewhere, so what was being
			// built up to here is not what it continues from.
			pc.resetState()
			pc.setTrans(t)

		case ops.TypeClip:
			var op ops.ClipOp
			op.Decode(encOp.Data)
			pc.clip(op)
		case ops.TypePopClip:
			pc.popArea()
		case ops.TypeTransform:
			t2, push := ops.DecodeTransform(encOp.Data)
			if push {
				q.transStack = append(q.transStack, t)
			}
			t = t.Mul(t2)
			pc.setTrans(t)
		case ops.TypePopTransform:
			n := len(q.transStack)
			t = q.transStack[n-1]
			q.transStack = q.transStack[:n-1]
			pc.setTrans(t)

		case ops.TypeInput:
			tag := encOp.Refs[0].(event.Tag)
			s := q.stateFor(tag)
			pc.inputOp(tag, &s.pointer)
			a := pc.currentArea()
			b := pc.currentAreaBounds()
			// Only a control that asked for the focus takes a place in the
			// focus order. Everything else that registers a tag -- a hover, a
			// scroll area, a drop target -- would otherwise be a stop on the
			// way through the interface with nothing to do when it is reached.
			if s.filter.focusable {
				kq.inputOp(tag, &s.key, t, a, b)
			}

		// Pointer operations.
		case ops.TypePass:
			pc.pass()
		case ops.TypePopPass:
			pc.popPass()
		case ops.TypeCursor:
			name := pointer.Cursor(encOp.Data[1])
			pc.cursor(name)
		case ops.TypeActionInput:
			act := system.Action(encOp.Data[1])
			pc.actionInputOp(act)
		case ops.TypeKeyInputHint:
			op := key.InputHintOp{
				Tag:  encOp.Refs[0].(event.Tag),
				Hint: key.InputHint(encOp.Data[1]),
			}
			s := q.stateFor(op.Tag)
			s.key.inputHint(op.Hint)

		// Semantic operations. Each attaches to the area in force, which is why
		// a control pushes one of its own before describing itself.
		case ops.TypeSemanticLabel:
			lbl := *encOp.Refs[0].(*string)
			pc.semanticLabel(lbl)
		case ops.TypeSemanticDesc:
			desc := *encOp.Refs[0].(*string)
			pc.semanticDesc(desc)
		case ops.TypeSemanticClass:
			class := semantic.ClassOp(encOp.Data[1])
			pc.semanticClass(class)
		case ops.TypeSemanticSelected:
			pc.semanticSelected(encOp.Data[1] != 0)
		case ops.TypeSemanticEnabled:
			pc.semanticEnabled(encOp.Data[1] != 0)
		}
	}
}

// WakeupTime returns the earliest time another frame is wanted, and whether one
// is wanted at all. It clears the request.
//
// An undelivered event always wants one. A press that arrived after the frame
// it would have been read in would otherwise sit here, correct and invisible,
// until something unrelated woke the window -- which is a button that does
// nothing until the pointer is moved.
func (q *Router) WakeupTime() (time.Time, bool) {
	t, w := q.wakeupTime, q.wakeup
	q.wakeup = false
	if len(q.changes) > 1 || len(q.changes) == 1 && len(q.changes[0].events) > 0 {
		t, w = time.Time{}, true
	}
	return t, w
}

// String names the gestures in the set, separated by commas, and is empty for
// none.
//
// It named only the click for as long as there were two to name, so a control
// that could be scrolled was reported to a reader as offering nothing. The set
// exists to be both at once -- a row of a list is pressed and the list under it
// is scrolled -- and a description that can only say the first of the two makes
// the second unreachable for anybody navigating by it.
func (s SemanticGestures) String() string {
	var gestures []string
	if s&ClickGesture != 0 {
		gestures = append(gestures, "Click")
	}
	if s&ScrollGesture != 0 {
		gestures = append(gestures, "Scroll")
	}
	return strings.Join(gestures, ",")
}

// ImplementsEvent marks SystemEvent as an event.
func (SystemEvent) ImplementsEvent() {}
