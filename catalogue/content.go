package main

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/widget"
)

// content are the pieces a screen is written out of rather than operated with.
func content() []Entry {
	return []Entry{
		textEntry(),
		highlightEntry(),
		badgeEntry(),
		statusEntry(),
		statEntry(),
		kbdEntry(),
		avatarEntry(),
		separatorEntry(),
		figureEntry(),
		itemEntry(),
		cardEntry(),
		collapsibleEntry(),
		accordionEntry(),
	}
}

// roles is the closed set of what a piece of text is for, in the order the
// library declares it.
var roles = []widget.Role{
	widget.Body,
	widget.Display,
	widget.Heading,
	widget.Caption,
	widget.Mono,
}

// tones is the closed set of how much attention a line asks for.
var tones = []widget.Tone{
	widget.Normal,
	widget.Muted,
	widget.Danger,
	widget.Accent,
}

// textEntry draws every role and every tone.
//
// The two sets are separate on purpose -- a heading can be quiet and a caption
// can be a warning -- so they are drawn as two rows and not as a grid of twenty
// samples nobody reads.
func textEntry() Entry {
	return Entry{
		Control: "TextProps",
		Summary: "A piece of text, named by what it is for rather than by how big it is.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				rows := make([]ayra.Widget, 0, len(roles)+len(tones)+4)

				for _, role := range roles {
					role := role
					rows = append(rows, func(c ayra.Context) ayra.Dimensions {
						return widget.TextProps{
							Content: "The quick brown fox — " + role.String(),
							Role:    role,
						}.Layout(c)
					})
				}

				for _, tone := range tones {
					tone := tone
					rows = append(rows, func(c ayra.Context) ayra.Dimensions {
						return widget.TextProps{Content: "This line is " + tone.String(), Tone: tone}.Layout(c)
					})
				}

				rows = append(rows,
					func(c ayra.Context) ayra.Dimensions {
						return widget.TextProps{Content: "Set in the heavier weight", Bold: true}.Layout(c)
					},
					sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.TextProps{
							Content:  "One line only, so a long value pushes nothing out of place however long it turns out to be.",
							MaxLines: 1,
						}.Layout(c)
					}),
					sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.TextProps{Content: "Centred in the room it was given", Align: text.Middle}.Layout(c)
					}),
					sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.TextProps{Content: "Ending at the far edge", Align: text.End}.Layout(c)
					}),
				)

				return stack(10, rows...)(c)
			}
		},
	}
}

// highlightEntry draws a line with the match behind a wash, including the two
// cases a caller gets wrong by splitting the string themselves.
func highlightEntry() Entry {
	return Entry{
		Control: "HighlightProps",
		Summary: "A line with the part that matched marked, at whatever size the line is set in.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				return stack(10,
					func(c ayra.Context) ayra.Dimensions {
						return widget.HighlightProps{
							Text:  "resources/views/layouts/app",
							Match: "layouts",
						}.Layout(c)
					},
					labelled("the capitals do not have to agree", func(c ayra.Context) ayra.Dimensions {
						return widget.HighlightProps{Text: "Amsterdam", Match: "DAM"}.Layout(c)
					}),
					labelled("no query, so nothing is marked", func(c ayra.Context) ayra.Dimensions {
						return widget.HighlightProps{Text: "Amsterdam"}.Layout(c)
					}),
					labelled("at the heading size", func(c ayra.Context) ayra.Dimensions {
						return widget.HighlightProps{
							Text:  "Billing and invoices",
							Match: "invoice",
							Role:  widget.Heading,
						}.Layout(c)
					}),
				)(c)
			}
		},
	}
}

// badgeEntry draws the small label at every variant.
//
// Beside the button of the same variant, because the two share the word and
// the point of sharing it is that they share the colour: a destructive badge
// and a destructive button that are two different reds is the defect the shared
// vocabulary exists to prevent.
func badgeEntry() Entry {
	return Entry{
		Control: "BadgeProps",
		Summary: "A small label beside something else, round-ended at any size.",
		Build: func() ayra.Widget {
			presses := make([]widget.Button, len(variants))

			return func(c ayra.Context) ayra.Dimensions {
				badges := make([]ayra.Widget, 0, len(variants))
				pairs := make([]ayra.Widget, 0, len(variants))

				for index, variant := range variants {
					index, variant := index, variant

					badges = append(badges, func(c ayra.Context) ayra.Dimensions {
						return widget.BadgeProps{Label: variant.String(), Variant: variant}.Layout(c)
					})
					pairs = append(pairs, func(c ayra.Context) ayra.Dimensions {
						return flow(6,
							func(c ayra.Context) ayra.Dimensions {
								return widget.ButtonProps{
									Label:   variant.String(),
									Variant: variant,
									Size:    widget.ExtraSmall,
								}.Layout(c, &presses[index])
							},
							func(c ayra.Context) ayra.Dimensions {
								return widget.BadgeProps{Label: variant.String(), Variant: variant}.Layout(c)
							},
						)(c)
					})
				}

				return stack(16,
					labelled("variants", flow(8, badges...)),
					labelled("each beside the button that shares its word", flow(16, pairs...)),
				)(c)
			}
		},
	}
}

// statusEntry draws the dot and the word, at every tone.
func statusEntry() Entry {
	return Entry{
		Control: "StatusProps",
		Summary: "A state as a dot and a word, because a row of dots alone is a legend.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				labels := map[widget.Tone]string{
					widget.Normal: "Running",
					widget.Muted:  "Stopped",
					widget.Danger: "Failed",
					widget.Accent: "Deploying",
				}

				shown := make([]ayra.Widget, 0, len(tones))
				for _, tone := range tones {
					tone := tone
					shown = append(shown, func(c ayra.Context) ayra.Dimensions {
						return widget.StatusProps{Label: labels[tone], Tone: tone}.Layout(c)
					})
				}

				return flow(20, shown...)(c)
			}
		},
	}
}

// statEntry draws the measured figure, with and without the line under it.
func statEntry() Entry {
	return Entry{
		Control: "StatProps",
		Summary: "One figure and what it counts; how a number is written is a locale's decision.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				return flow(32,
					sized(180, func(c ayra.Context) ayra.Dimensions {
						return widget.StatProps{
							Value: "1,284",
							Label: "Members",
							Note:  "Up 4% this month",
						}.Layout(c)
					}),
					sized(180, func(c ayra.Context) ayra.Dimensions {
						return widget.StatProps{Value: "£8,410", Label: "Spend"}.Layout(c)
					}),
					sized(180, func(c ayra.Context) ayra.Dimensions {
						return widget.StatProps{Value: "0", Label: "Failures", Note: "Since the last release"}.Layout(c)
					}),
				)(c)
			}
		},
	}
}

// kbdEntry draws the key somebody is told to press.
func kbdEntry() Entry {
	return Entry{
		Control: "KbdProps",
		Summary: "A key, set in the face whose digits are all one width.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				keys := []string{"K", "Ctrl", "Esc", "Ctrl K", "Shift Tab"}

				shown := make([]ayra.Widget, 0, len(keys))
				for _, key := range keys {
					key := key
					shown = append(shown, func(c ayra.Context) ayra.Dimensions {
						return widget.KbdProps{Keys: key}.Layout(c)
					})
				}
				return flow(8, shown...)(c)
			}
		},
	}
}

// avatarEntry draws the mark that stands for a person, at several diameters.
func avatarEntry() Entry {
	return Entry{
		Control: "AvatarProps",
		Summary: "One or two letters standing for a person, on a circle.",
		Build: func() ayra.Widget {
			people := []struct {
				initials string
				diameter unit.Dp
			}{
				{"PL", 24}, {"AY", 0}, {"JO", 48}, {"K", 64},
			}

			return func(c ayra.Context) ayra.Dimensions {
				shown := make([]ayra.Widget, 0, len(people))
				for _, person := range people {
					person := person
					shown = append(shown, func(c ayra.Context) ayra.Dimensions {
						return widget.AvatarProps{
							Initials: person.initials,
							Size:     person.diameter,
						}.Layout(c)
					})
				}

				return stack(12,
					flow(12, shown...),
					note("the second takes the diameter a row of them is built from"),
				)(c)
			}
		},
	}
}

// separatorEntry draws the rule across and down, and with an inset at each end.
func separatorEntry() Entry {
	return Entry{
		Control: "SeparatorProps",
		Summary: "A rule that fills the axis it runs along and takes one pixel across.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				return stack(18,
					labelled("across", sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.SeparatorProps{}.Layout(c)
					})),
					labelled("across, stopping short of each end", sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.SeparatorProps{Inset: 40}.Layout(c)
					})),
					labelled("down, between two columns", flow(20,
						func(c ayra.Context) ayra.Dimensions {
							return widget.TextProps{Content: "Left", MaxLines: 1}.Layout(c)
						},
						stage(2, 48, func(c ayra.Context) ayra.Dimensions {
							return widget.SeparatorProps{Vertical: true}.Layout(c)
						}),
						func(c ayra.Context) ayra.Dimensions {
							return widget.TextProps{Content: "Right", MaxLines: 1}.Layout(c)
						},
					)),
				)(c)
			}
		},
	}
}

// figureEntry draws something with its caption, bordered and not.
func figureEntry() Entry {
	return Entry{
		Control: "FigureProps",
		Summary: "Something shown with the caption that belongs to it rather than to the line after it.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				block := func(c ayra.Context) ayra.Dimensions {
					return widget.SkeletonProps{Height: 90}.Layout(c)
				}

				return flow(24,
					sized(280, func(c ayra.Context) ayra.Dimensions {
						return widget.FigureProps{
							Caption:  "Bordered, for something whose own edges are pale.",
							Bordered: true,
						}.Layout(c, block)
					}),
					sized(280, func(c ayra.Context) ayra.Dimensions {
						return widget.FigureProps{Caption: "Without a border."}.Layout(c, block)
					}),
				)(c)
			}
		},
	}
}

// itemEntry draws the row a list is repeated out of, in each of its states.
func itemEntry() Entry {
	return Entry{
		Control: "ItemProps",
		Summary: "One row of a list: a title, a line under it, and something on the right.",
		Build: func() ayra.Widget {
			var plain, pressable, selected widget.Button
			pressed := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if pressable.Clicked(c) {
					pressed = "the pressable row"
				}
				if selected.Clicked(c) {
					pressed = "the marked row"
				}

				return stack(14,
					sized(420, func(c ayra.Context) ayra.Dimensions {
						return stack(2,
							func(c ayra.Context) ayra.Dimensions {
								return widget.ItemProps{
									Title: "Information, so it does not answer a press",
									Body:  "Added three days ago",
								}.Layout(c, &plain, nil)
							},
							func(c ayra.Context) ayra.Dimensions {
								return widget.ItemProps{
									Title:     "Navigation, with something on the right",
									Body:      "Press it",
									Pressable: true,
								}.Layout(c, &pressable, func(c ayra.Context) ayra.Dimensions {
									return widget.BadgeProps{Label: "New", Variant: widget.Secondary}.Layout(c)
								})
							},
							func(c ayra.Context) ayra.Dimensions {
								return widget.ItemProps{
									Title:     "The current one",
									Pressable: true,
									Selected:  true,
								}.Layout(c, &selected, func(c ayra.Context) ayra.Dimensions {
									return widget.KbdProps{Keys: "3"}.Layout(c)
								})
							},
						)(c)
					}),
					note("last pressed: "+pressed),
				)(c)
			}
		},
	}
}

// cardEntry draws the surface and its boundary, on both of the grounds it has.
func cardEntry() Entry {
	return Entry{
		Control: "CardProps",
		Summary: "A grouped region: a surface and a boundary, with no title of its own.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				body := func(content string) ayra.Widget {
					return func(c ayra.Context) ayra.Dimensions {
						return widget.TextProps{Content: content}.Layout(c)
					}
				}

				return stack(16,
					sized(420, func(c ayra.Context) ayra.Dimensions {
						return widget.CardProps{}.Layout(c, body("The room around this is what a card takes when nothing was asked for."))
					}),
					sized(420, func(c ayra.Context) ayra.Dimensions {
						return widget.CardProps{Padding: 32}.Layout(c, body("More room around it."))
					}),
					sized(420, func(c ayra.Context) ayra.Dimensions {
						return widget.CardProps{}.Layout(c, func(c ayra.Context) ayra.Dimensions {
							return stack(12,
								body("A card inside a card, where the ordinary ground would vanish."),
								func(c ayra.Context) ayra.Dimensions {
									return widget.CardProps{Muted: true}.Layout(c, body("The quieter of the two surfaces."))
								},
							)(c)
						})
					}),
				)(c)
			}
		},
	}
}

// collapsibleEntry draws the summary that reveals more.
func collapsibleEntry() Entry {
	return Entry{
		Control: "CollapsibleProps",
		Summary: "A line that is always visible, and content under it that is not.",
		Build: func() ayra.Widget {
			var open, closed widget.Disclosure
			open.Open()

			return func(c ayra.Context) ayra.Dimensions {
				open.Changed(c)
				closed.Changed(c)

				body := func(c ayra.Context) ayra.Dimensions {
					return widget.TextProps{
						Content: "Closed, this takes no room at all: a section that kept its height would push everything under it down for something nobody can see.",
					}.Layout(c)
				}

				return stack(8,
					sized(460, func(c ayra.Context) ayra.Dimensions {
						return widget.CollapsibleProps{Summary: "Open"}.Layout(c, &open, body)
					}),
					sized(460, func(c ayra.Context) ayra.Dimensions {
						return widget.CollapsibleProps{Summary: "Closed"}.Layout(c, &closed, body)
					}),
				)(c)
			}
		},
	}
}

// accordionEntry draws the set where opening one closes the rest.
func accordionEntry() Entry {
	return Entry{
		Control: "AccordionProps",
		Summary: "Sections of which the open one is the only one.",
		Build: func() ayra.Widget {
			var sections widget.Accordion
			sections.Open(0)

			names := []string{"What it costs", "How it is billed", "How to stop"}
			bodies := []string{
				"Per member, per month, and nothing for anybody invited and not yet signed in.",
				"On the day of the month the workspace was opened.",
				"Any time. The workspace is readable until the period it was paid for ends.",
			}

			return func(c ayra.Context) ayra.Dimensions {
				return stack(12,
					sized(460, func(c ayra.Context) ayra.Dimensions {
						return widget.AccordionProps{Sections: names}.Layout(c, &sections, func(index int) ayra.Widget {
							return func(c ayra.Context) ayra.Dimensions {
								return widget.TextProps{Content: bodies[index]}.Layout(c)
							}
						})
					}),
					note("open: "+openSection(&sections, names)),
				)(c)
			}
		},
	}
}

// openSection names the section showing, or says that none is.
func openSection(state *widget.Accordion, names []string) string {
	at := state.Showing()
	if at < 0 || at >= len(names) {
		return "none"
	}
	return names[at]
}
