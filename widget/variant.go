// Package widget draws the controls, and owns the state machines that a page
// used to get from the document.
//
// A control here is two halves, and the split is deliberate. State -- pressed,
// focused, what the caret is doing -- lives in a value the caller holds across
// frames, because a control redrawn thirty times a second cannot remember
// anything by itself. Appearance is a value computed fresh each frame from the
// theme and a few fields, and it remembers nothing.
//
// The vocabulary is the one the browser half of the product already uses:
// Variant and Size mean here what they mean there, so a developer who can
// describe a control on one side can describe it on the other. What is not
// shared is the drawing, because a class name and a cascade have no meaning in
// a list of shapes.
package widget

// Variant is what a control is for, which is what decides its colours.
//
// The set is closed, and closing it is the point. A variant added per screen
// is a design system with one exception per team, and the exception outlives
// whoever added it. What does not fit one of these is drawn by writing the
// colours out, which is visible in review.
type Variant uint8

const (
	// Default is the ordinary control: the one a screen has several of, and
	// the one a reader's eye skips over on the way to the important one.
	Default Variant = iota
	// Secondary is present but not asking to be pressed.
	Secondary
	// Outline is a border and no fill, for a control beside a filled one.
	Outline
	// Ghost carries no border and no fill until it is pointed at. It is for a
	// control that would be noise at rest -- a row action, a toolbar item.
	Ghost
	// Destructive removes something that does not come back. It is the one
	// variant that is a warning rather than a weight.
	Destructive
	// Link reads as text and behaves as a control.
	Link
)

// String names the variant, for a diagnostic screen and for a test failure.
func (v Variant) String() string {
	switch v {
	case Secondary:
		return "secondary"
	case Outline:
		return "outline"
	case Ghost:
		return "ghost"
	case Destructive:
		return "destructive"
	case Link:
		return "link"
	}
	return "default"
}

// Size is how much room a control takes, which is a decision about density
// rather than about importance.
type Size uint8

const (
	// Medium is the default and the one a form is built from.
	Medium Size = iota
	// ExtraSmall is for a control inside a dense row.
	ExtraSmall
	// Small is a toolbar.
	Small
	// Large is a single call to action on a screen that has one.
	Large
	// Icon is square, for a control whose whole label is its mark.
	Icon
)

// String names the size.
func (s Size) String() string {
	switch s {
	case ExtraSmall:
		return "xs"
	case Small:
		return "sm"
	case Large:
		return "lg"
	case Icon:
		return "icon"
	}
	return "default"
}
