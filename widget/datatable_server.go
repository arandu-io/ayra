package widget

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/arandu-io/ayra/client"
)

// DataSource is what an application implements to hand a listing its rows. It
// is [Source] under the name the rest of Arandu uses for it: the table asks for
// a [Query] and is answered with a [Result], and owns no data.
//
// An application implements it with the client and whatever credential it
// holds -- a session cookie, or a bearer token put on the requests by the
// http.Client the [client.Client] was built over -- or takes [ServerSource],
// which speaks the contract of an Arandu route.
type DataSource = Source

// Getter is what [ServerSource] asks for a page of the list. A [*client.Client]
// is one, and a test or an application with its own transport hands another.
type Getter interface {
	Get(ctx context.Context, path client.Path) (client.Page, error)
}

// ServerSource is the [DataSource] that talks to the route of an Arandu list,
// the same route the web half draws with Kyse's DataTable.
//
// # What it asks
//
// The address of the route with the state of the table as its query, written by
// [DataTableValues] -- the encoding Kyse's DataTable writes into its links -- and
// the media type the client always sends, so the handler that answers a browser
// with markup answers this with the values it was going to be drawn from. There
// is no second handler: a route that lists for the browser lists for the app,
// and the listing it declares (hesape's pagination.Listing) is what reads the
// search, the order, the facets, the page and the size, from a whitelist of its
// own. Authorization is the server's, by the session or the token the request
// carries.
//
// # What it reads back
//
// The server answers {"view", "data", "token"}, where data is the struct the
// handler gave its view. The rows are an array in it, at the path named by
// [ServerSource.Rows] ("Rows" by default), each one an object whose fields are
// the cells, named by [ServerSource.Fields] in column order. The total comes
// from the page the listing returned (hesape serialises a LengthAwarePage as
// {"total": 57, "per_page": 20, ...}), found at [ServerSource.Total] or, when
// that is empty, at the first of the places a listing is conventionally kept:
// Table.Result.Page.total, Result.Page.total, Page.total and total. A server
// that says none of them is a server that does not count: the total is then
// unknown and the table pages until a page comes back short.
//
// The count beside each facet value is read from "facets" next to the page
// (see [ServerSource.Facets]) when the listing's result serialises them.
//
// Path segments are matched without regard to case, because a Go struct without
// tags is written with its field names and one with tags is not, and which one
// a handler has is not the table's business.
type ServerSource struct {
	// Client asks for the pages. Required.
	Client Getter
	// Path is the address of the list, without a query.
	Path client.Path
	// Names are the parameter names the route's listing reads. Zero fields are
	// the conventional ones.
	Names DataTableParams
	// DefaultSize is the page size the route's listing opens with, which the
	// address leaves out. Zero writes the size every time, which is always
	// understood.
	DefaultSize int

	// Rows is the path to the array of rows in the values, with dots. Empty is
	// "Rows".
	Rows string
	// Key is the field of a row that identifies it. Empty is "ID", then "id".
	Key string
	// Fields are the fields of a row that make its cells, in column order. A
	// column's cell is the field of the same index. [FieldsOf] makes the list
	// from the columns.
	Fields []string
	// Total is the path to the number of rows the whole query matches. Empty
	// looks in the conventional places.
	Total string
	// Facets is the path to the counts per facet value, an object of arrays of
	// {"value", "count"}. Empty looks in Table.Result.facets, Result.facets and
	// facets.
	Facets string
}

// FieldsOf is the keys of the columns, which is the list of fields a
// [ServerSource] reads when each column is named after the field it shows.
func FieldsOf(columns []DataColumn) []string {
	fields := make([]string, len(columns))
	for i, column := range columns {
		fields[i] = column.Key
	}
	return fields
}

// Fetch asks the route for the page the query names.
func (s ServerSource) Fetch(ctx context.Context, q Query) (Result, error) {
	if s.Client == nil {
		return Result{}, errors.New("ayra/widget: a ServerSource has no client to ask")
	}
	values := q.State(s.Names, s.DefaultSize).Values(DataTableChange{})
	page, err := s.Client.Get(ctx, s.Path.Query(values))
	if err != nil {
		return Result{}, err
	}
	return s.Decode(page)
}

// Decode reads the rows, the total and the facet counts out of an answer. It is
// the half of Fetch that needs no network, and what a test of a route's
// response asks.
func (s ServerSource) Decode(page client.Page) (Result, error) {
	var data any
	decoder := json.NewDecoder(bytes.NewReader(page.Data))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return Result{}, fmt.Errorf("ayra/widget: the answer of view %q holds no values: %w", page.View, err)
	}

	rows, ok := lookup(data, orDefault(s.Rows, "Rows"))
	list, isList := rows.([]any)
	if !ok || (!isList && rows != nil) {
		return Result{}, fmt.Errorf("ayra/widget: view %q has no list of rows at %q", page.View, orDefault(s.Rows, "Rows"))
	}

	result := Result{Total: -1}
	for position, item := range list {
		object, _ := item.(map[string]any)
		row := Row{Data: object}
		row.Key = s.keyOf(object, position)
		for _, field := range s.Fields {
			value, _ := lookupIn(object, field)
			row.Cells = append(row.Cells, cellText(value))
		}
		result.Rows = append(result.Rows, row)
	}

	for _, place := range s.totals() {
		if value, ok := lookup(data, place); ok {
			if n, ok := number(value); ok {
				result.Total = n
				break
			}
		}
	}
	for _, place := range s.counts() {
		if value, ok := lookup(data, place); ok {
			if facets := facetCounts(value); facets != nil {
				result.Facets = facets
				break
			}
		}
	}
	return result, nil
}

func (s ServerSource) totals() []string {
	if s.Total != "" {
		return []string{s.Total}
	}
	return []string{"Table.Result.Page.total", "Result.Page.total", "Page.total", "total"}
}

func (s ServerSource) counts() []string {
	if s.Facets != "" {
		return []string{s.Facets}
	}
	return []string{"Table.Result.facets", "Result.facets", "facets"}
}

// keyOf is the identity of a row: the field that names it, else its position,
// so that a selection still has something to hold when a route lists rows with
// no identifier.
func (s ServerSource) keyOf(object map[string]any, position int) string {
	for _, name := range []string{s.Key, "ID", "id"} {
		if name == "" {
			continue
		}
		if value, ok := lookupIn(object, name); ok && value != nil {
			return cellText(value)
		}
	}
	return strconv.Itoa(position)
}

// lookup follows a dotted path through decoded JSON.
func lookup(data any, path string) (any, bool) {
	current := data
	for _, name := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := lookupIn(object, name)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

// lookupIn is one field of an object, found without regard to case.
func lookupIn(object map[string]any, name string) (any, bool) {
	if value, ok := object[name]; ok {
		return value, true
	}
	for key, value := range object {
		if strings.EqualFold(key, name) {
			return value, true
		}
	}
	return nil, false
}

func number(value any) (int, bool) {
	switch n := value.(type) {
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
		if f, err := n.Float64(); err == nil {
			return int(f), true
		}
	case float64:
		return int(n), true
	}
	return 0, false
}

// cellText is a value as the text of a cell.
func cellText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// facetCounts reads {"status": [{"value": "ativo", "count": 3}]}.
func facetCounts(value any) map[string][]FacetCount {
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string][]FacetCount{}
	for key, list := range object {
		items, _ := list.([]any)
		for _, item := range items {
			entry, _ := item.(map[string]any)
			v, _ := lookupIn(entry, "value")
			count, _ := lookupIn(entry, "count")
			n, _ := number(count)
			out[key] = append(out[key], FacetCount{Value: cellText(v), Count: n})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
