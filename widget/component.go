package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	engine "github.com/arandu-io/ayra/engine/widget"

	"github.com/arandu-io/ayra/theme"
)

// ComponentProps is what every restylable widget takes besides its own data: a
// variant of the product's own, and the style of each named part of this one
// instance. It is the counterpart of Kyse's ComponentProps and is embedded in
// the same way.
//
//	widget.ButtonProps{
//		Label: "Pagar",
//		ComponentProps: widget.ComponentProps{
//			Custom: "brand",
//			Parts:  theme.Parts{"root": {Radius: theme.Pt(999)}},
//		},
//	}
//
// A widget that takes it publishes the parts it draws through PartNames, by the
// names the web component publishes in data-part, and a test compares that list
// with what the widget resolves. The theme holds the same thing for the whole
// application -- [theme.Theme.Look] -- and a variant the product names is
// registered in [theme.Theme.Variants]; the instance beats both.
//
// Kyse's Theme field, which writes data-theme on one element, has a smaller
// counterpart here: the theme is a value on the context, so a subtree drawn in
// another one is drawn with another Context.
type ComponentProps = theme.Component

// Everything below is the set of primitives the widgets of this package are
// written from, exported so that a widget of the product's own -- a licence
// plate, a camera card -- is written from the same ones and looks like the other
// fifty-nine instead of a copy of them. A primitive here takes a resolved
// [theme.Style]; asking the theme for one is [theme.Theme.Resolve].

// Surface draws the shape a [theme.Style] describes behind a widget and answers
// the room the widget took: its background, its border and the width of it, its
// radius, and the padding around the content.
//
// It lays the content out first, without drawing, to learn how much there is to
// cover, and paints the background underneath afterwards. Written per widget,
// one of them eventually paints the fill over its own text.
//
// A radius or a border width the style leaves unset is the control's: the
// theme's radius and a hairline.
func Surface(c ayra.Context, s theme.Style, content ayra.Widget) ayra.Dimensions {
	radius := controlRadius(c)
	if s.Radius.IsSet() {
		radius = c.Dp(unit.Dp(s.Radius.Points()))
	}
	width := hairline(c)
	if s.BorderWidth.IsSet() {
		width = c.Dp(unit.Dp(s.BorderWidth.Points()))
	}

	inner := content
	if s.Padding.IsSet() {
		inner = func(c ayra.Context) ayra.Dimensions {
			return insetOf(s.Padding).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				return content(c.With(gtx))
			})
		}
	}
	return surfaceWidth(c, s.Background.NRGBA(), s.Border.NRGBA(), radius, width, inner)
}

// Hairline is one point, and never less than one pixel: the width a border has
// when nobody asked for another. At one pixel per point a rounded hairline is
// zero, and zero is not a thin line, it is no line.
func Hairline(c ayra.Context) int { return hairline(c) }

// ControlRadius is the corner every control shares, in pixels.
func ControlRadius(c ayra.Context) int { return controlRadius(c) }

// TextOptions are the choices about a line of text that are not its look.
type TextOptions struct {
	// MaxLines caps the height. Zero is as many as the text needs.
	MaxLines int
	// Align is where the text sits in the room it was given.
	Align text.Alignment
	// Monospace sets it in the theme's monospace face.
	Monospace bool
}

// DrawText is the one place text is put on screen: the content in the font and
// ink a [theme.Style] says. An unset size is the theme's body size and an unset
// foreground is the theme's foreground, and the face and weight go through the
// theme's scale, so a product's own fonts reach the widgets written from this.
func DrawText(c ayra.Context, content string, s theme.Style, o TextOptions) ayra.Dimensions {
	if !s.Foreground.IsSet() {
		s.Foreground = theme.Colour(c.Theme.Colours.Foreground)
	}
	base := font.Font{}
	if o.Monospace {
		base.Typeface = theme.MonoTypeface
	}
	return drawPart(c, content, s, unit.Sp(c.Theme.Type.Body), o.MaxLines, o.Align, base)
}

// Fade is a colour at half its opacity, which is how a widget says "not now"
// without the palette naming a grey for it.
func Fade(c color.NRGBA) color.NRGBA { return fade(c) }

// Press is a fill as it looks while the pointer is over it or on it. A fill with
// nothing to darken answers a wash instead.
func Press(c color.NRGBA, held bool) color.NRGBA { return press(c, held) }

// Announce gives a widget a node of its own in the tree a screen reader walks,
// and describes it. The area stays open until the returned function is called,
// and the caller keeps it open across the content.
func Announce(c ayra.Context, size image.Point, a Announcement) (close func()) {
	return announce(c, size, a)
}

// Clickable is the state half of a widget that can be pressed: what a button
// remembers between frames, for a widget the product draws itself. The caller
// holds it, one per widget; one value drawn twice is one widget in two places.
type Clickable struct {
	click engine.Clickable
}

// Clicked reports a completed press, once, and consumes it. Ask before drawing,
// so that what the press changes is on the screen in the same frame.
func (p *Clickable) Clicked(c ayra.Context) bool { return p.click.Clicked(c.Context) }

// Pressed reports whether the widget is held down right now.
func (p *Clickable) Pressed() bool { return p.click.Pressed() }

// Hovered reports whether the pointer is over the widget.
func (p *Clickable) Hovered() bool { return p.click.Hovered() }

// Layout draws the content as the press target and returns the room it took.
// A disabled widget is drawn from a context that is [layout.Context.Disabled],
// which is no target for the pointer or the keyboard.
func (p *Clickable) Layout(c ayra.Context, content ayra.Widget) ayra.Dimensions {
	return p.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return content(c.With(gtx))
	})
}

// insetOf is a style's padding as the layout engine takes it.
func insetOf(i theme.Insets) layout.Inset {
	return layout.Inset{Top: unit.Dp(i.Top), Right: unit.Dp(i.Right), Bottom: unit.Dp(i.Bottom), Left: unit.Dp(i.Left)}
}

// paintShape fills and strokes the rounded rectangle of a size, under a style
// whose radius and border width are already in pixels.
func paintShape(c ayra.Context, size image.Point, fill, border color.NRGBA, radius, width int) {
	shape := clip.UniformRRect(image.Rectangle{Max: size}, radius)
	if fill.A > 0 {
		paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	}
	if border.A > 0 && width > 0 {
		stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(width)}
		paint.FillShape(c.Ops, border, stroke.Op())
	}
}

// surfaceWidth is surface with the border width chosen.
func surfaceWidth(c ayra.Context, fill, border color.NRGBA, radius, width int, content ayra.Widget) ayra.Dimensions {
	measure := op.Record(c.Ops)
	dims := content(c)
	drawn := measure.Stop()

	paintShape(c, dims.Size, fill, border, radius, width)

	drawn.Add(c.Ops)
	return dims
}
