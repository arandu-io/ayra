package widget

import (
	"context"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// Query is what a listing asks of its source: the state the person has put it
// in. It is the same state the web half carries in the address of a listing --
// a search, a sort, a facet, a page and a size -- and the source answers it the
// way the server answers a request, from a whitelist of its own.
type Query struct {
	// Search is the text typed into the search field. Empty is no search.
	Search string
	// Sort is the column the rows are ordered by.
	Sort Sort
	// Filters are the values chosen in each facet, by the facet's key. A facet
	// with none chosen is absent.
	Filters map[string][]string
	// Page counts from one.
	Page int
	// PageSize is how many rows a page holds.
	PageSize int
}

// Sort is an ordering: the key of a column, and which way.
type Sort struct {
	// Key is the [DataColumn.Key]. Empty is the source's own order.
	Key string
	// Descending reverses it.
	Descending bool
}

// Row is one line of the answer.
type Row struct {
	// Key identifies the row to the source, and is what a selection holds. It
	// is the row's identity and not its position, which changes with every sort.
	Key string
	// Cells are the values, in column order.
	Cells []string
	// Data is the row as the source had it, field by field, for a cell that is
	// drawn with a widget of the application's own -- a plate, a status, a badge
	// -- and needs more than the text of its column. A [ServerSource] fills it
	// with the row's object; a source of the application's may leave it nil.
	Data map[string]any
}

// FacetCount is how many rows carry one value of a facet under the rest of the
// query, so that a list of choices can say what each would leave.
type FacetCount struct {
	// Value is the facet's value, as it appears in [Query.Filters].
	Value string
	// Count is how many rows have it.
	Count int
}

// Result is the answer to a [Query].
type Result struct {
	// Rows are the page asked for.
	Rows []Row
	// Total is how many rows match the whole query, across every page. It is what
	// the pager and the count are drawn from, and it is the source's to say: a
	// table that counted what it had been given would count one page. Negative
	// says the source does not count (a route that pages by position): the table
	// then draws no total, offers the next page for as long as a page comes back
	// full, and the infinite list ends at the first short one.
	Total int
	// Facets are the counts per facet key, when the source gives them.
	Facets map[string][]FacetCount
}

// Source is what an application implements to hand a listing its rows. The table
// owns no data and never holds the whole set: it asks for a page, the way the
// web half asks the server for one.
//
// Fetch runs off the frame loop, in its own goroutine, so it may wait on a
// network. The context is cancelled when a newer query supersedes this one, and
// an implementation that honours it stops work nobody will read. Authorization
// is the source's: it answers for the person the application is signed in as,
// and the table has no other way to reach data.
type Source interface {
	Fetch(ctx context.Context, query Query) (Result, error)
}

// SourceFunc is a function as a [Source].
type SourceFunc func(ctx context.Context, query Query) (Result, error)

// Fetch calls the function.
func (f SourceFunc) Fetch(ctx context.Context, query Query) (Result, error) { return f(ctx, query) }

// DataColumn is one column of a [DataTableProps].
type DataColumn struct {
	// Key is the name the column goes by in a [Sort], and the only part of the
	// column the source sees. A column with no key cannot be sorted.
	Key string
	// Title is the heading.
	Title string
	// Sortable lets a press on the heading sort by the column.
	Sortable bool
	// Align, Flex and Numeric are the column's, and mean what they mean in a
	// [Column].
	Align   Align
	Flex    float32
	Numeric bool
	// Primary makes the column the title of a row drawn as a card on a narrow
	// screen. None marked is the first column.
	Primary bool
	// CardHidden leaves the column out of the card, for a value that only makes
	// sense under a heading.
	CardHidden bool
	// Cell draws the column's value with a widget of the application's own. It
	// is asked for each row, and returns the widget that draws that row's cell,
	// so that a plate, a status or a badge is the application's component and
	// not a string. Nil draws the text of the cell.
	Cell func(Row) ayra.Widget
}

// Facet is a list of values a listing can be narrowed by.
type Facet struct {
	// Key is the name the facet goes by in [Query.Filters] and the parameter it
	// travels in, as Kyse's DataTableFilter.Key is.
	Key string
	// Title is what the control that opens it says.
	Title string
	// Kind is how the value is chosen. Zero is a choice among Options.
	Kind FacetKind
	// EndKey is, for a [FacetPeriod], the parameter the last day travels in:
	// Key carries the first ("de") and EndKey the last ("ate"), as two ISO days.
	EndKey string
	// Placeholder is what stands in a text or period field that holds nothing.
	Placeholder string
	// Options are the values on offer, with the words that name them. A text and
	// a period facet have none.
	Options []FacetOption
}

// FacetKind is how a facet's value is chosen.
type FacetKind uint8

const (
	// FacetChoice is a choice among the options, any number of them.
	FacetChoice FacetKind = iota
	// FacetText is a short piece of text the rows are narrowed by.
	FacetText
	// FacetPeriod is a period chosen in one field: two days, travelling as
	// [Facet.Key] and [Facet.EndKey].
	FacetPeriod
)

// FacetOption is one value of a facet.
type FacetOption struct {
	// Value is what the source is sent.
	Value string
	// Label is what is drawn. Empty draws the value.
	Label string
}

// DataTable is the state of a listing that lives on a server: the query, the
// answer to it and the work in flight. The caller holds one per table.
//
// It asks its source for a page when it is first drawn and again each time the
// query changes -- a search typed, a heading pressed, a facet chosen, a page
// turned -- and keeps the last answer on screen, inert, until the next one
// arrives. A query superseded while it is still running is cancelled and its
// answer is discarded, so a slow reply to an old search can never overwrite the
// reply to the new one.
type DataTable struct {
	mu      sync.Mutex
	result  Result
	loaded  bool
	err     error
	loading bool
	cancel  context.CancelFunc
	seq     int

	query     Query
	started   bool
	search    Input
	typedAt   time.Time
	typed     string
	retry     Button
	clear     Button
	headers   []Button
	facets    []Menu
	triggers  []Button
	sizes     Menu
	sizeBtn   Button
	pages     Pages
	selected  map[string]bool
	rows      []Button
	checks    []Toggle
	everyone  Toggle
	activated string

	// m is what the narrow layout, the infinite list, the pull to refresh, the
	// live updates and the row actions hold.
	m tableExtras
}

// Query is the query the table is showing, or is waiting for the answer to.
func (d *DataTable) Query() Query { return d.query }

// Selected is the keys of the rows ticked, in no order. A selection survives a
// search, a sort and a page turn, because it holds keys and not positions.
func (d *DataTable) Selected() []string {
	keys := make([]string, 0, len(d.selected))
	for key, on := range d.selected {
		if on {
			keys = append(keys, key)
		}
	}
	return keys
}

// ClearSelection unticks every row.
func (d *DataTable) ClearSelection() { d.selected = nil }

// Activated reports the key of the row that was pressed, once, and consumes it.
// It is empty when none was.
func (d *DataTable) Activated() string {
	key := d.activated
	d.activated = ""
	return key
}

// Refresh asks the source again for the page on screen, for a table whose data
// changed under it.
func (d *DataTable) Refresh(c ayra.Context, p DataTableProps) { d.fetch(c, p) }

// Loading reports whether an answer is on its way.
func (d *DataTable) Loading() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.loading
}

// Err is the error of the last query, and nil when it succeeded.
func (d *DataTable) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// DataTableProps is a listing that asks a server for its rows -- the counterpart
// of Kyse's DataTable, with the same state: a search, a sort, facets, a page and
// a size, and a selection that outlives them.
type DataTableProps struct {
	// ComponentProps is what the caller changes on this one instance. The table
	// publishes "root", "toolbar", "search", "header", "row", "footer" and
	// "empty".
	ComponentProps
	// Source answers the queries. Nil draws an empty table.
	Source Source
	// Columns are the headings and how each one is drawn.
	Columns []DataColumn
	// Facets are the lists the rows can be narrowed by.
	Facets []Facet
	// Searchable draws the search field.
	Searchable bool
	// Selectable draws a checkbox on each row and in the heading.
	Selectable bool
	// PageSizes are the sizes on offer. Empty is 10, 25 and 50, and the first is
	// the one the table opens with.
	PageSizes []int
	// InitialSort is the order the table opens in.
	InitialSort Sort
	// Debounce is how long the search waits for the next keystroke before it asks.
	// Zero takes 300 ms.
	Debounce time.Duration
	// Words are the table's own text. Zero values draw English.
	Words DataTableWords

	// Layout chooses between the table and the cards. Zero decides by the width
	// the table is given.
	Mode DataTableMode
	// Breakpoint is the width, in points, under which the cards are drawn
	// instead of the table. Zero takes 600, the width of a tablet held upright.
	Breakpoint unit.Dp
	// Infinite replaces the pager by a list that asks for the next page as the
	// end comes into view, in the cards. The table of a wide screen keeps its
	// pager: columns read down are read a page at a time.
	Infinite bool
	// Actions are the buttons at the end of each row: icons in the cards and in
	// the table. A press is read with [DataTable.Action].
	Actions []RowAction
	// Live is told when the rows changed on the server. The table asks again for
	// what it shows, without losing its place. Nil is a table that changes when
	// the person does.
	Live Live
	// NoRefresh turns off the pull to refresh of the cards.
	NoRefresh bool
}

// DataTableWords are the sentences a table says, in the application's language.
type DataTableWords struct {
	Search, Empty, Loading, Retry, Failed, Clear string
	// Filters is the button that opens the sheet of filters, Apply the one that
	// asks with them, Reset the one that empties them and Close the one that
	// puts the sheet away without asking.
	Filters, Apply, Reset, Close, SortBy string
	// Refresh is what the pull says while it is pulled and Refreshing once it
	// asked. LoadMore is the button at the end of a list that cannot tell when
	// it is reached, and End the line under the last row of an infinite list.
	Refresh, Release, Refreshing, LoadMore, End string
	// Actions names the buttons at the end of a row for a screen reader.
	Actions string
	// Rows is the count: "%d row(s)" with the number put where the text has {n}.
	Rows string
	// Page is the position: "Page {page} of {pages}".
	Page string
}

// rows is the count line. The placeholders are the two spellings of the same
// number -- {n}, and {total} as Kyse's DataTableLines.Rows writes it -- so a
// sentence taken from the web half's catalogue works here unchanged.
func (w DataTableWords) rows(n int) string {
	text := strings.ReplaceAll(orDefault(w.Rows, "{total} row(s)."), "{total}", strconv.Itoa(n))
	return strings.ReplaceAll(text, "{n}", strconv.Itoa(n))
}

// page is the position line, with Kyse's placeholders ({n} of {total}) or the
// ones this table began with ({page} of {pages}).
func (w DataTableWords) page(page, pages int) string {
	text := orDefault(w.Page, "Page {n} of {total}")
	text = strings.NewReplacer(
		"{n}", strconv.Itoa(page), "{total}", strconv.Itoa(pages),
		"{page}", strconv.Itoa(page), "{pages}", strconv.Itoa(pages),
	).Replace(text)
	return text
}

// pageCount is how many pages the answer makes: the total over the size, and
// when the source does not count, the page being shown plus one while a page
// comes back full.
func pageCount(result Result, q Query) int {
	size := max(q.PageSize, 1)
	if result.Total >= 0 {
		return max((result.Total+size-1)/size, 1)
	}
	if len(result.Rows) >= size {
		return q.Page + 1
	}
	return max(q.Page, 1)
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p DataTableProps) PartNames() []string {
	return []string{"root", "toolbar", "header", "row", "footer", "empty"}
}

func (p DataTableProps) sizes() []int {
	if len(p.PageSizes) > 0 {
		return p.PageSizes
	}
	return []int{10, 25, 50}
}

// fetch starts a query and supersedes the one in flight.
func (d *DataTable) fetch(c ayra.Context, p DataTableProps) { d.ask(c, p, fetchReplace) }

// fetchMode is what an answer does to the rows already on screen.
type fetchMode uint8

const (
	// fetchReplace puts the answer where the rows were. The rows stay on screen,
	// inert, until it arrives.
	fetchReplace fetchMode = iota
	// fetchAppend puts the answer after the rows: the next page of an infinite
	// list.
	fetchAppend
	// fetchSilent asks again for everything on screen, as the server has it now,
	// and swaps it in when all of it has arrived. Nothing goes inert and nothing
	// moves: it is what a "the table changed" notice asks for.
	fetchSilent
)

func (d *DataTable) ask(c ayra.Context, p DataTableProps, mode fetchMode) {
	if p.Source == nil {
		return
	}
	d.mu.Lock()
	if d.cancel != nil {
		d.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.seq++
	seq := d.seq
	d.loading = mode != fetchSilent
	d.m.more = mode == fetchAppend
	query := d.query
	pages := d.m.pages
	d.mu.Unlock()

	redraw := c.Redraw
	go func() {
		var result Result
		var err error
		if mode == fetchSilent {
			result, err = refetch(ctx, p.Source, query, pages)
		} else {
			result, err = p.Source.Fetch(ctx, query)
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		if seq != d.seq {
			// Superseded while it ran: this answer is for a question nobody is
			// asking any more.
			return
		}
		d.loading, d.m.more, d.m.refreshing = false, false, false
		switch {
		case err != nil && mode == fetchSilent:
			// A notice that could not be followed leaves what is on screen where
			// it is: the rows were right a moment ago, and an error in their
			// place would be worse than a list a second late.
		case err != nil:
			d.err = err
		default:
			if mode != fetchSilent || d.m.pages <= 1 {
				d.m.last = len(result.Rows)
			}
			if mode == fetchAppend {
				result.Rows = append(slices.Clone(d.result.Rows), newRows(d.result.Rows, result.Rows)...)
				d.m.pages++
			} else if mode == fetchReplace {
				d.m.pages = 1
			}
			d.err, d.result, d.loaded = nil, result, true
		}
		redraw()
	}()
}

// Layout draws the table and returns the room it took.
func (p DataTableProps) Layout(c ayra.Context, state *DataTable) ayra.Dimensions {
	t := c.Theme
	if !state.started {
		state.started = true
		state.query = Query{Sort: p.InitialSort, Page: 1, PageSize: p.sizes()[0]}
		state.fetch(c, p)
		state.m.start(c, p, state)
	}
	for len(state.headers) < len(p.Columns) {
		state.headers = append(state.headers, Button{})
	}
	for len(state.facets) < len(p.Facets) {
		state.facets = append(state.facets, Menu{})
		state.triggers = append(state.triggers, Button{})
	}

	changed := p.apply(c, state)
	if p.readFields(c, state) {
		changed = true
	}
	if changed {
		state.query.Page = 1
		state.pages.Show(1)
		state.fetch(c, p)
	}
	if page := state.pages.Current(); page != state.query.Page {
		state.query.Page = page
		state.fetch(c, p)
	}

	state.m.follow(c, p, state)
	if p.narrow(c) {
		return p.cards(c, state)
	}

	state.mu.Lock()
	result, loaded, err, loading := state.result, state.loaded, state.err, state.loading
	state.mu.Unlock()

	root := t.Resolve("datatable", "root", p.VariantName("default"), theme.Style{}, p.ComponentProps)
	return Surface(c, root, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.toolbar(c.With(gtx), state) }),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.table(c.With(gtx), state, result, loaded, err, loading)
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.footer(c.With(gtx), state, result, loaded) }),
		)
	})
}

// apply reads what was pressed or typed this frame and moves the query with it,
// and says whether the rows have to be asked for again from the first page.
func (p DataTableProps) apply(c ayra.Context, state *DataTable) (changed bool) {
	debounce := p.Debounce
	if debounce == 0 {
		debounce = 300 * time.Millisecond
	}

	// The search waits for a pause in the typing: a request per keystroke is
	// what makes a listing flicker and a server work for answers nobody reads.
	if p.Searchable {
		typed := state.search.Text()
		switch {
		case typed != state.typed:
			state.typed, state.typedAt = typed, c.Now
		case typed != state.query.Search && c.Now.Sub(state.typedAt) >= debounce:
			state.query.Search = typed
			changed = true
		case typed != state.query.Search:
			c.Execute(op.InvalidateCmd{At: state.typedAt.Add(debounce)})
		}
	}

	for index, column := range p.Columns {
		if !column.Sortable || column.Key == "" || !state.headers[index].Clicked(c) {
			continue
		}
		switch {
		case state.query.Sort.Key != column.Key:
			state.query.Sort = Sort{Key: column.Key}
		case !state.query.Sort.Descending:
			state.query.Sort.Descending = true
		default:
			state.query.Sort = Sort{}
		}
		changed = true
	}

	for index, facet := range p.Facets {
		if chosen := state.facets[index].Chosen(); chosen >= 0 && chosen < len(facet.Options) {
			state.query.Filters = toggle(state.query.Filters, facet.Key, facet.Options[chosen].Value)
			changed = true
		}
	}
	if state.clear.Clicked(c) {
		state.query.Filters = nil
		changed = true
	}
	if state.retry.Clicked(c) {
		changed = true
	}

	if chosen := state.sizes.Chosen(); chosen >= 0 && chosen < len(p.sizes()) {
		state.query.PageSize = p.sizes()[chosen]
		changed = true
	}
	return changed
}

// toggle adds a value to a facet's choices, or removes it if it is there.
func toggle(filters map[string][]string, key, value string) map[string][]string {
	next := map[string][]string{}
	for k, v := range filters {
		next[k] = append([]string(nil), v...)
	}
	chosen := next[key]
	for i, v := range chosen {
		if v == value {
			chosen = append(chosen[:i], chosen[i+1:]...)
			if len(chosen) == 0 {
				delete(next, key)
			} else {
				next[key] = chosen
			}
			if len(next) == 0 {
				return nil
			}
			return next
		}
	}
	next[key] = append(chosen, value)
	return next
}

func (p DataTableProps) toolbar(c ayra.Context, state *DataTable) ayra.Dimensions {
	t := c.Theme
	style := t.Resolve("datatable", "toolbar", p.VariantName("default"), theme.Style{}, p.ComponentProps)

	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		children := []layout.FlexChild{}
		if p.Searchable {
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return InputProps{
					ComponentProps: ComponentProps{Custom: p.Custom, Parts: p.Parts},
					widget:         "datatable-search",
					Placeholder:    orDefault(p.Words.Search, "Search"),
				}.Layout(c.With(gtx), &state.search)
			}), layout.Rigid(layout.Spacer{Width: 8}.Layout))
		} else {
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }))
		}

		active := 0
		for index, facet := range p.Facets {
			index, facet := index, facet
			if facet.Kind != FacetChoice {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.field(c.With(gtx), state, facet)
				}), layout.Rigid(layout.Spacer{Width: 6}.Layout))
				continue
			}
			active += len(state.query.Filters[facet.Key])
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				entries := make([]string, len(facet.Options))
				for i, option := range facet.Options {
					mark := "[ ] "
					for _, v := range state.query.Filters[facet.Key] {
						if v == option.Value {
							mark = "[x] "
						}
					}
					entries[i] = mark + orDefault(option.Label, option.Value)
				}
				label := facet.Title
				if n := len(state.query.Filters[facet.Key]); n > 0 {
					label += " (" + strconv.Itoa(n) + ")"
				}
				return MenuProps{Entries: entries, Width: 220}.Layout(inner, &state.facets[index], func(c ayra.Context) ayra.Dimensions {
					return ButtonProps{Label: label, Variant: Outline, Size: Small}.Layout(c, &state.triggers[index])
				})
			}), layout.Rigid(layout.Spacer{Width: 6}.Layout))
		}
		if active > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				return ButtonProps{Label: orDefault(p.Words.Clear, "Clear filters"), Variant: Ghost, Size: Small}.Layout(inner, &state.clear)
			}))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
	})
}

func (p DataTableProps) table(c ayra.Context, state *DataTable, result Result, loaded bool, err error, loading bool) ayra.Dimensions {
	t := c.Theme
	for len(state.rows) < len(result.Rows) {
		state.rows = append(state.rows, Button{})
	}

	headerStyle := t.Resolve("datatable", "header", p.VariantName("default"), theme.Style{}, p.ComponentProps)

	columns := make([]Column, len(p.Columns))
	for i, column := range p.Columns {
		columns[i] = Column{Title: column.Title, Align: column.Align, Flex: column.Flex, Numeric: column.Numeric}
	}
	grid := TableProps{Columns: columns}

	rows := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return Surface(c.With(gtx), headerStyle, func(c ayra.Context) ayra.Dimensions { return p.headings(c, state, result) })
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return SeparatorProps{}.Layout(c.With(gtx)) }),
	}

	switch {
	case err != nil && !loaded:
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return AlertProps{Title: orDefault(p.Words.Failed, "The list could not be loaded"), Body: err.Error(), Severity: Failure}.Layout(inner.With(gtx))
				}),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := inner.With(gtx)
					inner.Constraints.Min = image.Point{}
					return ButtonProps{Label: orDefault(p.Words.Retry, "Try again"), Variant: Outline, Size: Small}.Layout(inner, &state.retry)
				}),
			)
		}))
	case !loaded:
		for i := 0; i < 5; i++ {
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return SkeletonProps{}.Layout(c.With(gtx))
				})
			}))
		}
	case len(result.Rows) == 0:
		empty := t.Resolve("datatable", "empty", p.VariantName("default"), theme.Style{}, p.ComponentProps)
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return Surface(c.With(gtx), empty, func(c ayra.Context) ayra.Dimensions {
				c.Constraints.Min.X = c.Constraints.Max.X
				return layout.Inset{Top: 24, Bottom: 24}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					return drawText(c.With(gtx), orDefault(p.Words.Empty, "No results."), unit.Sp(t.Type.Body), t.Colours.MutedForeground, 1, text.Middle, plain())
				})
			})
		}))
	default:
		for index, row := range result.Rows {
			index, row := index, row
			if index > 0 {
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return SeparatorProps{}.Layout(c.With(gtx)) }))
			}
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.row(c.With(gtx), state, grid, row, index, loading)
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
}

// headings draws the heading row, with the sort mark on the column it orders by.
func (p DataTableProps) headings(c ayra.Context, state *DataTable, result Result) ayra.Dimensions {
	t := c.Theme
	cells := []layout.FlexChild{}

	if p.Selectable {
		cells = append(cells, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			if state.everyone.Changed(inner) {
				if state.selected == nil {
					state.selected = map[string]bool{}
				}
				for _, row := range result.Rows {
					state.selected[row.Key] = state.everyone.On()
				}
			}
			return CheckboxProps{}.Layout(inner, &state.everyone)
		}))
	}
	for index, column := range p.Columns {
		index, column := index, column
		share := column.Flex
		if share == 0 {
			share = 1
		}
		cells = append(cells, layout.Flexed(share, func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			if !column.Sortable {
				return layout.Inset{Top: 8, Bottom: 8, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return drawText(inner.With(gtx), column.Title, unit.Sp(t.Type.Small), t.Colours.MutedForeground, 1, column.Align.alignment(), semibold())
				})
			}
			inner.Constraints.Min = image.Point{}
			sorted := state.query.Sort.Key == column.Key && column.Key != ""
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ButtonProps{Label: column.Title, Variant: Ghost, Size: ExtraSmall}.Layout(inner.With(gtx), &state.headers[index])
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !sorted {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return sortMark(inner.With(gtx), state.query.Sort.Descending, t.Colours.Foreground)
					})
				}),
			)
		}))
	}
	if len(p.Actions) > 0 {
		cells = append(cells, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(p.actionsWidth(c), 0)}
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, cells...)
}

// row draws one row, inert while the next page is being fetched.
func (p DataTableProps) row(c ayra.Context, state *DataTable, grid TableProps, row Row, index int, loading bool) ayra.Dimensions {
	if loading {
		c.Context = c.Context.Disabled()
	}
	for len(state.checks) <= index {
		state.checks = append(state.checks, Toggle{})
	}

	cells := []layout.FlexChild{}
	if p.Selectable {
		on := state.selected[row.Key]
		state.checks[index].Set(on)
		cells = append(cells, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			if state.checks[index].Changed(inner) {
				if state.selected == nil {
					state.selected = map[string]bool{}
				}
				state.selected[row.Key] = state.checks[index].On()
			}
			return CheckboxProps{}.Layout(inner, &state.checks[index])
		}))
	}
	cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return state.rows[index].click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return p.cells(c.With(gtx), grid, row)
		})
	}))
	if state.rows[index].Clicked(c) {
		state.activated = row.Key
	}
	if len(p.Actions) > 0 {
		cells = append(cells, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			width := p.actionsWidth(c)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				return p.actions(inner, state, row, index)
			})
		}))
	}

	style := c.Theme.Resolve("datatable", "row", p.VariantName("default"), theme.Style{}, p.ComponentProps)
	if state.selected[row.Key] && !style.Background.IsSet() {
		style.Background = theme.Colour(c.Theme.Colours.Muted)
	}
	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, cells...)
	})
}

// footer draws the count, the size and the pager.
func (p DataTableProps) footer(c ayra.Context, state *DataTable, result Result, loaded bool) ayra.Dimensions {
	t := c.Theme
	style := t.Resolve("datatable", "footer", p.VariantName("default"), theme.Style{}, p.ComponentProps)

	size := max(state.query.PageSize, 1)
	pages := pageCount(result, state.query)

	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				count := ""
				if loaded && result.Total >= 0 {
					count = p.Words.rows(result.Total)
					if p.Selectable {
						count = strconv.Itoa(len(state.Selected())) + " / " + count
					}
				}
				return drawText(c.With(gtx), count, unit.Sp(t.Type.Small), t.Colours.MutedForeground, 1, text.Start, plain())
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				labels := make([]string, len(p.sizes()))
				for i, s := range p.sizes() {
					labels[i] = strconv.Itoa(s)
				}
				return MenuProps{Entries: labels, Above: true, Width: 80}.Layout(inner, &state.sizes, func(c ayra.Context) ayra.Dimensions {
					return ButtonProps{Label: strconv.Itoa(size) + " / page", Variant: Outline, Size: Small}.Layout(c, &state.sizeBtn)
				})
			}),
			layout.Rigid(layout.Spacer{Width: 12}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				if !loaded {
					return layout.Dimensions{}
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawText(inner.With(gtx), p.Words.page(state.pages.Current(), pages), unit.Sp(t.Type.Small), t.Colours.MutedForeground, 1, text.Start, plain())
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return PaginationProps{Total: pages, Window: 1}.Layout(inner.With(gtx), &state.pages)
					}),
				)
			}),
		)
	})
}

// sortMark draws the small triangle that says which way a column is ordered.
func sortMark(c ayra.Context, descending bool, ink color.NRGBA) ayra.Dimensions {
	side := c.Dp(unit.Dp(8))
	var path clip.Path
	path.Begin(c.Ops)
	if descending {
		path.MoveTo(f32.Pt(0, 0))
		path.LineTo(f32.Pt(float32(side), 0))
		path.LineTo(f32.Pt(float32(side)/2, float32(side)*0.8))
	} else {
		path.MoveTo(f32.Pt(0, float32(side)*0.8))
		path.LineTo(f32.Pt(float32(side), float32(side)*0.8))
		path.LineTo(f32.Pt(float32(side)/2, 0))
	}
	path.Close()
	paint.FillShape(c.Ops, ink, clip.Outline{Path: path.End()}.Op())
	return ayra.Dimensions{Size: image.Pt(side, side)}
}

// SetSearch types into the search field, for a screen that fills it from
// somewhere else -- a link into a filtered list -- and for a test. The query
// moves when the typing has paused, as it does for a person.
func (d *DataTable) SetSearch(text string) { d.search.SetText(text) }

// Select ticks or unticks a row by its key, for a screen that restores a
// selection and for a test.
func (d *DataTable) Select(key string, on bool) {
	if d.selected == nil {
		d.selected = map[string]bool{}
	}
	d.selected[key] = on
}
