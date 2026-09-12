// Package key is the vocabulary of the keyboard, and of the input methods that
// stand in for one.
//
// Almost nothing here does anything. What is here is the words the two halves
// of a running application use to talk about a keystroke without ever meeting:
// the platform layer turns whatever the operating system handed it into these
// values, a control says which of them it wants with a [Filter], and neither
// side names the other. That is what lets one control work against a desktop
// keyboard, an on-screen one, and an input method that builds a character out
// of several keystrokes and hands over the finished character.
//
// A press and typed text are deliberately not the same event. An [Event] is a
// key going down or coming back up, which is what a shortcut is made of; an
// [EditEvent] is text arriving, which is what a field is made of. Reading text
// off presses works for one keyboard layout and fails for every other: a dead
// key, a composed character and a paste all produce text with no press behind
// them, and a press on a layout that is not US produces a character nobody
// spelled out.
package key

import (
	"strconv"
	"strings"

	"github.com/arandu-io/ayra/engine/io/event"
)

// Name is what a key is called.
//
// A letter is its upper case form, so "a" and "A" are both [Name] "A". Shift is
// the one modifier the layout has already applied by the time a name is made --
// shift-1 is "!" on a US keyboard -- and every other modifier is reported
// separately in [Event.Modifiers] rather than folded into the name. So
// ctrl-shift-1 is also "!", held with ctrl.
//
// Keys with no character of their own are named by the constants below.
type Name string

// The names of the keys that have no character to be called by.
//
// The symbols are the ones printed on the hardware, and they are the name
// rather than a label because a name is compared against, not shown: a control
// asks for [NameEscape] and the platform layer produces it, with nothing
// between them but these exact runes.
const (
	NameLeftArrow      Name = "←"
	NameRightArrow     Name = "→"
	NameUpArrow        Name = "↑"
	NameDownArrow      Name = "↓"
	NameReturn         Name = "⏎"
	NameEnter          Name = "⌤"
	NameEscape         Name = "⎋"
	NameHome           Name = "⇱"
	NameEnd            Name = "⇲"
	NameDeleteBackward Name = "⌫"
	NameDeleteForward  Name = "⌦"
	NamePageUp         Name = "⇞"
	NamePageDown       Name = "⇟"
	NameTab            Name = "Tab"
	NameSpace          Name = "Space"
	NameCtrl           Name = "Ctrl"
	NameShift          Name = "Shift"
	NameAlt            Name = "Alt"
	NameSuper          Name = "Super"
	NameCommand        Name = "⌘"
	NameF1             Name = "F1"
	NameF2             Name = "F2"
	NameF3             Name = "F3"
	NameF4             Name = "F4"
	NameF5             Name = "F5"
	NameF6             Name = "F6"
	NameF7             Name = "F7"
	NameF8             Name = "F8"
	NameF9             Name = "F9"
	NameF10            Name = "F10"
	NameF11            Name = "F11"
	NameF12            Name = "F12"
	NameBack           Name = "Back"
)

// Modifiers is the set of modifier keys held down.
//
// It is a set rather than one key because that is what a person's hands do, and
// because the question asked of it is almost never "which one" but "are these
// the ones" -- see [Modifiers.Contain].
type Modifiers uint32

// The modifier keys, one bit each.
//
// Ctrl and command are separate entries even though most keyboards only have
// one of them. Folding them together would make ctrl-C on Apple hardware mean
// copy, and it does not: the two keys exist side by side there and do different
// things. What picks the right one for a shortcut is [ModShortcut].
const (
	// ModCtrl is the ctrl key.
	ModCtrl Modifiers = 1 << iota
	// ModCommand is the command key, found on Apple keyboards.
	ModCommand
	// ModShift is the shift key.
	ModShift
	// ModAlt is the alt key, called option on Apple keyboards.
	ModAlt
	// ModSuper is the logo key, usually printed with a Windows logo.
	ModSuper
)

// Contain reports whether every modifier in wanted is held.
//
// An empty wanted is held by every set, which is what makes a filter that names
// no modifier match an unmodified key. Asked the other way round -- whether a
// set is exactly some combination -- it gives the wrong answer on purpose:
// ctrl-shift-S contains ctrl-S, and a shortcut that wants only ctrl-S has to
// say so by leaving shift out of both [Filter.Required] and [Filter.Optional].
func (m Modifiers) Contain(wanted Modifiers) bool {
	return m&wanted == wanted
}

// String is the combination as a person reads it, such as "Ctrl-Shift".
func (m Modifiers) String() string {
	var held []string
	for _, modifier := range modifierNames {
		if m.Contain(modifier.bit) {
			held = append(held, string(modifier.name))
		}
	}
	return strings.Join(held, "-")
}

// modifierNames is every modifier and the word it is said with, in the order
// they are said.
//
// The order is written down here rather than taken from the bits, because this
// text is shown to a person -- it is what appears beside a menu entry -- and an
// order that followed the bit layout would rearrange itself the day a modifier
// is added in the middle. Two spellings of one combination is a person checking
// whether they are the same shortcut.
var modifierNames = []struct {
	bit  Modifiers
	name Name
}{
	{ModCtrl, NameCtrl},
	{ModCommand, NameCommand},
	{ModShift, NameShift},
	{ModAlt, NameAlt},
	{ModSuper, NameSuper},
}

// State is whether a key went down or came back up.
type State uint8

const (
	// Press is a key going down.
	Press State = iota
	// Release is a key coming back up.
	//
	// Not every platform reports one. macOS, Linux, Windows and the browser do;
	// a control that only works if it is told about the release will therefore
	// work on those and quietly do nothing on the others, so the release is for
	// things that improve with it rather than things that need it.
	Release
)

// String is the state as a person reads it.
//
// A state this package does not know still reads back as its number rather than
// stopping the program. Printing an unknown value is exactly how anybody finds
// out what it was, and a String that gave up at that moment would take away the
// one way of asking.
func (s State) String() string {
	switch s {
	case Press:
		return "Press"
	case Release:
		return "Release"
	}
	return "State(" + strconv.FormatUint(uint64(s), 10) + ")"
}

// An Event is a key going down or coming back up.
//
// It is not how text is received: use [EditEvent] for that. This is what a
// shortcut, an arrow key and a page key are made of.
type Event struct {
	// Name of the key.
	Name Name
	// Modifiers is the set held down when the key moved.
	Modifiers Modifiers
	// State is whether it went down or came back up.
	State State
}

func (Event) ImplementsEvent() {}

// Filter asks for the key events a control wants.
//
// The two modifier sets are separate because a shortcut and a movement key need
// different things from them. Required is the combination that has to be held:
// leave it empty and the key matches unmodified. Optional is what may also be
// held without spoiling the match, which is how one filter covers an arrow key
// pressed alone and the same arrow extending a selection with shift. A modifier
// in neither set refuses the event, so ctrl-shift-S does not reach a filter
// that asked for ctrl-S.
type Filter struct {
	// Focus is the tag that has to hold the keyboard focus for this filter to
	// match. A nil Focus matches whatever is focused, including nothing.
	Focus event.Tag
	// Required is the set of modifiers that must be held.
	Required Modifiers
	// Optional is the set of modifiers that may be held as well.
	Optional Modifiers
	// Name of the key. As a special case, an empty Name matches every key no
	// other filter asked for by name.
	//
	// Keys the platform already does something with -- tab and shift-tab move
	// the focus -- are the exception: they reach only a filter that names them,
	// because a catch-all taking them would break moving around the screen with
	// the keyboard in every application at once. See
	// [github.com/arandu-io/ayra/engine/io/input.SystemEvent].
	Name Name
}

func (Filter) ImplementsFilter() {}
