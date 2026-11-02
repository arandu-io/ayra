package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// TestAControlInAGroupDrawsNoRingOfItsOwn: only "root" was bare, so a control
// that held the keyboard or was rejected drew its square border inside the
// group's rounded one.
func TestAControlInAGroupDrawsNoRingOfItsOwn(t *testing.T) {
	parts := widget.GroupedParts().Parts
	for _, name := range []string{"root", "focus", "invalid"} {
		part, ok := parts[name]
		if !ok || !part.Border.IsSet() || part.Border.NRGBA().A != 0 {
			t.Errorf("the grouped %q part keeps a border: %+v", name, part)
		}
	}
}

// TestAnAddonKeepsOffTheGroupsBorder: a bare text put in the group touched the
// border at its end.
func TestAnAddonKeepsOffTheGroupsBorder(t *testing.T) {
	p := newPanel()
	bare := p.draw(func(c ayra.Context) ayra.Dimensions { return widget.TextProps{Content: ".com.br"}.Layout(c) })
	addon := p.draw(func(c ayra.Context) ayra.Dimensions { return widget.InputAddonProps{Text: ".com.br"}.Layout(c) })
	if addon.Size.X < bare.Size.X+24 {
		t.Errorf("the addon is %d wide and its text %d: no room at either side", addon.Size.X, bare.Size.X)
	}
}

// TestAGroupReportsTheFocusOfWhatIsInIt: the group draws the ring, and asks the
// control whether it holds the keyboard.
func TestAGroupReportsTheFocusOfWhatIsInIt(t *testing.T) {
	p := newPanel()
	var in widget.Input
	var seen bool
	screen := func(c ayra.Context) ayra.Dimensions {
		seen = in.Focused(c)
		return widget.InputProps{}.Layout(c, &in)
	}
	p.draw(screen)
	tap(p, screen, 100, 15)
	p.draw(screen)
	if !seen {
		t.Error("a tapped field says it has no focus")
	}
}
