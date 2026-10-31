package compat_test

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"testing"

	"github.com/arandu-io/ayra/client"
	"github.com/arandu-io/ayra/widget"
	"github.com/arandu-io/hesape/http"
	"github.com/arandu-io/hesape/pagination"
	"github.com/arandu-io/kyse/components"
)

// bases are states a list is found in, from nothing to everything.
func bases() []widget.DataTableValues {
	return []widget.DataTableValues{
		{},
		{Q: "abc"},
		{Sort: "plate"},
		{Sort: "plate", Dir: "desc"},
		{CurrentPage: 4, Size: 25, DefaultSize: 10},
		{CurrentPage: 3, Size: 10, DefaultSize: 10},
		{Size: 10, DefaultSize: 10},
		{Q: "x", Facets: map[string][]string{"status": {"read"}}},
		{Facets: map[string][]string{"status": {"unread", "read", "read", ""}, "kind": {"a"}, "empty": nil}},
		{Hide: []string{"b", "a", "a", ""}},
		{Names: widget.DataTableParams{Search: "busca", Page: "pagina", PerPage: "tamanho", Sort: "ordem", Dir: "sentido", Hide: "ocultas"},
			Q: "x", Sort: "plate", Dir: "desc", CurrentPage: 2, Size: 25, DefaultSize: 10, Hide: []string{"a"}},
		{Q: "a", Sort: "s", Dir: "nonsense", CurrentPage: -3, Size: -1, DefaultSize: 20,
			Facets: map[string][]string{"de": {"2026-09-15"}, "ate": {"2026-10-07"}}},
	}
}

// changes are every step a control asks for.
func changes() []widget.DataTableChange {
	return []widget.DataTableChange{
		{},
		{Sort: "plate"},
		{Sort: "plate", Dir: "desc"},
		{Sort: "status", Dir: "asc"},
		{Facet: "status", Value: "read"},
		{Facet: "status", Value: "unread"},
		{Facet: "status", Value: "new"},
		{Facet: "status"},
		{Facet: "", Value: "read"},
		{ClearFacet: "status"},
		{ClearFacet: "kind"},
		{Reset: true},
		{Page: 1},
		{Page: 5},
		{PerPage: 5},
		{PerPage: 10},
		{PerPage: 25},
		{PerPage: 50},
		{Hide: "a"},
		{Hide: "c"},
		{CursorName: "after", Cursor: "tok"},
		{CursorName: "after"},
		{Reset: true, Sort: "plate"},
	}
}

func kyse(v widget.DataTableValues) components.DataTableValues {
	return components.DataTableValues{
		Names: components.DataTableParams{
			Search: v.Names.Search, Sort: v.Names.Sort, Dir: v.Names.Dir,
			Page: v.Names.Page, PerPage: v.Names.PerPage, Hide: v.Names.Hide,
		},
		Q: v.Q, Sort: v.Sort, Dir: v.Dir, Facets: v.Facets, CurrentPage: v.CurrentPage,
		Size: v.Size, DefaultSize: v.DefaultSize, Hide: v.Hide,
	}
}

func change(c widget.DataTableChange) components.DataTableChange {
	return components.DataTableChange{
		Sort: c.Sort, Dir: c.Dir, Facet: c.Facet, Value: c.Value, ClearFacet: c.ClearFacet, Reset: c.Reset,
		Page: c.Page, PerPage: c.PerPage, Hide: c.Hide, CursorName: c.CursorName, Cursor: c.Cursor,
	}
}

func TestTheStateAnswersWhatKyseAnswersForTheSameInput(t *testing.T) {
	n := 0
	for i, base := range bases() {
		for j, step := range changes() {
			n++
			ours, theirs := base.Values(step), kyse(base).Values(change(step))
			if !reflect.DeepEqual(ours, theirs) {
				t.Errorf("base %d, step %d (%+v): ayra writes %v, kyse writes %v", i, j, step, ours, theirs)
			}
		}
		k := kyse(base)
		if base.Search() != k.Search() || base.SortKey() != k.SortKey() || base.SortDir() != k.SortDir() ||
			base.Page() != k.Page() || base.PageSize() != k.PageSize() ||
			!reflect.DeepEqual(base.Hidden(), k.Hidden()) || !reflect.DeepEqual(base.Chosen("status"), k.Chosen("status")) {
			t.Errorf("base %d reads differently: ayra %v, kyse %v", i, base, k)
		}
		if got, want := base.Params(), k.Params(); !reflect.DeepEqual(
			[6]string{got.Search, got.Sort, got.Dir, got.Page, got.PerPage, got.Hide},
			[6]string{want.Search, want.Sort, want.Dir, want.Page, want.PerPage, want.Hide}) {
			t.Errorf("base %d names are %v, kyse's %v", i, got, want)
		}
	}
	t.Logf("%d states and steps compared", n)
}

// The address the app asks is read by the listing the route declares, which is
// the code that reads the address Kyse's links carry.
func listing(t *testing.T) *pagination.Listing {
	t.Helper()
	l, err := pagination.NewListing(pagination.ListingSpec{
		Search:    []string{"plate", "camera"},
		Sorts:     map[string]string{"plate": "plate", "camera": "camera", "at": "read_at"},
		Facets:    []pagination.FacetSpec{{Key: "status", Column: "status", Values: []string{"read", "unread"}}},
		PageSizes: []int{10, 25, 50}, DefaultPageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestTheRouteReadsWhatTheAppAsksTheSameWayItReadsABrowser(t *testing.T) {
	l := listing(t)
	q := widget.Query{
		Search: "abc", Sort: widget.Sort{Key: "camera", Descending: true},
		Filters: map[string][]string{"status": {"unread", "read"}, "de": {"2026-09-15"}},
		Page:    3, PageSize: 25,
	}
	values := q.State(widget.DataTableParams{}, l.DefaultPageSize()).Values(widget.DataTableChange{})
	state := l.Parse(values)

	if state.Search() != "abc" || state.SortKey() != "camera" || state.Direction() != pagination.SortDesc ||
		state.Page() != 3 || state.PerPage() != 25 || !reflect.DeepEqual(state.Facet("status"), []string{"read", "unread"}) {
		t.Errorf("the listing read %+v from %v", state, values)
	}

	// And the other way: the web half, drawing the state the listing read,
	// writes the address the app wrote -- one encoding.
	web := components.DataTableStateFrom(state).Values(components.DataTableChange{})
	for _, key := range []string{"q", "sort", "dir", "page", "per_page", "status"} {
		if !reflect.DeepEqual(values[key], web[key]) {
			t.Errorf("%s: the app wrote %v, the web table writes %v", key, values[key], web[key])
		}
	}
	// A filter the listing does not declare -- the period -- is not its to read
	// and not its to drop: it stays in the address for the handler that does.
	if values.Get("de") != "2026-09-15" {
		t.Errorf("the period was lost from %v", values)
	}
}

func TestADefaultTheListingOpensWithIsLeftOutOfTheAddress(t *testing.T) {
	l := listing(t)
	q := widget.Query{Page: 1, PageSize: 10}
	values := q.State(widget.DataTableParams{}, l.DefaultPageSize()).Values(widget.DataTableChange{})
	if len(values) != 0 {
		t.Errorf("the first page of an untouched list has the address %v", values)
	}
	if web := components.DataTableStateFrom(l.Parse(nil)).Values(components.DataTableChange{}); len(web) != 0 {
		t.Errorf("the web table writes %v for the same list", web)
	}
}

// answer is the real thing: hesape's ViewData around what a handler gives its
// view, with the listing's own result and its page, serialised the way hesape
// serialises them.
func answer(t *testing.T, l *pagination.Listing, q url.Values) client.Page {
	t.Helper()
	state := l.Parse(q)
	page := pagination.NewLengthAwarePage(2, 57, state.PerPage(), state.Page(), state.PageOptions("/painel/leituras"))
	result := pagination.NewListingResult(state, page, map[string][]pagination.FacetCount{
		"status": {{Value: "read", Count: 30}, {Value: "unread", Count: 27}},
	})
	type row struct {
		ID     string
		Plate  string
		Status string
	}
	data := struct {
		Table struct {
			URL    string
			Result *pagination.ListingResult
		}
		Rows []row
	}{Rows: []row{{"r1", "ABC1D23", "read"}, {"r2", "BRA2E19", "unread"}}}
	data.Table.URL, data.Table.Result = "/painel/leituras", result

	body, err := json.Marshal(http.ViewData{View: "leituras::index", Data: data, Token: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	var out client.Page
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

type route struct {
	t    *testing.T
	l    *pagination.Listing
	path []string
}

func (r *route) Get(_ context.Context, p client.Path) (client.Page, error) {
	u, err := url.Parse(p.String())
	if err != nil {
		return client.Page{}, err
	}
	r.path = append(r.path, u.RawQuery)
	return answer(r.t, r.l, u.Query()), nil
}

func TestTheSameRouteServesTheAppWhatItServesTheWebTable(t *testing.T) {
	r := &route{t: t, l: listing(t)}
	source := widget.ServerSource{
		Client: r, Path: client.At("painel", "leituras"), DefaultSize: 10,
		Fields: []string{"Plate", "Status"},
	}
	result, err := source.Fetch(context.Background(), widget.Query{Page: 2, PageSize: 25, Search: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 57 {
		t.Errorf("total %d: the listing's page says 57", result.Total)
	}
	if len(result.Rows) != 2 || result.Rows[0].Key != "r1" || result.Rows[1].Cells[0] != "BRA2E19" {
		t.Errorf("rows %+v", result.Rows)
	}
	if r.path[0] != "page=2&per_page=25&q=abc" {
		t.Errorf("the route was asked %q", r.path[0])
	}
}

// What the listing's result says about its facets today. The app reads counts
// from "facets" when the result serialises them; hesape's ListingResult keeps
// them unexported and serialises none, which is the one thing the native table
// would need that the web table gets by calling a method. See UPGRADE.md.
func TestTheFacetCountsAreOnlyThereOnceHesapeSerialisesThem(t *testing.T) {
	page := answer(t, listing(t), nil)
	result, err := widget.ServerSource{Fields: []string{"Plate"}}.Decode(page)
	if err != nil {
		t.Fatal(err)
	}
	if result.Facets != nil {
		t.Skipf("hesape serialises the counts now: %v", result.Facets)
	}
	t.Logf("hesape's ListingResult serialises no facet counts: the native table draws the options without them. The envelope: %s", page.Data)
}
