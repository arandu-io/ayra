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

import "image/color"

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
	// Type is the size of each named role, in points.
	Type Typography
}

// Typography is the size of each named text role, in points.
//
// The scale is written here rather than generated, because the stylesheet
// expresses it in utility classes rather than in custom properties: there is
// no `--text-body` to read. The numbers are the ones those classes resolve to.
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
