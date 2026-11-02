package frame_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// The listing as Kyse draws it from v0.48 and v0.49, in pictures: the card whose
// title is the first column it keeps, with the empty cell left out and the actions
// as its foot under a rule; the table whose headings stand over their values; and
// the footer in the singular. A picture of one is no evidence of the others, so
// each is its own screen.

func parityScreens() []screen {
	return []screen{
		{name: "listing-cards-bar", width: 390, height: 560, draw: func() ayra.Widget { return barListing(390, 560, widget.ModeAuto, 8, false) }},
		{name: "listing-card-long", width: 390, height: 420, draw: func() ayra.Widget { return barListing(390, 420, widget.ModeAuto, 2, true) }},
		{name: "listing-table-narrow", width: 390, height: 420, draw: func() ayra.Widget { return barListing(390, 420, widget.ModeTable, 5, false) }},
		{name: "listing-empty-cards", width: 390, height: 300, draw: func() ayra.Widget { return barListing(390, 300, widget.ModeAuto, 0, false) }},
		{name: "listing-empty-table", width: 768, height: 300, draw: func() ayra.Widget { return barListing(768, 300, widget.ModeAuto, 0, false) }},
		{name: "listing-cards-parity", width: 390, height: 700, draw: func() ayra.Widget { return parityListing(390, 700, 14, 6) }},
		{name: "listing-one-row", width: 390, height: 330, draw: func() ayra.Widget { return parityListing(390, 330, 1, 6) }},
		{name: "listing-table-parity", width: 768, height: 360, draw: func() ayra.Widget { return parityListing(768, 360, 14, 4) }},
	}
}

func TestListingsDrawAsKyseDoesFromV048(t *testing.T) {
	for _, screen := range parityScreens() {
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

// parityReads answers n readings: the camera of every other one is empty, and only
// an alert has an action of its own to take.
func parityReads(n int) widget.Source {
	return widget.SourceFunc(func(_ context.Context, q widget.Query) (widget.Result, error) {
		var rows []widget.Row
		for i := 0; i < n; i++ {
			plate := fmt.Sprintf("ABC%dD%02d", i%10, i)
			camera := []string{"Garagem", "Portão", "Recepção", "Doca 2"}[i%4]
			if i%2 == 1 {
				camera = ""
			}
			status := []string{"Lida", "Alerta", "Lida"}[i%3]
			rows = append(rows, widget.Row{
				Key:   plate,
				Cells: []string{fmt.Sprint(1000 + i), plate, camera, fmt.Sprintf("%d%%", 90+i%10), status, ""},
				Data:  map[string]any{"plate": plate, "mercosul": i%3 != 0},
			})
		}
		from := min((q.Page-1)*q.PageSize, len(rows))
		return widget.Result{Rows: rows[from:min(from+q.PageSize, len(rows))], Total: len(rows)}, nil
	})
}

func parityListing(width, height, rows, size int) ayra.Widget {
	var table widget.DataTable
	props := widget.DataTableProps{
		Source:     parityReads(rows),
		Searchable: true,
		Columns: []widget.DataColumn{
			// The id is the first column and is left out of the card, so the plate
			// is its title with nothing marked Primary.
			{Key: "id", Title: "Código", CardHidden: true, Numeric: true},
			{Key: "plate", Title: "Placa", Sortable: true, Flex: 2, Cell: func(row widget.Row) ayra.Widget {
				// The plate is the second cell of the row, after the code.
				shifted := row
				shifted.Cells = row.Cells[1:]
				return plateCell(shifted)
			}},
			{Key: "camera", Title: "Câmera", Sortable: true, Flex: 2},
			{Key: "confidence", Title: "Confiança", Numeric: true, Align: widget.End},
			{Key: "status", Title: "Estado", Cell: statusCell},
			{Key: "act", Title: "Ação", Actions: true, Cell: func(row widget.Row) ayra.Widget {
				if row.Cells[4] != "Alerta" {
					return nil
				}
				return func(c ayra.Context) ayra.Dimensions {
					return widget.BadgeProps{Label: "Revisar", Variant: widget.Outline}.Layout(c)
				}
			}},
		},
		Facets:    []widget.Facet{{Key: "status", Title: "Estado", Options: []widget.FacetOption{{Value: "Lida"}, {Value: "Alerta"}}}},
		Actions:   []widget.RowAction{{Key: "open", Label: "Abrir", Icon: stroke(0)}},
		PageSizes: []int{size},
		Words: widget.DataTableWords{
			Search: "Buscar placa", Empty: "Nenhum resultado.", Filters: "Filtros", Apply: "Aplicar", Reset: "Limpar",
			Close: "Fechar", SortBy: "Ordenar por", Page: "Página {n} de {total}",
			RowsOne: "{total} leitura.", RowsMany: "{total} leituras.", Selected: "{n} de {total} leituras selecionadas.",
			PerPage: "{n} por página", Previous: "Anterior", Next: "Próxima",
		},
	}
	props.Selectable = width >= 600
	if props.Selectable {
		table.Select("ABC1D01", true)
	}
	warm(props, &table, width, height)
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, &table) }
}

// barListing is a list of cameras with the controls Kyse keeps on a phone: columns
// that order the list, a menu of columns, a value on two lines against its label,
// and the sentence of an empty list long enough to need the box it stands in.
func barListing(width, height int, mode widget.DataTableMode, rows int, long bool) ayra.Widget {
	var table widget.DataTable
	source := widget.SourceFunc(func(_ context.Context, q widget.Query) (widget.Result, error) {
		var out []widget.Row
		for i := 0; i < rows; i++ {
			name := []string{"Cancela leste", "Doca de carga", "Garagem subsolo", "Portaria principal"}[i%4]
			place := "Bloco B"
			if long {
				place = "Estacionamento subterrâneo do bloco B, corredor leste, próximo à rampa"
			}
			out = append(out, widget.Row{Key: fmt.Sprint(i), Cells: []string{name, "RTSP", place, "Data e hora " + fmt.Sprint(i)}})
		}
		return widget.Result{Rows: out, Total: len(out)}, nil
	})
	props := widget.DataTableProps{
		Source: source, Searchable: true, Mode: mode,
		Columns: []widget.DataColumn{
			{Key: "name", Title: "Nome", Sortable: true, Flex: 2},
			{Key: "protocol", Title: "Protocolo", Sortable: true, Hideable: true},
			{Key: "place", Title: "Local", Hideable: true, Flex: 2},
			{Key: "when", Title: "Data e hora", Sortable: true, Align: widget.End},
		},
		PageSizes: []int{25},
		Words: widget.DataTableWords{
			Search: "Nome ou local", Columns: "Colunas", Filters: "Filtros",
			Empty:   "Nenhuma câmera foi encontrada com esses filtros. Limpe a busca ou escolha outro período.",
			RowsOne: "{total} linha.", RowsMany: "{total} linhas.", PerPage: "{n} por página", Previous: "Anterior", Next: "Próxima",
			Page: "Página {n} de {total}",
		},
	}
	warm(props, &table, width, height)
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, &table) }
}
