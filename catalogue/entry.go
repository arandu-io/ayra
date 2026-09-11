package main

import "github.com/arandu-io/ayra"

// Entry is one control of the library, and what draws it.
type Entry struct {
	// Control is the name of the props type this entry demonstrates, spelled
	// exactly as the library spells it.
	//
	// It is a field rather than a heading written into the drawing because the
	// completeness gate reads it: a control added to the library and not to
	// this list is a hole nobody can see, and a name a test can compare is what
	// turns that into a failure. Spelling it differently here is the same
	// failure with a different message.
	Control string

	// Summary is the one line under the name. It says what the control is for,
	// not what the demonstration shows: a reader who wants to know what is on
	// the screen is looking at it.
	Summary string

	// Build makes the demonstration, and is called once.
	//
	// Once, because state lives in the closure it returns. A control's state is
	// a value the caller holds across frames, so a demonstration rebuilt every
	// frame would hand each control a state that had just been made: no press
	// would ever complete, nothing typed would survive, and every list would
	// reopen on the frame after it was closed.
	Build func() ayra.Widget
}

// Section is one page of the catalogue.
//
// Fifty-nine controls do not fit on a screen and do not fit in a reader's head
// either. The pages are cut by what a control is for, because that is the
// question somebody arrives with -- they are looking for the way to ask a
// question, or the way to say something went wrong, and not for a name
// beginning with S.
type Section struct {
	// Name is what the tab says.
	Name string
	// Entries are the controls on the page, in the order they are drawn.
	Entries []Entry
}

// sections is the catalogue: every control, in the order it is offered.
//
// It is a value rather than a switch or a series of calls, and the test that
// keeps this complete is why. A gate that had to parse a function to learn what
// was drawn would be a second implementation of the catalogue, and the two
// would disagree the first time somebody wrote an entry in an unusual shape. A
// slice is read.
func sections() []Section {
	return []Section{
		{Name: "Actions", Entries: actions()},
		{Name: "Forms", Entries: forms()},
		{Name: "Navigation", Entries: navigation()},
		{Name: "Feedback", Entries: feedback()},
		{Name: "Content", Entries: content()},
		{Name: "Data", Entries: data()},
	}
}

// controls answers the name of every control the catalogue draws, in the order
// the pages offer them.
func controls() []string {
	var names []string
	for _, section := range sections() {
		for _, entry := range section.Entries {
			names = append(names, entry.Control)
		}
	}
	return names
}
