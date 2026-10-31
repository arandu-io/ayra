// Package theme is the palette, the radius and the type scale a drawn control
// reads.
//
// It is the one place a colour literal appears. Every other package asks for
// the name -- Primary, Destructive, MutedForeground -- so that a control and
// the next control agree, which is the whole difference between an application
// that looks like one thing and a set of screens that each look like whoever
// wrote them.
//
// The palette is generated from the stylesheet the browser half of the product
// already uses, so a colour called the same thing on both sides is the same
// colour. Changing one means changing the stylesheet and regenerating; editing
// the generated file is refused by a test.
package theme

//go:generate go run ../internal/tokengen basecoat/base.css tokens_gen.go

import (
	"image/color"
	"slices"

	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/text"
)

// Scheme is which palette a Theme answers with.
type Scheme int

const (
	// Light is the default, and it is the default because the stylesheet
	// declares it under the bare selector and the dark one as an override.
	Light Scheme = iota
	// Dark is the override.
	Dark
)

// String names the scheme, for a diagnostic screen and for a test failure.
func (s Scheme) String() string {
	if s == Dark {
		return "dark"
	}
	return "light"
}

// Theme is what a control reads to draw itself.
//
// It is a value, not a pointer, and it is passed down rather than looked up:
// a control that reached a package-level current theme could not be drawn
// twice on one screen in two schemes, which is exactly what a preview of both
// needs to do.
type Theme struct {
	// Scheme is which palette this is, and it is kept so that a control
	// choosing between two treatments -- a shadow that reads on light and
	// disappears on dark -- can ask instead of comparing colours.
	Scheme Scheme
	// Colours are the named surfaces.
	Colours Palette
	// Metrics are the lengths, in points.
	Metrics Metrics
	// Type is the scale and the faces of the text.
	Type Typography

	// Styles are what a product lays over the default look: the tokens of its
	// stylesheet, a look for the whole application and the variants it names.
	// Nil is the default theme, and the zero Theme is comparable with == because
	// of it, which it would not be if the maps were fields of Theme itself.
	//
	// The value is shared between copies of the Theme, as a map is: build it
	// once, before the first window, and do not write to it afterwards.
	Styles *Styles
}

// Styles are the parts of a theme that are open sets, by name.
type Styles struct {
	// Tokens are the colours of the stylesheet the theme was made from that the
	// palette has no field for: a product's --tone-read, --box-green. They are
	// keyed by the property's name without the leading dashes, so "tone-read",
	// and a variant a product writes reads them with [Theme.Token] instead of
	// repeating the value.
	Tokens map[string]color.NRGBA

	// Look is how the application restyles its widgets, by widget and by part.
	// It is the stylesheet of the native half: see [Look].
	Look Look

	// Variants are the variants the product registers by name, "brand" or
	// "alert", each a [Look] of its own. A widget draws one by its
	// [Component.Custom] name, and a built-in variant is restyled the same way,
	// by its own name ("destructive").
	Variants map[string]Look
}

// Typography is the scale and the faces of the text.
//
// The scale is written here rather than generated, because the stylesheet
// expresses it in utility classes rather than in custom properties: there is
// no `--text-body` to read. The numbers are the ones those classes resolve to.
// A product with a scale of its own writes it over these, and its faces come
// from the same collection the window is opened with ([shell.Config.Fonts]).
type Typography struct {
	// Display is the largest, for the one line a screen is about.
	Display float32
	// Heading is a section.
	Heading float32
	// Body is running text and the label on a control.
	Body float32
	// Small is a description under a field, and a caption.
	Small float32
	// Mono is code, an identifier, and a column of figures that has to line
	// up. Same size as Body, different face.
	Mono float32

	// Sans is the typeface ordinary text is set in, by the name the font
	// collection registers it under. Empty takes the first face of the
	// collection, which is what a theme that never named one has always drawn.
	Sans string
	// Monospace is the typeface of code, identifiers and columns of figures.
	// Empty takes [MonoTypeface].
	Monospace string
	// Weights say which weight each of the four a widget asks for is drawn in.
	// A face that does not carry a weight is how a product maps a widget's
	// SemiBold to the nearest one it does carry.
	Weights Weights
}

// MonoTypeface is the typeface a widget asks for when it wants monospace. The
// theme's [Typography.Monospace] stands in for it when it names another.
const MonoTypeface = "Go Mono"

// Weights are the weights the four a widget asks for resolve to. A zero field
// keeps the ordinary one: 400, 500, 600 and 700.
type Weights struct {
	Regular, Medium, SemiBold, Bold Weight
}

// Resolve answers the font a widget asked for as the font this theme draws.
//
// A widget asks for a role -- the ordinary face, the monospace one, the medium
// or the bold weight -- and never for a typeface by name. This is where the role
// becomes a face and a weight of the application's own, and it is the one place
// that does, so a widget that was never taught about a product's fonts still
// draws in them.
func (t Typography) Resolve(f font.Font) font.Font {
	switch f.Typeface {
	case "":
		if t.Sans != "" {
			f.Typeface = font.Typeface(t.Sans)
		}
	case MonoTypeface:
		if t.Monospace != "" {
			f.Typeface = font.Typeface(t.Monospace)
		}
	}

	var weight Weight
	switch f.Weight {
	case font.Normal:
		weight = t.Weights.Regular
	case font.Medium:
		weight = t.Weights.Medium
	case font.SemiBold:
		weight = t.Weights.SemiBold
	case font.Bold:
		weight = t.Weights.Bold
	}
	if weight != 0 {
		f.Weight = weight.engine()
	}
	return f
}

// Apply lays a part's [Font] over a font a widget asked for, and resolves the
// result against the theme. The part's face is a typeface by name and is taken
// as it is; its weight is on the stylesheet's scale.
func (t Typography) Apply(base font.Font, over Font) font.Font {
	f := t.Resolve(base)
	if over.Face != "" {
		f.Typeface = font.Typeface(over.Face)
	}
	if over.Weight != 0 {
		f.Weight = over.Weight.engine()
	}
	return f
}

// Missing answers the typefaces this scale names that a font collection does
// not carry. A name the collection lacks does not fail: the shaper falls back,
// and a product whose text is in the wrong face finds out by looking. Checked
// when the window opens, it is a mistake with a name instead.
func (t Typography) Missing(faces []text.FontFace) []string {
	have := map[string]bool{}
	for _, face := range faces {
		have[string(face.Font.Typeface)] = true
	}
	var missing []string
	for _, name := range []string{t.Sans, t.Monospace} {
		if name != "" && !have[name] && !slices.Contains(missing, name) {
			missing = append(missing, name)
		}
	}
	return missing
}

// typography is the one scale, shared by both schemes: a heading does not
// change size when the lights go out.
var typography = Typography{
	Display: 30,
	Heading: 20,
	Body:    14,
	Small:   12,
	Mono:    14,
}

// New returns the theme for a scheme.
func New(s Scheme) Theme {
	colours := lightPalette
	if s == Dark {
		colours = darkPalette
	}
	return Theme{Scheme: s, Colours: colours, Metrics: metrics, Type: typography}
}

// Contrast returns the colour that reads against the given surface.
//
// It exists so that a control drawing on a surface it was handed -- a card
// inside a dialog inside a sidebar -- does not have to know which of the
// ten paired surfaces it landed on to pick its text colour. The pairs are the ones
// the stylesheet declares: every surface has a foreground beside it, and this
// is that mapping in one direction.
//
// A surface this does not know answers with the plain foreground, because a
// caller drawing on a colour of its own has already decided the look and the
// readable default is better than a zero colour.
func (t Theme) Contrast(surface color.NRGBA) color.NRGBA {
	switch surface {
	case t.Colours.Background:
		return t.Colours.Foreground
	case t.Colours.Card:
		return t.Colours.CardForeground
	case t.Colours.Popover:
		return t.Colours.PopoverForeground
	case t.Colours.Primary:
		return t.Colours.PrimaryForeground
	case t.Colours.Secondary:
		return t.Colours.SecondaryForeground
	case t.Colours.Muted:
		return t.Colours.MutedForeground
	case t.Colours.Accent:
		return t.Colours.AccentForeground
	case t.Colours.Sidebar:
		return t.Colours.SidebarForeground
	case t.Colours.SidebarPrimary:
		return t.Colours.SidebarPrimaryForeground
	case t.Colours.SidebarAccent:
		return t.Colours.SidebarAccentForeground
	}
	return t.Colours.Foreground
}
