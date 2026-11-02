package frame_test

import (
	"image"
	"strconv"
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
		{screen: screen{name: "dialog-over-bars", width: 390, height: 640, draw: dialogOverBars}},
		{screen: screen{name: "drawer-over-bars", width: 390, height: 640, draw: drawerOverBars}},
		{screen: screen{name: "tabs-phone", width: 390, height: 260, draw: tabsPhone}},
		{screen: screen{name: "menu-flip-phone", width: 390, height: 520, draw: menuAtTheFootOfAForm}},
		{screen: screen{name: "menu-capped-phone", width: 390, height: 400, draw: menuLongerThanTheRoom}},
		{screen: screen{name: "disclosure-phone", width: 390, height: 400, draw: disclosurePhone}},
		{screen: screen{name: "carousel-phone", width: 390, height: 260, draw: carouselPhone}},
		{screen: screen{name: "navigation-phone", width: 390, height: 360, draw: navigationPhone}},
		{screen: screen{name: "tooltip-phone", width: 390, height: 160, draw: tooltipEdges}},
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

// appFrame draws what an application is wrapped in -- a bar across the top and a
// tab bar across the bottom -- around a card, and hands the card a box 120 points
// high to draw its overlay in. The overlay is called from inside the card, as it
// is from a row of a list.
func appFrame(overlay func(c ayra.Context)) ayra.Widget {
	var tabs widget.Tabs
	return func(c ayra.Context) ayra.Dimensions {
		c.Constraints.Min = image.Point{}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 14, Bottom: 14, Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return line("Câmeras", widget.Heading, true)(c.With(gtx))
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				layout.Inset{Left: 16, Right: 16, Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.Y = 120
					inner := c.With(gtx)
					line("Portão principal: 8 leituras por minuto.", widget.Body, false)(inner)
					overlay(inner)
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 120)}
				})
				return layout.Dimensions{Size: size}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return widget.TabsProps{Labels: []string{"Início", "Câmeras", "Leituras"}}.Layout(c.With(gtx), &tabs)
			}),
		)
	}
}

// dialogOverBars is a dialog opened from inside a card: its scrim covers the bar
// across the top and the tab bar across the bottom as well.
func dialogOverBars() ayra.Widget {
	var state widget.Dialog
	var confirm, cancel widget.Button
	state.Open()
	return appFrame(func(c ayra.Context) {
		widget.DialogProps{Modal: true, Dismissible: true}.Layout(c, &state, func(c ayra.Context) ayra.Dimensions {
			return column(c, 300,
				line("Excluir câmera", widget.Heading, true),
				spacer(6),
				line("As leituras dela continuam no histórico.", widget.Body, false),
				spacer(14),
				button(widget.ButtonProps{Label: "Excluir", Variant: widget.Destructive}, &confirm),
				spacer(8),
				button(widget.ButtonProps{Label: "Manter", Variant: widget.Outline}, &cancel),
			)
		})
	})
}

// drawerOverBars is a sheet of filters opened from inside a card.
func drawerOverBars() ayra.Widget {
	var state widget.Dialog
	var apply, clear widget.Button
	state.Open()
	return appFrame(func(c ayra.Context) {
		widget.DrawerProps{Side: widget.Bottom, Size: 260, Modal: true, Dismissible: true}.Layout(c, &state, func(c ayra.Context) ayra.Dimensions {
			return column(c, 340,
				line("Filtros", widget.Heading, true),
				spacer(8),
				line("Mostrar só as leituras de uma câmera.", widget.Body, false),
				spacer(14),
				button(widget.ButtonProps{Label: "Aplicar"}, &apply),
				spacer(8),
				button(widget.ButtonProps{Label: "Limpar", Variant: widget.Outline}, &clear),
			)
		})
	})
}

// tabsPhone is three rows of tabs on a phone: a few short ones, five with long
// words that scroll and the last of them selected, and three with icons.
func tabsPhone() ayra.Widget {
	var few, many, iconic widget.Tabs
	many.Select(4)
	iconic.Select(1)
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 16, Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.TabsProps{Labels: []string{"Detalhes", "Atividade", "Permissões"}}.Layout(inner.With(gtx), &few)
				}),
				layout.Rigid(layout.Spacer{Height: 16}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.TabsProps{Labels: []string{"Visão geral das câmeras", "Leituras de placas do dia", "Configurações avançadas do motor", "Faturamento", "Integrações com outros sistemas"}}.Layout(inner.With(gtx), &many)
				}),
				layout.Rigid(layout.Spacer{Height: 16}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.TabsProps{Labels: []string{"Câmeras", "Leituras", "Alertas"}, Icons: []icons.Icon{icons.Camera(), icons.Gear(), icons.Camera()}}.Layout(inner.With(gtx), &iconic)
				}),
			)
		})
	}
}

// menuAtTheFootOfAForm is the last field of a form of 390 points, a list of
// choices under a control with no room under it: it opens over it.
func menuAtTheFootOfAForm() ayra.Widget {
	var plate, owner widget.Input
	var menu widget.Menu
	menu.Open()
	var trigger widget.Button
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 16, Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			inner := c.With(gtx)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.FieldProps{Label: "Placa"}.Layout(inner.With(gtx), func(c ayra.Context) ayra.Dimensions {
						return widget.InputProps{Placeholder: "ABC1D23"}.Layout(c, &plate)
					})
				}),
				layout.Rigid(layout.Spacer{Height: 12}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.FieldProps{Label: "Proprietário"}.Layout(inner.With(gtx), func(c ayra.Context) ayra.Dimensions {
						return widget.InputProps{Placeholder: "Nome"}.Layout(c, &owner)
					})
				}),
				layout.Rigid(layout.Spacer{Height: 260}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					g := inner.With(gtx)
					g.Constraints.Min.X = g.Constraints.Max.X
					return widget.MenuProps{Entries: []string{"Portão principal", "Garagem do subsolo", "Entrada de serviço", "Doca de carga"}, Width: 358}.
						Layout(g, &menu, func(c ayra.Context) ayra.Dimensions {
							return widget.ButtonProps{Label: "Câmera", Variant: widget.Outline}.Layout(c, &trigger)
						})
				}),
			)
		})
	}
}

// menuLongerThanTheRoom is forty entries under a control near the top of a short
// window: the list is held to the window and scrolls.
func menuLongerThanTheRoom() ayra.Widget {
	var menu widget.Menu
	menu.Open()
	var trigger widget.Button
	entries := make([]string, 40)
	for i := range entries {
		entries[i] = "Câmera " + strconv.Itoa(i+1)
	}
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 16, Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return widget.MenuProps{Entries: entries, Width: 220}.Layout(c.With(gtx), &menu, func(c ayra.Context) ayra.Dimensions {
				return widget.ButtonProps{Label: "Câmeras", Variant: widget.Outline}.Layout(c, &trigger)
			})
		})
	}
}

// disclosurePhone is a collapsible and an accordion whose summaries are longer
// than a line of a phone.
func disclosurePhone() ayra.Widget {
	var open, closed widget.Disclosure
	open.Open()
	var acc widget.Accordion
	acc.Open(1)
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 16, Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			inner := c.With(gtx)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.CollapsibleProps{Summary: "Configurações avançadas da câmera do portão principal da garagem"}.
						Layout(inner.With(gtx), &open, line("As leituras continuam no histórico.", widget.Body, false))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.CollapsibleProps{Summary: "Fechada"}.Layout(inner.With(gtx), &closed, line("x", widget.Body, false))
				}),
				layout.Rigid(layout.Spacer{Height: 16}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.AccordionProps{Sections: []string{"Câmeras do portão principal", "Leituras de placas dos últimos trinta dias", "Faturamento e cobrança pelo Peráta da organização"}}.
						Layout(inner.With(gtx), &acc, func(int) ayra.Widget { return line("Onze câmeras leem placas no portão.", widget.Body, false) })
				}),
			)
		})
	}
}

// carouselPhone is a carousel with its steps on a phone: arrows, not words, and
// the slide left the room.
func carouselPhone() ayra.Widget {
	var one widget.Carousel
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 16, Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return widget.CarouselProps{Count: 4, Arrows: true, Dots: true, PreviousLabel: "Anterior", NextLabel: "Próxima"}.
				Layout(c.With(gtx), &one, func(c ayra.Context, i int) ayra.Dimensions {
					return line("Portão principal, leituras ao vivo", widget.Heading, true)(c)
				})
		})
	}
}

// navigationPhone is a trail longer than a phone over a sidebar whose current
// entry is marked.
func navigationPhone() ayra.Widget {
	var crumbs widget.Crumbs
	var sidebar widget.Sidebar
	sidebar.Show(2)
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 16, Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			inner := c.With(gtx)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.BreadcrumbProps{Steps: []string{"Organização Condomínio Jardins", "Câmeras do portão principal", "Configurações avançadas", "Leituras"}}.
						Layout(inner.With(gtx), &crumbs)
				}),
				layout.Rigid(layout.Spacer{Height: 12}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.SidebarProps{Title: "Espaço de trabalho", Entries: []string{"Visão geral", "Câmeras", "Leituras do dia de hoje", "Configurações da organização e integrações"}, Width: 358}.
						Layout(inner.With(gtx), &sidebar)
				}),
			)
		})
	}
}

// tooltipEdges is a tooltip on a control at each edge of a phone: each is kept inside the window.
func tooltipEdges() ayra.Widget {
	var left, right widget.Hover
	left.Hold(true)
	right.Hold(true)
	var a, b widget.Button
	tip := "Leituras por minuto da câmera do portão principal"
	row := func(at layout.Direction, state *widget.Hover, button *widget.Button) func(ayra.Context) ayra.Dimensions {
		return func(c ayra.Context) ayra.Dimensions {
			return at.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				return widget.TooltipProps{Text: tip, Below: true}.Layout(c.With(gtx), state, func(c ayra.Context) ayra.Dimensions {
					return widget.ButtonProps{Icon: icons.Gear(), Size: widget.Icon, Variant: widget.Outline}.Layout(c, button)
				})
			})
		}
	}
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 12, Left: 4, Right: 4}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			inner := c.With(gtx)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return row(layout.W, &left, &a)(inner.With(gtx))
				}),
				layout.Rigid(layout.Spacer{Height: 52}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return row(layout.E, &right, &b)(inner.With(gtx))
				}),
			)
		})
	}
}
