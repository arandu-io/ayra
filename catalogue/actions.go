package main

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// actions are the controls a press does something with.
func actions() []Entry {
	return []Entry{
		buttonEntry(),
		splitEntry(),
		buttonGroupEntry(),
		segmentedEntry(),
		copyEntry(),
		linkEntry(),
		menuEntry(),
		menubarEntry(),
	}
}

// buttonEntry draws the whole grid: every variant at every size.
//
// Thirty buttons rather than one of each, because the two sets multiply and the
// product is where they disagree -- a ghost at the icon size has no border and
// no fill and is the one place a label that is a point off centre has nothing
// around it to hide behind.
func buttonEntry() Entry {
	return Entry{
		Control: "ButtonProps",
		Summary: "A press that does something on this screen.",
		Build: func() ayra.Widget {
			presses := make([][]widget.Button, len(variants))
			for index := range presses {
				presses[index] = make([]widget.Button, len(sizes))
			}
			var unavailable widget.Button

			return func(c ayra.Context) ayra.Dimensions {
				rows := make([]ayra.Widget, 0, len(variants)+1)

				for v, variant := range variants {
					v, variant := v, variant

					row := make([]ayra.Widget, 0, len(sizes))
					for s, size := range sizes {
						s, size := s, size
						row = append(row, func(c ayra.Context) ayra.Dimensions {
							return widget.ButtonProps{
								Label:   size.String(),
								Variant: variant,
								Size:    size,
							}.Layout(c, &presses[v][s])
						})
					}
					rows = append(rows, labelled(variant.String(), flow(8, row...)))
				}

				rows = append(rows, labelled("disabled", tight(func(c ayra.Context) ayra.Dimensions {
					return widget.ButtonProps{Label: "Unavailable", Disabled: true}.Layout(c, &unavailable)
				})))

				return stack(16, rows...)(c)
			}
		},
	}
}

// splitEntry draws the main action with its second half, at every variant and
// every size.
func splitEntry() Entry {
	return Entry{
		Control: "SplitProps",
		Summary: "The action most people want, with the rest behind the half beside it.",
		Build: func() ayra.Widget {
			byVariant := make([]widget.Split, len(variants))
			bySize := make([]widget.Split, len(sizes))
			var unavailable widget.Split
			opened := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				for index := range byVariant {
					if byVariant[index].Opened(c) {
						opened = variants[index].String()
					}
				}

				shownVariants := make([]ayra.Widget, 0, len(variants))
				for index, variant := range variants {
					index, variant := index, variant
					shownVariants = append(shownVariants, func(c ayra.Context) ayra.Dimensions {
						return widget.SplitProps{
							Label:   "Invite",
							Variant: variant,
						}.Layout(c, &byVariant[index])
					})
				}

				shownSizes := make([]ayra.Widget, 0, len(sizes))
				for index, size := range sizes {
					index, size := index, size
					shownSizes = append(shownSizes, func(c ayra.Context) ayra.Dimensions {
						return widget.SplitProps{Label: size.String(), Size: size}.Layout(c, &bySize[index])
					})
				}

				return stack(16,
					labelled("variants", flow(10, shownVariants...)),
					labelled("sizes", flow(10, shownSizes...)),
					labelled("disabled", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.SplitProps{Label: "Invite", Disabled: true}.Layout(c, &unavailable)
					})),
					note("second half last pressed on: "+opened),
				)(c)
			}
		},
	}
}

// buttonGroupEntry draws the group as a choice and as a row of actions.
//
// Both, because the difference between them is one field: a group with nothing
// marked is three things somebody can do, and a group with one marked is one
// thing set to one of three values.
func buttonGroupEntry() Entry {
	return Entry{
		Control: "ButtonGroupProps",
		Summary: "Buttons drawn as one object, with a division rather than a gap between them.",
		Build: func() ayra.Widget {
			var choice, actions widget.Group
			bySize := make([]widget.Group, len(sizes))

			selected := 1
			pressed := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if index := choice.Clicked(c); index >= 0 {
					selected = index
				}
				if index := actions.Clicked(c); index >= 0 {
					pressed = []string{"Copy", "Move", "Delete"}[index]
				}

				shownSizes := make([]ayra.Widget, 0, len(sizes))
				for index, size := range sizes {
					index, size := index, size
					shownSizes = append(shownSizes, func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonGroupProps{
							Labels:   []string{"One", "Two", size.String()},
							Selected: -1,
							Size:     size,
						}.Layout(c, &bySize[index])
					})
				}

				return stack(16,
					labelled("a choice", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonGroupProps{
							Labels:   []string{"Day", "Week", "Month"},
							Selected: selected,
						}.Layout(c, &choice)
					})),
					labelled("actions, nothing marked", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonGroupProps{
							Labels:   []string{"Copy", "Move", "Delete"},
							Selected: -1,
						}.Layout(c, &actions)
					})),
					labelled("sizes", flow(10, shownSizes...)),
					note("last action pressed: "+pressed),
				)(c)
			}
		},
	}
}

// segmentedEntry draws the exclusive set that keeps its own choice.
func segmentedEntry() Entry {
	return Entry{
		Control: "SegmentedProps",
		Summary: "A small fixed set of exclusive choices, all on one row.",
		Build: func() ayra.Widget {
			var filter widget.Segments
			filter.Choose(0)

			bySize := make([]widget.Segments, len(sizes))
			for index := range bySize {
				bySize[index].Choose(1)
			}

			return func(c ayra.Context) ayra.Dimensions {
				shownSizes := make([]ayra.Widget, 0, len(sizes))
				for index, size := range sizes {
					index, size := index, size
					shownSizes = append(shownSizes, func(c ayra.Context) ayra.Dimensions {
						return widget.SegmentedProps{
							Options: []string{"Off", size.String()},
							Size:    size,
						}.Layout(c, &bySize[index])
					})
				}

				return stack(16,
					labelled("chosen", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.SegmentedProps{Options: []string{"All", "Admins", "Invited"}}.Layout(c, &filter)
					})),
					labelled("sizes", flow(10, shownSizes...)),
					note("showing: "+[]string{"All", "Admins", "Invited"}[filter.Selected()]),
				)(c)
			}
		},
	}
}

// copyEntry draws the control that puts a value on the clipboard, and says so.
func copyEntry() Entry {
	return Entry{
		Control: "CopyProps",
		Summary: "Puts a value on the clipboard and confirms that it did.",
		Build: func() ayra.Widget {
			var plain, named widget.Copier
			bySize := make([]widget.Copier, len(sizes))

			return func(c ayra.Context) ayra.Dimensions {
				shownSizes := make([]ayra.Widget, 0, len(sizes))
				for index, size := range sizes {
					index, size := index, size
					shownSizes = append(shownSizes, func(c ayra.Context) ayra.Dimensions {
						return widget.CopyProps{
							Value: size.String(),
							Label: size.String(),
							Size:  size,
						}.Layout(c, &bySize[index])
					})
				}

				return stack(16,
					labelled("the words it says by default", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.CopyProps{Value: "ayra"}.Layout(c, &plain)
					})),
					labelled("words of its own", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.CopyProps{
							Value: "0f2e4a91",
							Label: "Copy the reference",
							Done:  "On the clipboard",
						}.Layout(c, &named)
					})),
					labelled("sizes", flow(10, shownSizes...)),
				)(c)
			}
		},
	}
}

// linkEntry draws text that leaves the screen.
func linkEntry() Entry {
	return Entry{
		Control: "LinkProps",
		Summary: "Text that goes somewhere, with the rule under it that says so.",
		Build: func() ayra.Widget {
			var ordinary, muted widget.Button
			followed := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if ordinary.Clicked(c) {
					followed = "the ordinary one"
				}
				if muted.Clicked(c) {
					followed = "the muted one"
				}

				return stack(12,
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.LinkProps{Text: "Read what changed"}.Layout(c, &ordinary)
					}),
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.LinkProps{Text: "Terms", Muted: true}.Layout(c, &muted)
					}),
					note("last followed: "+followed),
				)(c)
			}
		},
	}
}

// menuEntry draws the list of actions that appears under a control.
//
// Two of them, opening in the two directions, because the choice of direction
// belongs to where the control sits on a screen and there is no way to see that
// it works without a second one.
func menuEntry() Entry {
	return Entry{
		Control: "MenuProps",
		Summary: "A list of actions under the control that opens it.",
		Build: func() ayra.Widget {
			var below, above widget.Menu
			var belowTrigger, aboveTrigger widget.Button

			entries := []string{"Rename", "Duplicate", "", "Delete"}
			chosen := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if index := below.Chosen(); index >= 0 {
					chosen = entries[index]
				}
				if index := above.Chosen(); index >= 0 {
					chosen = entries[index]
				}

				return stack(14,
					labelled("opens downwards", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.MenuProps{Entries: entries}.Layout(c, &below,
							func(c ayra.Context) ayra.Dimensions {
								return widget.ButtonProps{
									Label:   "Actions",
									Variant: widget.Outline,
									Size:    widget.Small,
								}.Layout(c, &belowTrigger)
							})
					})),
					labelled("opens upwards, and is as wide as it was told", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.MenuProps{Entries: entries, Width: 240, Above: true}.Layout(c, &above,
							func(c ayra.Context) ayra.Dimensions {
								return widget.ButtonProps{
									Label:   "Actions",
									Variant: widget.Outline,
									Size:    widget.Small,
								}.Layout(c, &aboveTrigger)
							})
					})),
					note("last chosen: "+chosen),
				)(c)
			}
		},
	}
}

// menubarEntry draws the row of menus an application is commanded from.
func menubarEntry() Entry {
	return Entry{
		Control: "MenubarProps",
		Summary: "A row of menus, of which one is open at a time.",
		Build: func() ayra.Widget {
			var bar widget.Menubar

			titles := []string{"File", "Edit", "View"}
			entries := [][]string{
				{"New", "Open", "", "Save", "Save as"},
				{"Undo", "Redo", "", "Cut", "Copy", "Paste"},
				{"Zoom in", "Zoom out", "", "Full screen"},
			}
			chosen := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if menu, entry := bar.Chosen(); menu >= 0 {
					chosen = titles[menu] + " / " + entries[menu][entry]
				}

				return stack(14,
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.MenubarProps{Titles: titles, Entries: entries}.Layout(c, &bar)
					}),
					note("last chosen: "+chosen),
				)(c)
			}
		},
	}
}
