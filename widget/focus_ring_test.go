package widget_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// TestAFieldThatHoldsTheKeyboardIsRingedAsKyseRingsIt: the focus was a border
// that went from light grey to a slightly darker one, which on a phone is no
// focus at all. Kyse puts a ring of three points around the field, outside its
// border, and every field here does the same.
func TestAFieldThatHoldsTheKeyboardIsRingedAsKyseRingsIt(t *testing.T) {
	var (
		input    widget.Input
		area     widget.Input
		masked   widget.Masked
		secret   widget.Secret
		code     widget.OneTimeCode
		suggest  widget.Autocomplete
		combo    widget.Combobox
		grouped  widget.Input
		withRing bool
	)
	cases := []struct {
		name string
		draw func(c ayra.Context) ayra.Dimensions
	}{
		{"input", func(c ayra.Context) ayra.Dimensions { return widget.InputProps{}.Layout(c, &input) }},
		{"textarea", func(c ayra.Context) ayra.Dimensions { return widget.TextareaProps{}.Layout(c, &area) }},
		{"masked", func(c ayra.Context) ayra.Dimensions { return widget.MaskedProps{Mask: brazil.CPF}.Layout(c, &masked) }},
		{"password", func(c ayra.Context) ayra.Dimensions { return widget.PasswordProps{}.Layout(c, &secret) }},
		{"onetimecode", func(c ayra.Context) ayra.Dimensions { return widget.OneTimeCodeProps{}.Layout(c, &code) }},
		{"suggest", func(c ayra.Context) ayra.Dimensions {
			return widget.AutocompleteProps{Entries: []string{"a"}}.Layout(c, &suggest)
		}},
		{"combobox", func(c ayra.Context) ayra.Dimensions { return widget.ComboboxProps{}.Layout(c, &combo) }},
		{"input group", func(c ayra.Context) ayra.Dimensions {
			return widget.InputGroupProps{Focused: grouped.Focused(c)}.Layout(c, func(c ayra.Context) ayra.Dimensions {
				return widget.InputProps{ComponentProps: widget.GroupedParts()}.Layout(c, &grouped)
			})
		}},
	}
	_ = withRing
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		for _, tc := range cases {
			t.Run(tc.name+"/"+scheme.String(), func(t *testing.T) {
				screen := func(c ayra.Context) ayra.Dimensions {
					return layout.UniformInset(16).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min = image.Point{}
						return tc.draw(c.With(gtx))
					})
				}
				r := newRig(t, 300, 120, 2, scheme)
				ground := theme.New(scheme).Colours.Background

				outside := func() color.RGBA {
					r.settle(screen)
					if err := r.win.Frame(&r.ops); err != nil {
						t.Fatal(err)
					}
					img := image.NewRGBA(image.Rectangle{Max: r.size})
					if err := r.win.Screenshot(img); err != nil {
						t.Fatal(err)
					}
					// Two pixels left of the field's edge, at the middle of its first line.
					return img.At(30, 70).(color.RGBA)
				}
				before := outside()
				if before != (color.RGBA{R: ground.R, G: ground.G, B: ground.B, A: 255}) {
					t.Fatalf("outside the unfocused field is %v, not the ground %v", before, ground)
				}
				r.tap(screen, 100, 30)
				after := outside()
				if after == before {
					t.Errorf("outside the focused field is still %v: there is no ring", after)
				}
			})
		}
	}
}
