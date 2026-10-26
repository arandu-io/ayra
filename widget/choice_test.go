package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// TestASelectionPastTheOptionsIsNoSelection is a list that got shorter under a
// choice: a role picked from three, and the server now sending one.
//
// The control drew the placeholder, because the index named nothing, while
// Selected still answered the index -- so a form built from it submitted a
// choice the screen said had not been made.
func TestASelectionPastTheOptionsIsNoSelection(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.Select
	state.Choose(2)

	widget.SelectProps{Options: []string{"viewer"}, Placeholder: "Pick a role"}.Layout(c, &state)

	if got := state.Selected(); got != -1 {
		t.Errorf("the screen shows the placeholder and Selected answers %d", got)
	}
}

// TestASelectionInsideTheOptionsIsKept keeps the reset to the case it is for.
func TestASelectionInsideTheOptionsIsKept(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.Select
	state.Choose(1)

	widget.SelectProps{Options: []string{"viewer", "admin"}}.Layout(c, &state)

	if got := state.Selected(); got != 1 {
		t.Errorf("a choice inside the options was dropped: %d", got)
	}
}
