package key

import "github.com/arandu-io/ayra/engine/io/event"

// Exactly one thing on the screen has the keyboard at a time, and the keyboard
// is the only way some people drive an application at all.
//
// So focus is a single value the router holds, not a flag each control keeps: a
// control that decided for itself that it was focused would be focused at the
// same time as the last one that decided the same, and a keystroke would arrive
// twice. What a control does instead is say where the focus may be put
// ([FocusFilter]), ask for it ([FocusCmd]), and be told when it gained or lost
// it ([FocusEvent]).

// A FocusEvent says a control gained or lost the keyboard.
//
// Both directions are delivered, and the losing one is the half that is easy to
// forget: a field that draws a caret and is never told it lost the focus draws
// two carets on the screen, on two fields, and only one of them is taking the
// typing.
type FocusEvent struct {
	// Focus is whether the target now has the keyboard.
	Focus bool
}

func (FocusEvent) ImplementsEvent() {}

// FocusFilter asks for everything that happens to one target's focus: its
// [FocusEvent], and the [EditEvent], [SnippetEvent], [SelectionEvent] and
// [CompositionEvent] an input method sends to whatever is focused.
//
// It is one filter rather than five because they are one subscription: a
// control that took the text an input method composed but not the news that it
// lost the focus would go on composing into a field the person has left.
type FocusFilter struct {
	// Target is the tag a previous event.Op registered.
	Target event.Tag
}

func (FocusFilter) ImplementsFilter() {}

// FocusCmd moves the keyboard focus, or clears it.
type FocusCmd struct {
	// Tag is what gains the focus. The focus is cleared if Tag is nil, and also
	// if Tag is something no [github.com/arandu-io/ayra/engine/io/event.Op]
	// registered -- a control that is not on the screen cannot be typed into,
	// and asking for it has to end somewhere rather than leave the keyboard
	// pointed at nothing.
	Tag event.Tag
}

func (FocusCmd) ImplementsCommand() {}

// FocusDirection is which way the focus moves when it is moved by a key.
//
// The four compass directions are for the arrow keys and are answered by where
// controls are on the screen; forward and backward are for tab and shift-tab
// and are answered by the order the controls were drawn in. They are separate
// because the two orders disagree, and both are right: a person pressing the
// down arrow means the control below, and a person pressing tab means the next
// one in the form.
type FocusDirection int

const (
	// FocusRight moves to the nearest control to the right.
	FocusRight FocusDirection = iota
	// FocusLeft moves to the nearest control to the left.
	FocusLeft
	// FocusUp moves to the nearest control above.
	FocusUp
	// FocusDown moves to the nearest control below.
	FocusDown
	// FocusForward moves to the next control in drawing order.
	FocusForward
	// FocusBackward moves to the previous control in drawing order.
	FocusBackward
)
