package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// TestATrailLongerThanAPhoneWrapsAndKeepsThePageItIsOn: the row was cut at the
// edge of the window, and the last step -- the page the person is on -- was the
// part that was cut off.
func TestATrailLongerThanAPhoneWrapsAndKeepsThePageItIsOn(t *testing.T) {
	p := phone(300)
	var crumbs widget.Crumbs
	steps := []string{"Organização Condomínio Jardins", "Câmeras do portão principal", "Configurações avançadas", "Leituras"}

	dims := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.BreadcrumbProps{Steps: steps}.Layout(c, &crumbs)
	})
	if dims.Size.X > 390 {
		t.Errorf("the trail is %d points wide in a window of 390", dims.Size.X)
	}
	if dims.Size.Y < 2*44 {
		t.Errorf("the trail is %d points high: it did not break into lines, so the last steps are off the screen", dims.Size.Y)
	}
}

// TestAStepOfATrailIsAsHighAsAFinger: the words are 17 points tall.
func TestAStepOfATrailIsAsHighAsAFinger(t *testing.T) {
	p := phone(300)
	var crumbs widget.Crumbs
	dims := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.BreadcrumbProps{Steps: []string{"Câmeras", "Leituras"}}.Layout(c, &crumbs)
	})
	if dims.Size.Y < 44 {
		t.Errorf("a short trail is %d points high, want at least 44", dims.Size.Y)
	}
}
