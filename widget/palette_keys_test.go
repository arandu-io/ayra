package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/widget"
)

// TestThePaletteTakesTypingAndTheArrowsAsSoonAsItOpens: it opened with the
// keyboard on the dialog, so nothing typed reached the field, and the arrow keys
// were asked for on a state that is never focused.
func TestThePaletteTakesTypingAndTheArrowsAsSoonAsItOpens(t *testing.T) {
	p := newPanel()
	var pal widget.Palette
	pal.Open()
	screen := func(c ayra.Context) ayra.Dimensions {
		return widget.PaletteProps{Commands: []string{"Ir para câmeras", "Ir para leituras", "Adicionar câmera"}}.Layout(c, &pal)
	}
	p.draw(screen)
	p.draw(screen)
	typeText(p, screen, "ir")
	p.draw(screen)
	pressKey(p, screen, key.NameDownArrow)
	pressKey(p, screen, key.NameReturn)
	if pal.Query() != "ir" {
		t.Fatalf("the field holds %q after typing with nothing tapped", pal.Query())
	}
	if got := pal.Chosen(); got != 1 {
		t.Errorf("down and return ran command %d, want 1", got)
	}
}
