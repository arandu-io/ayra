package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

const longSummary = "Configurações avançadas da câmera do portão principal da garagem do subsolo"

// TestASectionSummaryIsAtLeast44PointsHigh: the line a finger presses was 33.
func TestASectionSummaryIsAtLeast44PointsHigh(t *testing.T) {
	p := phone(300)
	var d widget.Disclosure
	dims := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.CollapsibleProps{Summary: "Fechada"}.Layout(c, &d, func(c ayra.Context) ayra.Dimensions { return ayra.Dimensions{} })
	})
	if dims.Size.Y < 44 {
		t.Errorf("the summary is %d points high, want at least 44", dims.Size.Y)
	}

	var a widget.Accordion
	a.Open(-1)
	dims = p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.AccordionProps{Sections: []string{"Uma"}}.Layout(c, &a, nil)
	})
	if dims.Size.Y < 44 {
		t.Errorf("the accordion's section is %d points high, want at least 44", dims.Size.Y)
	}
}

// TestALongSummaryWrapsInsteadOfBeingCut: it was cut at the edge with "...", and
// the words after it could not be read anywhere.
func TestALongSummaryWrapsInsteadOfBeingCut(t *testing.T) {
	p := phone(400)
	var d widget.Disclosure
	short := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.CollapsibleProps{Summary: "Curta"}.Layout(c, &d, func(c ayra.Context) ayra.Dimensions { return ayra.Dimensions{} })
	})
	long := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.CollapsibleProps{Summary: longSummary}.Layout(c, &d, func(c ayra.Context) ayra.Dimensions { return ayra.Dimensions{} })
	})
	if long.Size.Y <= short.Size.Y {
		t.Errorf("a summary of 75 characters is %d points high, as high as one of 5 (%d): it is cut, not wrapped", long.Size.Y, short.Size.Y)
	}
}
