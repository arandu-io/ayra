package frame_test

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// The widgets that had no picture: the ones that cover the screen or float over
// it, which a control at rest never shows, and a few of the quiet ones that were
// only ever drawn as part of another. Each is drawn in both schemes at both
// densities, with the panel open where there is a panel, because a closed
// Dialog is a picture of nothing.

// sample is a screen drawn under a theme of the product's own, which is how the
// locale reaches a widget.
type sample struct {
	screen
	locale theme.Locale
}

// options builds what frame.Draw is handed, with the sample's locale laid over
// the default theme.
func (s sample) options(scheme theme.Scheme, density float32) frame.Options {
	return frame.Options{
		Width: s.width, Height: s.height, Density: density, Scheme: scheme, Fonts: fonts(),
		Theme: func(scheme theme.Scheme) theme.Theme {
			th := theme.New(scheme)
			th.Locale = s.locale
			return th
		},
	}
}

func TestTheOverlaysAndTheQuietWidgetsLookRightInBothSchemes(t *testing.T) {
	for _, s := range []sample{
		{screen: screen{name: "parts", width: 360, height: 640, draw: partsScreen}},
		{screen: screen{name: "dialog", width: 360, height: 360, draw: dialogScreen}},
		{screen: screen{name: "drawer", width: 360, height: 420, draw: drawerScreen}},
		{screen: screen{name: "drawer-bottom", width: 360, height: 420, draw: drawerBottomScreen}},
		{screen: screen{name: "palette", width: 360, height: 420, draw: paletteScreen}},
		{screen: screen{name: "floating", width: 360, height: 420, draw: floatingScreen}},
		{screen: screen{name: "toast", width: 360, height: 300, draw: toastScreen}},
		{screen: screen{name: "triggers", width: 360, height: 360, draw: triggersScreen}},
	} {
		drawSample(t, s)
	}
}

// TestTheCalendarsSpeakPortuguese draws every widget that opens a calendar under
// the pt-BR locale of the theme: the month and the weekdays in Portuguese, the
// week starting on Sunday, and "Hoje", "Limpar", "Anterior" and "Próximo" on the
// controls. None of the Props carries a word of it.
func TestTheCalendarsSpeakPortuguese(t *testing.T) {
	pt := theme.PortugueseBrazil()
	for _, s := range []sample{
		{screen: screen{name: "calendar-pt-br", width: 360, height: 420, draw: calendarPortuguese}, locale: pt},
		{screen: screen{name: "datepicker-pt-br", width: 360, height: 460, draw: datePickerPortuguese}, locale: pt},
		{screen: screen{name: "daterange-pt-br", width: 380, height: 560, draw: dateRangePortuguese}, locale: pt},
		{screen: screen{name: "datetime-pt-br", width: 380, height: 640, draw: dateTimePortuguese}, locale: pt},
		{screen: screen{name: "datemonth-pt-br", width: 380, height: 520, draw: monthPortuguese}, locale: pt},
	} {
		drawSample(t, s)
	}
}

func drawSample(t *testing.T, s sample) {
	t.Helper()
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		for _, density := range []float32{1, 2} {
			name := pictureName(s.name, scheme, density)
			t.Run(name, func(t *testing.T) {
				got, err := frame.Draw(s.options(scheme, density), s.draw())
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

// partsScreen draws the three quiet ones: an accordion with a section open, the
// empty state and a figure with its caption.
func partsScreen() ayra.Widget {
	var sections widget.Accordion
	sections.Open(1)
	var shot widget.Image
	shot.Set(gradient(240, 90))

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Accordion", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.AccordionProps{Sections: []string{"Cameras", "Plates", "Billing"}}.
					Layout(c, &sections, func(index int) ayra.Widget {
						return line("Eleven cameras read at the main gate.", widget.Body, false)
					})
			},
			spacer(18),
			line("Empty", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.EmptyProps{Title: "No reads yet", Body: "A plate read by a camera shows up here."}.Layout(c)
			},
			spacer(18),
			line("Figure", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.FigureProps{Caption: "Main gate, 09:30", Bordered: true}.Layout(c, func(c ayra.Context) ayra.Dimensions {
					return widget.ImageProps{Width: 240, Height: 90}.Layout(c, &shot)
				})
			},
		)
	}
}

// behind is what a modal panel covers, so the picture shows the scrim over
// something and not over the bare ground.
func behind() ayra.Widget {
	var save, cancel widget.Button
	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			spacer(20),
			line("Gate cameras", widget.Heading, true),
			spacer(8),
			line("Eleven cameras read plates at the main gate.", widget.Body, false),
			spacer(14),
			button(widget.ButtonProps{Label: "Save"}, &save),
			spacer(8),
			button(widget.ButtonProps{Label: "Cancel", Variant: widget.Outline}, &cancel),
		)
	}
}

func dialogScreen() ayra.Widget {
	var state widget.Dialog
	var confirm, cancel widget.Button
	state.Open()
	page := behind()
	return func(c ayra.Context) ayra.Dimensions {
		page(c)
		c.Constraints.Min = c.Constraints.Max
		return widget.DialogProps{Modal: true, Dismissible: true, Width: 280}.Layout(c, &state, func(c ayra.Context) ayra.Dimensions {
			return column(c, 240,
				line("Remove camera", widget.Heading, true),
				spacer(6),
				line("The reads it made stay in the history.", widget.Body, false),
				spacer(14),
				button(widget.ButtonProps{Label: "Remove", Variant: widget.Destructive}, &confirm),
				spacer(8),
				button(widget.ButtonProps{Label: "Cancel", Variant: widget.Outline}, &cancel),
			)
		})
	}
}

func sheet(props widget.DrawerProps) func() ayra.Widget {
	return func() ayra.Widget {
		var state widget.Dialog
		var apply, clear widget.Button
		state.Open()
		page := behind()
		return func(c ayra.Context) ayra.Dimensions {
			page(c)
			return props.Layout(c, &state, func(c ayra.Context) ayra.Dimensions {
				return column(c, 300,
					line("Filters", widget.Heading, true),
					spacer(8),
					line("Show only the reads of one camera.", widget.Body, false),
					spacer(14),
					button(widget.ButtonProps{Label: "Apply"}, &apply),
					spacer(8),
					button(widget.ButtonProps{Label: "Clear", Variant: widget.Outline}, &clear),
				)
			})
		}
	}
}

func drawerScreen() ayra.Widget {
	return sheet(widget.DrawerProps{Modal: true, Dismissible: true, Size: 240})()
}

func drawerBottomScreen() ayra.Widget {
	return sheet(widget.DrawerProps{Side: widget.Bottom, Modal: true, Dismissible: true, Size: 240})()
}

func paletteScreen() ayra.Widget {
	var state widget.Palette
	state.Open()
	page := behind()
	return func(c ayra.Context) ayra.Dimensions {
		page(c)
		c.Constraints.Min = c.Constraints.Max
		return widget.PaletteProps{
			Commands:    []string{"Go to cameras", "Go to reads", "Add a camera", "Open billing"},
			Placeholder: "Type a command",
			Empty:       "No command matches",
		}.Layout(c, &state)
	}
}

// floatingScreen draws a menu of the bar open and a popover open, side by side
// in a column so neither covers the other.
func floatingScreen() ayra.Widget {
	var bar widget.Menubar
	bar.Open(0)
	var pop widget.Menu
	pop.Open()
	var trigger widget.Button

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			func(c ayra.Context) ayra.Dimensions {
				return widget.MenubarProps{
					Titles:  []string{"File", "Edit", "View"},
					Entries: [][]string{{"New camera", "Import", "Export"}, {"Undo", "Redo"}, {"Zoom in", "Zoom out"}},
				}.Layout(c, &bar)
			},
			spacer(150),
			func(c ayra.Context) ayra.Dimensions {
				return widget.PopoverProps{Width: 260}.Layout(c, &pop,
					func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Label: "Camera details", Variant: widget.Outline}.Layout(c, &trigger)
					},
					func(c ayra.Context) ayra.Dimensions {
						return line("Main gate. Reads at 8 frames a second.", widget.Body, false)(c)
					})
			},
		)
	}
}

func toastScreen() ayra.Widget {
	var toasts widget.Toasts
	toasts.Show(widget.Toast{Title: "Camera saved"})
	toasts.Show(widget.Toast{Title: "The stream dropped", Severity: widget.Failure})
	var hover widget.Hover
	hover.Hold(true)
	var info widget.Button

	return func(c ayra.Context) ayra.Dimensions {
		column(c, 320,
			spacer(30),
			func(c ayra.Context) ayra.Dimensions {
				return widget.TooltipProps{Text: "Reads per minute", Below: true}.Layout(c, &hover,
					func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Icon: icons.Gear(), Size: widget.Icon, Variant: widget.Outline}.Layout(c, &info)
					})
			},
		)
		return widget.ToastProps{Width: 300}.Layout(c, &toasts)
	}
}

// triggersScreen draws the button that opens a dialog and a picture that comes in
// two sizes. The picture loads on its own goroutine, so the screen waits for the
// redraw the load asks for: otherwise the second pass would race it.
func triggersScreen() ayra.Widget {
	var dialog widget.Dialog
	var trigger, iconOnly widget.Button
	var responsive widget.ResponsiveImage

	loaded := make(chan struct{})
	source := func(w, h int) widget.ImageSource {
		return widget.ImageSourceFunc(func(context.Context) (image.Image, error) { return gradient(w, h), nil })
	}
	waited := false

	return func(c ayra.Context) ayra.Dimensions {
		if !waited {
			once := false
			c.Invalidate = func() {
				if !once {
					once = true
					close(loaded)
				}
			}
		}
		dims := column(c, 320,
			line("Triggers", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.DialogTriggerProps{Label: "Add camera", Icon: icons.Camera(), Dialog: &dialog}.Layout(c.With(gtx), &trigger)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.DialogTriggerProps{Icon: icons.Gear(), Size: widget.Icon, Variant: widget.Outline, Dialog: &dialog}.Layout(c.With(gtx), &iconOnly)
					}),
				)
			},
			spacer(16),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ResponsiveImageProps{
					Variants: []widget.ImageVariant{{Width: 160, Source: source(160, 60)}, {Width: 640, Source: source(640, 240)}},
					Width:    300, Height: 110, Alt: "Main gate",
				}.Layout(c, &responsive)
			},
		)
		if !waited {
			waited = true
			select {
			case <-loaded:
			case <-time.After(5 * time.Second):
			}
		}
		return dims
	}
}

func calendarPortuguese() ayra.Widget {
	var days widget.Calendar
	days.Select(reference.AddDate(0, 0, 4))
	return top(func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return line("Calendário", widget.Heading, true)(c.With(gtx))
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.CalendarProps{Today: reference}.Layout(c.With(gtx), &days)
			}),
		)
	})
}

func datePickerPortuguese() ayra.Widget {
	var open, closed widget.DatePicker
	open.Select(reference)
	open.Open()
	return top(func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.DatePickerProps{Today: reference}.Layout(c.With(gtx), &closed)
			}),
			layout.Rigid(layout.Spacer{Height: 10}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.DatePickerProps{Today: reference}.Layout(c.With(gtx), &open)
			}),
		)
	})
}

func dateRangePortuguese() ayra.Widget {
	var empty widget.DateRange
	empty.Open()
	return top(func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.DateRangePickerProps{
					Today: reference, DateFormat: "02/01/2006",
					Presets: []widget.DatePreset{widget.LastDays("Hoje", reference, 1), widget.LastDays("7 dias", reference, 7), widget.ThisMonth("Este mês", reference)},
				}.Layout(c.With(gtx), &empty)
			}),
		)
	})
}

func dateTimePortuguese() ayra.Widget {
	var state widget.DateTime
	state.Select(reference, false)
	state.Open()
	return top(func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.DateTimePickerProps{Today: reference}.Layout(c.With(gtx), &state)
			}),
		)
	})
}

func monthPortuguese() ayra.Widget {
	var state widget.DateTime
	state.Select(reference, false)
	state.Open()
	return top(func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.DateTimePickerProps{Today: reference, Kind: widget.MonthOnly}.Layout(c.With(gtx), &state)
			}),
		)
	})
}
