package widget

import (
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
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
	// ComponentProps is what the caller changes on this one instance. An alert
	// publishes "root", "title" and "message".
	ComponentProps
	// Title is the one line somebody reads if they read nothing else.
	Title string
	// Body is the detail. Empty draws the title alone, which is right for a
	// message that fits in one line.
	Body string
	// Severity is how bad the news is. Zero is context.
	Severity Severity

	// endRoom is room kept free at the end of the band, in points, for a mark a
	// widget draws over it: the toast's close.
	endRoom float32
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p AlertProps) PartNames() []string { return []string{"root", "title", "message"} }

// Layout draws the alert and returns the room it took.
//
// It fills the width it is given, unlike a badge: an alert is a band across the
// region it belongs to, and one sized to its text would be a label floating in
// the middle of a form.
func (p AlertProps) Layout(c ayra.Context) ayra.Dimensions {
	fill, ink, border := p.colours(c.Theme)
	variant := p.VariantName(p.Severity.String())

	root := c.Theme.Resolve("alert", "root", variant, theme.Style{
		Background: theme.Colour(fill),
		Border:     theme.Colour(border),
		Padding:    theme.Uniform(12),
	}, p.ComponentProps)
	root.Padding.Right += p.endRoom
	// A note's title is in the card's own foreground, as Kyse's is
	// (text-card-foreground), and its message in the quieter one: the two were
	// one grey, and the title held against the message by its weight alone. A
	// failure is the destructive colour throughout.
	titleInk := ink
	if p.Severity != Failure {
		titleInk = c.Theme.Colours.CardForeground
	}
	title := c.Theme.Resolve("alert", "title", variant, theme.Style{
		Foreground: theme.Colour(titleInk),
		Font:       theme.Font{Size: theme.Pt(c.Theme.Type.Body)},
	}, p.ComponentProps)
	message := c.Theme.Resolve("alert", "message", variant, theme.Style{
		Foreground: theme.Colour(ink),
		Font:       theme.Font{Size: theme.Pt(c.Theme.Type.Small)},
	}, p.ComponentProps)

	return Surface(c, root, func(c ayra.Context) ayra.Dimensions {
		line := func(c ayra.Context, content string, s theme.Style, face font.Font) ayra.Dimensions {
			return drawPart(c, content, s, unit.Sp(c.Theme.Type.Body), 0, text.Start, face)
		}
		if p.Body == "" {
			return line(c, p.Title, title, semibold())
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return line(c.With(gtx), p.Title, title, semibold())
			}),
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return line(c.With(gtx), p.Body, message, plain())
			}),
		)
	})
}

// colours answers the fill, the ink and the border for this severity.
//
// The border and the ink are the severity's own colour, which is one decision
// rather than eight: a palette that named a colour for every severity would
// need one per scheme, and the pairs drift.
func (p AlertProps) colours(t theme.Theme) (fill, ink, border color.NRGBA) {
	if p.Severity == Failure {
		// On the card, as Kyse draws it (bg-card, text-destructive). A tint of
		// the red under red text left the small message at 4.0:1 in the light
		// scheme and 4.4:1 in the dark, short of the 4.5:1 a line that size
		// needs; the card gives 4.8:1 and 6.5:1.
		return t.Colours.Card, t.Colours.Destructive, t.Colours.Destructive
	}
	return t.Colours.Muted, t.Colours.MutedForeground, t.Colours.Border
}

// wash is a colour at the opacity a background can carry text over.
func wash(c color.NRGBA) color.NRGBA {
	c.A = 26
	return c
}
