package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// TestAScreenCanPutTheFocusInAFieldWithoutATap: Kyse's autofocus. The screen
// that asks for the code it just sent calls Focus on the frame it first draws,
// and what is typed next lands in that field, with no tap to find it first.
func TestAScreenCanPutTheFocusInAFieldWithoutATap(t *testing.T) {
	var (
		input  widget.Input
		masked widget.Masked
		secret widget.Secret
		code   widget.OneTimeCode
	)
	cases := []struct {
		name  string
		focus func(ayra.Context)
		draw  func(ayra.Context) ayra.Dimensions
		held  func(ayra.Context) bool
		typed func() string
		want  string
	}{
		{
			"input", func(c ayra.Context) { input.Focus(c) },
			func(c ayra.Context) ayra.Dimensions { return widget.InputProps{}.Layout(c, &input) },
			func(c ayra.Context) bool { return input.Focused(c) },
			func() string { return input.Text() }, "abc",
		},
		{
			"masked", func(c ayra.Context) { masked.Focus(c) },
			func(c ayra.Context) ayra.Dimensions { return widget.MaskedProps{Mask: brazil.CEP}.Layout(c, &masked) },
			func(c ayra.Context) bool { return masked.Focused(c) },
			func() string { return masked.Text() }, "01310-100",
		},
		{
			"password", func(c ayra.Context) { secret.Focus(c) },
			func(c ayra.Context) ayra.Dimensions { return widget.PasswordProps{}.Layout(c, &secret) },
			func(c ayra.Context) bool { return secret.Focused(c) },
			func() string { return secret.Text() }, "abc",
		},
		{
			"one-time code", func(c ayra.Context) { code.Focus(c) },
			func(c ayra.Context) ayra.Dimensions { return widget.OneTimeCodeProps{}.Layout(c, &code) },
			func(c ayra.Context) bool { return code.Focused(c) },
			func() string { return code.Code() }, "123456",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var held bool
			first := true
			screen := func(c ayra.Context) ayra.Dimensions {
				return layout.UniformInset(16).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					if first {
						first = false
						tc.focus(inner)
					}
					d := tc.draw(inner)
					held = tc.held(inner)
					return d
				})
			}
			r := newRig(t, 300, 120, 2, theme.Light)
			r.settle(screen)
			if !held {
				t.Fatal("the field does not hold the focus after Focus, with no tap")
			}
			digits := tc.want
			if tc.name == "masked" {
				digits = "01310100"
			}
			r.typed(screen, digits)
			if got := tc.typed(); got != tc.want {
				t.Errorf("typing after Focus left %q, want %q", got, tc.want)
			}
		})
	}
}
