package widget

import (
	"context"
	"image"
	"slices"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// DataTableMode is how a table lays its rows out.
type DataTableMode uint8

const (
	// ModeAuto draws cards under [DataTableProps.Breakpoint] and the table at
	// and above it. Zero.
	ModeAuto DataTableMode = iota
	// ModeTable always draws the table.
	ModeTable
	// ModeCards always draws the cards.
	ModeCards
)

// RowAction is a button at the end of a row: an icon with a name, the way the web
// half draws a row's actions. A press is read with [DataTable.Action].
type RowAction struct {
	// Key names the action to the caller. It is what [DataTable.Action] reports.
	Key string
	// Label names the action, for a screen reader and for the case where no
	// icon is given.
	Label string
	// Icon draws the glyph, in a square the size of a thumb. Nil draws the label
	// as a text button: the icons of the library are a separate piece, and an
	// application that has them hands one here.
	Icon ayra.Widget
	// Destructive draws the action in the colour of a consequence.
	Destructive bool
}

// Action reports the row action that was pressed, once, and consumes it: the key
// of the row and the key of the action. ok is false when none was.
func (d *DataTable) Action() (row, action string, ok bool) {
	pressed := d.m.acted
	d.m.acted = [2]string{}
	return pressed[0], pressed[1], pressed[1] != ""
}

// Rows are the rows on screen now: the page, or every page an infinite list has
// loaded.
func (d *DataTable) Rows() []Row {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.result.Rows)
}

// Total is how many rows the whole query matches, as the source said; negative
// when it does not count and zero before the first answer.
func (d *DataTable) Total() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.result.Total
}

// State is the query the table is showing as the same state Kyse's DataTable
// draws, under the given parameter names. defaultSize is the page size the
// address leaves out. Its Values are the address of the next step.
func (d *DataTable) State(names DataTableParams, defaultSize int) DataTableValues {
	return d.query.State(names, defaultSize)
}

// tableExtras is what the narrow layout and the things around it hold. It is
// a field of [DataTable] and never a type a caller sees.
type tableExtras struct {
	// pages is how many pages an infinite list holds. It is one for a paged
	// table. more is true while the next page is being appended.
	pages int
	more  bool
	// last is how many rows the last answer brought, which is how an infinite
	// list whose source does not count knows it reached the end.
	last int
	// refreshing is true from a pull until its answer.
	refreshing bool

	// dirty is a notice that the rows changed, waiting for the next frame.
	dirty  atomic.Bool
	redraw func()
	stop   context.CancelFunc

	acted [2]string

	sheet     Dialog
	opener    Button
	apply     Button
	reset     Button
	close     Button
	loaded    bool
	toggle    map[string]*Toggle
	typed     map[string]*Input
	span      map[string]*DateRange
	sorts     []Button
	seen      map[string]string
	seenAt    map[string]time.Time
	sheetList layout.List

	list     layout.List
	loadMore Button
	pull     pull
	actions  [][]Button
}

// narrow says whether the cards are drawn instead of the table.
func (p DataTableProps) narrow(c ayra.Context) bool {
	switch p.Mode {
	case ModeTable:
		return false
	case ModeCards:
		return true
	}
	breakpoint := p.Breakpoint
	if breakpoint == 0 {
		breakpoint = 600
	}
	return c.Constraints.Max.X < c.Dp(breakpoint)
}

// follow reads what arrived outside the frame -- a live notice -- and what the
// list's end asks for, before anything is drawn.
func (e *tableExtras) follow(c ayra.Context, p DataTableProps, d *DataTable) {
	d.mu.Lock()
	e.redraw = c.Redraw
	d.mu.Unlock()
	if e.dirty.Swap(false) {
		d.mu.Lock()
		waiting := d.loading
		d.mu.Unlock()
		if !waiting {
			d.ask(c, p, fetchSilent)
		}
	}
}

// subscribe starts the live notices.
func (e *tableExtras) start(c ayra.Context, p DataTableProps, d *DataTable) {
	d.mu.Lock()
	e.redraw = c.Redraw
	d.mu.Unlock()
	e.subscribe(p, d)
}

// hasMore says whether another page exists after the ones held.
func (d *DataTable) hasMore() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.loaded {
		return false
	}
	if d.result.Total >= 0 {
		return len(d.result.Rows) < d.result.Total
	}
	return d.m.last >= max(d.query.PageSize, 1)
}

// more asks for the next page of an infinite list.
func (d *DataTable) next(c ayra.Context, p DataTableProps) {
	d.mu.Lock()
	busy := d.loading
	d.mu.Unlock()
	if busy || !d.hasMore() {
		return
	}
	d.query.Page = d.m.pages + 1
	d.pages.Show(d.query.Page)
	d.ask(c, p, fetchAppend)
}

// activeFilters counts what narrows the list besides the search.
func (p DataTableProps) activeFilters(state *DataTable) int {
	n := 0
	for _, facet := range p.Facets {
		switch facet.Kind {
		case FacetPeriod:
			if len(state.query.Filters[facet.Key]) > 0 || len(state.query.Filters[facet.EndKey]) > 0 {
				n++
			}
		default:
			n += len(state.query.Filters[facet.Key])
		}
	}
	return n
}

// cards draws the table as a list of cards, for a narrow screen.
func (p DataTableProps) cards(c ayra.Context, state *DataTable) ayra.Dimensions {
	t := c.Theme

	// A press is read before the control is drawn, as every control here is asked:
	// it is the frame after the one that registered it that delivers the event.
	if state.m.opener.Clicked(c) {
		p.openSheet(state)
	}

	state.mu.Lock()
	result, loaded, err, loading := state.result, state.loaded, state.err, state.loading
	more := state.m.more
	state.mu.Unlock()

	for len(state.rows) < len(result.Rows) {
		state.rows = append(state.rows, Button{})
	}
	for len(state.checks) < len(result.Rows) {
		state.checks = append(state.checks, Toggle{})
	}

	bounded := c.Constraints.Max.Y < boundedHeight
	root := t.Resolve("datatable", "root", p.VariantName("default"), theme.Style{}, p.ComponentProps)

	body := func(c ayra.Context) ayra.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.cardsToolbar(c.With(gtx), state) }),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
		}
		list := func(gtx layout.Context) layout.Dimensions {
			return p.cardsList(c.With(gtx), state, result, loaded, err, loading, more, bounded)
		}
		if bounded {
			children = append(children, layout.Flexed(1, list))
		} else {
			children = append(children, layout.Rigid(list))
		}
		if !p.Infinite {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.footer(c.With(gtx), state, result, loaded) }),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
	}

	dims := Surface(c, root, body)
	if bounded {
		dims.Size.Y = c.Constraints.Max.Y
	}
	p.sheetLayout(c, state)
	return dims
}

// boundedHeight is the height under which a table is taken to have been given a
// screen to fill, and above which it has been given a column to grow in.
const boundedHeight = 100_000

// cardsToolbar draws the search field and the button that opens the filters.
func (p DataTableProps) cardsToolbar(c ayra.Context, state *DataTable) ayra.Dimensions {
	t := c.Theme
	style := t.Resolve("datatable", "toolbar", p.VariantName("default"), theme.Style{}, p.ComponentProps)
	filtered := p.activeFilters(state)
	hasSheet := len(p.Facets) > 0 || p.sortable()

	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		children := []layout.FlexChild{}
		if p.Searchable {
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return InputProps{
					ComponentProps: ComponentProps{Custom: p.Custom, Parts: p.Parts},
					widget:         "datatable-search",
					Placeholder:    orDefault(p.Words.Search, "Search"),
				}.Layout(c.With(gtx), &state.search)
			}))
		} else {
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }))
		}
		if hasSheet {
			label := orDefault(p.Words.Filters, "Filters")
			if filtered > 0 {
				label += " (" + strconv.Itoa(filtered) + ")"
			}
			variant := Outline
			if filtered > 0 {
				variant = Secondary
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min = image.Point{}
					return ButtonProps{Label: label, Variant: variant}.Layout(inner, &state.m.opener)
				}),
			)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
	})
}

// sortable says whether any column can be ordered.
func (p DataTableProps) sortable() bool {
	for _, column := range p.Columns {
		if column.Sortable && column.Key != "" {
			return true
		}
	}
	return false
}

// primary is the column that titles a card.
func (p DataTableProps) primary() int {
	for i, column := range p.Columns {
		if column.Primary {
			return i
		}
	}
	return 0
}

// cardsList draws the state of the list -- loading, failed, empty -- or the cards.
func (p DataTableProps) cardsList(c ayra.Context, state *DataTable, result Result, loaded bool, err error, loading, more, bounded bool) ayra.Dimensions {
	t := c.Theme
	switch {
	case err != nil && !loaded:
		return p.failure(c, state, err)
	case !loaded:
		children := make([]layout.FlexChild, 0, 4)
		for i := 0; i < 4; i++ {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return SkeletonProps{}.Layout(c.With(gtx))
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
	case len(result.Rows) == 0:
		empty := t.Resolve("datatable", "empty", p.VariantName("default"), theme.Style{}, p.ComponentProps)
		return Surface(c, empty, func(c ayra.Context) ayra.Dimensions {
			c.Constraints.Min.X = c.Constraints.Max.X
			return layout.Inset{Top: 24, Bottom: 24}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), orDefault(p.Words.Empty, "No results."), unit.Sp(t.Type.Body), t.Colours.MutedForeground, 1, text.Middle, plain())
			})
		})
	}

	n := len(result.Rows)
	hasMore := p.Infinite && state.hasMore()
	card := func(gtx layout.Context, i int) layout.Dimensions {
		return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return p.card(c.With(gtx), state, result.Rows[i], i, loading && !more)
		})
	}
	tail := func(gtx layout.Context) layout.Dimensions {
		return p.tail(c.With(gtx), state, result, hasMore, more, bounded)
	}

	if !bounded {
		children := make([]layout.FlexChild, 0, n+1)
		for i := range result.Rows {
			i := i
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return card(gtx, i) }))
		}
		children = append(children, layout.Rigid(tail))
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
	}

	// The list scrolls inside the room it was given, so a pull from its top and
	// an end that comes into view are both things the table can see.
	state.m.list.Axis = layout.Vertical
	top := state.m.list.Position.First == 0 && state.m.list.Position.Offset == 0
	height := state.m.pull.offset(c, state, p)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.pulled(c.With(gtx), state, height) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			if !p.NoRefresh {
				state.m.pull.watch(c.With(gtx), top)
			}
			dims := state.m.list.Layout(gtx, n+1, func(gtx layout.Context, i int) layout.Dimensions {
				if i == n {
					return tail(gtx)
				}
				return card(gtx, i)
			})
			if !p.NoRefresh {
				state.m.pull.declare(c.With(gtx), size)
			}
			return dims
		}),
	)

	if position := state.m.list.Position; hasMore && position.First+position.Count >= n {
		state.next(c, p)
	}
	return dims
}

// tail is what stands under the last card: the line that says the list ended, the
// indicator of the page on its way, or the button of a list that cannot tell when
// its end is reached.
func (p DataTableProps) tail(c ayra.Context, state *DataTable, result Result, hasMore, loading, bounded bool) ayra.Dimensions {
	t := c.Theme
	switch {
	case loading:
		return layout.Inset{Top: 8, Bottom: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return SkeletonProps{}.Layout(c.With(gtx))
		})
	case hasMore && !bounded:
		inner := c
		inner.Constraints.Min = image.Point{}
		if state.m.loadMore.Clicked(c) {
			state.next(c, p)
		}
		return layout.Inset{Top: 4, Bottom: 8}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return ButtonProps{Label: orDefault(p.Words.LoadMore, "Load more"), Variant: Outline}.Layout(inner.With(gtx), &state.m.loadMore)
		})
	case p.Infinite && !hasMore:
		count := ""
		if result.Total >= 0 {
			count = p.Words.rows(result.Total)
		} else {
			count = orDefault(p.Words.End, "")
		}
		return layout.Inset{Top: 4, Bottom: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			c.Constraints.Min.X = c.Constraints.Max.X
			return drawText(c.With(gtx), count, unit.Sp(t.Type.Small), t.Colours.MutedForeground, 1, text.Middle, plain())
		})
	}
	return layout.Dimensions{}
}

// failure draws the error of a list that never loaded, with the button that
// asks again.
func (p DataTableProps) failure(c ayra.Context, state *DataTable, err error) ayra.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return AlertProps{Title: orDefault(p.Words.Failed, "The list could not be loaded"), Body: err.Error(), Severity: Failure}.Layout(c.With(gtx))
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return ButtonProps{Label: orDefault(p.Words.Retry, "Try again"), Variant: Outline, Size: Small}.Layout(inner, &state.retry)
		}),
	)
}

// card draws one row as a card: the primary column as its title, the others as
// lines of label and value, and the actions as icons under them.
func (p DataTableProps) card(c ayra.Context, state *DataTable, row Row, index int, inert bool) ayra.Dimensions {
	t := c.Theme
	if inert {
		c.Context = c.Context.Disabled()
	}
	for len(state.m.actions) <= index {
		state.m.actions = append(state.m.actions, nil)
	}
	title := p.primary()

	style := t.Resolve("datatable", "row", p.VariantName("default"), theme.Style{}, p.ComponentProps)
	if !style.Border.IsSet() {
		style.Border = theme.Colour(t.Colours.Border)
	}
	if state.selected[row.Key] && !style.Background.IsSet() {
		style.Background = theme.Colour(t.Colours.Muted)
	}
	if !style.Padding.IsSet() {
		style.Padding = theme.Uniform(12)
	}

	if state.rows[index].Clicked(c) {
		state.activated = row.Key
	}
	if p.Selectable {
		state.checks[index].Set(state.selected[row.Key])
	}

	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		content := func(c ayra.Context) ayra.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.cell(c.With(gtx), row, title, true, false) }),
			}
			for i, column := range p.Columns {
				if i == title || column.CardHidden {
					continue
				}
				i, column := i, column
				children = append(children,
					layout.Rigid(layout.Spacer{Height: 6}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
								return drawText(c.With(gtx), column.Title, unit.Sp(t.Type.Small), t.Colours.MutedForeground, 1, text.Start, plain())
							}),
							layout.Flexed(3, func(gtx layout.Context) layout.Dimensions {
								return p.cell(c.With(gtx), row, i, false, false)
							}),
						)
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
		}

		lead := []layout.FlexChild{}
		if p.Selectable {
			lead = append(lead, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				if state.checks[index].Changed(inner) {
					if state.selected == nil {
						state.selected = map[string]bool{}
					}
					state.selected[row.Key] = state.checks[index].On()
				}
				return layout.Inset{Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return CheckboxProps{}.Layout(inner.With(gtx), &state.checks[index])
				})
			}))
		}
		lead = append(lead, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			rows := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.rows[index].click.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return content(c.With(gtx)) })
				}),
			}
			if len(p.Actions) > 0 {
				rows = append(rows,
					layout.Rigid(layout.Spacer{Height: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						inner := c.With(gtx)
						inner.Constraints.Min = image.Point{}
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return p.actions(inner.With(gtx), state, row, index) })
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		}))
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(c.Context, lead...)
	})
}

// cell draws one value of a row: the widget the column gives it, or its text.
func (p DataTableProps) cell(c ayra.Context, row Row, column int, title, table bool) ayra.Dimensions {
	t := c.Theme
	if column < len(p.Columns) && p.Columns[column].Cell != nil {
		if draw := p.Columns[column].Cell(row); draw != nil {
			c.Constraints.Min = image.Point{}
			return draw(c)
		}
	}
	value := ""
	if column < len(row.Cells) {
		value = row.Cells[column]
	}
	face, size := plain(), unit.Sp(t.Type.Body)
	if title {
		face = semibold()
	} else if column < len(p.Columns) && p.Columns[column].Numeric {
		face = mono()
	}
	maxLines := 1
	if title {
		maxLines = 2
	}
	align := text.Start
	if table && column < len(p.Columns) {
		align = p.Columns[column].Align.alignment()
	}
	return drawText(c, value, size, t.Colours.Foreground, maxLines, align, face)
}

// actionsWidth is the room the actions of a row keep at the end of it in the table,
// the same for every row and for the heading over them, so the columns line up:
// a square for an icon, a button for a label.
func (p DataTableProps) actionsWidth(c ayra.Context) int {
	width := 0
	for _, action := range p.Actions {
		if action.Icon != nil {
			width += c.Dp(40)
		} else {
			width += c.Dp(92)
		}
	}
	return width
}

// cells draws the values of a row in the table's columns, with the widgets of the
// columns that have one.
func (p DataTableProps) cells(c ayra.Context, grid TableProps, row Row) ayra.Dimensions {
	custom := false
	for _, column := range p.Columns {
		custom = custom || column.Cell != nil
	}
	if !custom {
		return grid.row(c, row.Cells, 8, false)
	}
	children := make([]layout.FlexChild, 0, len(p.Columns))
	for i, column := range p.Columns {
		i, column := i, column
		share := column.Flex
		if share == 0 {
			share = 1
		}
		children = append(children, layout.Flexed(share, func(gtx layout.Context) layout.Dimensions {
			// The cell takes the share it was allotted whatever it draws, so the
			// columns of every row stand under their headings: a flex places the
			// next child after what this one took and not after what it was given.
			share := gtx.Constraints.Max.X
			gtx.Constraints.Min.X = share
			dims := layout.Inset{Top: 8, Bottom: 8, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = 0
				place := layout.W
				switch column.Align {
				case End:
					place = layout.E
				case Middle:
					place = layout.Center
				}
				return place.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return p.cell(c.With(gtx), row, i, false, true)
				})
			})
			dims.Size.X = share
			return dims
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
}

// actions draws the row's action buttons and records a press.
func (p DataTableProps) actions(c ayra.Context, state *DataTable, row Row, index int) ayra.Dimensions {
	for len(state.m.actions) <= index {
		state.m.actions = append(state.m.actions, nil)
	}
	for len(state.m.actions[index]) < len(p.Actions) {
		state.m.actions[index] = append(state.m.actions[index], Button{})
	}
	children := make([]layout.FlexChild, 0, len(p.Actions))
	for i, action := range p.Actions {
		i, action := i, action
		button := &state.m.actions[index][i]
		if button.Clicked(c) {
			state.m.acted = [2]string{row.Key, action.Key}
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.action(c.With(gtx), button, action)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
}

// action draws one action: a square the size of a thumb holding the icon.
func (p DataTableProps) action(c ayra.Context, button *Button, action RowAction) ayra.Dimensions {
	t := c.Theme
	ink := t.Colours.Foreground
	if action.Destructive {
		ink = t.Colours.Destructive
	}
	if action.Icon == nil {
		variant := Ghost
		if action.Destructive {
			variant = Destructive
		}
		c.Constraints.Min = image.Point{}
		return ButtonProps{Label: action.Label, Variant: variant, Size: Small}.Layout(c, button)
	}
	side := c.Dp(40)
	return button.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(image.Pt(side, side))
		fill := ink
		if button.Hovered() || button.Pressed() {
			fill = Fade(ink)
		}
		inner := c.With(gtx)
		inner.Theme.Colours.Foreground = fill
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			inner := inner.With(gtx)
			inner.Constraints.Min = image.Point{}
			return action.Icon(inner)
		})
	})
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// pull is the gesture of pulling the list down from its top to ask again.
type pull struct {
	tag      bool
	pressed  bool
	armed    bool
	origin   f32.Point
	distance float32
	fired    bool
}

// pullThreshold is how far, in points, a pull has to go to ask.
const pullThreshold = 72

// declare puts the handler over the list, above it so that it is the first to be
// reached, and passes every event on so that the list still gets its own.
func (g *pull) declare(c ayra.Context, size image.Point) {
	area := clip.Rect(image.Rectangle{Max: size}).Push(c.Ops)
	pass := pointer.PassOp{}.Push(c.Ops)
	event.Op(c.Ops, g)
	pass.Pop()
	area.Pop()
}

// watch follows a pull from the top of the list. It takes the pointer only for a
// drag that began at the top of the list and went down; every other drag is the
// list's. It runs before the list reads the same events, so that the pointer is
// taken from it and not the other way round.
func (g *pull) watch(c ayra.Context, atTop bool) {
	for {
		ev, ok := c.Event(pointer.Filter{Target: g, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			g.pressed, g.armed, g.origin, g.distance = true, atTop, e.Position, 0
		case pointer.Drag:
			if g.pressed && g.armed {
				diff := e.Position.Sub(g.origin)
				if e.Priority < pointer.Grabbed && diff.Y > float32(c.Dp(10)) && diff.Y > abs32(diff.X) {
					// A drag that begins at the top and goes down is a pull, and is
					// ours from here: taken before the list reads the same events, so
					// the list does not take it to scroll. One that goes up is left
					// to the list, which takes it as it always did.
					c.Execute(pointer.GrabCmd{Tag: g, ID: e.PointerID})
				}
				g.distance = max(diff.Y, 0) / c.Metric.PxPerDp
			}
		case pointer.Release, pointer.Cancel:
			// A cancel is the pointer being taken from us, so only a release counts.
			if e.Kind == pointer.Release && g.pressed && g.armed && g.distance >= pullThreshold {
				// The finger is up and no input will draw the next frame, which is the
				// one that asks.
				g.fired = true
				c.Redraw()
			}
			g.pressed, g.armed, g.distance = false, false, 0
		}
	}
}

// offset is how tall the indicator over the list is: the pull as it is made, or
// the room the refreshing line keeps while the answer is on its way. It also
// turns a finished pull into a request.
func (g *pull) offset(c ayra.Context, state *DataTable, p DataTableProps) int {
	if g.fired {
		g.fired = false
		state.mu.Lock()
		state.m.refreshing = true
		state.mu.Unlock()
		state.ask(c, p, fetchSilent)
	}
	state.mu.Lock()
	refreshing := state.m.refreshing
	state.mu.Unlock()
	switch {
	case refreshing:
		return c.Dp(36)
	case g.distance > 0:
		return c.Dp(unit.Dp(min(g.distance*0.5, pullThreshold/2)))
	}
	return 0
}

// pulled draws the line that opens over the list while it is pulled.
func (p DataTableProps) pulled(c ayra.Context, state *DataTable, height int) ayra.Dimensions {
	if height <= 0 {
		return layout.Dimensions{}
	}
	state.mu.Lock()
	refreshing := state.m.refreshing
	state.mu.Unlock()
	words := orDefault(p.Words.Refresh, "Pull to refresh")
	switch {
	case refreshing:
		words = orDefault(p.Words.Refreshing, "Refreshing...")
	case state.m.pull.distance >= pullThreshold:
		words = orDefault(p.Words.Release, "Release to refresh")
	}
	size := image.Pt(c.Constraints.Max.X, height)
	c.Constraints = layout.Exact(size)
	layout.Center.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return drawText(c.With(gtx), words, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Middle, plain())
	})
	return layout.Dimensions{Size: size}
}
