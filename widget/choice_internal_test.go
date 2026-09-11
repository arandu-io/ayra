package widget

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
)

// options are what these tests choose between.
var options = []string{"Free", "Team", "Enterprise"}

// TestNothingChosenIsNotTheFirstOption fixes a form that arrives already filled
// in with a value nobody picked.
//
// Zero is an option -- the first -- so a control storing "nothing picked" as
// zero answers with it from the frame it is drawn. On a submit that is the
// wrong answer sent without anybody being asked.
func TestNothingChosenIsNotTheFirstOption(t *testing.T) {
	var state Select
	props := SelectProps{Options: options, Placeholder: "Choose one"}

	if at := state.Selected(); at != -1 {
		t.Errorf("a fresh control answered %d", at)
	}

	for range 3 {
		c, _ := field(t, theme.Light, 300)
		props.Layout(c, &state)

		if at := state.Selected(); at != -1 {
			t.Fatalf("after being drawn the control answered %d with nobody having chosen", at)
		}
	}

	state.Choose(2)
	if at := state.Selected(); at != 2 {
		t.Errorf("after choosing the third option the control answered %d", at)
	}
}

// TestThePlaceholderIsDrawnUntilSomethingIsChosen is the same fault read from
// the screen rather than from the answer.
//
// The control takes the width it is offered either way, so nothing about its
// size says which string is in it. What says so is the decision itself, which
// is why that decision is a function.
func TestThePlaceholderIsDrawnUntilSomethingIsChosen(t *testing.T) {
	props := SelectProps{Options: options, Placeholder: "Choose one"}

	var empty Select
	if text, picked := props.label(&empty); picked || text != props.Placeholder {
		t.Errorf("with nothing chosen the control draws %q as a chosen value: %v", text, picked)
	}

	var first Select
	first.Choose(0)
	if text, picked := props.label(&first); !picked || text != options[0] {
		t.Errorf("with the first option chosen the control draws %q, picked %v", text, picked)
	}

	// An index the options no longer reach falls back rather than panicking: a
	// screen that arrives with a stored choice and a shorter list is ordinary.
	var stale Select
	stale.Choose(9)
	if text, picked := props.label(&stale); picked || text != props.Placeholder {
		t.Errorf("an index past the end drew %q, picked %v", text, picked)
	}
}

// TestAMenuReportsNothingBeforeAnybodyPresses fixes a menu whose first action
// runs on the frame it is first drawn.
func TestAMenuReportsNothingBeforeAnybodyPresses(t *testing.T) {
	var state Menu
	props := MenuProps{Entries: []string{"Rename", "Duplicate", "", "Delete"}}

	if at := state.Chosen(); at != -1 {
		t.Errorf("a fresh menu answered %d", at)
	}

	for range 3 {
		c, _ := field(t, theme.Light, 300)
		props.Layout(c, &state, func(c ayra.Context) ayra.Dimensions {
			return ButtonProps{Label: "Actions"}.Layout(c, &Button{})
		})

		if at := state.Chosen(); at != -1 {
			t.Fatalf("after being drawn the menu answered %d with nobody having pressed", at)
		}
	}
}

// TestAMenubarReportsNothingBeforeAnybodyPresses is the same, for the row of
// menus that used to work around it by building each one by hand with the
// sentinel already in it.
func TestAMenubarReportsNothingBeforeAnybodyPresses(t *testing.T) {
	var state Menubar
	props := MenubarProps{
		Titles:  []string{"File", "Edit"},
		Entries: [][]string{{"New", "Open"}, {"Undo", "Redo"}},
	}

	for range 3 {
		c, _ := field(t, theme.Light, 400)
		props.Layout(c, &state)

		if menu, entry := state.Chosen(); menu != -1 || entry != -1 {
			t.Fatalf("the menubar answered menu %d entry %d with nobody having pressed", menu, entry)
		}
	}
}
