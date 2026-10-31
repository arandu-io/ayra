package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// size draws a widget under a theme and answers the room it took.
func size(t *testing.T, th func(theme.Theme) theme.Theme, draw func(ayra.Context) ayra.Dimensions) image.Point {
	t.Helper()
	c, _ := frame(t, theme.Light, 400)
	c.Theme = th(c.Theme)
	return draw(c).Size
}

func same(th theme.Theme) theme.Theme { return th }

func button(props widget.ButtonProps) func(ayra.Context) ayra.Dimensions {
	var state widget.Button
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, &state) }
}

// TestWithNothingRegisteredAWidgetDrawsAsItAlwaysDid is the compatibility
// promise: the zero ComponentProps and an empty theme change nothing.
func TestWithNothingRegisteredAWidgetDrawsAsItAlwaysDid(t *testing.T) {
	plain := size(t, same, button(widget.ButtonProps{Label: "Pagar"}))
	empty := size(t, func(th theme.Theme) theme.Theme {
		th.Styles = &theme.Styles{Look: theme.Look{}, Variants: map[string]theme.Look{}}
		return th
	}, button(widget.ButtonProps{Label: "Pagar", ComponentProps: widget.ComponentProps{Parts: theme.Parts{}}}))

	if plain != empty {
		t.Errorf("an empty theme and empty parts changed the button from %v to %v", plain, empty)
	}
}

// TestAPartIsRestyledFromTheThemeAndFromTheInstance fixes the two places an
// override can come from.
func TestAPartIsRestyledFromTheThemeAndFromTheInstance(t *testing.T) {
	base := size(t, same, button(widget.ButtonProps{Label: "Pagar"}))

	fromTheme := size(t, func(th theme.Theme) theme.Theme {
		th.Styles = &theme.Styles{Look: theme.Look{"button": {"root": {Padding: theme.Uniform(30)}}}}
		return th
	}, button(widget.ButtonProps{Label: "Pagar"}))
	if fromTheme.X <= base.X || fromTheme.Y <= base.Y {
		t.Errorf("the theme's padding left the button at %v from %v", fromTheme, base)
	}

	fromInstance := size(t, same, button(widget.ButtonProps{
		Label:          "Pagar",
		ComponentProps: widget.ComponentProps{Parts: theme.Parts{"root": {Padding: theme.Uniform(30)}}},
	}))
	if fromInstance != fromTheme {
		t.Errorf("the same padding from the instance drew %v, and from the theme %v", fromInstance, fromTheme)
	}

	shorthand := size(t, same, button(widget.ButtonProps{
		Label:          "Pagar",
		ComponentProps: widget.ComponentProps{Style: theme.Style{Padding: theme.Uniform(30)}},
	}))
	if shorthand != fromTheme {
		t.Errorf("Style, the shorthand for Parts[root], drew %v, want %v", shorthand, fromTheme)
	}
}

// TestTheInstanceBeatsTheVariantAndTheVariantBeatsTheTheme fixes the order the
// layers are applied in, which is the order a class beats a rule.
func TestTheInstanceBeatsTheVariantAndTheVariantBeatsTheTheme(t *testing.T) {
	registered := func(th theme.Theme) theme.Theme {
		th.Styles = &theme.Styles{
			Look: theme.Look{"button": {"root": {Padding: theme.Uniform(10)}}},
			Variants: map[string]theme.Look{
				"brand": {"button": {"root": {Padding: theme.Uniform(20)}}},
			},
		}
		return th
	}

	theme10 := size(t, registered, button(widget.ButtonProps{Label: "Pagar"}))
	brand20 := size(t, registered, button(widget.ButtonProps{Label: "Pagar", ComponentProps: widget.ComponentProps{Custom: "brand"}}))
	instance30 := size(t, registered, button(widget.ButtonProps{
		Label: "Pagar",
		ComponentProps: widget.ComponentProps{
			Custom: "brand",
			Parts:  theme.Parts{"root": {Padding: theme.Uniform(30)}},
		},
	}))

	if !(theme10.X < brand20.X && brand20.X < instance30.X) {
		t.Errorf("widths are %d, %d, %d for the theme, the variant and the instance; each must beat the one before",
			theme10.X, brand20.X, instance30.X)
	}
}

// TestAVariantOfTheProductsOwnNeedsNoEnum is the open half of the set that used
// to be closed: a name nobody declared in this package draws, and a name the
// theme does not register draws the ordinary look.
func TestAVariantOfTheProductsOwnNeedsNoEnum(t *testing.T) {
	withBrand := func(th theme.Theme) theme.Theme {
		th.Styles = &theme.Styles{Variants: map[string]theme.Look{"read": {"badge": {"root": {Padding: theme.InsetXY(40, 2)}}}}}
		return th
	}
	badge := func(custom string) func(ayra.Context) ayra.Dimensions {
		return func(c ayra.Context) ayra.Dimensions {
			return widget.BadgeProps{Label: "ABC1D23", ComponentProps: widget.ComponentProps{Custom: custom}}.Layout(c)
		}
	}

	ordinary := size(t, withBrand, badge(""))
	read := size(t, withBrand, badge("read"))
	typo := size(t, withBrand, badge("raed"))

	if read.X <= ordinary.X {
		t.Errorf("the registered variant drew %v, no wider than the ordinary %v", read, ordinary)
	}
	if typo != ordinary {
		t.Errorf("an unregistered variant drew %v, want the ordinary look %v", typo, ordinary)
	}

	th := withBrand(theme.New(theme.Light))
	if !th.HasVariant("read") || th.HasVariant("raed") {
		t.Error("HasVariant does not tell a registered variant from a typo")
	}
}

// TestABuiltInVariantIsRestyledByItsName covers the other use of Variants: the
// product changes what "destructive" looks like without a variant of its own.
func TestABuiltInVariantIsRestyledByItsName(t *testing.T) {
	th := func(th theme.Theme) theme.Theme {
		th.Styles = &theme.Styles{Variants: map[string]theme.Look{"destructive": {"button": {"root": {Padding: theme.Uniform(30)}}}}}
		return th
	}
	ordinary := size(t, th, button(widget.ButtonProps{Label: "Apagar"}))
	destructive := size(t, th, button(widget.ButtonProps{Label: "Apagar", Variant: widget.Destructive}))

	if destructive.X <= ordinary.X {
		t.Errorf("the destructive button drew %v, no wider than the default %v", destructive, ordinary)
	}
}

// TestTheInnerPartsOfAnAlertAreReachable covers a widget with more than a root.
func TestTheInnerPartsOfAnAlertAreReachable(t *testing.T) {
	alert := func(parts theme.Parts) func(ayra.Context) ayra.Dimensions {
		return func(c ayra.Context) ayra.Dimensions {
			return widget.AlertProps{Title: "Falha", Body: "A leitura nao foi gravada.", ComponentProps: widget.ComponentProps{Parts: parts}}.Layout(c)
		}
	}
	base := size(t, same, alert(nil))

	for _, part := range []string{"title", "message"} {
		got := size(t, same, alert(theme.Parts{part: {Font: theme.Font{Size: theme.Pt(40)}}}))
		if got.Y <= base.Y {
			t.Errorf("a type size on the %s part left the alert at %v from %v", part, got, base)
		}
	}
	padded := size(t, same, alert(theme.Parts{"root": {Padding: theme.Uniform(40)}}))
	if padded.Y <= base.Y {
		t.Errorf("padding on the root part left the alert at %v from %v", padded, base)
	}
}

// TestTheThemesFacesReachTheText shows a widget that was never taught about a
// product's fonts drawing in them: the label asks for a role, and the scale
// says which face that is.
func TestTheThemesFacesReachTheText(t *testing.T) {
	label := func(c ayra.Context) ayra.Dimensions {
		return widget.TextProps{Content: "iiiiiiiiii WWWWWWWWWW"}.Layout(c)
	}
	ordinary := size(t, same, label)
	mono := size(t, func(th theme.Theme) theme.Theme {
		th.Type.Sans = "Go Mono"
		return th
	}, label)

	if mono == ordinary {
		t.Error("naming a monospace face as the sans one did not change the width of a line of text")
	}
}

// TestTheWeightsOfTheScaleAreRemapped does the same for the weights.
func TestTheWeightsOfTheScaleAreRemapped(t *testing.T) {
	bold := func(c ayra.Context) ayra.Dimensions {
		return widget.TextProps{Content: "Leituras de hoje", Bold: true}.Layout(c)
	}
	ordinary := size(t, same, bold)
	light := size(t, func(th theme.Theme) theme.Theme {
		th.Type.Weights.Bold = theme.Regular
		return th
	}, bold)

	if light.X >= ordinary.X {
		t.Errorf("mapping bold to regular drew %v, no narrower than bold %v", light, ordinary)
	}
}

// TestAWidgetOfTheProductsOwnIsWrittenFromTheSamePrimitives is the example from
// the documentation: a card for a camera, built from Surface, DrawText and a
// Clickable, restylable like the ones in this package.
func TestAWidgetOfTheProductsOwnIsWrittenFromTheSamePrimitives(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	c.Theme.Styles = &theme.Styles{Look: theme.Look{"cameracard": {"root": {Padding: theme.Uniform(20)}}}}

	var press widget.Clickable
	draw := func(c ayra.Context) ayra.Dimensions {
		root := c.Theme.Resolve("cameracard", "root", "", theme.Style{
			Background: theme.Colour(c.Theme.Colours.Card),
			Border:     theme.Colour(c.Theme.Colours.Border),
			Padding:    theme.Uniform(8),
		}, theme.Component{})
		return press.Layout(c, func(c ayra.Context) ayra.Dimensions {
			return widget.Surface(c, root, func(c ayra.Context) ayra.Dimensions {
				return widget.DrawText(c, "Portaria", theme.Style{Font: theme.Font{Weight: theme.SemiBold}}, widget.TextOptions{MaxLines: 1})
			})
		})
	}

	got := draw(c).Size
	if got.X <= 0 || got.Y <= 0 {
		t.Fatalf("the product's own widget took %v", got)
	}
	c.Theme.Styles = nil
	if smaller := draw(c).Size; smaller.Y >= got.Y {
		t.Errorf("the theme's padding for the product's widget was ignored: %v with it, %v without", got, smaller)
	}
}
