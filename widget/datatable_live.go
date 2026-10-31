package widget

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
)

// Live tells a table that the rows it shows changed on the server.
//
// It is the hook for the mechanism an Arandu application already has for its web
// tables: the server writes a row, and a socket carries one small notice -- "the
// table changed", with no row, id or count in it -- to whoever is looking at the
// table, who then repeats the request they already made. A native table does the
// same: when the notice arrives it asks again for exactly what it shows, the
// search, the order, the facets and every page loaded, and swaps the answer in
// when all of it is there. Nothing goes inert and nothing scrolls: the person
// reading the third screen of a list keeps reading it.
//
// The transport is the application's, because the socket, its address and the
// credential it opens with are. Subscribe returns a channel that receives once
// per notice and is closed, or simply abandoned, when the context ends.
type Live interface {
	Subscribe(ctx context.Context) <-chan struct{}
}

// LiveFunc is a function as a [Live].
type LiveFunc func(ctx context.Context) <-chan struct{}

// Subscribe calls the function.
func (f LiveFunc) Subscribe(ctx context.Context) <-chan struct{} { return f(ctx) }

// LiveEvent is the [Live] for a table whose notices are frames of the form the
// web tables use, {"event": "cameras.changed", "data": {...}}. It forwards each
// frame that names the event and drops the others, so a socket shared by
// several tables can feed each of them. frames opens the socket and yields its
// text messages; it is the application's.
func LiveEvent(event string, frames func(ctx context.Context) <-chan []byte) Live {
	return LiveFunc(func(ctx context.Context) <-chan struct{} {
		notices := make(chan struct{}, 1)
		go func() {
			defer close(notices)
			source := frames(ctx)
			for {
				select {
				case <-ctx.Done():
					return
				case frame, ok := <-source:
					if !ok {
						return
					}
					var message struct {
						Event string `json:"event"`
					}
					if json.Unmarshal(frame, &message) != nil || message.Event != event {
						continue
					}
					select {
					case notices <- struct{}{}:
					default:
						// One notice is already waiting: a burst is one repeat of
						// the request, not as many as there were frames.
					}
				}
			}
		}()
		return notices
	})
}

// subscribe starts listening for notices. It is called once, when the table is
// first drawn, and lives until [DataTable.Close].
func (e *tableExtras) subscribe(p DataTableProps, d *DataTable) {
	if p.Live == nil {
		return
	}
	ctx, stop := context.WithCancel(context.Background())
	e.stop = stop
	notices := p.Live.Subscribe(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-notices:
				if !ok {
					return
				}
				d.Notify()
			}
		}
	}()
}

// Notify says the rows changed on the server. The next frame asks again for what
// is on screen. It is safe to call from any goroutine, and is what a [Live]
// does; an application with its own way of knowing calls it directly.
func (d *DataTable) Notify() {
	d.m.dirty.Store(true)
	d.mu.Lock()
	redraw := d.m.redraw
	d.mu.Unlock()
	if redraw != nil {
		redraw()
	}
}

// Close ends the subscription to the live notices and abandons the query in
// flight. A screen that is left calls it; a table that is never closed keeps a
// goroutine waiting for a notice.
func (d *DataTable) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.m.stop != nil {
		d.m.stop()
		d.m.stop = nil
	}
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	d.seq++
}

// refetch asks again for everything that was on screen: the page, or in an
// infinite list every page loaded so far, in order. It answers only when all of
// them have, so the swap is whole.
func refetch(ctx context.Context, source Source, q Query, pages int) (Result, error) {
	if pages <= 1 {
		return source.Fetch(ctx, q)
	}
	var all Result
	for page := 1; page <= pages; page++ {
		q.Page = page
		part, err := source.Fetch(ctx, q)
		if err != nil {
			return Result{}, err
		}
		all.Total, all.Facets = part.Total, part.Facets
		all.Rows = append(all.Rows, newRows(all.Rows, part.Rows)...)
	}
	return all, nil
}

// newRows are the rows of next that are not already among have. A row inserted
// on the server while the list was open shifts every page by one, and the row at
// the seam arrives twice; showing it twice would be two rows with one key.
func newRows(have, next []Row) []Row {
	keys := make(map[string]bool, len(have))
	for _, row := range have {
		keys[row.Key] = true
	}
	out := make([]Row, 0, len(next))
	for _, row := range next {
		if keys[row.Key] {
			continue
		}
		keys[row.Key] = true
		out = append(out, row)
	}
	return slices.Clip(out)
}

// textOf trims a text facet's value the way a search is trimmed: the spaces a
// keyboard adds are not part of what was asked.
func textOf(value string) string { return strings.TrimSpace(value) }
