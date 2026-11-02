package frame_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/icons"
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
		{screen: screen{name: "menu-phone", width: 390, height: 300, draw: menuRowScreen}},
		{screen: screen{name: "popover-phone", width: 390, height: 260, draw: popoverRowScreen}},
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

// menuRowScreen is the "..." at the end of a row on a phone, with the lines that
// follow it: the list opens against the right edge, over them, and not off the
// screen behind them.
func menuRowScreen() ayra.Widget {
	var menu widget.Menu
	menu.Open()
	var trigger, below widget.Button
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 16, Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return line("Câmera do portão principal", widget.Body, true)(inner.With(gtx))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return widget.MenuProps{Entries: []string{"Editar", "Duplicar câmera", "", "Excluir câmera e leituras"}, End: true}.
								Layout(inner.With(gtx), &menu, func(c ayra.Context) ayra.Dimensions {
									return widget.ButtonProps{Icon: icons.DotsThree(), IconOnly: true, Size: widget.Icon, Variant: widget.Ghost, Label: "Mais opções"}.Layout(c, &trigger)
								})
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					g := inner.With(gtx)
					g.Constraints.Min.X = g.Constraints.Max.X
					return widget.ButtonProps{Label: "Abrir a câmera"}.Layout(g, &below)
				}),
			)
		})
	}
}

// popoverRowScreen is a popover at the right edge, opening against it.
func popoverRowScreen() ayra.Widget {
	var state widget.Menu
	state.Open()
	var trigger widget.Button
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 16, Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			inner := c.With(gtx)
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.PopoverProps{Width: 280, End: true}.Layout(inner.With(gtx), &state,
						func(c ayra.Context) ayra.Dimensions {
							return widget.ButtonProps{Label: "Detalhes", Variant: widget.Outline}.Layout(c, &trigger)
						},
						func(c ayra.Context) ayra.Dimensions {
							return line("Portão principal. Leituras a 8 quadros por segundo.", widget.Body, false)(c)
						})
				}),
			)
		})
	}
}
