package widget

import (
	"image"

	"github.com/arandu-io/ayra/engine/io/semantic"
	"github.com/arandu-io/ayra/engine/op/clip"

	"github.com/arandu-io/ayra"
)

// A control announces itself here and nowhere else.
//
// The platform that reads these walks a tree, and the node a description
// attaches to is the clip most recently pushed -- so where the call sits in a
// control decides which shape the announcement belongs to, and a control that
// announced itself in its own way would attach to whatever it happened to have
// pushed. One place means one answer to that.
//
// It is also how the promise stays checkable. A control that names something on
// the screen has to say what it is, and a test reads this package's source to
// find any that does not. That test can exist because there is one function to
// look for.

// Class is what a control is, to whatever reads the screen aloud.
//
// The set is the platform's, not this library's: five classes are what the
// tree carries, and a sixth invented here would be a word that reaches nothing.
// A control whose nearest match is none of them announces its label without a
// class, which is what the platform does with anything it has no word for.
type Class int

const (
	// Plain is a control the tree has no class for. Its label still travels.
	Plain Class = iota
	// Pressable is anything that acts when pressed.
	Pressable
	// Checkable is a box that is ticked or not.
	Checkable
	// Editable is a field that takes typing.
	Editable
	// Choosable is one option of a set where picking one drops the others.
	Choosable
	// Switchable is a control that is on or off.
	Switchable
)

// class answers the platform's word for a role.
func (c Class) class() semantic.ClassOp {
	switch c {
	case Pressable:
		return semantic.Button
	case Checkable:
		return semantic.CheckBox
	case Editable:
		return semantic.Editor
	case Choosable:
		return semantic.RadioButton
	case Switchable:
		return semantic.Switch
	}
	return semantic.Unknown
}

// Announcement is what a control says about itself.
type Announcement struct {
	// Label is the text a person sees on it, and it is the same text. A label
	// written for the screen reader alone drifts from the one on the screen,
	// and the two then describe different controls -- which is worse than the
	// one that says nothing, because it is wrong rather than absent.
	Label string
	// Description is what the label does not say and a person would need. It
	// is usually empty: a button reading Delete needs no second sentence, and
	// a description repeating the label is noise read aloud every time.
	Description string
	// Class is what kind of control this is.
	Class Class
	// Selected is meaningful on a control that is ticked, chosen or switched
	// on, and ignored on the others.
	Selected bool
	// Disabled says the control is drawn but does not answer.
	Disabled bool
}

// announce gives a control a node of its own in the tree, and describes it.
//
// It pushes its own area rather than writing into whatever is in force. A
// description attaches to the most recent area, and a control does not always
// have one: the press target a live control registers is not registered when
// the control is unavailable, because there is nothing to press. An
// announcement written into that target therefore vanished exactly when it
// mattered most -- what reached a screen reader for a disabled button was its
// text, with no class and nothing saying it could not be used, which reads as
// an ordinary line of prose.
//
// Owning the area also means the bounds are the control's. A description on a
// borrowed node is a description whose shape is somebody else's, and what reads
// a screen aloud uses that shape to say where the control is.
//
// The area stays open until the returned function is called, and the caller
// keeps it open across the content. That is what makes the control's own text a
// child of it rather than a second node beside it: every piece of text in this
// library announces itself, so a control whose node closed before its label was
// drawn produced two entries with the same words -- the control, and then the
// same words again as ordinary prose.
//
// Called with an empty label it does nothing: a control with no name has
// nothing true to say, and a node with an empty label is a stop a person cannot
// act on.
func announce(c ayra.Context, size image.Point, a Announcement) (close func()) {
	if a.Label == "" {
		return func() {}
	}

	area := clip.Rect{Max: size}.Push(c.Ops)

	semantic.LabelOp(a.Label).Add(c.Ops)
	if class := a.Class.class(); class != semantic.Unknown {
		class.Add(c.Ops)
	}
	if a.Description != "" {
		semantic.DescriptionOp(a.Description).Add(c.Ops)
	}
	if a.Class == Checkable || a.Class == Choosable || a.Class == Switchable {
		semantic.SelectedOp(a.Selected).Add(c.Ops)
	}
	semantic.EnabledOp(!a.Disabled).Add(c.Ops)

	return area.Pop
}
