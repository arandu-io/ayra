package widget

import (
	"net/url"
	"slices"
	"sort"
	"strconv"
)

// DataTableParams are the names a listing's state travels under in an address.
//
// It is the counterpart of Kyse's DataTableParams and means the same thing: a
// zero field is the conventional name -- q, sort, dir, page, per_page and hide
// -- and the names are the state's to choose, so a list whose handler reads
// "sheet" instead of "page" is asked with the address the handler understands.
// What reads the request on the server is the listing the route declares
// (hesape's pagination.Listing); this is the name list it is asked with.
type DataTableParams struct {
	// Search is the text typed into the search box.
	Search string
	// Sort and Dir are the column ordered by and its direction, "asc" or "desc".
	Sort string
	Dir  string
	// Page is the page being shown, counting from one.
	Page string
	// PerPage is how many rows a page holds.
	PerPage string
	// Hide is the column keys switched off; the name repeats once per column.
	Hide string
}

// Filled is p with every empty name replaced by its conventional one.
func (p DataTableParams) Filled() DataTableParams {
	fill := func(name *string, conventional string) {
		if *name == "" {
			*name = conventional
		}
	}
	fill(&p.Search, "q")
	fill(&p.Sort, "sort")
	fill(&p.Dir, "dir")
	fill(&p.Page, "page")
	fill(&p.PerPage, "per_page")
	fill(&p.Hide, "hide")
	return p
}

// DataTableChange is one step from the state in force to the next, which is
// what every control of a data table asks its state for. It is Kyse's, field
// for field.
//
// Only one field is set at a time. The state decides what else the step moves:
// a new search, a facet or a new order sends the reader back to the first page,
// a new page size keeps the first row of the page being shown on screen, and
// switching a column on or off keeps the page where it is.
type DataTableChange struct {
	// Sort and Dir order the list by a column. An empty Dir means ascending.
	Sort string
	Dir  string
	// Facet and Value switch one value of a facet on when it is off and off
	// when it is on.
	Facet string
	Value string
	// ClearFacet drops every value of one facet.
	ClearFacet string
	// Reset drops the search and every facet, and keeps the order, the page
	// size and the columns.
	Reset bool
	// Page goes to a page, counting from one.
	Page int
	// PerPage sets how many rows a page holds.
	PerPage int
	// Hide switches one column off when it is shown and on when it is hidden.
	Hide string
	// CursorName and Cursor step to the page a signed cursor names, for a list
	// that knows no page numbers. The page number is dropped.
	CursorName string
	Cursor     string
}

// DataTableState is the state of a listing as a data table reads it: what is
// searched, how it is ordered, which facets are chosen, which page and how many
// rows, and which columns are hidden.
//
// It is the interface of Kyse's DataTableState with nothing added or removed,
// so that the address a native table asks is the address the web table draws
// for the same state -- one route, one handler, one encoding. Every address
// comes from the state: a value is deduplicated, written in a stable order and
// left out when it is the default, in one place.
type DataTableState interface {
	// Params are the names the state travels under.
	Params() DataTableParams
	// Search is the text being searched for.
	Search() string
	// SortKey and SortDir are the order in force; SortKey is empty for none.
	SortKey() string
	SortDir() string
	// Chosen are the values chosen for one facet, empty for none.
	Chosen(facet string) []string
	// Page is the page being shown, counting from one.
	Page() int
	// PageSize is how many rows a page holds, zero when the state keeps the
	// default to itself.
	PageSize() int
	// Hidden are the keys of the columns switched off.
	Hidden() []string
	// Values is the query of the state after change. A zero change is the state
	// in force. The result is the caller's to keep: it is a fresh value, never
	// one the state holds on to.
	Values(change DataTableChange) url.Values
}

// DataTableValues is a plain [DataTableState]: the values themselves, and the
// canonical encoding of the next step. It is Kyse's DataTableValues.
//
// The encoding leaves a value out when it is the default, writes a repeated
// parameter once per distinct value in sorted order, and sends every change but
// a new page back to the first page. The zero value is an unfiltered list on
// its first page.
type DataTableValues struct {
	// Names are the parameter names; a zero field says its conventional name.
	Names DataTableParams
	// Q is the text searched for.
	Q string
	// Sort and Dir are the order in force.
	Sort string
	Dir  string
	// Facets are the values chosen, by facet key.
	Facets map[string][]string
	// CurrentPage is the page being shown; zero is the first.
	CurrentPage int
	// Size is how many rows a page holds; zero keeps DefaultSize.
	Size int
	// DefaultSize is the size the query leaves out because it is the default.
	DefaultSize int
	// Hide are the keys of the columns switched off.
	Hide []string
}

// Params implements [DataTableState].
func (v DataTableValues) Params() DataTableParams { return v.Names.Filled() }

// Search implements [DataTableState].
func (v DataTableValues) Search() string { return v.Q }

// SortKey implements [DataTableState].
func (v DataTableValues) SortKey() string { return v.Sort }

// SortDir implements [DataTableState]; it is "asc" when an order has no
// direction written.
func (v DataTableValues) SortDir() string {
	if v.Sort == "" {
		return ""
	}
	if v.Dir == "desc" {
		return "desc"
	}
	return "asc"
}

// Chosen implements [DataTableState].
func (v DataTableValues) Chosen(facet string) []string { return distinctValues(v.Facets[facet]) }

// Page implements [DataTableState].
func (v DataTableValues) Page() int { return max(1, v.CurrentPage) }

// PageSize implements [DataTableState].
func (v DataTableValues) PageSize() int {
	if v.Size > 0 {
		return v.Size
	}
	return v.DefaultSize
}

// Hidden implements [DataTableState].
func (v DataTableValues) Hidden() []string { return distinctValues(v.Hide) }

// Values implements [DataTableState].
func (v DataTableValues) Values(change DataTableChange) url.Values {
	names := v.Names.Filled()
	query := v.Q
	sortKey, dir := v.Sort, v.SortDir()
	page := v.Page()
	size := v.PageSize()
	facets := map[string][]string{}
	for key, chosen := range v.Facets {
		if chosen = distinctValues(chosen); len(chosen) > 0 {
			facets[key] = chosen
		}
	}
	hide := v.Hidden()
	cursor := [2]string{}

	if change.Reset {
		query, facets, page = "", map[string][]string{}, 1
	}
	if change.Sort != "" {
		sortKey, dir, page = change.Sort, "asc", 1
		if change.Dir == "desc" {
			dir = "desc"
		}
	}
	if change.Facet != "" && change.Value != "" {
		chosen := facets[change.Facet]
		if slices.Contains(chosen, change.Value) {
			chosen = withoutValue(chosen, change.Value)
		} else {
			chosen = distinctValues(append(append([]string{}, chosen...), change.Value))
		}
		if len(chosen) == 0 {
			delete(facets, change.Facet)
		} else {
			facets[change.Facet] = chosen
		}
		page = 1
	}
	if change.ClearFacet != "" {
		delete(facets, change.ClearFacet)
		page = 1
	}
	if change.PerPage > 0 {
		// The first row on screen stays on screen: the page it lands on under
		// the new size is the one that holds it.
		first := (page-1)*max(1, size) + 1
		size = change.PerPage
		page = (first-1)/size + 1
	}
	if change.Hide != "" {
		if slices.Contains(hide, change.Hide) {
			hide = withoutValue(hide, change.Hide)
		} else {
			hide = distinctValues(append(append([]string{}, hide...), change.Hide))
		}
	}
	if change.Page > 0 {
		page = change.Page
	}
	if change.CursorName != "" && change.Cursor != "" {
		cursor = [2]string{change.CursorName, change.Cursor}
		page = 1
	}

	values := url.Values{}
	if query != "" {
		values.Set(names.Search, query)
	}
	if sortKey != "" {
		values.Set(names.Sort, sortKey)
		values.Set(names.Dir, dir)
	}
	for key, chosen := range facets {
		values[key] = chosen
	}
	if page > 1 {
		values.Set(names.Page, strconv.Itoa(page))
	}
	if size > 0 && size != v.DefaultSize {
		values.Set(names.PerPage, strconv.Itoa(size))
	}
	if len(hide) > 0 {
		values[names.Hide] = hide
	}
	if cursor[0] != "" {
		values.Set(cursor[0], cursor[1])
	}
	return values
}

// distinctValues is values without repeats, empty values dropped, sorted so the
// same set always encodes the same way.
func distinctValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func withoutValue(values []string, drop string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != drop {
			out = append(out, value)
		}
	}
	return out
}

// State is the query as a [DataTableValues], under the given names. defaultSize
// is the page size the address leaves out, which is the size the route's listing
// opens with; zero writes the size every time.
func (q Query) State(names DataTableParams, defaultSize int) DataTableValues {
	v := DataTableValues{
		Names:       names,
		Q:           q.Search,
		Sort:        q.Sort.Key,
		CurrentPage: q.Page,
		Size:        q.PageSize,
		DefaultSize: defaultSize,
	}
	if q.Sort.Key != "" {
		v.Dir = "asc"
		if q.Sort.Descending {
			v.Dir = "desc"
		}
	}
	if len(q.Filters) > 0 {
		v.Facets = make(map[string][]string, len(q.Filters))
		for key, chosen := range q.Filters {
			v.Facets[key] = slices.Clone(chosen)
		}
	}
	return v
}

// QueryOf is a state read back as the [Query] a source answers: the same search,
// order, facets, page and size.
func QueryOf(state DataTableState) Query {
	q := Query{
		Search:   state.Search(),
		Sort:     Sort{Key: state.SortKey(), Descending: state.SortDir() == "desc"},
		Page:     state.Page(),
		PageSize: state.PageSize(),
	}
	if v, ok := state.(DataTableValues); ok {
		for key, chosen := range v.Facets {
			if chosen = distinctValues(chosen); len(chosen) > 0 {
				if q.Filters == nil {
					q.Filters = map[string][]string{}
				}
				q.Filters[key] = chosen
			}
		}
	}
	return q
}
