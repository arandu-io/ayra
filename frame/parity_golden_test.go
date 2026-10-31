package frame_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// Every widget that arrived with the parity work is drawn in both schemes at two
// densities, the way the others are. A widget whose colours were right in one
// scheme and written as a fixed value would be right in the light picture and
// wrong in the dark, and the dark picture is what shows it.

var reference = time.Date(2026, time.September, 10, 9, 30, 0, 0, time.UTC)

// the whole sample of the icon set the screens draw.
func iconSet() []icons.Icon {
	return []icons.Icon{
		icons.Trash(), icons.Gear(), icons.Camera(), icons.MagnifyingGlass(), icons.Check(), icons.Warning(),
		icons.Car(), icons.Bell(), icons.ArrowRight(), icons.CloudArrowUp(), icons.Funnel(), icons.House(),
		icons.User(), icons.Lock(), icons.Eye(), icons.Pencil(), icons.Plus(), icons.Minus(),
		icons.X(), icons.Star(), icons.Heart(), icons.Clock(), icons.Calendar(), icons.Envelope(),
		icons.Phone(), icons.MapPin(), icons.Download(), icons.Upload(), icons.Link(), icons.Copy(),
		icons.Share(), icons.Info(), icons.Question(), icons.Moon(), icons.Sun(), icons.WifiHigh(),
		icons.Lightning(), icons.Globe(), icons.Key(), icons.Shield(), icons.Tag(), icons.Folder(),
		icons.File(), icons.Image(), icons.Play(), icons.Pause(), icons.List(), icons.DotsThree(),
	}
}

func iconGrid() ayra.Widget {
	set := iconSet()
	return func(c ayra.Context) ayra.Dimensions {
		rows := []ayra.Widget{line("Icons", widget.Heading, true), spacer(8)}
		for start := 0; start < len(set); start += 8 {
			group := set[start:min(start+8, len(set))]
			rows = append(rows, func(c ayra.Context) ayra.Dimensions { return iconRow(c, group...) }, spacer(8))
		}
		return column(c, 320, rows...)
	}
}

func iconUse() ayra.Widget {
	var a, b, c2, d, e, f, g widget.Button
	var first, second widget.Tabs
	second.Select(1)

	return func(c ayra.Context) ayra.Dimensions {
		tabs := func(state *widget.Tabs) ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				return widget.TabsProps{
					Labels: []string{"Reads", "Cameras", "Alerts"},
					Icons:  []icons.Icon{icons.Car(), icons.Camera(), icons.Bell()},
				}.Layout(c, state)
			}
		}
		button := func(label string, v widget.Variant, icon icons.Icon, state *widget.Button) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: 8, Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min = layout.Constraints{}.Min
					return widget.ButtonProps{Label: label, Variant: v, Icon: icon}.Layout(inner, state)
				})
			})
		}
		return column(c, 330,
			line("Buttons", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
					button("Save", widget.Default, icons.Check(), &a),
					button("Edit", widget.Secondary, icons.Pencil(), &b),
					button("Filter", widget.Outline, icons.Funnel(), &c2),
				)
			},
			func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
					button("Remove", widget.Destructive, icons.Trash(), &d),
					button("Back", widget.Ghost, icons.ArrowRight(), &e),
					button("Docs", widget.Link, icons.Info(), &f),
				)
			},
			spacer(8),
			line("A button in a page header", widget.Caption, false),
			spacer(4),
			func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return widget.TextProps{Content: "Cameras", Role: widget.Heading, Bold: true}.Layout(c.With(gtx))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						inner := c.With(gtx)
						inner.Constraints.Min = layout.Constraints{}.Min
						return widget.ButtonProps{Label: "Add camera", Icon: icons.Plus(), Variant: widget.Outline, Size: widget.Small}.Layout(inner, &g)
					}),
				)
			},
			spacer(14),
			tabs(&first),
			spacer(10),
			tabs(&second),
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.IconProps{Icon: icons.Car(), Size: 24, Tone: widget.Normal}.Layout(c.With(gtx))
					}),
					layout.Rigid(layout.Spacer{Width: 10}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.IconProps{Icon: icons.Car(), Size: 24, Tone: widget.Muted}.Layout(c.With(gtx))
					}),
					layout.Rigid(layout.Spacer{Width: 10}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.IconProps{Icon: icons.Warning(), Size: 24, Tone: widget.Danger}.Layout(c.With(gtx))
					}),
					layout.Rigid(layout.Spacer{Width: 10}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.IconProps{Icon: icons.Star(), Size: 24, Tone: widget.Accent}.Layout(c.With(gtx))
					}),
				)
			},
		)
	}
}

// top draws a screen from the top of the picture instead of the middle of it: a
// panel that opens below its field is drawn outside what the field measures, so a
// column centred on the field would put the panel off the bottom of the picture.
func top(content func(ayra.Context) ayra.Dimensions) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 24, Left: 20, Right: 20}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return content(c.With(gtx))
		})
	}
}

func rangeOpen() ayra.Widget {
	var state widget.DateRange
	state.Set(reference.AddDate(0, 0, -6), reference)
	state.Open()
	return top(func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return line("Period", widget.Heading, true)(c.With(gtx)) }),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.DateRangePickerProps{
					Today: reference, Monday: true, DateFormat: "02/01/2006",
					Presets: []widget.DatePreset{
						widget.LastDays("Today", reference, 1), widget.LastDays("Last 7 days", reference, 7),
						widget.ThisMonth("This month", reference), widget.PreviousMonth("Last month", reference),
					},
				}.Layout(c.With(gtx), &state)
			}),
		)
	})
}

func timeOpen(kind widget.DateTimeKind) func() ayra.Widget {
	return func() ayra.Widget {
		var state widget.DateTime
		props := widget.DateTimePickerProps{Today: reference, Monday: true, Kind: kind}
		state.Select(reference, false)
		state.Open()
		return top(func(c ayra.Context) ayra.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return line("Moment", widget.Heading, true)(c.With(gtx)) }),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return props.Layout(c.With(gtx), &state) }),
			)
		})
	}
}

func formOpen() ayra.Widget {
	var state widget.FormDialog
	state.Open()
	return func(c ayra.Context) ayra.Dimensions {
		return widget.FormDialogProps{
			Title: "Add a vehicle", Description: "The plate is checked by the server.", Width: 340,
			Errors: map[string]string{"plate": "A plate has seven characters."},
			Fields: []widget.FormField{
				{Name: "plate", Label: "Plate", Mask: brazil.Plate, Placeholder: "ABC1D23", Required: true, Value: "ABC1"},
				{Name: "document", Label: "Owner's CPF", Mask: brazil.CPF, Kind: widget.Number, Value: "12345678909"},
			},
		}.Layout(c, &state)
	}
}

func misc() ayra.Widget {
	var armed, idle widget.DeleteRow
	armed.Arm()
	var busy, ready widget.LoadMore
	var editing, resting widget.EditInPlace
	editing.Edit("Gate camera 1")
	var group widget.Input
	var send, toggle widget.Button
	var waiting, loaded widget.LazyLoad
	loaded.Ready()
	var optimistic widget.OptimisticToggle
	optimistic.Set(true)
	var upload widget.FileUpload
	var search widget.ActiveSearch
	var combo widget.Combobox
	var hover widget.HoverCard
	var hoverButton widget.Button
	hover.Open()
	uploadAdded := false

	return func(c ayra.Context) ayra.Dimensions {
		if !uploadAdded {
			uploadAdded = true
			upload.Add(c, widget.FileUploadProps{Accept: []string{".pdf"}},
				widget.File{Name: "contract.pdf", Size: 3 << 20}, widget.File{Name: "notes.txt", Size: 900})
		}
		row := func(children ...layout.FlexChild) ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
			}
		}
		tight := func(w ayra.Widget) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = layout.Constraints{}.Min
				return w(inner)
			})
		}
		gap := layout.Rigid(layout.Spacer{Width: 10}.Layout)

		return column(c, 340,
			row(
				tight(func(c ayra.Context) ayra.Dimensions { return widget.DeleteRowProps{}.Layout(c, &idle) }), gap,
				tight(func(c ayra.Context) ayra.Dimensions { return widget.DeleteRowProps{}.Layout(c, &armed) }), gap,
				tight(func(c ayra.Context) ayra.Dimensions { return widget.ThemeToggleProps{}.Layout(c, &toggle) }),
			),
			spacer(8),
			row(
				tight(func(c ayra.Context) ayra.Dimensions { return widget.LoadMoreProps{}.Layout(c, &ready) }), gap,
				tight(func(c ayra.Context) ayra.Dimensions { return widget.LoadMoreProps{IsLoading: true}.Layout(c, &busy) }),
			),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.EditInPlaceProps{Value: "Gate camera 1"}.Layout(c, &editing)
			},
			spacer(4),
			func(c ayra.Context) ayra.Dimensions {
				return widget.EditInPlaceProps{Value: "Gate camera 1"}.Layout(c, &resting)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.InputGroupProps{}.Layout(c,
					func(c ayra.Context) ayra.Dimensions {
						return widget.InputProps{Placeholder: "Plate", ComponentProps: widget.GroupedParts()}.Layout(c, &group)
					},
					func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Label: "Search", Variant: widget.Ghost, ComponentProps: widget.GroupedParts()}.Layout(c, &send)
					})
			},
			spacer(8),
			row(
				tight(func(c ayra.Context) ayra.Dimensions {
					return widget.OutputProps{Label: "Total", Value: "R$ 1.234,56", Role: widget.Heading}.Layout(c)
				}), gap,
				tight(func(c ayra.Context) ayra.Dimensions {
					return widget.RelativeTimeProps{At: reference.Add(-3 * time.Hour), Now: reference}.Layout(c)
				}),
			),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.LazyLoadProps{}.Layout(c, &waiting, func(c ayra.Context) ayra.Dimensions { return ayra.Dimensions{} })
			},
			spacer(4),
			func(c ayra.Context) ayra.Dimensions {
				return widget.LazyLoadProps{}.Layout(c, &loaded, func(c ayra.Context) ayra.Dimensions {
					return widget.TextProps{Content: "What was fetched."}.Layout(c)
				})
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.OptimisticToggleProps{Label: "Notify me"}.Layout(c, &optimistic)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ActiveSearchProps{Placeholder: "Search a plate"}.Layout(c, &search)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ComboboxProps{Placeholder: "Search a city"}.Layout(c, &combo)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.FileUploadProps{Accept: []string{".pdf"}}.Layout(c, &upload)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.HoverCardProps{Width: 240}.Layout(c, &hover,
					func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Label: "Gate camera", Variant: widget.Link}.Layout(c, &hoverButton)
					},
					func(c ayra.Context) ayra.Dimensions {
						return widget.TextProps{Content: "Reads plates at the main gate.", Tone: widget.Muted}.Layout(c)
					})
			},
		)
	}
}

func TestEveryNewWidgetLooksRightInBothSchemes(t *testing.T) {
	for _, s := range []screen{
		{name: "icon-set", width: 360, height: 360, draw: iconGrid},
		{name: "icon-use", width: 380, height: 520, draw: iconUse},
		{name: "dates-range", width: 380, height: 760, draw: rangeOpen},
		{name: "dates-time", width: 380, height: 820, draw: timeOpen(widget.DateAndTime)},
		{name: "dates-month", width: 380, height: 620, draw: timeOpen(widget.MonthOnly)},
		{name: "form-dialog", width: 380, height: 520, draw: formOpen},
		{name: "misc", width: 380, height: 1100, draw: misc},
	} {
		for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
			for _, density := range []float32{1, 2} {
				name := pictureName(s.name, scheme, density)
				t.Run(name, func(t *testing.T) {
					got, err := frame.Draw(frame.Options{
						Width: s.width, Height: s.height, Density: density, Scheme: scheme, Fonts: fonts(),
					}, s.draw())
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
}

// TestAnIconFollowsTheSchemeAndIsNeverAFixedColour draws an icon in each scheme
// and reads the pixels back: the ink of its body is the theme's foreground, in
// the light scheme and in the dark one, and not a value somebody wrote.
//
// The two schemes are the whole of the proof. An icon drawn in a fixed colour is
// the right colour in one of them and invisible or wrong in the other, and no
// reading of the source says which.
func TestAnIconFollowsTheSchemeAndIsNeverAFixedColour(t *testing.T) {
	for _, tone := range []struct {
		name string
		tone widget.Tone
		want func(theme.Theme) color.NRGBA
	}{
		{"normal", widget.Normal, func(th theme.Theme) color.NRGBA { return th.Colours.Foreground }},
		{"muted", widget.Muted, func(th theme.Theme) color.NRGBA { return th.Colours.MutedForeground }},
		{"danger", widget.Danger, func(th theme.Theme) color.NRGBA { return th.Colours.Destructive }},
	} {
		seen := map[theme.Scheme]color.NRGBA{}
		for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
			th := theme.New(scheme)
			got, err := frame.Draw(frame.Options{Width: 80, Height: 80, Density: 2, Scheme: scheme, Fonts: fonts()},
				func(c ayra.Context) ayra.Dimensions {
					return layout.Center.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
						return widget.IconProps{Icon: icons.Car(), Size: 48, Tone: tone.tone}.Layout(c.With(gtx))
					})
				})
			if err != nil {
				if noSurface(err) {
					t.Skipf("no drawing surface on this machine: %v", err)
				}
				t.Fatal(err)
			}

			// The most common colour that is not the ground is the ink: the body
			// of the icon is the largest area drawn at full strength.
			ground := th.Colours.Background
			counts := map[color.NRGBA]int{}
			for y := 0; y < got.Bounds().Dy(); y++ {
				for x := 0; x < got.Bounds().Dx(); x++ {
					px := got.RGBAAt(x, y)
					c := color.NRGBA{R: px.R, G: px.G, B: px.B, A: 255}
					if near(c, ground, 12) {
						continue
					}
					counts[c]++
				}
			}
			var best color.NRGBA
			most := 0
			for c, n := range counts {
				if n > most {
					best, most = c, n
				}
			}
			want := tone.want(th)
			if !near(best, want, 6) {
				t.Errorf("%s in the %s scheme is drawn in %+v, want the theme's %+v", tone.name, scheme, best, want)
			}
			seen[scheme] = best
		}
		if tone.tone == widget.Normal && seen[theme.Light] == seen[theme.Dark] {
			t.Error("the icon is the same colour in both schemes: it is drawn in a fixed colour")
		}
	}
}

func near(a, b color.NRGBA, tolerance int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= tolerance && d(a.G, b.G) <= tolerance && d(a.B, b.B) <= tolerance
}
