package widget

import (
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// Severity is what an alert is telling somebody, and it decides its colours.
//
// Two, and they are the two the browser half of this product has. A first
// attempt here had four -- note, success, caution, failure -- and the palette
// could not draw them: it is generated from the stylesheet, the stylesheet
// declares no colour for success or for caution, and what came out was a
// warning in pale blue on pale blue that could barely be read.
//
// Inventing the two missing colours here would put a green in the native
// application that no page on the web has. The vocabulary is shared on purpose,
// and this is what sharing it costs: a fifth severity starts in the stylesheet,
// for both sides at once, and arrives here generated.
type Severity uint8

const (
	// Note is something worth saying. Zero, because most of them are.
	Note Severity = iota
	// Failure is something that went wrong.
	Failure
)

// String names the severity, for a diagnostic screen and a test failure.
func (s Severity) String() string {
	if s == Failure {
		return "failure"
	}
	return "note"
}

// AlertProps is what a message about the screen it is on is drawn from.
type AlertProps struct {
	// Title is the one line somebody reads if they read nothing else.
	Title string
	// Body is the detail. Empty draws the title alone, which is right for a
	// message that fits in one line.
	Body string
	// Severity is how bad the news is. Zero is context.
	Severity Severity
}

// Layout draws the alert and returns the room it took.
//
// It fills the width it is given, unlike a badge: an alert is a band across the
// region it belongs to, and one sized to its text would be a label floating in
// the middle of a form.
func (p AlertProps) Layout(c ayra.Context) ayra.Dimensions {
	fill, ink, border := p.colours(c.Theme)

	return surface(c, fill, border, controlRadius(c), func(c ayra.Context) ayra.Dimensions {
		return layout.UniformInset(12).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			if p.Body == "" {
				return drawLine(inner, p.Title, unit.Sp(c.Theme.Type.Body), ink, semibold())
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(inner.Context,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawLine(inner.With(gtx), p.Title, unit.Sp(c.Theme.Type.Body), ink, semibold())
				}),
				layout.Rigid(layout.Spacer{Height: 4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawLine(inner.With(gtx), p.Body, unit.Sp(c.Theme.Type.Small), ink, plain())
				}),
			)
		})
	})
}

// colours answers the fill, the ink and the border for this severity.
//
// The fill is the severity's own colour at low opacity and the border is the
// same colour at full, which is one decision rather than eight: a palette that
// named a background for every severity would need one per scheme, and the
// pairs drift.
func (p AlertProps) colours(t theme.Theme) (fill, ink, border color.NRGBA) {
	if p.Severity == Failure {
		return wash(t.Colours.Destructive), t.Colours.Destructive, t.Colours.Destructive
	}
	return t.Colours.Muted, t.Colours.MutedForeground, t.Colours.Border
}

// wash is a colour at the opacity a background can carry text over.
func wash(c color.NRGBA) color.NRGBA {
	c.A = 26
	return c
}
