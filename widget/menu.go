package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
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
	// chosen is the entry a press picked, and picked says whether there is one.
	//
	// Two fields rather than a sentinel in one, because zero is an entry: the
	// first. A menu that stored "nothing chosen" as zero reported its first
	// action as taken on the frame it was first drawn, and whatever that action
	// does happened before anybody pressed anything. It was written with a
	// guard that was meant to seed the sentinel and could not -- it asked
	// whether the entry list was empty, and by then the list had just been
	// filled in -- and with the one caller that noticed building its menus by
	// hand with the sentinel already in them.
	chosen int
	picked bool

	// barrier is the target behind the panel that reads a press outside it, and
	// hit is the panel itself. Two separate targets for the reason the dialog's
	// are: "was this press outside" is a question only the two of them together
	// can answer.
	barrier gesture.Click
	hit     gesture.Click

	// anchor is declared over the control the panel opens from, and is what the
	// router is asked where the control is. scroll is the column of the panel,
	// for one taller than the room there is.
	anchor byte
	scroll layout.List
}

// Showing reports whether the list is open.
func (m *Menu) Showing() bool { return m.open.Showing() }

// Open shows the list, for a screen that opens it by some other means than the
// press on its own control -- a keyboard shortcut, or a gesture somewhere else.
//
// It exists as the other half of Close. A control that could be closed and not
// opened is one a caller has to reach around, and reaching around a control is
// how a second way to do the same thing begins.
func (m *Menu) Open() { m.open.Open() }

// Close hides it, for a screen that acted on a choice.
func (m *Menu) Close() { m.open.Close() }

// Chosen reports which entry was picked since the last frame, once, and -1 when
// none was.
//
// The press that set it has already closed the list, because every entry is an
// action and a menu that stayed open over what the action just changed is a
// menu somebody has to dismiss before they can see the result.
func (m *Menu) Chosen() int {
	if !m.picked {
		return -1
	}
	m.picked = false
	return m.chosen
}

// MenuProps is a list of actions under a control.
type MenuProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Entries are the actions, in order. An empty string is a divider: a menu
	// long enough to need grouping is one where the groups are the only way
	// anybody finds anything.
	Entries []string
	// Width is how wide the list is. Zero takes the room it is offered, and
	// never less than the floor a one-word action is legible at.
	Width unit.Dp
	// Above opens it upwards, for a control near the bottom of a window.
	Above bool
	// End lines the list's right edge up with the control's, for a control at
	// the right edge of the window -- the "..." at the end of a row. Without it
	// the list starts at the control's left edge and runs off the screen.
	End bool

	// modeless leaves out the barrier: a menubar's titles stay pressable while
	// one of its menus is open, which is how a desktop menubar moves between
	// menus.
	modeless bool

	// ticked marks the entries that are on, by index, with the tick Kyse draws
	// at the end of a row of a filter menu. A widget that offers several choices
	// from a menu sets it; the menu itself is a list of actions.
	ticked func(index int) bool
	// tickOwner is the Kyse component whose tick it is.
	tickOwner string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p MenuProps) PartNames() []string { return []string{"root"} }

// Layout draws the control that opens the menu, and the list while it is open.
func (p MenuProps) Layout(c ayra.Context, state *Menu, trigger ayra.Widget) ayra.Dimensions {
	return withRoot(c, "menu", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state, trigger)
	})
}

// layout is Layout without the root part around it.
func (p MenuProps) layout(c ayra.Context, state *Menu, trigger ayra.Widget) ayra.Dimensions {
	for len(state.entries) < len(p.Entries) {
		state.entries = append(state.entries, Button{})
	}
	state.open.Changed(c)
	state.dismiss(c, !p.modeless)
	for index := range p.Entries {
		if p.Entries[index] == "" {
			continue
		}
		if state.entries[index].Clicked(c) {
			state.chosen, state.picked = index, true
			state.open.Close()
		}
	}

	dims := state.open.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return trigger(c.With(gtx))
	})
	state.declareAnchor(c, dims)

	if state.open.Showing() {
		p.list(c, state, dims)
	} else {
		state.scroll.Position = layout.Position{}
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
			var tick ayra.Widget
			if p.ticked != nil && p.ticked(index) {
				tick = func(c ayra.Context) ayra.Dimensions {
					return drawInner(c, innerIcon(p.tickOwner, "Check"), innerPixels(c, innerSide), c.Theme.Colours.Foreground, 0)
				}
			}
			return ItemProps{Title: entry, Pressable: true}.Layout(c.With(gtx), &state.entries[index], tick)
		}))
	}

	inner := c
	if p.Width != 0 {
		inner.Constraints.Max.X = c.Dp(p.Width)
	}
	inner.Constraints.Min.X = min(c.Dp(unit.Dp(180)), inner.Constraints.Max.X)

	column := func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
	}
	build := func(maxHeight int) (op.CallOp, image.Point) {
		return state.panel(inner, 4, maxHeight, column)
	}
	state.float(c, trigger.Size, placement{gap: c.Dp(unit.Dp(4)), above: p.Above, end: p.End, barrier: !p.modeless}, build)
}

// panel records the box a list or a popover is drawn in, and answers it with its
// size. A maxHeight over zero holds it to that height, with what is in it
// scrolling: a list of forty entries, or a calendar, on a screen that has room
// for ten.
func (m *Menu) panel(c ayra.Context, inset unit.Dp, maxHeight int, content ayra.Widget) (op.CallOp, image.Point) {
	if maxHeight > 0 {
		c.Constraints.Max.Y = maxHeight
	} else if c.Overlay != nil {
		// The room the control's parent had left under it is not the panel's:
		// the panel floats over what follows, and was cut to a sliver by a form
		// whose last field had nothing under it. It is as tall as the window.
		c.Constraints.Max.Y = c.Overlay.Safe.Dy()
	}
	recording := op.Record(c.Ops)
	size := surface(c, c.Theme.Colours.Popover, c.Theme.Colours.Border, controlRadius(c), func(c ayra.Context) ayra.Dimensions {
		return layout.UniformInset(inset).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			if maxHeight > 0 {
				return scrolled(&m.scroll, c.With(gtx), content)
			}
			return content(c.With(gtx))
		})
	}).Size
	return recording.Stop(), size
}

// declareAnchor registers, over the control the panel opens from, the tag the
// router is asked about.
func (m *Menu) declareAnchor(c ayra.Context, trigger ayra.Dimensions) {
	// Pass-through: a handler that wants no events is still a handler, and one
	// over the control would stop the press the control is waiting for.
	pass := pointer.PassOp{}.Push(c.Ops)
	area := clip.Rect(image.Rectangle{Max: trigger.Size}).Push(c.Ops)
	event.Op(c.Ops, &m.anchor)
	area.Pop()
	pass.Pop()
}

// placement is where a floating panel goes in relation to its control.
type placement struct {
	// gap is the room between the control and the panel, in pixels.
	gap int
	// above asks for the panel over the control; end for its right edge on the
	// control's.
	above, end bool
	// barrier puts a target under the panel that reads a press outside it.
	barrier bool
}

// farAway is how far the barrier reaches in every direction, in pixels: further
// than any window, because a control does not know where in the window it is.
const farAway = 1 << 20

// dismiss reads what puts away a list that is open: escape, the back key, and a
// press that landed outside it. A press outside is taken and not passed on, as
// Android's own menus do: the press that closes a menu is not also a press on
// whatever was under the finger.
func (m *Menu) dismiss(c ayra.Context, barrier bool) {
	if !m.open.Showing() {
		return
	}
	for {
		e, ok := c.Event(
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameBack},
		)
		if !ok {
			break
		}
		if press, isKey := e.(key.Event); isKey && press.State == key.Press {
			m.open.Close()
		}
	}
	for barrier {
		if _, ok := m.barrier.Update(c.Source); !ok {
			break
		}
		m.open.Close()
	}
	for {
		if _, ok := m.hit.Update(c.Source); !ok {
			break
		}
	}
}

// float puts the panel over the screen beside its control, and the barrier under
// it.
//
// It is deferred to the end of the frame: drawn in layout order, the next row of
// a list or the next paragraph was drawn over the list and took the presses
// meant for it. The offset is from where the control is, and the transform in
// force when it is called is the control's, which is what Defer keeps.
//
// Where the control is in the window is asked of the router, which knows it from
// the last frame, and the panel is placed by it: under the control when there is
// room, over it when there is not and there is more there, held to the room that
// is left with what is in it scrolling, and kept off the sides of the window.
// With no window to ask -- no Overlay -- it is where it was asked to be, at its
// natural size.
func (m *Menu) float(c ayra.Context, trigger image.Point, pl placement, build func(maxHeight int) (op.CallOp, image.Point)) {
	panel, size := build(0)
	left := 0
	if pl.end {
		left = trigger.X - size.X
	}
	above := pl.above

	if bounds, ok := m.anchorBounds(c); ok {
		safe := c.Overlay.Safe
		below := safe.Max.Y - bounds.Max.Y - pl.gap
		over := bounds.Min.Y - safe.Min.Y - pl.gap

		fits := func(room int) bool { return size.Y <= room }
		switch {
		case above && !fits(over) && fits(below):
			above = false
		case !above && !fits(below) && (fits(over) || over > below):
			above = true
		}
		room := below
		if above {
			room = over
		}
		if size.Y > room && room > 0 {
			panel, size = build(room)
			if pl.end {
				left = trigger.X - size.X
			}
		}

		if bounds.Min.X+left+size.X > safe.Max.X {
			left = safe.Max.X - size.X - bounds.Min.X
		}
		if bounds.Min.X+left < safe.Min.X {
			left = safe.Min.X - bounds.Min.X
		}
	}

	top := trigger.Y + pl.gap
	if above {
		top = -size.Y - pl.gap
	}

	floating := op.Record(c.Ops)
	if pl.barrier {
		area := clip.Rect(image.Rect(-farAway, -farAway, farAway, farAway)).Push(c.Ops)
		m.barrier.Add(c.Ops)
		event.Op(c.Ops, &m.barrier)
		area.Pop()
	}
	offset := op.Offset(image.Pt(left, top)).Push(c.Ops)
	clipped := clip.Rect(image.Rectangle{Max: size}).Push(c.Ops)
	m.hit.Add(c.Ops)
	event.Op(c.Ops, &m.hit)
	panel.Add(c.Ops)
	clipped.Pop()
	offset.Pop()
	op.Defer(c.Ops, floating.Stop())
}

// anchorBounds is where the control the panel opens from is, in the coordinates
// the screen is drawn in, and false when the window cannot say.
func (m *Menu) anchorBounds(c ayra.Context) (image.Rectangle, bool) {
	if c.Overlay == nil {
		return image.Rectangle{}, false
	}
	bounds, ok := c.Source.Bounds(&m.anchor)
	if !ok {
		return image.Rectangle{}, false
	}
	// The frame is written offset by the system bars, and the screen from the
	// corner of what they leave.
	return bounds.Add(c.Overlay.Window.Min), true
}

// PopoverProps is a panel that appears beside the control that opened it.
//
// It differs from a menu in what it holds: a menu is a list of actions and this
// is anything at all -- a form, a preview, a set of filters. What they share is
// where they sit, which is why the anchoring is written once.
type PopoverProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Width is how wide the panel is. Zero takes one a short form reads at.
	Width unit.Dp
	// Above opens it upwards.
	Above bool
	// End lines the panel's right edge up with the control's, for a control at
	// the right edge of the window.
	End bool

	// light leaves out the barrier, for a panel that is open while the pointer
	// rests on its control: the barrier would cover the control, and a control
	// that is covered is no longer hovered.
	light bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p PopoverProps) PartNames() []string { return []string{"root"} }

// Layout draws the control and, while it is open, the panel by it.
func (p PopoverProps) Layout(c ayra.Context, state *Menu, trigger, content ayra.Widget) ayra.Dimensions {
	return withRoot(c, "popover", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state, trigger, content)
	})
}

// layout is Layout without the root part around it.
func (p PopoverProps) layout(c ayra.Context, state *Menu, trigger, content ayra.Widget) ayra.Dimensions {
	state.open.Changed(c)
	state.dismiss(c, !p.light)

	dims := state.open.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return trigger(c.With(gtx))
	})
	state.declareAnchor(c, dims)
	if !state.open.Showing() {
		state.scroll.Position = layout.Position{}
		return dims
	}

	width := p.Width
	if width == 0 {
		width = 280
	}

	inner := c
	inner.Constraints.Max.X = min(c.Dp(width), c.Constraints.Max.X)
	inner.Constraints.Min.X = inner.Constraints.Max.X

	build := func(maxHeight int) (op.CallOp, image.Point) {
		return state.panel(inner, 12, maxHeight, content)
	}
	state.float(c, dims.Size, placement{gap: c.Dp(unit.Dp(6)), above: p.Above, end: p.End, barrier: !p.light}, build)
	return dims
}

// MenubarProps is a row of menus across the top of a window.
type MenubarProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Titles are the menus, in order.
	Titles []string
	// Entries are the actions under each, in the same order.
	Entries [][]string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p MenubarProps) PartNames() []string { return []string{"root"} }

// Menubar is the state half: one menu per title.
type Menubar struct {
	menus []Menu
}

// Open puts one menu up, as a press on its title does. The others come down on
// the next frame: one is open at a time. It is for a screen that restores a
// menu and for a picture of one open.
func (m *Menubar) Open(index int) {
	if index < 0 {
		return
	}
	for len(m.menus) <= index {
		m.menus = append(m.menus, Menu{})
	}
	m.menus[index].Open()
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
	return withRoot(c, "menubar", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p MenubarProps) layout(c ayra.Context, state *Menubar) ayra.Dimensions {
	for len(state.menus) < len(p.Titles) {
		state.menus = append(state.menus, Menu{})
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
			return MenuProps{Entries: entries, modeless: true}.Layout(inner, &state.menus[index], func(c ayra.Context) ayra.Dimensions {
				return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					return drawText(c.With(gtx), title, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Start, plain())
				})
			})
		}))
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
}
