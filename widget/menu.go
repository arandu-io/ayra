package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Menu is the state half of a list of actions that appears on demand.
type Menu struct {
	open    Disclosure
	entries []Button
	chosen  int
}

// Showing reports whether the list is open.
func (m *Menu) Showing() bool { return m.open.Showing() }

// Close hides it, for a screen that acted on a choice.
func (m *Menu) Close() { m.open.Close() }

// Chosen reports which entry was picked since the last frame, and -1 when none
// was.
//
// Reading it closes the menu, because every entry is an action and a menu that
// stayed open after one covers what the action just changed.
func (m *Menu) Chosen() int {
	chosen := m.chosen
	m.chosen = -1
	return chosen
}

// MenuProps is a list of actions under a control.
type MenuProps struct {
	// Entries are the actions, in order. An empty string is a divider: a menu
	// long enough to need grouping is one where the groups are the only way
	// anybody finds anything.
	Entries []string
	// Width is how wide the list is. Zero fits the longest entry.
	Width unit.Dp
	// Above opens it upwards, for a control near the bottom of a window.
	Above bool
}

// Layout draws the control that opens the menu, and the list while it is open.
func (p MenuProps) Layout(c ayra.Context, state *Menu, trigger ayra.Widget) ayra.Dimensions {
	for len(state.entries) < len(p.Entries) {
		state.entries = append(state.entries, Button{})
	}
	if state.chosen == 0 && len(state.entries) == 0 {
		state.chosen = -1
	}

	state.open.Changed(c)
	for index := range p.Entries {
		if p.Entries[index] == "" {
			continue
		}
		if state.entries[index].Clicked(c) {
			state.chosen = index
			state.open.Close()
		}
	}

	dims := state.open.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return trigger(c.With(gtx))
	})

	if state.open.Showing() {
		p.list(c, state, dims)
	}
	return dims
}

// list draws the entries beside the control that opened them.
func (p MenuProps) list(c ayra.Context, state *Menu, trigger ayra.Dimensions) {
	children := make([]layout.FlexChild, 0, len(p.Entries))
	for index, entry := range p.Entries {
		index, entry := index, entry

		if entry == "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return SeparatorProps{}.Layout(c.With(gtx))
				})
			}))
			continue
		}

		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ItemProps{Title: entry, Pressable: true}.Layout(c.With(gtx), &state.entries[index], nil)
		}))
	}

	inner := c
	if p.Width != 0 {
		inner.Constraints.Max.X = c.Dp(p.Width)
	}
	inner.Constraints.Min.X = min(c.Dp(unit.Dp(180)), inner.Constraints.Max.X)

	panel := op.Record(c.Ops)
	size := surface(inner, inner.Theme.Colours.Popover, inner.Theme.Colours.Border, controlRadius(inner), func(c ayra.Context) ayra.Dimensions {
		return layout.UniformInset(4).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(c.With(gtx).Context, children...)
		})
	}).Size
	drawn := panel.Stop()

	top := trigger.Size.Y + c.Dp(unit.Dp(4))
	if p.Above {
		top = -size.Y - c.Dp(unit.Dp(4))
	}

	offset := op.Offset(image.Pt(0, top)).Push(c.Ops)
	clipped := clip.Rect(image.Rectangle{Max: size}).Push(c.Ops)
	drawn.Add(c.Ops)
	clipped.Pop()
	offset.Pop()
}

// PopoverProps is a panel that appears beside the control that opened it.
//
// It differs from a menu in what it holds: a menu is a list of actions and this
// is anything at all -- a form, a preview, a set of filters. What they share is
// where they sit, which is why the anchoring is written once.
type PopoverProps struct {
	// Width is how wide the panel is. Zero takes one a short form reads at.
	Width unit.Dp
	// Above opens it upwards.
	Above bool
}

// Layout draws the control and, while it is open, the panel by it.
func (p PopoverProps) Layout(c ayra.Context, state *Menu, trigger, content ayra.Widget) ayra.Dimensions {
	state.open.Changed(c)

	dims := state.open.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return trigger(c.With(gtx))
	})
	if !state.open.Showing() {
		return dims
	}

	width := p.Width
	if width == 0 {
		width = 280
	}

	inner := c
	inner.Constraints.Max.X = min(c.Dp(width), c.Constraints.Max.X)
	inner.Constraints.Min.X = inner.Constraints.Max.X

	panel := op.Record(c.Ops)
	size := surface(inner, inner.Theme.Colours.Popover, inner.Theme.Colours.Border, controlRadius(inner), func(c ayra.Context) ayra.Dimensions {
		return layout.UniformInset(12).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return content(c.With(gtx))
		})
	}).Size
	drawn := panel.Stop()

	top := dims.Size.Y + c.Dp(unit.Dp(6))
	if p.Above {
		top = -size.Y - c.Dp(unit.Dp(6))
	}

	offset := op.Offset(image.Pt(0, top)).Push(c.Ops)
	clipped := clip.Rect(image.Rectangle{Max: size}).Push(c.Ops)
	drawn.Add(c.Ops)
	clipped.Pop()
	offset.Pop()

	return dims
}

// MenubarProps is a row of menus across the top of a window.
type MenubarProps struct {
	// Titles are the menus, in order.
	Titles []string
	// Entries are the actions under each, in the same order.
	Entries [][]string
}

// Menubar is the state half: one menu per title.
type Menubar struct {
	menus []Menu
}

// Chosen reports which menu and which entry were picked, and -1, -1 when
// nothing was.
func (m *Menubar) Chosen() (menu, entry int) {
	for index := range m.menus {
		if chosen := m.menus[index].Chosen(); chosen >= 0 {
			return index, chosen
		}
	}
	return -1, -1
}

// Layout draws the row and returns the room it took.
func (p MenubarProps) Layout(c ayra.Context, state *Menubar) ayra.Dimensions {
	for len(state.menus) < len(p.Titles) {
		state.menus = append(state.menus, Menu{chosen: -1})
	}

	// One open at a time. Two menus open at once overlap, and the one behind
	// takes presses meant for the one in front.
	for index := range state.menus {
		if !state.menus[index].Showing() {
			continue
		}
		for other := range state.menus {
			if other != index {
				state.menus[other].Close()
			}
		}
		break
	}

	children := make([]layout.FlexChild, 0, len(p.Titles))
	for index, title := range p.Titles {
		index, title := index, title

		var entries []string
		if index < len(p.Entries) {
			entries = p.Entries[index]
		}

		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return MenuProps{Entries: entries}.Layout(inner, &state.menus[index], func(c ayra.Context) ayra.Dimensions {
				return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					return drawText(c.With(gtx), title, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Start, plain())
				})
			})
		}))
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
}
