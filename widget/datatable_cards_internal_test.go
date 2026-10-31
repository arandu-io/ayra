package widget

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

func TestTheCardsReplaceTheTableUnderTheBreakpoint(t *testing.T) {
	for _, test := range []struct {
		name       string
		mode       DataTableMode
		width      int
		breakpoint float32
		cards      bool
	}{
		{"a phone", ModeAuto, 360, 0, true},
		{"a tablet upright", ModeAuto, 599, 0, true},
		{"a tablet", ModeAuto, 600, 0, false},
		{"a desktop", ModeAuto, 1200, 0, false},
		{"a breakpoint of the product's", ModeAuto, 700, 800, true},
		{"cards on a wide screen", ModeCards, 1200, 0, true},
		{"a table on a phone", ModeTable, 360, 0, false},
	} {
		c, _ := field(t, theme.Light, test.width)
		p := DataTableProps{Mode: test.mode, Breakpoint: unit.Dp(test.breakpoint)}
		if got := p.narrow(c); got != test.cards {
			t.Errorf("%s: cards=%v, want %v", test.name, got, test.cards)
		}
	}
}

type recorder struct {
	queries chan Query
}

func (r recorder) Fetch(_ context.Context, q Query) (Result, error) {
	r.queries <- q
	return Result{Rows: []Row{{Key: "1", Cells: []string{"a", "b"}}}, Total: 1}, nil
}

func facets() []Facet {
	return []Facet{
		{Key: "status", Title: "Status", Options: []FacetOption{{Value: "online"}, {Value: "parada"}}},
		{Key: "q2", Title: "Local", Kind: FacetText},
		{Key: "de", EndKey: "ate", Title: "Período", Kind: FacetPeriod},
	}
}

// What the sheet's controls say is applied as one query, from the first page,
// and what the table was given beyond them is kept.
func TestTheSheetAppliesEveryControlAsOneQuery(t *testing.T) {
	var state DataTable
	state.query = Query{Page: 3, PageSize: 10, Filters: map[string][]string{"tenant_scope": {"keep"}, "status": {"parada"}}}
	p := DataTableProps{Facets: facets()}

	p.openSheet(&state)
	if !state.m.toggle["status\x00parada"].On() || state.m.toggle["status\x00online"].On() {
		t.Fatal("the sheet did not open on the filters in force")
	}
	state.m.toggle["status\x00online"].Set(true)
	state.m.typed["q2"].SetText("  Garagem ")
	from := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	state.m.span["de"].Set(from, to)

	got := p.applied(&state)
	want := map[string][]string{
		"tenant_scope": {"keep"},
		"status":       {"online", "parada"},
		"q2":           {"Garagem"},
		"de":           {"2026-09-15"},
		"ate":          {"2026-10-07"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("applied %v, want %v", got, want)
	}
}

func TestThePeriodTravelsAsTwoIsoDaysAndAnEmptyOneAsNone(t *testing.T) {
	facet := Facet{Key: "de", EndKey: "ate", Kind: FacetPeriod}
	from := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

	set := withPeriod(map[string][]string{"status": {"x"}}, facet, from, to)
	if set["de"][0] != "2026-09-15" || set["ate"][0] != "2026-10-07" || set["status"][0] != "x" {
		t.Errorf("the period is %v", set)
	}
	if cleared := withPeriod(set, facet, time.Time{}, time.Time{}); !reflect.DeepEqual(cleared, map[string][]string{"status": {"x"}}) {
		t.Errorf("clearing the period left %v", cleared)
	}
	if set["de"] == nil {
		t.Error("withPeriod changed the map it was given")
	}
}

// The wide table asks for a text filter after a pause, and the narrow one does
// not read it at all: it is waiting for the button.
func TestATextFilterIsReadAfterAPauseWhenWideAndNeverWhenNarrow(t *testing.T) {
	p := DataTableProps{Facets: facets(), Debounce: time.Millisecond}

	var wide DataTable
	c, _ := field(t, theme.Light, 900)
	wide.m.fieldsFor(p.Facets)
	wide.m.typed["q2"].SetText("Gar")
	if p.readFields(c, &wide) {
		t.Fatal("the text was asked for while it was still being typed")
	}
	c.Now = c.Now.Add(time.Second)
	if !p.readFields(c, &wide) || wide.query.Filters["q2"][0] != "Gar" {
		t.Errorf("a pause did not ask for the text: %v", wide.query.Filters)
	}

	var narrow DataTable
	cn, _ := field(t, theme.Light, 360)
	narrow.m.fieldsFor(p.Facets)
	narrow.m.typed["q2"].SetText("Gar")
	cn.Now = cn.Now.Add(time.Second)
	p.readFields(cn, &narrow)
	cn.Now = cn.Now.Add(time.Second)
	if p.readFields(cn, &narrow) || narrow.query.Filters != nil {
		t.Errorf("the narrow table asked before the button: %v", narrow.query.Filters)
	}
}

// A pull that goes far enough asks once; one that stops short asks nothing.
func TestAPullAsksOnceWhenItWentFarEnough(t *testing.T) {
	c, _ := field(t, theme.Light, 360)
	source := recorder{queries: make(chan Query, 4)}
	p := DataTableProps{Source: source}
	var state DataTable
	state.query = Query{Page: 1, PageSize: 10}

	state.m.pull.distance = pullThreshold - 1
	if got := state.m.pull.offset(c, &state, p); got <= 0 {
		t.Error("a pull drew no room for the indicator")
	}
	select {
	case <-source.queries:
		t.Fatal("a pull that stopped short asked")
	case <-time.After(20 * time.Millisecond):
	}

	state.m.pull.fired = true
	state.m.pull.offset(c, &state, p)
	select {
	case <-source.queries:
	case <-time.After(time.Second):
		t.Fatal("a pull that went far enough did not ask")
	}
	for i := 0; i < 20; i++ {
		state.mu.Lock()
		waiting := state.m.refreshing
		state.mu.Unlock()
		if !waiting {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if state.m.pull.fired {
		t.Error("the pull stayed armed and would ask again on the next frame")
	}
}
