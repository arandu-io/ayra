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
// small calendar that says it opens a month. It is restylable under the name of
// the widget that draws it, by the part "root".
func dateField(c ayra.Context, inst ComponentProps, name, label string, ink color.NRGBA, disabled bool) ayra.Dimensions {
	t := c.Theme
	style := t.Resolve(name, "root", inst.VariantName("default"), theme.Style{
		Background: theme.Colour(t.Colours.Background),
		Foreground: theme.Colour(ink),
		Border:     theme.Colour(t.Colours.Border),
		Radius:     theme.Pt(t.Metrics.Radius),
		Padding:    theme.InsetXY(12, 8),
		Font:       theme.Font{Size: theme.Pt(t.Type.Body)},
	}, inst)

	if disabled {
		style.Background = theme.Colour(fade(style.Background.NRGBA()))
		style.Border = theme.Colour(fade(style.Border.NRGBA()))
		style.Foreground = theme.Colour(fade(style.Foreground.NRGBA()))
		c.Context = c.Context.Disabled()
	}

	inner := style
	inner.Padding = theme.Insets{}
	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return drawPart(c.With(gtx), label, inner, unit.Sp(t.Type.Body), 1, textStart, plain())
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return calendarMark(c.With(gtx), style.Foreground.NRGBA())
			}),
		)
	})
}

// textStart is where a line of text begins.
const textStart = text.Start
