package widget

import (
	"image"
	"strconv"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Pages is the state half of a pager: which page, and the presses.
type Pages struct {
	current  int
	previous Button
	next     Button
	// numbers holds a button for each page in the row, by page, and for no
	// other: the total comes from the server, and a button per page is an
	// allocation the server sizes. shown is the row as it was last drawn,
	// which is the order the buttons are asked in.
	numbers map[int]*Button
	shown   []int
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
	// Window is how many numbers to show either side of the current one, at
	// most: a row that would not fit narrows the window rather than running off
	// the screen. Zero asks for two.
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

	if state.previous.Clicked(c) && state.current > 1 {
		state.current--
	}
	if state.next.Clicked(c) && state.current < p.Total {
		state.current++
	}
	for _, page := range state.shown {
		if button := state.numbers[page]; button != nil && button.Clicked(c) && page <= p.Total {
			state.current = page
		}
	}

	// The window shrinks until the row fits. A pager wider than its column
	// loses whatever is on the right, and what is on the right is Next -- the
	// one control most people use. Nothing about that looks broken: the row
	// just ends after a number.
	window := p.window()
	for window > 0 && p.wider(c, state, window) {
		window--
	}

	c.Constraints.Min = image.Point{}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.step(c.With(gtx), &state.previous, "Previous", state.current > 1)
		}),
	}

	row := p.visible(state.current, window)
	state.hold(row)
	for _, page := range row {
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
				Layout(inner, state.numbers[page])
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

// hold keeps a button for each page in row, and drops the rest.
//
// A page that leaves the row loses its button, and with it any press half made
// on it: the number it stood for is no longer on the screen.
func (p *Pages) hold(row []int) {
	if p.numbers == nil {
		p.numbers = make(map[int]*Button, len(row))
	}
	kept := make(map[int]bool, len(row))
	for _, page := range row {
		if page == 0 {
			continue
		}
		kept[page] = true
		if p.numbers[page] == nil {
			p.numbers[page] = &Button{}
		}
	}
	for page := range p.numbers {
		if !kept[page] {
			delete(p.numbers, page)
		}
	}
	p.shown = append(p.shown[:0], row...)
}

// step draws one of the two ends.
func (p PaginationProps) step(c ayra.Context, button *Button, label string, available bool) ayra.Dimensions {
	c.Constraints.Min = image.Point{}
	return ButtonProps{Label: label, Variant: Outline, Size: Small, Disabled: !available}.Layout(c, button)
}

// window is how many numbers sit either side of the current page.
func (p PaginationProps) window() int {
	if p.Window == 0 {
		return 2
	}
	return p.Window
}

// wider reports whether the row would be wider than the room there is.
//
// It measures rather than counts, because the answer depends on the width of
// the numbers themselves: a pager over nine pages and one over nine hundred
// draw a different number of digits in the same number of slots.
func (p PaginationProps) wider(c ayra.Context, state *Pages, window int) bool {
	available := c.Constraints.Max.X
	if available <= 0 {
		return false
	}

	measure := op.Record(c.Ops)
	inner := c
	inner.Constraints.Min = image.Point{}

	width := ButtonProps{Label: "Previous", Variant: Outline, Size: Small}.Layout(inner, &state.previous).Size.X
	width += ButtonProps{Label: "Next", Variant: Outline, Size: Small}.Layout(inner, &state.next).Size.X

	gap := inner.Dp(unit.Dp(4))
	for _, page := range p.visible(state.current, window) {
		width += gap
		label := "..."
		if page != 0 {
			label = strconv.Itoa(page)
		}
		width += ButtonProps{Label: label, Variant: Ghost, Size: Small}.Layout(inner, &Button{}).Size.X
	}
	measure.Stop()

	return width > available
}

// visible answers the pages to draw, with 0 standing for a gap.
//
// The ends are always shown, because "page one" and "the last page" are the two
// a person looks for and neither is reachable by pressing next. One gap stands
// for any run of pages, and two gaps never sit beside each other: a row reading
// "1 ... ... 40" says nothing the first one did not.
//
// It is computed from the window rather than by walking the pages, because the
// total is the server's: walking it was ten million steps, and an allocation
// of ten million, on every frame of a pager that shows nine numbers.
func (p PaginationProps) visible(current, window int) []int {
	if p.Total <= 0 {
		return nil
	}
	current = min(max(current, 1), p.Total)
	window = max(window, 0)

	low, high := max(current-window, 1), min(current+window, p.Total)
	pages := make([]int, 0, high-low+5)
	if low > 1 {
		pages = append(pages, 1)
		if low > 2 {
			pages = append(pages, 0)
		}
	}
	for page := low; page <= high; page++ {
		pages = append(pages, page)
	}
	if high < p.Total {
		if high < p.Total-1 {
			pages = append(pages, 0)
		}
		pages = append(pages, p.Total)
	}
	return pages
}
