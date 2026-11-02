package frame_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// The overlays as a phone shows them: 390 points wide, and short once the
// keyboard is up. The pictures that were approved for them were drawn at 360
// wide with room to spare, which is where a dialog that touched both edges and
// one taller than the window both looked fine.

func TestTheOverlaysFitAPhone(t *testing.T) {
	for _, s := range []sample{
		{screen: screen{name: "dialog-phone", width: 390, height: 640, draw: dialogPhoneScreen}},
		{screen: screen{name: "dialog-phone-keyboard", width: 390, height: 440, draw: dialogTallScreen}},
	} {
		drawSample(t, s)
	}
}

// dialogPhoneScreen is a dialog that asks for its full width on a window that is
// not wide enough, with the sentence a person in Brazil would read.
func dialogPhoneScreen() ayra.Widget {
	var state widget.Dialog
	var confirm, cancel widget.Button
	state.Open()
	page := behind()
	return func(c ayra.Context) ayra.Dimensions {
		page(c)
		c.Constraints.Min = c.Constraints.Max
		return widget.DialogProps{Modal: true, Dismissible: true}.Layout(c, &state, func(c ayra.Context) ayra.Dimensions {
			return column(c, 340,
				line("Remover câmera", widget.Heading, true),
				spacer(6),
				line("Remover a câmera do portão principal e todas as leituras de placas associadas.", widget.Body, false),
				spacer(14),
				button(widget.ButtonProps{Label: "Remover câmera e leituras", Variant: widget.Destructive}, &confirm),
				spacer(8),
				button(widget.ButtonProps{Label: "Manter", Variant: widget.Outline}, &cancel),
			)
		})
	}
}

// dialogTallScreen is a dialog with more lines than a window with the keyboard
// up has room for.
func dialogTallScreen() ayra.Widget {
	var state widget.Dialog
	var accept widget.Button
	state.Open()
	return func(c ayra.Context) ayra.Dimensions {
		c.Constraints.Min = c.Constraints.Max
		return widget.DialogProps{Modal: true, Dismissible: true}.Layout(c, &state, func(c ayra.Context) ayra.Dimensions {
			items := []ayra.Widget{line("Termos de uso", widget.Heading, true)}
			for i := 0; i < 14; i++ {
				items = append(items, spacer(6), line("Parágrafo longo de texto para forçar a altura do diálogo além da tela.", widget.Body, false))
			}
			items = append(items, spacer(10), button(widget.ButtonProps{Label: "Aceitar"}, &accept))
			return column(c, 340, items...)
		})
	}
}
