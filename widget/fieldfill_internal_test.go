package widget

import (
	"testing"

	"github.com/arandu-io/ayra/theme"
)

// TestAFieldIsFilledAsKyseFillsIt: the box took the page's background, so in a
// card -- white on a grey page -- it was a grey box on a white one, and the hint
// and the typed value were two greys on that grey.
func TestAFieldIsFilledAsKyseFillsIt(t *testing.T) {
	light := fieldFill(theme.New(theme.Light))
	if !light.IsSet() || light.NRGBA().A != 0 {
		t.Errorf("a light field is filled with %v, want nothing: the card shows through", light.NRGBA())
	}

	dark := theme.New(theme.Dark)
	fill := fieldFill(dark).NRGBA()
	if fill.A != 255 {
		t.Fatalf("a dark field is drawn with alpha %d, want an opaque mix: the engine blends translucent paint in linear light", fill.A)
	}
	card := dark.Colours.Card
	if fill.R <= card.R || int(fill.R)-int(card.R) > 24 {
		t.Errorf("a dark field is %d over a card of %d: it is a tint of 30%% of the input colour, not a panel", fill.R, card.R)
	}
}
