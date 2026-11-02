package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/widget"
)

// atTheFoot draws a control of a form as the last field of a 600-point window,
// with only the status bar's strip over it, 24 points: the control is at 494.
func atTheFoot(top int, control func(c ayra.Context) ayra.Dimensions) func(ayra.Context) ayra.Dimensions {
	return func(c ayra.Context) ayra.Dimensions {
		c.Constraints.Min = image.Point{}
		return layout.Inset{Top: unitDp(top), Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return control(c.With(gtx))
		})
	}
}

// TestASelectNearTheBottomOpensItsListAboveIt: the list was always under the
// control, and under the foot of a form it ran off the window with its options out
// of reach.
func TestASelectNearTheBottomOpensItsListAboveIt(t *testing.T) {
	p := phone(600)
	var s widget.Select
	s.Open()
	screen := atTheFoot(470, func(c ayra.Context) ayra.Dimensions {
		return widget.SelectProps{Options: []string{"Portão", "Garagem", "Serviço", "Doca"}, Placeholder: "Câmera"}.Layout(c, &s)
	})
	for range 3 {
		p.windowed(24, screen)
	}

	// Just over the control, where the last option of a list above it is.
	p.press(image.Pt(100, 494-4-4-16), screen)

	if got := s.Selected(); got != 3 {
		t.Errorf("a press on the last option over the control chose %d, want 3: the list did not open above", got)
	}
}

// TestASelectListClosesOnAPressOutsideAndOnBack is Kyse's, and a phone's.
func TestASelectListClosesOnAPressOutsideAndOnBack(t *testing.T) {
	p := phone(600)
	var s widget.Select
	screen := atTheFoot(40, func(c ayra.Context) ayra.Dimensions {
		return widget.SelectProps{Options: []string{"Um", "Dois"}, Placeholder: "Escolha"}.Layout(c, &s)
	})
	s.Open()
	for range 3 {
		p.windowed(24, screen)
	}
	p.press(image.Pt(100, 400), screen)
	if s.Showing() {
		t.Error("a press outside left the list open")
	}
	if s.Selected() != -1 {
		t.Error("a press outside chose an option")
	}
}

// TestAnAutocompleteNearTheBottomOpensItsListAboveTheField is the same flip for
// the field that suggests.
func TestAnAutocompleteNearTheBottomOpensItsListAboveTheField(t *testing.T) {
	p := phone(600)
	var a widget.Autocomplete
	screen := atTheFoot(470, func(c ayra.Context) ayra.Dimensions {
		return widget.AutocompleteProps{Entries: []string{"Alfa", "Beta", "Gama"}, Placeholder: "Câmera"}.Layout(c, &a)
	})
	for range 2 {
		p.windowed(24, screen)
	}
	// Somebody touches the field and types: that is what opens the list.
	p.press(image.Pt(100, 510), screen)
	p.router.Queue(key.EditEvent{Text: "a"})
	for range 4 {
		p.windowed(24, screen)
	}

	// The field is at 494, and the last suggestion is just above it.
	p.press(image.Pt(100, 494-4-4-10), screen)

	if got := a.Chosen(); got != 2 {
		t.Errorf("a press on the last suggestion over the field chose %d, want 2: the list did not open above", got)
	}
}
