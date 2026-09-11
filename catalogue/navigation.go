package main

import (
	"strconv"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/widget"
)

// navigation are the controls that move somebody around an application.
func navigation() []Entry {
	return []Entry{
		sidebarEntry(),
		tabsEntry(),
		breadcrumbEntry(),
		paginationEntry(),
		toolbarEntry(),
		treeEntry(),
		paletteEntry(),
		carouselEntry(),
	}
}

// sidebarEntry draws the column of places, with and without its heading.
func sidebarEntry() Entry {
	return Entry{
		Control: "SidebarProps",
		Summary: "The column of places a screen can go.",
		Build: func() ayra.Widget {
			var titled, bare widget.Sidebar
			titled.Show(1)

			entries := []string{"Overview", "Members", "Billing", "Settings"}
			pressed := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				// Which entry is current follows from what the screen navigated
				// to, so the mark is moved here and not by the control.
				if index := titled.Chosen(c); index >= 0 {
					titled.Show(index)
					pressed = entries[index]
				}
				if index := bare.Chosen(c); index >= 0 {
					bare.Show(index)
					pressed = entries[index]
				}

				return stack(16,
					flow(24,
						// Not on a surface of its own. A row of this control
						// paints the window's own ground when it is not the
						// current one, so putting it on any other colour draws
						// a block per row -- which is the control's doing and
						// not something a catalogue should stage away.
						labelled("with a heading", sized(200, func(c ayra.Context) ayra.Dimensions {
							return widget.SidebarProps{
								Title:   "Workspace",
								Entries: entries,
								Width:   180,
							}.Layout(c, &titled)
						})),
						labelled("without one", sized(200, func(c ayra.Context) ayra.Dimensions {
							return widget.SidebarProps{Entries: entries, Width: 180}.Layout(c, &bare)
						})),
					),
					note("last pressed: "+pressed),
				)(c)
			}
		},
	}
}

// tabsEntry draws the row, and says which panel a screen would draw under it.
func tabsEntry() Entry {
	return Entry{
		Control: "TabsProps",
		Summary: "A row of tabs; what each one shows belongs to the screen.",
		Build: func() ayra.Widget {
			var few, many widget.Tabs
			labels := []string{"Details", "Activity", "Permissions"}

			return func(c ayra.Context) ayra.Dimensions {
				few.Changed(c)
				many.Changed(c)

				return stack(16,
					sized(420, func(c ayra.Context) ayra.Dimensions {
						return widget.TabsProps{Labels: labels}.Layout(c, &few)
					}),
					note("the screen would draw: "+labels[few.Selected()]),
					labelled("more of them", sized(560, func(c ayra.Context) ayra.Dimensions {
						return widget.TabsProps{
							Labels: []string{"One", "Two", "Three", "Four", "Five", "Six"},
						}.Layout(c, &many)
					})),
				)(c)
			}
		},
	}
}

// breadcrumbEntry draws the trail, whose last step is the page and not a link.
func breadcrumbEntry() Entry {
	return Entry{
		Control: "BreadcrumbProps",
		Summary: "The trail back, with the current place drawn as a place rather than a link.",
		Build: func() ayra.Widget {
			var trail, deep widget.Crumbs
			steps := []string{"Workspace", "Projects", "Ayra", "Settings"}
			pressed := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if index := trail.Clicked(c); index >= 0 {
					pressed = steps[index]
				}

				return stack(14,
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.BreadcrumbProps{Steps: steps}.Layout(c, &trail)
					}),
					labelled("two steps", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.BreadcrumbProps{Steps: []string{"Workspace", "Billing"}}.Layout(c, &deep)
					})),
					note("last pressed: "+pressed),
				)(c)
			}
		},
	}
}

// paginationEntry draws the pager over a few pages and over many.
//
// Both, because the only logic in a pager is what it does when there are more
// numbers than room: a row of forty is not navigation, and the gap that stands
// in for the middle of them is the whole control.
func paginationEntry() Entry {
	return Entry{
		Control: "PaginationProps",
		Summary: "A pager that shows every page when there are few and the ends when there are many.",
		Build: func() ayra.Widget {
			var few, many, narrow widget.Pages
			many.Show(18)
			narrow.Show(9)

			return func(c ayra.Context) ayra.Dimensions {
				return stack(16,
					labelled("five pages", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.PaginationProps{Total: 5}.Layout(c, &few)
					})),
					labelled("forty, with a gap standing for the middle", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.PaginationProps{Total: 40}.Layout(c, &many)
					})),
					labelled("one page either side, in a narrow column", sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.PaginationProps{Total: 40, Window: 1}.Layout(c, &narrow)
					})),
					note("on page "+strconv.Itoa(many.Current())+" of forty"),
				)(c)
			}
		},
	}
}

// toolbarEntry draws the row above the thing it commands, with and without its
// rule.
func toolbarEntry() Entry {
	return Entry{
		Control: "ToolbarProps",
		Summary: "A row of controls above what they act on, and the room around it.",
		Build: func() ayra.Widget {
			var search, add widget.Input
			var divided, plainAdd widget.Button

			return func(c ayra.Context) ayra.Dimensions {
				return stack(18,
					labelled("with the rule", sized(480, func(c ayra.Context) ayra.Dimensions {
						return widget.ToolbarProps{Divided: true}.Layout(c,
							func(c ayra.Context) ayra.Dimensions {
								return widget.InputProps{Placeholder: "Search"}.Layout(c, &search)
							},
							func(c ayra.Context) ayra.Dimensions {
								return widget.ButtonProps{Label: "Add", Size: widget.Small}.Layout(c, &divided)
							},
						)
					})),
					labelled("without it, for a toolbar inside something already bordered", framed(func(c ayra.Context) ayra.Dimensions {
						return widget.ToolbarProps{}.Layout(c,
							func(c ayra.Context) ayra.Dimensions {
								return widget.InputProps{Placeholder: "Search"}.Layout(c, &add)
							},
							func(c ayra.Context) ayra.Dimensions {
								return widget.ButtonProps{Label: "Add", Size: widget.Small}.Layout(c, &plainAdd)
							},
						)
					})),
				)(c)
			}
		},
	}
}

// treeEntry draws the hierarchy, open and unavailable.
func treeEntry() Entry {
	return Entry{
		Control: "TreeProps",
		Summary: "Rows that open and close, addressed by where they are in the hierarchy.",
		Build: func() ayra.Widget {
			roots := []widget.TreeNode{
				{Label: "app", Children: []widget.TreeNode{
					{Label: "Http", Children: []widget.TreeNode{
						{Label: "Controllers", Detail: "4 files"},
						{Label: "Middleware", Detail: "2 files"},
					}},
					{Label: "Models", Detail: "9 files"},
				}},
				{Label: "resources", Children: []widget.TreeNode{
					{Label: "views"},
				}},
				{Label: "go.mod", Detail: "1.2 kB"},
			}

			var live, off widget.Tree
			live.Expand([]int{0, 0})
			live.Select([]int{0, 0, 1})
			off.Expand([]int{0})

			return func(c ayra.Context) ayra.Dimensions {
				marked := "nothing"
				if path := live.Selected(); len(path) > 0 {
					marked = pathText(path)
				}

				return stack(16,
					flow(24,
						labelled("open, and one row marked", sized(300, func(c ayra.Context) ayra.Dimensions {
							return widget.TreeProps{Roots: roots}.Layout(c, &live)
						})),
						labelled("set in further, and unavailable", sized(300, func(c ayra.Context) ayra.Dimensions {
							return widget.TreeProps{Roots: roots, Indent: 28, Disabled: true}.Layout(c, &off)
						})),
					),
					note("marked: "+marked),
				)(c)
			}
		},
	}
}

// pathText writes a tree path the way the control addresses it, which is the
// child index at each level rather than the row's place on screen.
func pathText(path []int) string {
	written := ""
	for index, step := range path {
		if index > 0 {
			written += "."
		}
		written += strconv.Itoa(step)
	}
	return written
}

// paletteEntry draws the list of commands over a box rather than over the
// window.
//
// It covers whatever it is handed, and what it is handed here is the box. Left
// to cover the page it would be a scrim over the catalogue with the palette in
// the middle of it, which is a demonstration nobody can see the rest of.
func paletteEntry() Entry {
	return Entry{
		Control: "PaletteProps",
		Summary: "Everything an application can do, searched by typing.",
		Build: func() ayra.Widget {
			var commands widget.Palette
			var open widget.Button

			names := []string{
				"Open a project", "Close the window", "Sign out",
				"Copy the reference", "Toggle the palette", "Show the log",
			}
			ran := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if open.Clicked(c) {
					commands.Open()
				}
				if index := commands.Chosen(); index >= 0 {
					ran = names[index]
				}

				return stack(14,
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Label: "Open the palette", Variant: widget.Outline}.Layout(c, &open)
					}),
					framed(stage(560, 300, func(c ayra.Context) ayra.Dimensions {
						return widget.PaletteProps{
							Commands:    names,
							Placeholder: "What do you want to do",
							Empty:       "Nothing matches that",
						}.Layout(c, &commands)
					})),
					note("last run: "+ran),
				)(c)
			}
		},
	}
}

// carouselEntry draws the row of slides, one at a time and two at a time.
func carouselEntry() Entry {
	return Entry{
		Control: "CarouselProps",
		Summary: "Slides shown a window at a time, with a step either side and a mark for each position.",
		Build: func() ayra.Widget {
			var single, paired, off widget.Carousel

			slide := func(c ayra.Context, index int) ayra.Dimensions {
				return widget.CardProps{Muted: true}.Layout(c, func(c ayra.Context) ayra.Dimensions {
					return widget.TextProps{
						Content:  "Slide " + strconv.Itoa(index+1),
						Role:     widget.Heading,
						Align:    text.Middle,
						MaxLines: 1,
					}.Layout(c)
				})
			}

			return func(c ayra.Context) ayra.Dimensions {
				return stack(18,
					labelled("one at a time, with steps and marks", sized(460, func(c ayra.Context) ayra.Dimensions {
						return widget.CarouselProps{
							Count:  5,
							Dots:   true,
							Arrows: true,
						}.Layout(c, &single, slide)
					})),
					note("at position "+strconv.Itoa(single.At()+1)),
					labelled("two at a time, and it wraps", sized(460, func(c ayra.Context) ayra.Dimensions {
						return widget.CarouselProps{
							Count:   6,
							PerView: 2,
							Loop:    true,
							Dots:    true,
							Arrows:  true,
							Gap:     16,
						}.Layout(c, &paired, slide)
					})),
					labelled("disabled", sized(460, func(c ayra.Context) ayra.Dimensions {
						return widget.CarouselProps{
							Count:    3,
							Dots:     true,
							Arrows:   true,
							Disabled: true,
						}.Layout(c, &off, slide)
					})),
				)(c)
			}
		},
	}
}
