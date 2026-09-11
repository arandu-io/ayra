package main

import (
	"strconv"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// feedback are the controls that tell somebody what happened, and the ones that
// come up over what they were doing.
func feedback() []Entry {
	return []Entry{
		alertEntry(),
		toastEntry(),
		dialogEntry(),
		drawerEntry(),
		popoverEntry(),
		tooltipEntry(),
		progressEntry(),
		meterEntry(),
		skeletonEntry(),
		emptyEntry(),
	}
}

// severities is the closed set of how bad a message is, in the order the
// library declares it.
//
// Two, and the reason is in the palette: the colours are generated from the
// stylesheet both halves of the product share, and it declares no colour for a
// success or a caution. A third invented here would be a colour the rest of the
// product does not have.
var severities = []widget.Severity{widget.Note, widget.Failure}

// sides is the closed set of edges a panel comes in from.
var sides = []widget.Side{widget.Right, widget.Left, widget.Bottom}

// alertEntry draws the band across a region, at both severities and with and
// without its second line.
func alertEntry() Entry {
	return Entry{
		Control: "AlertProps",
		Summary: "A message about the screen it is on, filling the width it is given.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				rows := make([]ayra.Widget, 0, len(severities)+1)
				for _, severity := range severities {
					severity := severity
					rows = append(rows, labelled(severity.String(), sized(480, func(c ayra.Context) ayra.Dimensions {
						return widget.AlertProps{
							Title:    "The invitation was not sent",
							Body:     "The address was refused by the server that holds it.",
							Severity: severity,
						}.Layout(c)
					})))
				}

				rows = append(rows, labelled("the title alone", sized(480, func(c ayra.Context) ayra.Dimensions {
					return widget.AlertProps{Title: "Saved"}.Layout(c)
				})))

				return stack(14, rows...)(c)
			}
		},
	}
}

// toastEntry draws the stack, and the controls that put something in it.
//
// It is worth pressing more than three times: the control drops the oldest once
// there are more than three, because a person shown four at once reads none of
// them, and that rule is invisible until somebody goes past it.
func toastEntry() Entry {
	return Entry{
		Control: "ToastProps",
		Summary: "A stack of messages in a corner, at most three at a time.",
		Build: func() ayra.Widget {
			var stackOf widget.Toasts
			var say, fail widget.Button
			shown := 0

			return func(c ayra.Context) ayra.Dimensions {
				if say.Clicked(c) {
					shown++
					stackOf.Show(widget.Toast{Title: "Saved, number " + strconv.Itoa(shown)})
				}
				if fail.Clicked(c) {
					shown++
					stackOf.Show(widget.Toast{
						Title:    "Refused, number " + strconv.Itoa(shown),
						Severity: widget.Failure,
					})
				}

				return stack(16,
					flow(10,
						func(c ayra.Context) ayra.Dimensions {
							return widget.ButtonProps{Label: "Say something", Variant: widget.Outline}.Layout(c, &say)
						},
						func(c ayra.Context) ayra.Dimensions {
							return widget.ButtonProps{Label: "Say something went wrong", Variant: widget.Destructive}.Layout(c, &fail)
						},
					),
					note("press a message to take it down"),
					sized(420, func(c ayra.Context) ayra.Dimensions {
						return widget.ToastProps{}.Layout(c, &stackOf)
					}),
				)(c)
			}
		},
	}
}

// dialogEntry draws the panel over a box rather than over the window.
//
// Over a box because that is what makes it a demonstration. A dialog covers
// whatever room it is handed, so one laid out against the page would put a
// scrim over the whole catalogue and centre its panel in the middle of it --
// correct behaviour, and nothing anybody could look at beside another control.
func dialogEntry() Entry {
	return Entry{
		Control: "DialogProps",
		Summary: "A panel over the screen that asks something and is answered.",
		Build: func() ayra.Widget {
			var asking, insisting widget.Dialog
			var openAsking, openInsisting, confirm, cancel, acknowledge widget.Button
			asking.Open()
			insisting.Open()

			outcome := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if openAsking.Clicked(c) {
					asking.Open()
				}
				if openInsisting.Clicked(c) {
					insisting.Open()
				}
				if confirm.Clicked(c) {
					asking.Close()
					outcome = "confirmed"
				}
				if cancel.Clicked(c) {
					asking.Close()
					outcome = "cancelled"
				}
				if acknowledge.Clicked(c) {
					insisting.Close()
					outcome = "acknowledged"
				}
				if asking.Dismissed() {
					outcome = "dismissed without an answer"
				}

				return stack(16,
					labelled("modal, and escape or a press outside takes it down", stack(10,
						tight(func(c ayra.Context) ayra.Dimensions {
							return widget.ButtonProps{Label: "Open it", Variant: widget.Outline}.Layout(c, &openAsking)
						}),
						framed(stage(520, 260, func(c ayra.Context) ayra.Dimensions {
							return widget.DialogProps{Modal: true, Dismissible: true}.Layout(c, &asking,
								func(c ayra.Context) ayra.Dimensions {
									return stack(12,
										func(c ayra.Context) ayra.Dimensions {
											return widget.TextProps{Content: "Remove the member", Role: widget.Heading, Bold: true}.Layout(c)
										},
										note("They lose access at once, and nothing they wrote is deleted."),
										flow(10,
											func(c ayra.Context) ayra.Dimensions {
												return widget.ButtonProps{Label: "Remove", Variant: widget.Destructive}.Layout(c, &confirm)
											},
											func(c ayra.Context) ayra.Dimensions {
												return widget.ButtonProps{Label: "Keep them", Variant: widget.Ghost}.Layout(c, &cancel)
											},
										),
									)(c)
								})
						})),
					)),
					labelled("modal and not dismissible, so only the button answers it", stack(10,
						tight(func(c ayra.Context) ayra.Dimensions {
							return widget.ButtonProps{Label: "Open it", Variant: widget.Outline}.Layout(c, &openInsisting)
						}),
						framed(stage(520, 220, func(c ayra.Context) ayra.Dimensions {
							return widget.DialogProps{Width: 320, Modal: true}.Layout(c, &insisting,
								func(c ayra.Context) ayra.Dimensions {
									return stack(12,
										func(c ayra.Context) ayra.Dimensions {
											return widget.TextProps{Content: "Your session ended", Role: widget.Heading, Bold: true}.Layout(c)
										},
										tight(func(c ayra.Context) ayra.Dimensions {
											return widget.ButtonProps{Label: "Sign in again"}.Layout(c, &acknowledge)
										}),
									)(c)
								})
						})),
					)),
					note("the first one was answered with: "+outcome),
				)(c)
			}
		},
	}
}

// drawerEntry draws the panel from each of the three edges.
func drawerEntry() Entry {
	return Entry{
		Control: "DrawerProps",
		Summary: "A panel anchored to an edge, holding a place somebody works in.",
		Build: func() ayra.Widget {
			panels := make([]widget.Dialog, len(sides))
			opens := make([]widget.Button, len(sides))
			closes := make([]widget.Button, len(sides))
			for index := range panels {
				panels[index].Open()
			}

			return func(c ayra.Context) ayra.Dimensions {
				rows := make([]ayra.Widget, 0, len(sides))

				for index, side := range sides {
					index, side := index, side

					if opens[index].Clicked(c) {
						panels[index].Open()
					}
					if closes[index].Clicked(c) {
						panels[index].Close()
					}

					rows = append(rows, labelled(side.String(), stack(10,
						tight(func(c ayra.Context) ayra.Dimensions {
							return widget.ButtonProps{
								Label:   "Open it",
								Variant: widget.Outline,
								Size:    widget.Small,
							}.Layout(c, &opens[index])
						}),
						framed(stage(420, 240, func(c ayra.Context) ayra.Dimensions {
							return widget.DrawerProps{
								Side:        side,
								Size:        200,
								Modal:       true,
								Dismissible: true,
							}.Layout(c, &panels[index], func(c ayra.Context) ayra.Dimensions {
								return stack(10,
									func(c ayra.Context) ayra.Dimensions {
										return widget.TextProps{Content: side.String(), Role: widget.Heading, Bold: true}.Layout(c)
									},
									tight(func(c ayra.Context) ayra.Dimensions {
										return widget.ButtonProps{
											Label:   "Close",
											Variant: widget.Ghost,
											Size:    widget.Small,
										}.Layout(c, &closes[index])
									}),
								)(c)
							})
						})),
					)))
				}

				return flow(24, rows...)(c)
			}
		},
	}
}

// popoverEntry draws the panel that appears beside what opened it, in both
// directions.
func popoverEntry() Entry {
	return Entry{
		Control: "PopoverProps",
		Summary: "A panel beside the control that opened it, holding anything at all.",
		Build: func() ayra.Widget {
			var below, above widget.Menu
			var belowTrigger, aboveTrigger widget.Button
			var amount widget.Slider
			var include widget.Toggle
			amount.SetValue(0.4)

			filters := func(c ayra.Context) ayra.Dimensions {
				include.Changed(c)
				return stack(12,
					func(c ayra.Context) ayra.Dimensions {
						return widget.TextProps{Content: "Filters", Bold: true}.Layout(c)
					},
					func(c ayra.Context) ayra.Dimensions {
						return widget.SliderProps{}.Layout(c, &amount)
					},
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.CheckboxProps{Label: "Include archived"}.Layout(c, &include)
					}),
				)(c)
			}

			return func(c ayra.Context) ayra.Dimensions {
				return stack(16,
					labelled("opens downwards", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.PopoverProps{}.Layout(c, &below,
							func(c ayra.Context) ayra.Dimensions {
								return widget.ButtonProps{Label: "Filter", Variant: widget.Outline, Size: widget.Small}.Layout(c, &belowTrigger)
							},
							filters)
					})),
					labelled("opens upwards, and wider", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.PopoverProps{Width: 340, Above: true}.Layout(c, &above,
							func(c ayra.Context) ayra.Dimensions {
								return widget.ButtonProps{Label: "Filter", Variant: widget.Outline, Size: widget.Small}.Layout(c, &aboveTrigger)
							},
							filters)
					})),
				)(c)
			}
		},
	}
}

// tooltipEntry draws the line that appears while the pointer rests.
//
// It needs a pointer, so on a touch screen there is nothing here to see -- which
// is the reason the control reports hovering and nothing else, and the reason a
// tooltip must never be the only place something is said.
func tooltipEntry() Entry {
	return Entry{
		Control: "TooltipProps",
		Summary: "A line beside a control while the pointer rests on it.",
		Build: func() ayra.Widget {
			var over, under widget.Hover
			var above, below widget.Button

			return func(c ayra.Context) ayra.Dimensions {
				return stack(16,
					note("rest the pointer on either one"),
					flow(16,
						func(c ayra.Context) ayra.Dimensions {
							return widget.TooltipProps{Text: "Sends the invitation now"}.Layout(c, &over,
								func(c ayra.Context) ayra.Dimensions {
									return widget.ButtonProps{Label: "Above", Variant: widget.Outline}.Layout(c, &above)
								})
						},
						func(c ayra.Context) ayra.Dimensions {
							return widget.TooltipProps{Text: "For a control at the top of a window", Below: true}.Layout(c, &under,
								func(c ayra.Context) ayra.Dimensions {
									return widget.ButtonProps{Label: "Below", Variant: widget.Outline}.Layout(c, &below)
								})
						},
					),
					note("hovering: "+strconv.FormatBool(over.Hovering() || under.Hovering())),
				)(c)
			}
		},
	}
}

// progressEntry draws work going somewhere, at several points and with its
// length unknown.
func progressEntry() Entry {
	return Entry{
		Control: "ProgressProps",
		Summary: "How far along something is, for work that ends.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				rows := make([]ayra.Widget, 0, 5)
				for _, fraction := range []float32{0, 0.25, 0.6, 1} {
					fraction := fraction
					rows = append(rows, labelled(strconv.FormatFloat(float64(fraction), 'f', 2, 32),
						sized(360, func(c ayra.Context) ayra.Dimensions {
							return widget.ProgressProps{Fraction: fraction}.Layout(c)
						})))
				}

				rows = append(rows, labelled("length unknown, so it claims nothing",
					sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.ProgressProps{Indeterminate: true}.Layout(c)
					})))

				return stack(14, rows...)(c)
			}
		},
	}
}

// meterEntry draws a level that goes up and down, inside its range and past it.
func meterEntry() Entry {
	return Entry{
		Control: "MeterProps",
		Summary: "A level inside a range, for something that never finishes.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				return stack(14,
					labelled("nought to one, the range it takes when none is given",
						sized(360, func(c ayra.Context) ayra.Dimensions {
							return widget.MeterProps{Value: 0.45}.Layout(c)
						})),
					labelled("a range of its own: 180 of 200 seats",
						sized(360, func(c ayra.Context) ayra.Dimensions {
							return widget.MeterProps{Value: 180, Low: 0, High: 200}.Layout(c)
						})),
					labelled("past what it should be",
						sized(360, func(c ayra.Context) ayra.Dimensions {
							return widget.MeterProps{Value: 240, High: 200, Over: true}.Layout(c)
						})),
				)(c)
			}
		},
	}
}

// skeletonEntry draws the shape of something that has not arrived yet.
func skeletonEntry() Entry {
	return Entry{
		Control: "SkeletonProps",
		Summary: "The shape of something on its way, which is not the same as nothing.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				return stack(14,
					labelled("a round one, and three lines", flow(12,
						func(c ayra.Context) ayra.Dimensions {
							return widget.SkeletonProps{Width: 36, Height: 36, Round: true}.Layout(c)
						},
						func(c ayra.Context) ayra.Dimensions {
							return stack(8,
								func(c ayra.Context) ayra.Dimensions {
									return widget.SkeletonProps{Width: 220}.Layout(c)
								},
								func(c ayra.Context) ayra.Dimensions {
									return widget.SkeletonProps{Width: 160}.Layout(c)
								},
								func(c ayra.Context) ayra.Dimensions {
									return widget.SkeletonProps{Width: 190, Height: 12}.Layout(c)
								},
							)(c)
						},
					)),
					labelled("no width, so it fills the room there is", sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.SkeletonProps{Height: 60}.Layout(c)
					})),
				)(c)
			}
		},
	}
}

// emptyEntry draws what a region with nothing in it says.
//
// It centres itself in the room it is given, so it is given a box. Handed a
// scrolling column it would centre in the whole scroll, and the message would
// be somewhere below the bottom of the window.
func emptyEntry() Entry {
	return Entry{
		Control: "EmptyProps",
		Summary: "What a region says when there is nothing in it, as against nothing yet.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				return flow(20,
					framed(stage(320, 140, func(c ayra.Context) ayra.Dimensions {
						return widget.EmptyProps{
							Title: "No invitations",
							Body:  "Invite somebody and they will appear here.",
						}.Layout(c)
					})),
					framed(stage(320, 140, func(c ayra.Context) ayra.Dimensions {
						return widget.EmptyProps{Title: "Nothing matched that"}.Layout(c)
					})),
				)(c)
			}
		},
	}
}
