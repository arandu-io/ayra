package widget_test

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// Found on 02/11/2026 by drawing the display widgets under the Alpr.sh theme at
// the width of a phone. Each test below fails on the code that was there.

// TestAFailureAlertIsReadable measures the red of a failure alert against what it
// is drawn on, in both schemes, on the grounds the Alpr.sh theme gives the page.
// The red over a tint of the same red left the small message at 4.0:1 in the
// light scheme and 4.4:1 in the dark, short of the 4.5:1 a line that size needs.
func TestAFailureAlertIsReadable(t *testing.T) {
	grounds := map[theme.Scheme]struct{ page, card color.NRGBA }{
		theme.Light: {color.NRGBA{R: 0xf4, G: 0xf4, B: 0xf4, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		theme.Dark:  {color.NRGBA{A: 255}, color.NRGBA{R: 0x11, G: 0x11, B: 0x11, A: 255}},
	}
	for scheme, ground := range grounds {
		t.Run(scheme.String(), func(t *testing.T) {
			th := theme.New(scheme)
			th.Colours.Background, th.Colours.Card = ground.page, ground.card

			p := newPanel()
			p.size = image.Pt(300, 90)
			p.under(th, time.Time{}, func(c ayra.Context) ayra.Dimensions {
				c.Fill(th.Colours.Background)
				return widget.AlertProps{Title: "Falha", Body: "x", Severity: widget.Failure}.Layout(c)
			})
			// Inside the band, in its padding, away from the text.
			band := p.raster(t).RGBAAt(6, 30)
			ink := th.Colours.Destructive

			if ratio := contrast(ink, color.NRGBA{R: band.R, G: band.G, B: band.B, A: 255}); ratio < 4.5 {
				t.Errorf("destructive text on the alert's ground (%v) is %.2f:1, want at least 4.5:1", band, ratio)
			}
		})
	}
}
