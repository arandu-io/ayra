package frame_test

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/theme/css"
	"github.com/arandu-io/ayra/widget"
)

// A product that has its own look is a product whose pictures are its own.
//
// The screens in golden_test.go draw the default theme, which is what keeps a
// widget from drifting. This draws the same widgets under a theme made from a
// product stylesheet -- the tokens of the Alpr.sh panel: paper #f4f4f4, a white
// card, graphite #222 for the primary, #d6d6d6 for the rule, a six-point
// radius, and white-on-black in the dark scheme -- with a look and three
// variants of the product's own on top, and two widgets the product wrote
// itself from the exported primitives.

// productTheme reads the product's stylesheet and registers what a stylesheet
// cannot say.
func productTheme(t *testing.T) func(theme.Scheme) theme.Theme {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "theme", "css", "testdata", "product.css"))
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := css.Parse(string(body))
	if err != nil {
		t.Fatal(err)
	}

	return func(scheme theme.Scheme) theme.Theme {
		th := sheet.Theme(scheme, css.Options{TokenPrefixes: []string{"tone-"}})

		read, act, in := th.Token("tone-read"), th.Token("tone-act"), th.Token("tone-in")
		on := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		if scheme == theme.Dark {
			on = color.NRGBA{A: 255}
		}

		if th.Styles == nil {
			th.Styles = &theme.Styles{}
		}

		// The look of the whole application: a squarer corner than the
		// default, on the widgets that have a corner.
		th.Styles.Look = theme.Look{
			"button": {"root": {Radius: theme.Pt(2)}},
			"card":   {"root": {Radius: theme.Pt(2)}},
			"input":  {"root": {Radius: theme.Pt(2)}},
		}

		// Three variants of the product's own: blue where a frame comes in,
		// green where a plate is read, red where it turns into an action.
		th.Styles.Variants = map[string]theme.Look{
			"in": {
				"button": {"root": {Background: theme.Colour(in), Foreground: theme.Colour(on)}},
				"badge":  {"root": {Background: theme.Colour(in), Foreground: theme.Colour(on), Border: theme.Transparent}},
			},
			"read": {
				"badge":  {"root": {Background: theme.Colour(read), Foreground: theme.Colour(on), Border: theme.Transparent}},
				"status": {"dot": {Background: theme.Colour(read)}},
			},
			"alert": {
				"badge":  {"root": {Background: theme.Colour(act), Foreground: theme.Colour(on), Border: theme.Transparent}},
				"button": {"root": {Background: theme.Colour(act), Foreground: theme.Colour(on)}},
				"alert": {
					"root":  {Background: theme.Colour(color.NRGBA{R: act.R, G: act.G, B: act.B, A: 26}), Border: theme.Colour(act)},
					"title": {Foreground: theme.Colour(act)},
				},
			},
		}
		return th
	}
}

// plate is a widget of the product's own, written from the exported
// primitives: the licence plate of a read, in the monospace face the stylesheet
// names, on a surface the theme can restyle by the name "plate".
func plate(c ayra.Context, text string) ayra.Dimensions {
	root := c.Theme.Resolve("plate", "root", "", theme.Style{
		Background:  theme.Colour(c.Theme.Colours.Card),
		Foreground:  theme.Colour(c.Theme.Colours.Foreground),
		Border:      theme.Colour(c.Theme.Colours.Foreground),
		BorderWidth: theme.Pt(2),
		Radius:      theme.Pt(4),
		Padding:     theme.InsetXY(10, 4),
		Font:        theme.Font{Weight: theme.Bold, Size: theme.Pt(18)},
	}, theme.Component{})

	return widget.Surface(c, root, func(c ayra.Context) ayra.Dimensions {
		return widget.DrawText(c, text, root, widget.TextOptions{MaxLines: 1, Monospace: true})
	})
}

// cameraCard is the second: a pressable card with a name, a status and the
// last plate read.
func cameraCard(c ayra.Context, press *widget.Clickable, name, plateText string) ayra.Dimensions {
	root := c.Theme.Resolve("cameracard", "root", "", theme.Style{
		Background: theme.Colour(c.Theme.Colours.Card),
		Border:     theme.Colour(c.Theme.Colours.Border),
		Radius:     theme.Pt(c.Theme.Metrics.Radius),
		Padding:    theme.Uniform(12),
	}, theme.Component{})
	if press.Hovered() {
		root.Background = theme.Colour(widget.Press(root.Background.NRGBA(), false))
	}

	return press.Layout(c, func(c ayra.Context) ayra.Dimensions {
		return widget.Surface(c, root, func(c ayra.Context) ayra.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.DrawText(c.With(gtx), name, theme.Style{Font: theme.Font{Weight: theme.SemiBold}}, widget.TextOptions{MaxLines: 1})
				}),
				layout.Rigid(layout.Spacer{Height: 4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.StatusProps{Label: "Lendo", ComponentProps: widget.ComponentProps{Custom: "read"}}.Layout(c.With(gtx))
				}),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return plate(c.With(gtx), plateText)
				}),
			)
		})
	})
}

func productPanel() ayra.Widget {
	var save, remove widget.Button
	var press widget.Clickable
	var field widget.Input
	field.SetText("Portaria")

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Cameras", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return cameraCard(c, &press, "Portaria", "ABC1D23")
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.InputProps{Placeholder: "Nome da camera"}.Layout(c, &field)
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.ButtonProps{Label: "Salvar", ComponentProps: widget.ComponentProps{Custom: "in"}}.Layout(c.With(gtx), &save)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.ButtonProps{Label: "Remover", Variant: widget.Outline, ComponentProps: widget.ComponentProps{Custom: "alert"}}.Layout(c.With(gtx), &remove)
					}),
				)
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.BadgeProps{Label: "Lida", ComponentProps: widget.ComponentProps{Custom: "read"}}.Layout(c.With(gtx))
					}),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.BadgeProps{Label: "Alerta", ComponentProps: widget.ComponentProps{Custom: "alert"}}.Layout(c.With(gtx))
					}),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.BadgeProps{Label: "Entrada", ComponentProps: widget.ComponentProps{Custom: "in"}}.Layout(c.With(gtx))
					}),
				)
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.AlertProps{
					Title: "Placa invalida",
					Body:  "A leitura nao tem o formato de uma placa e nao foi gravada.",
					ComponentProps: widget.ComponentProps{Custom: "alert", Parts: theme.Parts{
						"title": {Font: theme.Font{Weight: theme.Bold}},
					}},
				}.Layout(c)
			},
		)
	}
}

// TestAProductThemeLooksLikeTheProduct keeps the product's pictures, in both
// schemes at two densities, the way the default screens are kept.
func TestAProductThemeLooksLikeTheProduct(t *testing.T) {
	th := productTheme(t)

	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		for _, density := range []float32{1, 2} {
			name := pictureName("product", scheme, density)

			t.Run(name, func(t *testing.T) {
				got, err := frame.Draw(frame.Options{
					Width: 360, Height: 560, Density: density, Scheme: scheme,
					Fonts: fonts(), Theme: th,
				}, productPanel())
				if err != nil {
					if noSurface(err) {
						t.Skipf("no drawing surface on this machine: %v", err)
					}
					t.Fatalf("the screen did not draw: %v", err)
				}
				compare(t, name, got)
			})
		}
	}
}

// TestTheProductThemeIsTheStylesheetsAndNotTheDefault guards the picture
// against passing for the wrong reason: a golden approved with the default
// palette would still compare equal to itself.
func TestTheProductThemeIsTheStylesheetsAndNotTheDefault(t *testing.T) {
	light, dark := productTheme(t)(theme.Light), productTheme(t)(theme.Dark)

	want := color.NRGBA{R: 0xf4, G: 0xf4, B: 0xf4, A: 255}
	if light.Colours.Background != want {
		t.Errorf("the light background is %+v, want the stylesheet's %+v", light.Colours.Background, want)
	}
	if dark.Colours.Card != (color.NRGBA{R: 0x11, G: 0x11, B: 0x11, A: 255}) {
		t.Errorf("the dark card is %+v, want #111111", dark.Colours.Card)
	}
	if light.Metrics.Radius != 6 {
		t.Errorf("the radius is %v, want 6", light.Metrics.Radius)
	}
}
