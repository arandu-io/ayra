package theme

import (
	"image/color"

	"github.com/arandu-io/ayra/engine/font"
)

// What follows is how a product changes the look of a control without writing
// the control again.
//
// The web half of Arandu does it with a class on an element, and a data-part
// naming the element inside a component the class is meant for. A drawing list
// has no classes and no cascade, so the same three things are carried as data:
//
//   - a [Style] is what a class would have set -- a fill, a border, a radius,
//     an inset, a type -- and says nothing about what it was not asked to set;
//   - a [Look] is a set of those, by widget and by part, and the theme holds
//     one for the whole application, which is the stylesheet;
//   - a [Component] is the same thing written on one instance, which is the
//     class on one element.
//
// The names of the parts are the ones the Kyse components publish in data-part:
// "root" is the outermost shape of a widget and every widget has it, and a
// widget with an inner part a caller can reach -- the title of an alert, the
// initials of an avatar -- calls it what the web component calls it. The
// widget's doc comment lists the parts it draws, and a test compares that list
// with what is drawn.

// Paint is a colour a [Style] may or may not set.
//
// A bare colour cannot say "unset", because the zero colour is a real one:
// transparent, which is what "no border" is. So an override that wants no
// border says [Transparent] and an override that has no opinion says nothing,
// and the two are different values.
type Paint struct {
	colour color.NRGBA
	set    bool
}

// Colour is a paint of this colour.
func Colour(c color.NRGBA) Paint { return Paint{colour: c, set: true} }

// Transparent is a paint that draws nothing, and means it: a border set to this
// removes the border a widget would otherwise draw, where an unset paint leaves
// it alone.
var Transparent = Paint{set: true}

// IsSet reports whether the paint carries an opinion.
func (p Paint) IsSet() bool { return p.set }

// NRGBA is the colour, and the zero colour when the paint is not set.
func (p Paint) NRGBA() color.NRGBA { return p.colour }

// Or is the paint when it is set, and the fallback otherwise.
func (p Paint) Or(fallback Paint) Paint {
	if p.set {
		return p
	}
	return fallback
}

// Length is a size, in points, that a [Style] may or may not set.
//
// Zero is a real length -- a radius of zero is a square corner -- so "unset"
// cannot be the zero value of a float, which is why this is a type.
type Length struct {
	points float32
	set    bool
}

// Pt is a length of that many points.
func Pt(points float32) Length { return Length{points: points, set: true} }

// IsSet reports whether the length carries an opinion.
func (l Length) IsSet() bool { return l.set }

// Points is the length, and zero when it is not set.
func (l Length) Points() float32 { return l.points }

// Or is the length when it is set, and the fallback otherwise.
func (l Length) Or(fallback Length) Length {
	if l.set {
		return l
	}
	return fallback
}

// Insets is the room between the edge of a shape and what is inside it, in
// points, on each side.
type Insets struct {
	Top, Right, Bottom, Left float32
	set                      bool
}

// Inset is room of these sizes, in the order a stylesheet writes them.
func Inset(top, right, bottom, left float32) Insets {
	return Insets{Top: top, Right: right, Bottom: bottom, Left: left, set: true}
}

// InsetXY is room of one size at the sides and another above and below, which
// is what almost every control wants.
func InsetXY(x, y float32) Insets { return Inset(y, x, y, x) }

// Uniform is the same room on all four sides.
func Uniform(points float32) Insets { return Inset(points, points, points, points) }

// IsSet reports whether the insets carry an opinion.
func (i Insets) IsSet() bool { return i.set }

// Or is the insets when they are set, and the fallback otherwise.
func (i Insets) Or(fallback Insets) Insets {
	if i.set {
		return i
	}
	return fallback
}

// Weight is a font weight on the scale a stylesheet uses, 100 to 900, with 400
// the ordinary one. The drawing layer counts from 400 instead; this is the
// number a person copies out of a stylesheet, and the conversion is here once.
type Weight int

const (
	// Regular is the ordinary weight.
	Regular Weight = 400
	// Medium is a label on a control.
	Medium Weight = 500
	// SemiBold is a title that has to hold against a fill.
	SemiBold Weight = 600
	// Bold is the heavier of the two a text can ask for.
	Bold Weight = 700
)

// engine is the weight as the drawing layer counts it.
func (w Weight) engine() font.Weight { return font.Weight(w - 400) }

// Font is the type of one part: which face, how heavy, how large. Every field
// left at zero is left to the widget.
type Font struct {
	// Face is the typeface by name, as the font collection the window was
	// opened with registers it. Empty keeps the widget's own, which is the
	// theme's sans face -- or its monospace one, for a part that is code.
	Face string
	// Weight is the heaviness. Zero keeps the widget's own.
	Weight Weight
	// Size is the type size in points. Unset keeps the widget's own, which is
	// a role of the theme's scale.
	Size Length
}

// Or is the font with each unset field taken from the fallback.
func (f Font) Or(fallback Font) Font {
	if f.Face == "" {
		f.Face = fallback.Face
	}
	if f.Weight == 0 {
		f.Weight = fallback.Weight
	}
	f.Size = f.Size.Or(fallback.Size)
	return f
}

// Style is how one part of a widget looks. Every field is optional, and a field
// that is not set leaves the widget's own choice alone -- which is what makes a
// Style an override rather than a replacement.
//
// The zero Style changes nothing.
type Style struct {
	// Background fills the part's shape.
	Background Paint
	// Foreground is the ink: the colour the part's text and marks are drawn in.
	Foreground Paint
	// Border is the colour of the line around the shape. [Transparent] removes
	// it.
	Border Paint
	// BorderWidth is the thickness of that line, in points. Unset keeps the
	// hairline, one point and never less than one pixel.
	BorderWidth Length
	// Radius rounds the corners, in points.
	Radius Length
	// Padding is the room inside the shape, around its content.
	Padding Insets
	// Font is the type of the part's text.
	Font Font

	// HoverBackground replaces the fill while the pointer is over the part, on
	// a part that answers a pointer. Unset keeps the widget's own wash.
	HoverBackground Paint
	// PressedBackground replaces the fill while the part is held down.
	PressedBackground Paint
}

// Or is the style with each unset field taken from the fallback: s wins where it
// has an opinion and the fallback speaks where it does not.
func (s Style) Or(fallback Style) Style {
	s.Background = s.Background.Or(fallback.Background)
	s.Foreground = s.Foreground.Or(fallback.Foreground)
	s.Border = s.Border.Or(fallback.Border)
	s.BorderWidth = s.BorderWidth.Or(fallback.BorderWidth)
	s.Radius = s.Radius.Or(fallback.Radius)
	s.Padding = s.Padding.Or(fallback.Padding)
	s.Font = s.Font.Or(fallback.Font)
	s.HoverBackground = s.HoverBackground.Or(fallback.HoverBackground)
	s.PressedBackground = s.PressedBackground.Or(fallback.PressedBackground)
	return s
}

// Parts are the styles of the parts of one widget, by the name the widget
// publishes: "root", "title", "label".
type Parts map[string]Style

// Look is the styles of several widgets: the widget's name, then the part.
//
//	theme.Look{
//		"button": {"root": {Radius: theme.Pt(2)}},
//		"alert":  {"title": {Font: theme.Font{Weight: theme.Bold}}},
//	}
//
// The widget's name is the lower-case name of its Props type without "Props":
// ButtonProps is "button", DatePickerProps is "datepicker".
type Look map[string]Parts

// Component is what a caller writes on one instance of a widget. It is the
// counterpart of Kyse's ComponentProps, and a Props type embeds it as
// ComponentProps.
//
//	widget.ButtonProps{
//		Label: "Pagar",
//		ComponentProps: theme.Component{
//			Custom: "brand",
//			Parts:  theme.Parts{"root": {Radius: theme.Pt(999)}},
//		},
//	}
//
// It beats the theme's look and the variant, and it changes only the instance.
type Component struct {
	// Custom names a variant the theme registers in [Styles.Variants], by a name
	// of the product's own: "brand", "read", "alert". It takes the place of the
	// widget's Variant, and a name the theme does not register draws the
	// widget's ordinary look, so a typo is a widget in the wrong colour rather
	// than a failed frame; [Theme.HasVariant] is how a test catches it.
	Custom string
	// Style is shorthand for Parts["root"], spelled shorter because it is what
	// most calls want. Parts["root"] wins where both set a field.
	Style Style
	// Parts is what to change on each named part of this instance.
	Parts Parts
}

// VariantName is the variant in force: the custom one when the caller wrote it,
// and the widget's own otherwise.
func (c Component) VariantName(builtin string) string {
	if c.Custom != "" {
		return c.Custom
	}
	return builtin
}

// Resolve answers how one part of a widget looks.
//
// The widget passes what it would draw on its own as base, and four layers are
// laid over it, each beating the one before:
//
//  1. the theme's Look for that widget and part, which is the application's
//     stylesheet;
//  2. the variant's Look in [Styles.Variants], when the widget's variant -- the
//     custom one, or the built-in one by its name -- is registered there;
//  3. the instance's Style, for the root part;
//  4. the instance's Parts for that part.
//
// A widget calls it once per part per frame and draws from the result, so the
// answer is as cheap as a few map lookups, and with nothing registered it is
// the base returned unchanged.
func (t Theme) Resolve(widget, part, variant string, base Style, inst Component) Style {
	s := base
	if t.Styles != nil {
		if style, found := t.Styles.Look[widget][part]; found {
			s = style.Or(s)
		}
		if variant != "" {
			if style, found := t.Styles.Variants[variant][widget][part]; found {
				s = style.Or(s)
			}
		}
	}
	if part == "root" {
		s = inst.Style.Or(s)
	}
	if style, found := inst.Parts[part]; found {
		s = style.Or(s)
	}
	return s
}

// HasVariant reports whether the theme registers a variant of this name.
func (t Theme) HasVariant(name string) bool {
	if t.Styles == nil {
		return false
	}
	_, found := t.Styles.Variants[name]
	return found
}

// Token is a named colour from the stylesheet the theme was made from, beyond
// the ones the palette names: --tone-read becomes "tone-read". The zero colour
// answers for a name the theme does not carry, and [Theme.HasToken] is how to tell
// the two apart.
func (t Theme) Token(name string) color.NRGBA {
	if t.Styles == nil {
		return color.NRGBA{}
	}
	return t.Styles.Tokens[name]
}

// HasToken reports whether the theme carries a token of this name.
func (t Theme) HasToken(name string) bool {
	if t.Styles == nil {
		return false
	}
	_, found := t.Styles.Tokens[name]
	return found
}
