package widget

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/unit"
)

// CardProps is what a grouped region of a screen is drawn from.
//
// A card is a surface and a boundary, and nothing else: it has no title of its
// own, no close button and no state. What goes inside it is a widget, so a card
// containing a heading and a form is written by writing a heading and a form.
type CardProps struct {
	// Padding is the room between the border and the content. Zero takes a
	// value that leaves text clear of the corner radius.
	Padding unit.Dp

	// Muted draws it on the quieter of the two surfaces, for a card inside
	// another card -- where the ordinary one would vanish against its parent.
	Muted bool
}

// Layout draws the card around the content and returns the room it took.
func (p CardProps) Layout(c ayra.Context, content ayra.Widget) ayra.Dimensions {
	padding := p.Padding
	if padding == 0 {
		padding = 16
	}

	fill := c.Theme.Colours.Card
	if p.Muted {
		fill = c.Theme.Colours.Muted
	}

	return surface(c, fill, c.Theme.Colours.Border, controlRadius(c), func(c ayra.Context) ayra.Dimensions {
		return layout.UniformInset(padding).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return content(c.With(gtx))
		})
	})
}
