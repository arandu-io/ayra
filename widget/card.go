package widget

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// CardProps is what a grouped region of a screen is drawn from.
//
// A card is a surface and a boundary, and nothing else: it has no title of its
// own, no close button and no state. What goes inside it is a widget, so a card
// containing a heading and a form is written by writing a heading and a form.
type CardProps struct {
	// ComponentProps is what the caller changes on this one instance. A card
	// publishes one part, "root".
	ComponentProps

	// Padding is the room between the border and the content. Zero takes a
	// value that leaves text clear of the corner radius.
	Padding unit.Dp

	// Muted draws it on the quieter of the two surfaces, for a card inside
	// another card -- where the ordinary one would vanish against its parent.
	Muted bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p CardProps) PartNames() []string { return []string{"root"} }

// Layout draws the card around the content and returns the room it took.
func (p CardProps) Layout(c ayra.Context, content ayra.Widget) ayra.Dimensions {
	padding := p.Padding
	if padding == 0 {
		padding = 16
	}

	fill, variant := c.Theme.Colours.Card, "default"
	if p.Muted {
		fill, variant = c.Theme.Colours.Muted, "muted"
	}

	style := c.Theme.Resolve("card", "root", p.VariantName(variant), theme.Style{
		Background: theme.Colour(fill),
		Foreground: theme.Colour(c.Theme.Colours.CardForeground),
		Border:     theme.Colour(c.Theme.Colours.Border),
		Radius:     theme.Pt(c.Theme.Metrics.Radius),
		Padding:    theme.Uniform(float32(padding)),
	}, p.ComponentProps)
	return Surface(c, style, content)
}
