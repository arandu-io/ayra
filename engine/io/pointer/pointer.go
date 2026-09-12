package pointer

import (
	"strconv"
	"strings"
	"time"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/op"
)

// Event is one thing a pointer did.
//
// It is a single struct for every kind rather than a type per kind, because a
// control switches on [Event.Kind] in one place and reads the two or three
// fields that kind fills. Splitting it would replace that switch with a type
// assertion per kind and change nothing else, and the fields a kind does not
// fill are zero, which is the answer a control wants anyway: no buttons held is
// no buttons.
type Event struct {
	// Kind is what happened. It is exactly one kind on a delivered event; the
	// bitmask form is for [Filter.Kinds], which names a set.
	Kind Kind
	// Source says whether a mouse, a finger or a pen did it. A control reads
	// it where the difference is real -- a hover no finger can perform, a
	// target too small for one -- and ignores it everywhere else.
	Source Source
	// PointerID identifies one pointer from its Press to its Release, which is
	// what makes more than one finger on a screen tractable: two touches are
	// two ids, and a control tracking a drag follows the id it started with
	// rather than the most recent event.
	//
	// It is filled for Press, Release, Move, Drag, Enter, Leave and Cancel.
	// Scroll is not bound to a tracked pointer and leaves it zero.
	PointerID ID
	// Priority says whether this handler is alone in the set matched by the
	// pointer, which is how a gesture that is still ambiguous is told apart
	// from one that has been settled.
	Priority Priority
	// Time is when the event was received, measured from a base this package
	// does not define. It is a duration rather than a wall clock because the
	// only honest use of it is the difference between two events -- the gap
	// that separates a click from a double click, the gap a long press has to
	// exceed -- and a wall clock invites the one use that breaks, which is
	// comparing it to now.
	Time time.Duration
	// Buttons is the set of mouse buttons held down at this event, not the one
	// that caused it. A second button pressed while the first is down reports
	// both.
	Buttons Buttons
	// Position is where the pointer is, in the receiving control's own
	// coordinates: whatever transforms were in effect where the control
	// declared itself have been undone. A control compares it against its own
	// size and never has to know where on the window it was drawn.
	Position f32.Point
	// Scroll is how far this event asks to scroll, already clipped to the
	// range the receiving filter declared.
	Scroll f32.Point
	// Modifiers is the set of modifier keys held when the button was pressed.
	Modifiers key.Modifiers
}

// ID identifies one pointer among several. Two fingers on a screen are two ids.
type ID uint16

// Kind is what a pointer did, and in a [Filter] a set of those.
//
// The values are single bits so that a filter naming several reads as one
// or-ed expression. A delivered [Event] carries exactly one of them.
type Kind uint

const (
	// Cancel says a gesture in progress will not complete: another handler
	// grabbed the pointer, or the system took it. It is the event that undoes
	// whatever a handler did in anticipation -- a highlight, a half-drawn
	// selection -- and a handler that ignores it is one that leaves a button
	// lit after the press went elsewhere.
	Cancel Kind = 1 << iota
	// Press of a pointer.
	Press
	// Release of a pointer.
	Release
	// Move of a pointer that is not pressed.
	Move
	// Drag of a pointer that is pressed. It is separate from Move so that a
	// control need not track pressedness to tell them apart.
	Drag
	// Enter is the pointer arriving over the area.
	Enter
	// Leave is the pointer departing the area, and it also arrives when the
	// pointer is taken away without moving -- a window losing focus, a finger
	// lifted. A hover drawn on Enter is undone here.
	Leave
	// Scroll of a pointer, delivered by position rather than to a tracked
	// pointer.
	Scroll
)

// kindNames is the text of each kind, in the order of the bits above.
var kindNames = [...]string{
	"Cancel",
	"Press",
	"Release",
	"Move",
	"Drag",
	"Enter",
	"Leave",
	"Scroll",
}

// String names the kinds in the set, low bit first, joined by a pipe.
//
// The order is the order of the bits and not the order they were written in, so
// two filters asking for the same kinds read the same and the text is worth
// comparing.
//
// A value carrying a bit this package does not name still answers, and says the
// number. The only moment any of these is read is a moment when something is
// already wrong -- a comparison that failed, a log line about an event that
// should not have arrived -- and a String that panicked there would replace the
// value that was wrong with a stack trace from the code printing it.
func (k Kind) String() string {
	var text strings.Builder
	rest := k
	for i, name := range kindNames {
		bit := Kind(1) << i
		if k&bit == 0 {
			continue
		}
		rest &^= bit
		if text.Len() > 0 {
			text.WriteByte('|')
		}
		text.WriteString(name)
	}
	if rest != 0 || k == 0 {
		if text.Len() > 0 {
			text.WriteByte('|')
		}
		text.WriteString("Kind(0x" + strconv.FormatUint(uint64(rest), 16) + ")")
	}
	return text.String()
}

// Source is what performed the event.
type Source uint8

const (
	// Mouse generated event.
	Mouse Source = iota
	// Touch generated event, from a finger or a pen.
	Touch
)

// String names the source, or says the number of one this package does not
// name.
func (s Source) String() string {
	switch s {
	case Mouse:
		return "Mouse"
	case Touch:
		return "Touch"
	default:
		return "Source(" + strconv.FormatUint(uint64(s), 10) + ")"
	}
}

// Priority tells a handler whether the pointer is still ambiguous.
type Priority uint8

const (
	// Shared is for a handler in a matching set larger than one: the gesture
	// could still turn out to belong to another handler, and anything done now
	// has to be undoable on a Cancel.
	Shared Priority = iota
	// Grabbed is for a handler that is alone in the set. Nothing else will
	// claim this pointer.
	Grabbed
)

// String names the priority, or says the number of one this package does not
// name.
func (p Priority) String() string {
	switch p {
	case Shared:
		return "Shared"
	case Grabbed:
		return "Grabbed"
	default:
		return "Priority(" + strconv.FormatUint(uint64(p), 10) + ")"
	}
}

// Buttons is a set of mouse buttons, not one of them.
//
// A set rather than a single value because more than one can be down at once,
// and because [Event.Buttons] reports what is held rather than what changed:
// a press of the second button while the first is down reports both.
type Buttons uint8

const (
	// ButtonPrimary is the primary button, the left one under a right hand.
	// It is named by role and not by side because the two swap.
	ButtonPrimary Buttons = 1 << iota
	// ButtonSecondary is the secondary button, the right one under a right
	// hand, and the one that opens a context menu.
	ButtonSecondary
	// ButtonTertiary is the tertiary button, usually the wheel pressed down.
	ButtonTertiary
	// ButtonQuaternary is the fourth button, usually meaning backward.
	ButtonQuaternary
	// ButtonQuinary is the fifth button, usually meaning forward.
	ButtonQuinary
)

// buttonNames is the text of each button, in the order of the bits above.
var buttonNames = [...]string{
	"ButtonPrimary",
	"ButtonSecondary",
	"ButtonTertiary",
	"ButtonQuaternary",
	"ButtonQuinary",
}

// Contain reports whether every button of the argument is in the set.
//
// It is all-of, not any-of. Nothing is asked of an empty argument, so every set
// contains one -- including the empty set. A caller that wants any-of is asking
// a different question and has to write it, which is the reason this one is
// spelled out here rather than left to the reader of a bitwise and.
func (b Buttons) Contain(buttons Buttons) bool {
	return b&buttons == buttons
}

// String names the buttons in the set, low bit first, joined by a pipe. An
// empty set, and a set carrying a bit this package does not name, say the
// number for the reason given on [Kind.String].
func (b Buttons) String() string {
	var text strings.Builder
	rest := b
	for i, name := range buttonNames {
		bit := Buttons(1) << i
		if b&bit == 0 {
			continue
		}
		rest &^= bit
		if text.Len() > 0 {
			text.WriteByte('|')
		}
		text.WriteString(name)
	}
	if rest != 0 || b == 0 {
		if text.Len() > 0 {
			text.WriteByte('|')
		}
		text.WriteString("Buttons(0x" + strconv.FormatUint(uint64(rest), 16) + ")")
	}
	return text.String()
}

// Filter is what a control asks for.
//
// Only a tag declared with event.Op can be targeted: the declaration is what
// puts the tag in a hit area, and a filter naming a tag that was never declared
// matches nothing rather than matching everywhere.
type Filter struct {
	// Target is the tag declared with event.Op.
	Target event.Tag
	// Kinds is the set of kinds to match, or-ed together. A zero set matches
	// nothing, which is what a control that wants no pointer events should
	// have -- by not asking.
	Kinds Kind
	// ScrollX and ScrollY are how much of a scroll this control can use, per
	// axis. Every [Scroll] event delivered to Target satisfies
	//
	//	ScrollX.Min <= e.Scroll.X <= ScrollX.Max
	//	ScrollY.Min <= e.Scroll.Y <= ScrollY.Max
	//
	// and what is left over is offered to the next control under the pointer.
	// That is what makes a list inside a list behave: the inner one takes what
	// it can until it reaches its end, and the remainder moves the outer one
	// instead of being lost.
	//
	// Both are zero by default, so a control that asks for Scroll without
	// setting them receives events carrying no distance. A control that
	// scrolls has to say how far it can go.
	ScrollX ScrollRange
	ScrollY ScrollRange
}

// ImplementsFilter marks Filter as an event filter.
func (Filter) ImplementsFilter() {}

// ImplementsEvent marks Event as an event.
func (Event) ImplementsEvent() {}

// ScrollRange is how far a control can scroll in one axis, as a distance it
// will accept rather than a position it is at.
//
// Min is negative and Max positive for a control that can go both ways; a
// control already at its top leaves Min at zero and is offered nothing
// upward, which is what hands the gesture to whatever encloses it.
type ScrollRange struct {
	Min, Max int
}

// Union is the range that accepts what either range accepts.
//
// It is what merges the filters of one tag that asked more than once in a
// frame. The result is the widest of the two rather than the narrowest,
// because two asks from the same control are two things it is willing to do,
// and an intersection would let the second silently cancel the first.
func (s ScrollRange) Union(s2 ScrollRange) ScrollRange {
	return ScrollRange{
		Min: min(s.Min, s2.Min),
		Max: max(s.Max, s2.Max),
	}
}

// GrabCmd claims a pointer for one handler.
//
// Every other handler that matched the pointer is sent a [Cancel] and stops
// receiving it, and the claim holds until the grabbing handler stops being
// declared. It is how a gesture that was ambiguous gets settled: a drag that
// has travelled far enough to no longer be a tap grabs, and the tap handler is
// told to undo what it had drawn.
//
// The claim is refused unless ID names a pressed pointer and Tag is among the
// handlers it matched. A handler cannot grab a pointer that was never over it.
type GrabCmd struct {
	Tag event.Tag
	ID  ID
}

// ImplementsCommand marks GrabCmd as a command.
func (GrabCmd) ImplementsCommand() {}

// PassOp marks the handlers declared inside it as pass-through: they receive
// the event without stopping it reaching what is behind them.
//
// It is for an overlay whose hit area is deliberately larger than its ink --
// the edge strip that opens a drawer covers a band of the interface, and the
// interface underneath has to keep working while the drawer is shut.
type PassOp struct{}

// PassStack is a pushed [PassOp], and is popped to end its scope.
type PassStack struct {
	ops     *ops.Ops
	id      ops.StackID
	macroID uint32
}

// Push begins a pass-through scope, and returns the stack entry that ends it.
func (p PassOp) Push(o *op.Ops) PassStack {
	id, mid := ops.PushOp(&o.Internal, ops.PassStack)
	data := ops.Write(&o.Internal, ops.TypePassLen)
	data[0] = byte(ops.TypePass)
	return PassStack{ops: &o.Internal, id: id, macroID: mid}
}

// Pop ends the scope begun by Push, restoring the pass mode that was in effect.
func (p PassStack) Pop() {
	ops.PopOp(p.ops, ops.PassStack, p.id, p.macroID)
	data := ops.Write(p.ops, ops.TypePopPassLen)
	data[0] = byte(ops.TypePopPass)
}
