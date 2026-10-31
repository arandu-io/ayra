package widget_test

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/arandu-io/ayra/widget"
)

// The state is the encoding Kyse's DataTable writes into its links, so the same
// route serves both. These are the cases that encoding is defined by; the module
// under compat/ runs the same inputs through Kyse itself and compares.

func TestAnUnfilteredListHasNoQuery(t *testing.T) {
	if got := (widget.DataTableValues{}).Values(widget.DataTableChange{}); len(got) != 0 {
		t.Errorf("a list nobody touched has the query %v", got)
	}
}

func TestADefaultIsLeftOutAndARepeatedValueIsWrittenOnceInOrder(t *testing.T) {
	v := widget.DataTableValues{
		Q: "abc", Sort: "plate", Dir: "desc", CurrentPage: 3, Size: 25, DefaultSize: 10,
		Facets: map[string][]string{"status": {"unread", "read", "read", ""}, "kind": nil},
		Hide:   []string{"b", "a", "a"},
	}
	want := url.Values{
		"q": {"abc"}, "sort": {"plate"}, "dir": {"desc"}, "page": {"3"}, "per_page": {"25"},
		"status": {"read", "unread"}, "hide": {"a", "b"},
	}
	if got := v.Values(widget.DataTableChange{}); !reflect.DeepEqual(got, want) {
		t.Errorf("the query is %v, want %v", got, want)
	}
	v.Size = 10
	if got := v.Values(widget.DataTableChange{}); got.Has("per_page") {
		t.Errorf("the default size was written: %v", got)
	}
}

func TestEveryStepButAPageGoesBackToTheFirstPage(t *testing.T) {
	base := widget.DataTableValues{Q: "x", CurrentPage: 4, Size: 10, DefaultSize: 10, Sort: "plate"}
	for name, change := range map[string]widget.DataTableChange{
		"a sort":        {Sort: "status", Dir: "desc"},
		"a facet":       {Facet: "status", Value: "read"},
		"a cleared one": {ClearFacet: "status"},
		"a reset":       {Reset: true},
	} {
		if got := base.Values(change); got.Has("page") {
			t.Errorf("%s kept the page: %v", name, got)
		}
	}
	if got := base.Values(widget.DataTableChange{Page: 5}); got.Get("page") != "5" {
		t.Errorf("a page step is %v", got)
	}
	if got := base.Values(widget.DataTableChange{Hide: "b"}); got.Get("page") != "4" {
		t.Errorf("switching a column moved the page: %v", got)
	}
}

func TestAFacetValueIsSwitchedOnWhenOffAndOffWhenOn(t *testing.T) {
	v := widget.DataTableValues{Facets: map[string][]string{"status": {"read"}}}
	on := v.Values(widget.DataTableChange{Facet: "status", Value: "unread"})
	if !reflect.DeepEqual(on["status"], []string{"read", "unread"}) {
		t.Errorf("switching on gave %v", on["status"])
	}
	off := v.Values(widget.DataTableChange{Facet: "status", Value: "read"})
	if off.Has("status") {
		t.Errorf("switching off left %v", off["status"])
	}
}

func TestANewSizeKeepsTheFirstRowOnScreen(t *testing.T) {
	v := widget.DataTableValues{CurrentPage: 3, Size: 10, DefaultSize: 10}
	// Page 3 of ten starts at row 21; at 25 a page, row 21 is on page 1.
	if got := v.Values(widget.DataTableChange{PerPage: 25}); got.Has("page") || got.Get("per_page") != "25" {
		t.Errorf("size 25 from page 3 of 10: %v", got)
	}
	// At five a page it is on page 5.
	if got := v.Values(widget.DataTableChange{PerPage: 5}); got.Get("page") != "5" {
		t.Errorf("size 5 from page 3 of 10: %v", got)
	}
}

func TestTheNamesAreTheStatesToChoose(t *testing.T) {
	v := widget.DataTableValues{
		Names: widget.DataTableParams{Search: "busca", Page: "pagina", PerPage: "tamanho"},
		Q:     "abc", CurrentPage: 2, Size: 25, DefaultSize: 10,
	}
	want := url.Values{"busca": {"abc"}, "pagina": {"2"}, "tamanho": {"25"}}
	if got := v.Values(widget.DataTableChange{}); !reflect.DeepEqual(got, want) {
		t.Errorf("the query under the state's names is %v, want %v", got, want)
	}
	filled := v.Params()
	if filled.Sort != "sort" || filled.Dir != "dir" || filled.Hide != "hide" || filled.Search != "busca" {
		t.Errorf("the names are %+v: an empty one is the conventional name and a given one is kept", filled)
	}
}

func TestAQueryIsTheSameStateTheTableAsksWith(t *testing.T) {
	q := widget.Query{
		Search: "abc", Sort: widget.Sort{Key: "plate", Descending: true},
		Filters: map[string][]string{"status": {"read"}, "de": {"2026-09-15"}, "ate": {"2026-10-07"}},
		Page:    2, PageSize: 25,
	}
	state := q.State(widget.DataTableParams{}, 10)
	want := url.Values{
		"q": {"abc"}, "sort": {"plate"}, "dir": {"desc"}, "page": {"2"}, "per_page": {"25"},
		"status": {"read"}, "de": {"2026-09-15"}, "ate": {"2026-10-07"},
	}
	if got := state.Values(widget.DataTableChange{}); !reflect.DeepEqual(got, want) {
		t.Errorf("the address is %v, want %v", got, want)
	}
	if back := widget.QueryOf(state); !reflect.DeepEqual(back, q) {
		t.Errorf("the state read back as %+v, want %+v", back, q)
	}
}
