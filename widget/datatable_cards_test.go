package widget_test

import (
	"context"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// cardProps is the table of a phone: cards, an infinite list, a plate that is
// drawn by the application.
func cardProps(source widget.Source) widget.DataTableProps {
	p := tableProps(source)
	p.Mode = widget.ModeCards
	p.Infinite = true
	p.Selectable = false
	p.PageSizes = []int{10}
	return p
}

// fill draws until every query the table has made is answered and the list has
// nothing more to ask for on its own.
func fill(t *testing.T, ctx ayra.Context, props widget.DataTableProps, state *widget.DataTable) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	stable := 0
	last := -1
	for stable < 4 {
		props.Layout(ctx, state)
		if state.Loading() {
			stable = 0
		} else if n := len(state.Rows()); n == last {
			stable++
		} else {
			stable, last = 0, n
		}
		if time.Now().After(deadline) {
			t.Fatal("the table never settled")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestAnInfiniteListAsksForTheNextPageWhenItsEndIsOnScreen(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 360)
	ctx.Constraints.Max.Y = 4000 // the room of a screen that shows every card
	source := &plates{}
	props := cardProps(source)
	var state widget.DataTable

	fill(t, ctx, props, &state)

	if got := len(state.Rows()); got != 57 {
		t.Fatalf("the list holds %d rows, want the 57 there are", got)
	}
	pages := []int{}
	for _, q := range source.seen() {
		pages = append(pages, q.Page)
	}
	for i, page := range pages {
		if page != i+1 {
			t.Fatalf("the pages were asked in the order %v, want 1, 2, 3... with none twice", pages)
		}
	}
	if len(pages) != 6 {
		t.Errorf("%d pages were asked for 57 rows of ten", len(pages))
	}
	seen := map[string]bool{}
	for _, row := range state.Rows() {
		if seen[row.Key] {
			t.Errorf("row %s is in the list twice", row.Key)
		}
		seen[row.Key] = true
	}
}

func TestAnInfiniteListDoesNotAskPastItsEnd(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 360)
	ctx.Constraints.Max.Y = 4000
	source := &plates{}
	props := cardProps(source)
	var state widget.DataTable

	fill(t, ctx, props, &state)
	asked := len(source.seen())
	for i := 0; i < 5; i++ {
		props.Layout(ctx, &state)
	}
	time.Sleep(20 * time.Millisecond)
	if len(source.seen()) != asked {
		t.Errorf("the list asked again after the last row: %d queries, was %d", len(source.seen()), asked)
	}
}

// A source that does not count ends the list at the first short page.
func TestASourceThatDoesNotCountEndsTheListAtTheFirstShortPage(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 360)
	ctx.Constraints.Max.Y = 4000
	var asked atomic.Int32
	source := widget.SourceFunc(func(_ context.Context, q widget.Query) (widget.Result, error) {
		asked.Add(1)
		rows := []widget.Row{}
		for i := 0; i < q.PageSize && (q.Page-1)*q.PageSize+i < 25; i++ {
			key := string(rune('a' + (q.Page-1)*q.PageSize + i))
			rows = append(rows, widget.Row{Key: key, Cells: []string{key, "x"}})
		}
		return widget.Result{Rows: rows, Total: -1}, nil
	})
	props := cardProps(source)
	var state widget.DataTable

	fill(t, ctx, props, &state)

	if got := len(state.Rows()); got != 25 {
		t.Errorf("the list holds %d rows, want 25", got)
	}
	if asked.Load() != 3 {
		t.Errorf("%d pages were asked, want 3: two full and one short", asked.Load())
	}
	if state.Total() >= 0 {
		t.Errorf("the table reports a total of %d for a source that does not count", state.Total())
	}
}

func TestACellIsDrawnByTheWidgetTheColumnGives(t *testing.T) {
	for name, mode := range map[string]widget.DataTableMode{"cards": widget.ModeCards, "table": widget.ModeTable} {
		t.Run(name, func(t *testing.T) {
			ctx, _ := frame(t, theme.Light, 700)
			var mu sync.Mutex
			drawn := map[string]int{}
			props := tableProps(&plates{})
			props.Mode = mode
			props.Columns[1].Cell = func(row widget.Row) ayra.Widget {
				return func(c ayra.Context) ayra.Dimensions {
					mu.Lock()
					drawn[row.Key]++
					mu.Unlock()
					return ayra.Dimensions{Size: image.Pt(40, 18)}
				}
			}
			var state widget.DataTable

			settle(t, ctx, props, &state)

			mu.Lock()
			defer mu.Unlock()
			if len(drawn) != 10 || drawn["ABC0001"] == 0 || drawn["ABC0010"] == 0 {
				t.Errorf("the plate widget was asked for %v, want the ten rows of the page", drawn)
			}
		})
	}
}

func TestARowKeepsItsDataForTheWidgetThatDrawsIt(t *testing.T) {
	source := widget.SourceFunc(func(context.Context, widget.Query) (widget.Result, error) {
		return widget.Result{Rows: []widget.Row{{Key: "1", Cells: []string{"ABC1D23", "read"}, Data: map[string]any{"plate": "ABC1D23", "mercosul": true}}}, Total: 1}, nil
	})
	ctx, _ := frame(t, theme.Light, 360)
	props := cardProps(source)
	var got widget.Row
	props.Columns[0].Cell = func(row widget.Row) ayra.Widget {
		got = row
		return nil
	}
	var state widget.DataTable
	settle(t, ctx, props, &state)
	props.Layout(ctx, &state)

	if got.Data["mercosul"] != true {
		t.Errorf("the cell was handed %+v, without the row's data", got)
	}
}

func TestANoticeAsksAgainForWhatIsOnScreenWithoutGoingInert(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 360)
	ctx.Constraints.Max.Y = 4000
	source := &plates{}
	props := cardProps(source)
	var state widget.DataTable
	fill(t, ctx, props, &state)
	before := len(source.seen())
	if before != 6 {
		t.Fatalf("setup: %d pages asked", before)
	}

	// The next answer is slow, so that the moment it is in flight can be looked at.
	source.mu.Lock()
	source.delay = 60 * time.Millisecond
	source.mu.Unlock()
	state.Notify()
	props.Layout(ctx, &state)

	if state.Loading() {
		t.Error("a notice put the table into its loading state: the rows would go inert under the reader's thumb")
	}
	if got := len(state.Rows()); got != 57 {
		t.Errorf("rows on screen while the notice is followed: %d, want them all still there", got)
	}

	deadline := time.Now().Add(5 * time.Second)
	for len(source.seen()) < before+6 {
		props.Layout(ctx, &state)
		if time.Now().After(deadline) {
			t.Fatalf("the six pages on screen were not asked for again: %d queries", len(source.seen())-before)
		}
		time.Sleep(5 * time.Millisecond)
	}
	for i, q := range source.seen()[before : before+6] {
		if q.Page != i+1 {
			t.Errorf("the notice asked page %d at step %d: it has to ask 1..6 in order, the extent that was on screen", q.Page, i+1)
		}
	}
	if state.Err() != nil {
		t.Errorf("following a notice left an error: %v", state.Err())
	}
}

func TestANoticeThatFailsLeavesTheRowsWhereTheyWere(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 600)
	source := &plates{}
	props := tableProps(source)
	var state widget.DataTable
	settle(t, ctx, props, &state)

	source.mu.Lock()
	source.fail = context.DeadlineExceeded
	source.mu.Unlock()
	state.Notify()
	deadline := time.Now().Add(2 * time.Second)
	for len(source.seen()) < 2 && time.Now().Before(deadline) {
		props.Layout(ctx, &state)
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	props.Layout(ctx, &state)

	if state.Err() != nil || len(state.Rows()) != 10 {
		t.Errorf("a notice that failed left err=%v rows=%d, want no error and the ten rows", state.Err(), len(state.Rows()))
	}
}

func TestLiveEventsForwardOnlyTheEventTheTableIsOn(t *testing.T) {
	frames := make(chan []byte, 8)
	live := widget.LiveEvent("cameras.changed", func(context.Context) <-chan []byte { return frames })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notices := live.Subscribe(ctx)

	frames <- []byte(`{"event":"alertas.changed","data":{"table":"alertas"}}`)
	frames <- []byte(`not json`)
	select {
	case <-notices:
		t.Fatal("a frame for another table was taken for a notice")
	case <-time.After(30 * time.Millisecond):
	}

	frames <- []byte(`{"event":"cameras.changed","data":{"table":"cameras","at":"2026-10-31T15:00:00Z"}}`)
	select {
	case <-notices:
	case <-time.After(time.Second):
		t.Fatal("the frame of the table did not reach it")
	}

	// A burst is one repeat of the request.
	for i := 0; i < 5; i++ {
		frames <- []byte(`{"event":"cameras.changed"}`)
	}
	time.Sleep(30 * time.Millisecond)
	n := 0
loop:
	for {
		select {
		case <-notices:
			n++
		default:
			break loop
		}
	}
	if n != 1 {
		t.Errorf("five frames in a burst made %d notices, want 1", n)
	}
}

func TestALiveTableFollowsItsSubscriptionAndStopsWhenClosed(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 600)
	source := &plates{}
	notices := make(chan struct{}, 1)
	var stopped atomic.Bool
	props := tableProps(source)
	props.Live = widget.LiveFunc(func(c context.Context) <-chan struct{} {
		go func() { <-c.Done(); stopped.Store(true) }()
		return notices
	})
	var state widget.DataTable
	settle(t, ctx, props, &state)

	notices <- struct{}{}
	deadline := time.Now().Add(2 * time.Second)
	for len(source.seen()) < 2 && time.Now().Before(deadline) {
		props.Layout(ctx, &state)
		time.Sleep(2 * time.Millisecond)
	}
	if len(source.seen()) < 2 {
		t.Fatal("a notice on the subscription did not ask the source again")
	}

	state.Close()
	deadline = time.Now().Add(time.Second)
	for !stopped.Load() && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if !stopped.Load() {
		t.Error("closing the table left its subscription open")
	}
}

func TestAnActionIsReportedOnceAndOnlyWhenPressed(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 360)
	ctx.Constraints.Max.Y = 1 << 22 // a column that grows, so the height says what is inside
	props := cardProps(&plates{})
	props.Actions = []widget.RowAction{{Key: "edit", Label: "Edit"}, {Key: "delete", Label: "Delete", Destructive: true}}
	var state widget.DataTable
	settle(t, ctx, props, &state)

	if row, action, ok := state.Action(); ok {
		t.Errorf("an action was reported with nothing pressed: %s %s", row, action)
	}

	plain := cardProps(&plates{})
	var other widget.DataTable
	settle(t, ctx, plain, &other)
	with := props.Layout(ctx, &state).Size.Y
	without := plain.Layout(ctx, &other).Size.Y
	if with <= without {
		t.Errorf("cards with actions are %d tall, no taller than without them (%d)", with, without)
	}
}

func TestTheCardsTakeTheHeightTheyAreGivenAndTheTableOnlyWhatItNeeds(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 360)
	ctx.Constraints.Max.Y = 700
	props := cardProps(&plates{})
	var state widget.DataTable
	settle(t, ctx, props, &state)

	if h := props.Layout(ctx, &state).Size.Y; h != 700 {
		t.Errorf("a list of cards given a screen of 700 took %d: it has to fill it, or the end of it is never where the thumb is", h)
	}
}
