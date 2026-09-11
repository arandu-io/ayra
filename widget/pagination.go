package widget

import (
	"image"
	"strconv"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Pages is the state half of a pager: which page, and the presses.
type Pages struct {
	current  int
	previous Button
	next     Button
	numbers  []Button
}

// Current is the page showing, counting from one.
func (p *Pages) Current() int { return max(p.current, 1) }

// Show moves to a page, for the state a screen arrives with.
func (p *Pages) Show(page int) { p.current = page }

// PaginationProps is a pager over a fixed number of pages.
//
// It shows every page when there are few and the ends with a gap when there
// are many, which is the only part of a pager with any logic in it: a row of
// two hundred numbers is not navigation.
type PaginationProps struct {
	// Total is how many pages there are.
	Total int
	// Window is how many numbers to show around the current one. Zero shows
	// two either side.
	Window int
}

// Layout draws the pager and returns the room it took.
func (p PaginationProps) Layout(c ayra.Context, state *Pages) ayra.Dimensions {
	if state.current < 1 {
		state.current = 1
	}
	if p.Total > 0 && state.current > p.Total {
		state.current = p.Total
	}

	for len(state.numbers) < p.Total {
		state.numbers = append(state.numbers, Button{})
	}

	if state.previous.Clicked(c) && state.current > 1 {
		state.current--
	}
	if state.next.Clicked(c) && state.current < p.Total {
		state.current++
	}
	for index := range state.numbers[:min(len(state.numbers), p.Total)] {
		if state.numbers[index].Clicked(c) {
			state.current = index + 1
		}
	}

	c.Constraints.Min = image.Point{}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.step(c.With(gtx), &state.previous, "Previous", state.current > 1)
		}),
	}

	for _, page := range p.visible(state.current) {
		page := page
		children = append(children, layout.Rigid(layout.Spacer{Width: 4}.Layout))

		if page == 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				return layout.Inset{Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return drawText(c.With(gtx), "...", unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Middle, plain())
				})
			}))
			continue
		}

		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}

			variant := Ghost
			if page == state.current {
				variant = Secondary
			}
			return ButtonProps{Label: strconv.Itoa(page), Variant: variant, Size: Small}.
				Layout(inner, &state.numbers[page-1])
		}))
	}

	children = append(children,
		layout.Rigid(layout.Spacer{Width: 4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.step(c.With(gtx), &state.next, "Next", state.current < p.Total)
		}),
	)

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
}

// step draws one of the two ends.
func (p PaginationProps) step(c ayra.Context, button *Button, label string, available bool) ayra.Dimensions {
	c.Constraints.Min = image.Point{}
	return ButtonProps{Label: label, Variant: Outline, Size: Small, Disabled: !available}.Layout(c, button)
}

// visible answers the pages to draw, with 0 standing for a gap.
//
// The ends are always shown, because "page one" and "the last page" are the two
// a person looks for and neither is reachable by pressing next.
func (p PaginationProps) visible(current int) []int {
	window := p.Window
	if window == 0 {
		window = 2
	}
	if p.Total <= 0 {
		return nil
	}

	pages := make([]int, 0, p.Total)
	for page := 1; page <= p.Total; page++ {
		near := page >= current-window && page <= current+window
		if page == 1 || page == p.Total || near {
			pages = append(pages, page)
			continue
		}
		// One gap stands for any run of pages, and two gaps never sit beside
		// each other: a row reading "1 ... ... 40" says nothing the first one
		// did not.
		if len(pages) > 0 && pages[len(pages)-1] != 0 {
			pages = append(pages, 0)
		}
	}
	return pages
}
