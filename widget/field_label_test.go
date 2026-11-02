package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// TestALongLabelWrapsAndKeepsItsRequiredMark: the label was cut to one line with
// an ellipsis, and the asterisk that says the field is required is at the end
// of the text, so it was the first thing to go.
func TestALongLabelWrapsAndKeepsItsRequiredMark(t *testing.T) {
	p := newPanel()
	p.size = image.Pt(200, 400)

	one := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.LabelProps{Text: "Nome", Required: true}.Layout(c)
	})
	many := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.LabelProps{Text: "Rótulo bem comprido para ver se quebra a linha em uma tela de celular estreita", Required: true}.Layout(c)
	})
	if many.Size.Y < one.Size.Y*2 {
		t.Errorf("a label of eighty characters in 200 pt is %d tall against %d for one word: it was not wrapped", many.Size.Y, one.Size.Y)
	}
}
