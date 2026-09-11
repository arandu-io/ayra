package main

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// Catalogue is the window: which page is open, which palette is drawn, and the
// demonstration built for each control.
//
// The chrome is made of the controls it is showing -- the pages are tabs, the
// list down the side is a sidebar, the line saying where you are is a
// breadcrumb, the palette is chosen with a segmented control. That is not a
// flourish. A catalogue drawn with a layout written for the occasion would be
// the one screen in the product whose navigation nobody has used, and the first
// thing wrong with a tab row would be found by somebody else.
type Catalogue struct {
	sections []Section

	// drawn holds each entry's demonstration, built once and kept. A control's
	// state lives in the closure, so building these per frame would be building
	// a control that has never been pressed, every frame.
	drawn [][]ayra.Widget

	// Each page keeps its own scroll and its own mark. One list shared across
	// six pages is a reader who scrolls through Forms, moves to Actions and
	// arrives halfway down it.
	lists []layout.List
	navs  []widget.Sidebar

	pages  widget.Tabs
	scheme widget.Segments
	trail  widget.Crumbs
}

// New builds the catalogue and every demonstration in it.
//
// The scheme is the one the window opens in, and it is passed rather than
// assumed: the window paints its ground from what it was configured with, and a
// catalogue that started on the other palette would repaint the first frame --
// so the application would open showing one scheme and flash the other.
func New(scheme theme.Scheme) *Catalogue {
	found := sections()

	catalogue := &Catalogue{
		sections: found,
		drawn:    make([][]ayra.Widget, len(found)),
		lists:    make([]layout.List, len(found)),
		navs:     make([]widget.Sidebar, len(found)),
	}
	if scheme == theme.Dark {
		catalogue.scheme.Choose(1)
	}

	for page, section := range found {
		catalogue.lists[page].Axis = layout.Vertical
		catalogue.drawn[page] = make([]ayra.Widget, len(section.Entries))
		for index, entry := range section.Entries {
			catalogue.drawn[page][index] = entry.Build()
		}
	}
	return catalogue
}

// Layout draws the whole window.
func (cat *Catalogue) Layout(c ayra.Context) ayra.Dimensions {
	// The palette is chosen here and the ground is painted again under it. The
	// window was already filled with the scheme it opened in, and a switch that
	// only changed the controls would leave a dark page of light text on the
	// ground the window started with.
	scheme := cat.chosen()
	c.Theme = theme.New(scheme)
	c.Fill(c.Theme.Colours.Background)

	// Both presses are read before anything is drawn, so that what they change
	// is on the screen in the frame the press landed in. A tab read afterwards
	// costs a frame, and a frame of lag on navigation reads as a press that was
	// ignored.
	cat.pages.Changed(c)
	page := min(cat.pages.Selected(), len(cat.sections)-1)

	if chosen := cat.navs[page].Chosen(c); chosen >= 0 {
		cat.navs[page].Show(chosen)
		cat.scrollTo(page, chosen)
	}

	dims := layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
			return cat.tabs(c.With(gtx))
		}),
		layout.Flexed(1, func(gtx layout.Context) ayra.Dimensions {
			return cat.body(c.With(gtx), page)
		}),
	)

	// The segmented control was drawn inside that, so a press on it is known
	// only now -- after the frame it would have changed. Asking for another is
	// what puts the other palette on the screen on the press rather than on
	// whatever the person does next.
	if cat.chosen() != scheme {
		c.Redraw()
	}
	return dims
}

// chosen answers the palette the segmented control is set to.
func (cat *Catalogue) chosen() theme.Scheme {
	if cat.scheme.Selected() == 1 {
		return theme.Dark
	}
	return theme.Light
}

// tabs draws the row of pages across the top.
func (cat *Catalogue) tabs(c ayra.Context) ayra.Dimensions {
	names := make([]string, len(cat.sections))
	for index, section := range cat.sections {
		names[index] = section.Name
	}

	return layout.Inset{Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return widget.TabsProps{Labels: names}.Layout(c.With(gtx), &cat.pages)
	})
}

// body draws the page: the controls down the side, and the list beside them.
func (cat *Catalogue) body(c ayra.Context, page int) ayra.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
			return cat.side(c.With(gtx), page)
		}),
		layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
			return widget.SeparatorProps{Vertical: true}.Layout(c.With(gtx))
		}),
		layout.Flexed(1, func(gtx layout.Context) ayra.Dimensions {
			return cat.page(c.With(gtx), page)
		}),
	)
}

// side draws the column of control names, and marks the one the list is at.
func (cat *Catalogue) side(c ayra.Context, page int) ayra.Dimensions {
	section := cat.sections[page]

	names := make([]string, len(section.Entries))
	for index, entry := range section.Entries {
		names[index] = entry.Control
	}

	return layout.Inset{Top: 12, Bottom: 12, Left: 8, Right: 8}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return widget.SidebarProps{
			Title:   section.Name,
			Entries: names,
			Width:   200,
		}.Layout(c.With(gtx), &cat.navs[page])
	})
}

// page draws the toolbar and the scrolling column of entries under it.
func (cat *Catalogue) page(c ayra.Context, page int) ayra.Dimensions {
	return layout.Inset{Left: 20, Right: 20}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		return layout.Flex{Axis: layout.Vertical}.Layout(inner.Context,
			layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return cat.toolbar(inner.With(gtx), page)
			}),
			layout.Flexed(1, func(gtx layout.Context) ayra.Dimensions {
				return cat.entries(inner.With(gtx), page)
			}),
		)
	})
}

// toolbar draws where you are, and what palette you are looking at it in.
func (cat *Catalogue) toolbar(c ayra.Context, page int) ayra.Dimensions {
	section := cat.sections[page]

	at := min(cat.navs[page].Current(), len(section.Entries)-1)
	steps := []string{"Controls", section.Name}
	if at >= 0 {
		steps = append(steps, section.Entries[at].Control)
	}

	return widget.ToolbarProps{Divided: true}.Layout(c,
		func(c ayra.Context) ayra.Dimensions {
			if step := cat.trail.Clicked(c); step >= 0 {
				// The first step is the catalogue itself and the second is this
				// page. Neither is anywhere else to go, so both mean the same
				// thing: back to the top of what you are looking at.
				cat.scrollTo(page, 0)
				cat.navs[page].Show(0)
			}
			return widget.BreadcrumbProps{Steps: steps}.Layout(c, &cat.trail)
		},
		func(c ayra.Context) ayra.Dimensions {
			return widget.SegmentedProps{
				Options: []string{"Light", "Dark"},
				Size:    widget.Small,
			}.Layout(c, &cat.scheme)
		},
	)
}

// entries draws the page's controls, one card each, down a scrolling column.
func (cat *Catalogue) entries(c ayra.Context, page int) ayra.Dimensions {
	section := cat.sections[page]

	// The column is clipped to the room it has. It lays out one entry past each
	// end so that moving focus into it scrolls, and it does not clip those --
	// so whatever they draw outside their own height lands on the page.
	defer clip.Rect{Max: c.Constraints.Max}.Push(c.Ops).Pop()

	// An entry is given the height it asks for and not the height of the
	// window. Capped at the window, an entry taller than one is squashed to it
	// and goes on drawing past the bottom it reported -- and the column, which
	// spaces entries by what each one reported, then draws the next one over
	// the tail of the last. The controls that would take the column's own
	// answer literally -- the ones that cover what they are handed, or centre
	// themselves in it -- are each given a box of their own instead.
	return cat.lists[page].Layout(c.Context, len(section.Entries), func(gtx layout.Context, index int) layout.Dimensions {
		inner := c.With(gtx)

		return layout.Inset{Top: 12, Bottom: 12}.Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
			return cat.card(inner.With(gtx), section.Entries[index], cat.drawn[page][index])
		})
	})
}

// card draws one entry: its name, what it is for, and the control itself.
func (cat *Catalogue) card(c ayra.Context, entry Entry, demonstration ayra.Widget) ayra.Dimensions {
	return widget.CardProps{Padding: 20}.Layout(c, func(c ayra.Context) ayra.Dimensions {
		return stack(14,
			func(c ayra.Context) ayra.Dimensions {
				return widget.TextProps{
					Content:  entry.Control,
					Role:     widget.Heading,
					Bold:     true,
					MaxLines: 1,
				}.Layout(c)
			},
			note(entry.Summary),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SeparatorProps{}.Layout(c)
			},
			demonstration,
		)(c)
	})
}

// scrollTo puts an entry at the top of its page.
//
// The position is written rather than animated: what a press on a name in the
// side column means is "show me that one", and a column that slid there would
// be a column somebody is waiting for.
func (cat *Catalogue) scrollTo(page, index int) {
	cat.lists[page].Position = layout.Position{First: index}
}
