package frame_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// The on-screen keyboard at the size of the kit's screen, 1280 by 800, under the
// sign-in field it types into, and at the width of a phone. The page of symbols
// and the capitals on are the other two states a person sees. (That the keys
// type, and where the focus goes, is proved in widget/onscreen_keyboard_test.go:
// a picture has no way to press a key.)

// keyboardScreen is a field with its keyboard at the foot of the window, the way
// the kit's app lays it out: the content takes what the keyboard leaves.
func keyboardScreen(symbols bool, capitals int) func() ayra.Widget {
	return func() ayra.Widget {
		var email widget.Input
		var keys widget.Keyboard
		email.SetText("paulo@exemplo.com.br")
		if symbols {
			// The page key is a press target; the picture is of the state after it.
			keys.ShowSymbols(true)
		}
		switch capitals {
		case 1:
			keys.Capitalise(false)
		case 2:
			keys.Capitalise(true)
		}
		return func(c ayra.Context) ayra.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return column(c.With(gtx), 420,
						spacer(80),
						line("Entrar", widget.Heading, true),
						spacer(16),
						field(widget.InputProps{Placeholder: "E-mail", Kind: widget.Email}, &email),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.KeyboardProps{Labels: widget.KeyboardLabels{Space: "espaço", Close: "Fechar o teclado", Erase: "Apagar", Shift: "Maiúscula"}}.Layout(c.With(gtx), &keys)
				}),
			)
		}
	}
}

func keyboardPicture(t *testing.T, name string, width, height int, densities []float32, screen func() ayra.Widget) {
	t.Helper()
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		for _, density := range densities {
			picture := pictureName(name, scheme, density)
			t.Run(picture, func(t *testing.T) {
				got, err := frame.Draw(frame.Options{Width: width, Height: height, Density: density, Scheme: scheme, Fonts: fonts()}, screen())
				if err != nil {
					if noSurface(err) {
						t.Skipf("no drawing surface on this machine: %v", err)
					}
					t.Fatalf("the screen did not draw: %v", err)
				}
				compare(t, picture, got)
			})
		}
	}
}

func TestTheOnScreenKeyboardLooksLikeItDid(t *testing.T) {
	both := []float32{1}
	t.Run("kit", func(t *testing.T) { keyboardPicture(t, "keyboard-kit", 1280, 800, both, keyboardScreen(false, 0)) })
	t.Run("kit-symbols", func(t *testing.T) {
		keyboardPicture(t, "keyboard-kit-symbols", 1280, 800, both, keyboardScreen(true, 0))
	})
	t.Run("kit-capitals", func(t *testing.T) {
		keyboardPicture(t, "keyboard-kit-capitals", 1280, 800, both, keyboardScreen(false, 2))
	})
	t.Run("phone", func(t *testing.T) {
		keyboardPicture(t, "keyboard-phone", 360, 560, []float32{1, 2}, keyboardScreen(false, 1))
	})
}
