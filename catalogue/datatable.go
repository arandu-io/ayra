package main

import (
	"context"
	"fmt"
	"image"
	"slices"
	"sort"
	"strings"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/widget"
)

// dataTableEntry draws the listing twice, from the same props: the cards a phone
// is given, and the table a window is given.
func dataTableEntry() Entry {
	return Entry{
		Control: "DataTableProps",
		Summary: "A listing that asks a server for its rows: search, order, filters, pages or an endless list, and cards under 600 points.",
		Build: func() ayra.Widget {
			var phone, window widget.DataTable
			props := func(infinite bool) widget.DataTableProps {
				return widget.DataTableProps{
					Source:     plates{},
					Searchable: true,
					Infinite:   infinite,
					Columns: []widget.DataColumn{
						{Key: "plate", Title: "Plate", Sortable: true, Flex: 2, Numeric: true, Primary: true},
						{Key: "camera", Title: "Camera", Sortable: true, Flex: 2},
						{Key: "status", Title: "Status"},
					},
					Facets: []widget.Facet{
						{Key: "status", Title: "Status", Options: []widget.FacetOption{
							{Value: "read", Label: "Read"}, {Value: "unread", Label: "Unread"},
						}},
						{Key: "camera", Title: "Camera", Kind: widget.FacetText, Placeholder: "Camera name"},
						{Key: "from", EndKey: "to", Title: "Period", Kind: widget.FacetPeriod, Placeholder: "Choose a period"},
					},
					Actions: []widget.RowAction{
						{Key: "open", Label: "Open", Icon: rowGlyph},
						{Key: "delete", Label: "Delete", Icon: rowGlyph, Destructive: true},
					},
					PageSizes: []int{5, 10},
				}
			}
			cards, table := props(true), props(false)
			return func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// A phone's width and a screen's height: the list scrolls inside
						// the room it is given, so its end and its top are things it sees.
						inner := c.With(gtx)
						inner.Constraints.Max = image.Pt(min(c.Dp(360), gtx.Constraints.Max.X), c.Dp(520))
						inner.Constraints.Min = image.Pt(inner.Constraints.Max.X, 0)
						return cards.Layout(inner, &phone)
					}),
					layout.Rigid(layout.Spacer{Height: 24}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						inner := c.With(gtx)
						room := min(c.Dp(860), gtx.Constraints.Max.X)
						inner.Constraints.Min.X, inner.Constraints.Max.X = room, room
						return table.Layout(inner, &window)
					}),
				)
			}
		},
	}
}

// rowGlyph is the icon of a row action. The library's icons are a separate
// piece; the catalogue draws a stroke so the action has something to press.
func rowGlyph(c ayra.Context) ayra.Dimensions {
	side := c.Dp(18)
	var path clip.Path
	path.Begin(c.Ops)
	path.MoveTo(f32.Pt(float32(side)*0.15, float32(side)*0.85))
	path.LineTo(f32.Pt(float32(side)*0.85, float32(side)*0.15))
	paint.FillShape(c.Ops, c.Theme.Colours.Foreground, clip.Stroke{Path: path.End(), Width: float32(c.Dp(2))}.Op())
	return ayra.Dimensions{Size: image.Pt(side, side)}
}

// plates is the source the catalogue's table asks: a list in memory that answers
// a query the way a server would.
type plates struct{}

func (plates) Fetch(ctx context.Context, q widget.Query) (widget.Result, error) {
	var rows []widget.Row
	for i := 1; i <= 42; i++ {
		plate := fmt.Sprintf("ABC%dD%02d", i%10, i)
		status := []string{"read", "unread"}[i%2]
		if q.Search != "" && !strings.Contains(plate, strings.ToUpper(q.Search)) {
			continue
		}
		if want := q.Filters["status"]; len(want) > 0 && !slices.Contains(want, status) {
			continue
		}
		rows = append(rows, widget.Row{Key: plate, Cells: []string{plate, fmt.Sprintf("Gate %d", i%4+1), status}, Data: map[string]any{"plate": plate, "status": status}})
	}
	if q.Sort.Key != "" {
		column := map[string]int{"plate": 0, "camera": 1}[q.Sort.Key]
		sort.SliceStable(rows, func(a, b int) bool {
			if q.Sort.Descending {
				return rows[a].Cells[column] > rows[b].Cells[column]
			}
			return rows[a].Cells[column] < rows[b].Cells[column]
		})
	}
	from := min((q.Page-1)*q.PageSize, len(rows))
	to := min(from+q.PageSize, len(rows))
	return widget.Result{Rows: rows[from:to], Total: len(rows)}, nil
}
