// Package semantic is what a control tells software that reads a screen aloud.
//
// The descriptions form a tree, and the node an operation attaches to is the
// clip most recently pushed. Where the call sits in the source decides nothing
// on its own: two operations under one clip describe one control, and the same
// two with a clip between them describe two.
//
// The whole vocabulary is nine words -- five classes, a label, a description
// and two states -- and every one of them has to mean the same thing to every
// platform that walks the tree. That is why the set is closed. It is also why
// the set is smaller than what those platforms can be told, and the gaps are
// worth naming here, because a control that needs one of them gets no warning
// that it is asking for something the tree cannot carry:
//
//   - No operation points at another node. A field cannot say that the words
//     drawn beside it are its error message, so anything a listener has to hear
//     about a control belongs inside that control's own label or description.
//     Written anywhere else it is reached as unrelated prose, or not reached.
//
//   - Nothing here is urgent. An operation describes the node it is written
//     under, and what reads the screen finds out when it next walks the tree.
//     There is no way to say that a region has just changed and must be spoken
//     now, so an answer that lands somewhere other than where the person is
//     working arrives in silence.
//
//   - [SelectedOp] is a bool, so there is no third state. A box that is neither
//     ticked nor clear -- the one over a list where some rows are chosen and
//     some are not -- has to announce as one of the two, and both answers are
//     wrong about half of what they describe.
//
//   - A class is all a node gets to say about what kind of control it is. There
//     is no word for a range, for a position within a set of tabs, for how deep
//     a heading sits, or for whether a section is open. A control shaped like
//     one of those announces [Unknown], which tells the other side nothing
//     rather than telling it something close.
//
// Widening the set is not a change to this package alone. A word added here
// writes bytes, and those bytes reach a listener only once what walks the tree
// carries the word and every platform underneath has somewhere to put it. Until
// then it is a word nothing on the other side reads.
package semantic

import (
	"strconv"

	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/op"
)

// LabelOp is the name of a control: the words a person would use to ask for
// it, and the same words drawn on it.
//
// It is what makes a node worth stopping on. A node carrying a class and no
// label announces as a kind of control with no identity -- a button, and
// nothing about which button -- and a person moving through a screen one stop
// at a time has no way to tell it from the next one.
type LabelOp string

// DescriptionOp is what the label leaves out and a person still needs.
//
// It is usually absent, and that is right. It is read out after the label every
// single time the node is reached, so a description that only restates the
// label doubles the length of every stop and adds nothing to any of them.
type DescriptionOp string

// ClassOp is what kind of control a node is.
//
// The set is closed, and it belongs to the platforms that read the tree rather
// than to this package: a value from outside it arrives somewhere that has no
// word for it. The zero value is [Unknown], so a node that never said what it
// was says exactly that, instead of announcing as whichever kind of control
// happens to be declared first.
type ClassOp int

const (
	// Unknown is a control the set has no word for. Everything else the node
	// carries still travels; only the kind of control is missing.
	Unknown ClassOp = iota
	// Button is a control that acts when it is pressed.
	Button
	// CheckBox is a control that is ticked or clear, on its own.
	CheckBox
	// Editor is a control that takes typing.
	Editor
	// RadioButton is one option of a set, where choosing it drops the others.
	RadioButton
	// Switch is a control that is on or off.
	Switch
)

// SelectedOp is the state of a control that is ticked, chosen or switched on.
//
// It says nothing true about the rest, and a node that carries it anyway
// announces a state for a control that has none. Being a bool, it is also the
// whole of what a control may report: one that is genuinely in neither state
// has to claim one of them.
type SelectedOp bool

// EnabledOp says whether the control answers.
//
// A control drawn but unavailable is the case this exists for. The shape is on
// the screen either way, so without this the live one and the dead one are
// announced identically and the difference is found out by pressing.
type EnabledOp bool

// Add writes the label under the node most recently pushed onto o.
//
// An empty label is written like any other. Whether a control with no name
// should announce at all is decided by the caller and not here: this operation
// carries what it was handed, and the caller that built an empty name is the
// one that knows whether a name was supposed to exist.
func (l LabelOp) Add(o *op.Ops) { addText(o, ops.TypeSemanticLabel, string(l)) }

// Add writes the description under the node most recently pushed onto o.
func (d DescriptionOp) Add(o *op.Ops) { addText(o, ops.TypeSemanticDesc, string(d)) }

// Add writes the class under the node most recently pushed onto o.
//
// A class from outside the set is carried rather than corrected. This package
// cannot know which kind of control was meant, and a guess would announce the
// wrong one with no less confidence than a right answer.
func (c ClassOp) Add(o *op.Ops) { addByte(o, ops.TypeSemanticClass, byte(c)) }

// Add writes the selected state under the node most recently pushed onto o.
func (s SelectedOp) Add(o *op.Ops) { addByte(o, ops.TypeSemanticSelected, state(bool(s))) }

// Add writes the enabled state under the node most recently pushed onto o.
func (e EnabledOp) Add(o *op.Ops) { addByte(o, ops.TypeSemanticEnabled, state(bool(e))) }

// String names the class, and answers for a value from outside the set instead
// of refusing to.
//
// A class comes back off the list as a single byte, so a value the set does not
// name is reachable without any caller having written one. This is also the
// method a printer calls while reporting something else -- a failure saying
// which kind of control turned up is exactly where an unnamed value appears --
// and one that panicked there would replace the message somebody was reading
// with a note about a second failure.
func (c ClassOp) String() string {
	if c < 0 || int(c) >= len(classNames) {
		return "ClassOp(" + strconv.Itoa(int(c)) + ")"
	}
	return classNames[c]
}

// classNames is one name per class, indexed by the class itself.
//
// Written as a table beside the set rather than as a switch, so that the names
// and the constants are one list long and a class added without a name is a
// length that stopped matching rather than a case nobody noticed was missing.
var classNames = [...]string{
	Unknown:     "Unknown",
	Button:      "Button",
	CheckBox:    "CheckBox",
	Editor:      "Editor",
	RadioButton: "RadioButton",
	Switch:      "Switch",
}

// addText writes an operation whose words are held beside the list.
//
// The length comes from the tag rather than from the caller, which is the whole
// reason this is a function. Every operation on a list declares how many bytes
// it occupies, and the one after it is read from where that count ends: a tag
// paired with somebody else's length does not fail where it is written, it
// shifts everything after it and surfaces as an unrelated operation arriving
// mangled.
func addText(o *op.Ops, kind ops.OpType, words string) {
	data := ops.Write1String(&o.Internal, int(kind.Size()), words)
	data[0] = byte(kind)
}

// addByte writes an operation carrying one byte of payload.
func addByte(o *op.Ops, kind ops.OpType, value byte) {
	data := ops.Write(&o.Internal, int(kind.Size()))
	data[0] = byte(kind)
	data[1] = value
}

// state is a bool as the list carries it.
func state(on bool) byte {
	if on {
		return 1
	}
	return 0
}
