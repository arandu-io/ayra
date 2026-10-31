package frame_test

import (
	"context"
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// The listing is drawn twice, because it is two layouts and a picture of one
// says nothing about the other: the cards a phone gets, with the filter button
// beside the search and an action under each card, and the table a tablet or a
// window gets from the same props. Both hold the same rows, so the difference
// between the pictures is the layout and not the data.

func listings() []screen {
	return []screen{
		{name: "listing-narrow", width: 360, height: 640, draw: func() ayra.Widget { return listing(360, 640, false) }},
		{name: "listing-wide", width: 860, height: 420, draw: func() ayra.Widget { return listing(860, 420, false) }},
		{name: "listing-filters", width: 360, height: 640, draw: func() ayra.Widget { return listing(360, 640, true) }},
	}
}

func TestListingsLookLikeTheyDid(t *testing.T) {
	for _, screen := range listings() {
		for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
			for _, density := range []float32{1, 2} {
				name := pictureName(screen.name, scheme, density)

				t.Run(name, func(t *testing.T) {
					got, err := frame.Draw(frame.Options{
						Width: screen.width, Height: screen.height, Density: density, Scheme: scheme, Fonts: fonts(),
					}, screen.draw())
					if err != nil {
						if noSurface(err) {
							t.Skipf("no drawing surface on this machine: %v", err)
						}
						t.Fatalf("the screen did not draw: %v", err)
					}
					compare(t, name, got)
				})
			}
		}
	}
}

// reads is a list of plate readings the way a route would answer it: filtered,
// sorted and cut by the query, so the picture is the table's and not the data's.
func reads() widget.Source {
	cameras := []string{"Garagem", "Portão", "Recepção", "Doca 2"}
	return widget.SourceFunc(func(_ context.Context, q widget.Query) (widget.Result, error) {
		var rows []widget.Row
		for i := 0; i < 37; i++ {
			plate := fmt.Sprintf("ABC%dD%02d", i%10, i)
			if i%3 == 0 {
				plate = fmt.Sprintf("BRA%04d", 2000+i)
			}
			status := []string{"Lida", "Alerta", "Lida"}[i%3]
			if want := q.Filters["status"]; len(want) > 0 && !strings.Contains(strings.Join(want, ","), status) {
				continue
			}
			rows = append(rows, widget.Row{
				Key: plate,
				Cells: []string{
					plate, cameras[i%4], fmt.Sprintf("31/10/2026 %02d:%02d", 8+i%10, (i*7)%60), fmt.Sprintf("%d%%", 90+i%10), status,
				},
				Data: map[string]any{"plate": plate, "mercosul": i%3 != 0, "status": status},
			})
		}
		from := min((q.Page-1)*q.PageSize, len(rows))
		return widget.Result{Rows: rows[from:min(from+q.PageSize, len(rows))], Total: len(rows)}, nil
	})
}

// plateCell draws a plate with the application's own widget: the Mercosul band or
// the old grey one, and the characters in the monospaced face.
func plateCell(row widget.Row) ayra.Widget {
	mercosul, _ := row.Data["mercosul"].(bool)
	return func(c ayra.Context) ayra.Dimensions {
		style := theme.Style{}
		style.Background = theme.Colour(c.Theme.Colours.Muted)
		style.Border = theme.Colour(c.Theme.Colours.Foreground)
		style.BorderWidth = theme.Pt(1.5)
		style.Radius = theme.Pt(4)
		style.Padding = theme.InsetXY(8, 3)
		return widget.Surface(c, style, func(c ayra.Context) ayra.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !mercosul {
						return layout.Dimensions{}
					}
					size := image.Pt(gtx.Dp(6), gtx.Dp(16))
					paint.FillShape(gtx.Ops, c.Theme.Colours.Primary, clip.Rect{Max: size}.Op())
					return layout.Dimensions{Size: image.Pt(size.X+gtx.Dp(6), size.Y)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.DrawText(c.With(gtx), row.Cells[0], theme.Style{}, widget.TextOptions{Monospace: true, MaxLines: 1, Align: text.Start})
				}),
			)
		})
	}
}

// statusCell draws the badge of a reading.
func statusCell(row widget.Row) ayra.Widget {
	label := row.Cells[4]
	variant := widget.Secondary
	if label == "Alerta" {
		variant = widget.Destructive
	}
	return func(c ayra.Context) ayra.Dimensions {
		return widget.BadgeProps{Label: label, Variant: variant}.Layout(c)
	}
}

// stroke is an icon: the library's own icons are a separate piece, so the
// picture draws a stroke.
func stroke(rotate float32) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		side := c.Dp(unit.Dp(18))
		var path clip.Path
		path.Begin(c.Ops)
		path.MoveTo(f32.Pt(float32(side)*0.15, float32(side)*0.85))
		path.LineTo(f32.Pt(float32(side)*0.85, float32(side)*0.15+rotate))
		stroke := clip.Stroke{Path: path.End(), Width: float32(c.Dp(2))}.Op()
		paint.FillShape(c.Ops, c.Theme.Colours.Foreground, stroke)
		return ayra.Dimensions{Size: image.Pt(side, side)}
	}
}

func listing(width, height int, filters bool) ayra.Widget {
	var table widget.DataTable
	props := widget.DataTableProps{
		Source:     reads(),
		Searchable: true,
		Columns: []widget.DataColumn{
			{Key: "plate", Title: "Placa", Sortable: true, Flex: 2, Primary: true, Cell: plateCell},
			{Key: "camera", Title: "Câmera", Sortable: true, Flex: 2},
			{Key: "at", Title: "Lida em", Sortable: true, Flex: 3},
			{Key: "confidence", Title: "Confiança", Numeric: true, Align: widget.End},
			{Key: "status", Title: "Estado", Cell: statusCell},
		},
		Facets: []widget.Facet{
			{Key: "status", Title: "Estado", Options: []widget.FacetOption{{Value: "Lida"}, {Value: "Alerta"}}},
			{Key: "camera", Title: "Câmera", Kind: widget.FacetText, Placeholder: "Nome da câmera"},
			{Key: "de", EndKey: "ate", Title: "Período", Kind: widget.FacetPeriod, Placeholder: "Escolher período"},
		},
		Actions: []widget.RowAction{
			{Key: "open", Label: "Abrir", Icon: stroke(0)},
			{Key: "delete", Label: "Excluir", Icon: stroke(8), Destructive: true},
		},
		PageSizes: []int{6, 12},
		Infinite:  true,
		Words: widget.DataTableWords{
			Search: "Buscar placa", Empty: "Nenhum resultado.", Filters: "Filtros", Apply: "Aplicar", Reset: "Limpar",
			Close: "Fechar", SortBy: "Ordenar por", Rows: "{total} leitura(s).", Page: "Página {n} de {total}",
			Refresh: "Puxe para atualizar", Release: "Solte para atualizar", Refreshing: "Atualizando...",
		},
	}

	warm(props, &table, width, height)
	opened := false
	return func(c ayra.Context) ayra.Dimensions {
		if filters && !opened {
			opened = true
			props.OpenFilters(&table)
		}
		return props.Layout(c, &table)
	}
}

// warm settles the table before the picture is drawn: a frame is drawn once, and
// the answer of a source arrives after it. A throwaway frame asks and waits.
func warm(props widget.DataTableProps, table *widget.DataTable, width, height int) {
	ctx := ayra.Context{
		Context: layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(width, height)}},
		Theme:   theme.New(theme.Light),
		Shaper:  text.NewShaper(text.WithCollection(gofont.Collection())),
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ctx.Ops.Reset()
		props.Layout(ctx, table)
		if len(table.Rows()) > 0 && !table.Loading() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}
