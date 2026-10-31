package widget_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// plates is a source the way an application writes one: a list that answers a
// query by filtering, sorting and cutting it, as its server would.
type plates struct {
	mu      sync.Mutex
	queries []widget.Query
	fail    error
	delay   time.Duration
}

func (s *plates) seen() []widget.Query {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]widget.Query(nil), s.queries...)
}

func (s *plates) Fetch(ctx context.Context, q widget.Query) (widget.Result, error) {
	s.mu.Lock()
	s.queries = append(s.queries, q)
	fail, delay := s.fail, s.delay
	s.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return widget.Result{}, ctx.Err()
		}
	}
	if fail != nil {
		return widget.Result{}, fail
	}

	var matching []widget.Row
	for i := 1; i <= 57; i++ {
		plate := fmt.Sprintf("ABC%04d", i)
		status := []string{"read", "unread"}[i%2]
		if q.Search != "" && !strings.Contains(plate, strings.ToUpper(q.Search)) {
			continue
		}
		if want := q.Filters["status"]; len(want) > 0 && !contains(want, status) {
			continue
		}
		matching = append(matching, widget.Row{Key: plate, Cells: []string{plate, status}})
	}
	if q.Sort.Key == "plate" && q.Sort.Descending {
		for i, j := 0, len(matching)-1; i < j; i, j = i+1, j-1 {
			matching[i], matching[j] = matching[j], matching[i]
		}
	}

	from := min((q.Page-1)*q.PageSize, len(matching))
	to := min(from+q.PageSize, len(matching))
	return widget.Result{Rows: matching[from:to], Total: len(matching)}, nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func tableProps(source widget.Source) widget.DataTableProps {
	return widget.DataTableProps{
		Source:     source,
		Searchable: true,
		Selectable: true,
		Columns: []widget.DataColumn{
			{Key: "plate", Title: "Plate", Sortable: true, Flex: 2},
			{Key: "status", Title: "Status"},
		},
		Facets:   []widget.Facet{{Key: "status", Title: "Status", Options: []widget.FacetOption{{Value: "read"}, {Value: "unread"}}}},
		Debounce: time.Millisecond,
	}
}

// settle draws frames until the table has an answer, the way a window does when
// the source calls Redraw.
func settle(t *testing.T, ctx ayra.Context, props widget.DataTableProps, state *widget.DataTable) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		props.Layout(ctx, state)
		if !state.Loading() {
			props.Layout(ctx, state)
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the source never answered")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestATableAsksItsSourceForTheFirstPageAndNothingElse(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 600)
	source := &plates{}
	props := tableProps(source)
	var state widget.DataTable

	settle(t, ctx, props, &state)

	queries := source.seen()
	if len(queries) != 1 {
		t.Fatalf("the source was asked %d times for the first page", len(queries))
	}
	if q := queries[0]; q.Page != 1 || q.PageSize != 10 || q.Search != "" {
		t.Errorf("the first query is %+v", q)
	}
	if state.Err() != nil {
		t.Errorf("the first query failed: %v", state.Err())
	}
}

func TestATableDrawsWhatTheSourceAnswers(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 600)
	props := tableProps(&plates{})
	var state widget.DataTable

	settle(t, ctx, props, &state)
	loaded := props.Layout(ctx, &state).Size

	var empty widget.DataTable
	noSource := tableProps(nil)
	bare := noSource.Layout(ctx, &empty).Size

	if loaded.Y <= bare.Y {
		t.Errorf("ten rows drew %v, no taller than a table with none %v", loaded, bare)
	}
}

func TestTheSearchWaitsForAPauseInTheTyping(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 600)
	source := &plates{}
	props := tableProps(source)
	props.Debounce = time.Hour
	var state widget.DataTable

	settle(t, ctx, props, &state)
	if len(source.seen()) != 1 {
		t.Fatal("the first page was not fetched")
	}

	// Typing is what the search field's state holds; a pause is the clock moving.
	state.SetSearch("ABC00")
	for i := 0; i < 3; i++ {
		props.Layout(ctx, &state)
	}
	if len(source.seen()) != 1 {
		t.Errorf("the source was asked %d times while the typing had not paused", len(source.seen()))
	}

	props.Debounce = time.Millisecond
	time.Sleep(5 * time.Millisecond)
	ctx.Now = ctx.Now.Add(time.Second)
	settle(t, ctx, props, &state)

	queries := source.seen()
	if len(queries) != 2 || queries[1].Search != "ABC00" || queries[1].Page != 1 {
		t.Errorf("the search was not asked as typed, from the first page: %+v", queries)
	}
}

func TestAFailedQueryIsSaidAndCanBeAskedAgain(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 600)
	source := &plates{fail: errors.New("the server said no")}
	props := tableProps(source)
	var state widget.DataTable

	settle(t, ctx, props, &state)
	if state.Err() == nil || !strings.Contains(state.Err().Error(), "said no") {
		t.Fatalf("the error is %v", state.Err())
	}

	source.mu.Lock()
	source.fail = nil
	source.mu.Unlock()
	state.Refresh(ctx, props)
	settle(t, ctx, props, &state)
	if state.Err() != nil {
		t.Errorf("a query that worked left the error: %v", state.Err())
	}
}

func TestANewerQueryCancelsTheOneInFlight(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 600)
	source := &plates{delay: 40 * time.Millisecond}
	props := tableProps(source)
	var state widget.DataTable

	props.Layout(ctx, &state)
	state.Refresh(ctx, props)
	settle(t, ctx, props, &state)

	if state.Err() != nil {
		t.Errorf("the superseded query's cancellation reached the screen as an error: %v", state.Err())
	}
}

func TestASelectionHoldsKeysAndSurvivesTheQuery(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 600)
	props := tableProps(&plates{})
	var state widget.DataTable

	settle(t, ctx, props, &state)
	state.Select("ABC0003", true)
	state.Select("ABC0009", true)
	state.Select("ABC0003", false)

	state.Refresh(ctx, props)
	settle(t, ctx, props, &state)

	if got := state.Selected(); len(got) != 1 || got[0] != "ABC0009" {
		t.Errorf("the selection is %v, want only ABC0009", got)
	}
	state.ClearSelection()
	if len(state.Selected()) != 0 {
		t.Error("ClearSelection left rows ticked")
	}
}
