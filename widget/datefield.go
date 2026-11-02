package widget

import (
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// dateField draws the closed control of a date, a period or a date with a time:
// the value, or what stands in for it, with the border a text field has and the
// small calendar that says it opens a month: Kyse's CalendarBlank. owner is the
// Kyse component the field belongs to. It is restylable under the name of
// the widget that draws it, by the part "root".
func dateField(c ayra.Context, inst ComponentProps, name, owner, label string, ink color.NRGBA, disabled, focused bool) ayra.Dimensions {
	t := c.Theme
	style := t.Resolve(name, "root", inst.VariantName("default"), theme.Style{
		Background: fieldFill(t),
		Foreground: theme.Colour(ink),
		Border:     theme.Colour(t.Colours.Input),
		Radius:     theme.Pt(t.Metrics.Radius),
		Padding:    theme.InsetXY(12, 8),
		Font:       theme.Font{Size: theme.Pt(t.Type.Body)},
	}, inst)

	if disabled {
		ground := t.Colours.Background
		style.Background = theme.Colour(fadeOver(style.Background.NRGBA(), ground))
		style.Border = theme.Colour(fadeOver(style.Border.NRGBA(), ground))
		style.Foreground = theme.Colour(fadeOver(style.Foreground.NRGBA(), ground))
		c.Context = c.Context.Disabled()
	} else if focused {
		// The ring colour on the border; the ring around the field is drawn by
		// the caller, outside the area the press is registered on.
		style.Border = theme.Colour(t.Colours.Ring)
	}

	inner := style
	inner.Padding = theme.Insets{}
	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return drawPart(c.With(gtx), label, inner, unit.Sp(t.Type.Body), 1, textStart, plain())
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return calendarMark(c.With(gtx), owner, style.Foreground.NRGBA())
			}),
		)
	})
}

// fieldRing draws the focus ring around a field that holds the keyboard focus,
// outside the area its press is registered on, which clips to the field.
func fieldRing(c ayra.Context, field ayra.Dimensions, focused bool) {
	if focused {
		focusRing(c, field.Size, controlRadius(c), c.Theme.Colours.Ring)
	}
}

// textStart is where a line of text begins.
const textStart = text.Start
