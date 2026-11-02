package widget_test

import (
	"fmt"
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
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

// formOf draws a column of the given widgets, one under the other with no room
// between them, which is how a form is laid out.
func formOf(children ...ayra.Widget) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		items := make([]layout.FlexChild, 0, len(children))
		for _, child := range children {
			child := child
			items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return child(c.With(gtx))
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, items...)
	}
}

// TestTheOpenListIsDrawnOverTheFieldsAfterIt is the select in the middle of a
// form. The list was drawn where the control is, so the next field of the form
// -- drawn later -- painted over the options, and took the presses meant for
// them: a list nobody could choose from. It is put off to the end of the frame.
//
// The same list is drawn with a field after it and without one, and the part of
// the picture that is the list has to be the same in both.
func TestTheOpenListIsDrawnOverTheFieldsAfterIt(t *testing.T) {
	plans := []string{"Gratuito", "Equipe", "Empresarial"}

	list := func(after bool) *image.RGBA {
		var open, below widget.Select
		open.Open()
		children := []ayra.Widget{func(c ayra.Context) ayra.Dimensions {
			return widget.SelectProps{Options: plans, Placeholder: "Plano"}.Layout(c, &open)
		}}
		if after {
			children = append(children, func(c ayra.Context) ayra.Dimensions {
				return widget.SelectProps{Options: plans, Placeholder: "Outro plano"}.Layout(c, &below)
			})
		}
		return drawn(t, 360, 260, theme.Light, formOf(children...))
	}

	alone, crowded := list(false), list(true)

	// Under the control (about 40 px) and inside the list (about 120 px tall), a
	// little in from its edge, which is a hairline that blends with whatever is
	// beneath it and so is never the same twice.
	area := image.Rect(8, 48, 352, 150)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if alone.RGBAAt(x, y) != crowded.RGBAAt(x, y) {
				t.Fatalf("the list is not what is on top at %d,%d: the field after it was drawn over the options", x, y)
			}
		}
	}
}

// TestALongListScrollsInsteadOfRunningOffTheWindow is a list of the states of
// Brazil: twenty-seven rows of 36 points, which the window does not have, so the
// last options were somewhere nobody could reach. The list is as tall as eight
// rows and scrolls inside that.
func TestALongListScrollsInsteadOfRunningOffTheWindow(t *testing.T) {
	var options []string
	for i := 1; i <= 40; i++ {
		options = append(options, fmt.Sprintf("Opção %d", i))
	}
	var state widget.Select
	state.Open()

	picture := drawn(t, 360, 700, theme.Light, func(c ayra.Context) ayra.Dimensions {
		return widget.SelectProps{Options: options, Placeholder: "Estado"}.Layout(c, &state)
	})

	if !inked(picture, image.Rect(0, 60, 360, 100)) {
		t.Fatal("the list is not drawn under the control")
	}
	if inked(picture, image.Rect(0, 420, 360, 700)) {
		t.Error("the list runs on past eight rows: forty options are 1440 points of list")
	}
}

// TestADisabledSelectFadesItsCaret fixes the one part of it that stayed at full
// strength: the text went pale and the arrow beside it did not.
func TestADisabledSelectFadesItsCaret(t *testing.T) {
	strength := func(disabled bool) int {
		var state widget.Select
		picture := drawn(t, 360, 60, theme.Light, func(c ayra.Context) ayra.Dimensions {
			return widget.SelectProps{Options: []string{"Equipe"}, Placeholder: "Plano", Disabled: disabled}.Layout(c, &state)
		})
		darkest := 255
		for y := 8; y < 32; y++ {
			for x := 322; x < 346; x++ {
				darkest = min(darkest, luminance(picture.RGBAAt(x, y)))
			}
		}
		return darkest
	}

	if on, off := strength(false), strength(true); off <= on {
		t.Errorf("the caret of a disabled select is as dark as an enabled one's: %d against %d", off, on)
	}
}
