package widget

import (
	"errors"
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
)

// The footer says one row in the singular, as Kyse's DataTableLines does from
// v0.48: "1 row." and "{total} rows.", and Rows still says one sentence for
// every count for a project that wrote it before the split.
func TestTheFooterSaysOneRowInTheSingular(t *testing.T) {
	for _, test := range []struct {
		name  string
		words DataTableWords
		total int
		want  string
	}{
		{"one", DataTableWords{}, 1, "1 row."},
		{"many", DataTableWords{}, 37, "37 rows."},
		{"none", DataTableWords{}, 0, "0 rows."},
		{"the two forms", DataTableWords{RowsOne: "{total} linha.", RowsMany: "{total} linhas."}, 1, "1 linha."},
		{"the plural", DataTableWords{RowsOne: "{total} linha.", RowsMany: "{total} linhas."}, 2, "2 linhas."},
		{"the old sentence, one", DataTableWords{Rows: "{total} linha(s)."}, 1, "1 linha(s)."},
		{"the old sentence, many", DataTableWords{Rows: "{total} linha(s)."}, 8, "8 linha(s)."},
		{"a form beside the old sentence wins for its count", DataTableWords{Rows: "{total} linha(s).", RowsOne: "{total} linha."}, 1, "1 linha."},
		{"and the old sentence says the other", DataTableWords{Rows: "{total} linha(s).", RowsOne: "{total} linha."}, 3, "3 linha(s)."},
		{"the first spelling of the number", DataTableWords{Rows: "{n} leituras"}, 5, "5 leituras"},
	} {
		if got := test.words.rows(test.total); got != test.want {
			t.Errorf("%s: the footer of %d says %q, want %q", test.name, test.total, got, test.want)
		}
	}
}

// A selectable table says how many rows are ticked of the whole list, in Kyse's
// sentence, and of the rows it has when its source does not count.
func TestTheFooterOfASelectionSaysWhatIsTickedOfTheTotal(t *testing.T) {
	if got, want := (DataTableWords{}).selected(2, 37), "2 of 37 row(s) selected."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := (DataTableWords{}).selected(2, -1), "2 row(s) selected."; got != want {
		t.Errorf("a source that does not count: got %q, want %q", got, want)
	}
	if got, want := (DataTableWords{Selected: "{n} de {total} selecionadas."}).selected(1, 9), "1 de 9 selecionadas."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The title of a card is the first column marked Primary, and with none marked
// the first one the card does not leave out: a column marked CardHidden is never
// the title, as in Kyse's PrimaryColumn.
func TestTheTitleOfACardIsTheFirstColumnItKeeps(t *testing.T) {
	for _, test := range []struct {
		name    string
		columns []DataColumn
		want    int
	}{
		{"none marked", []DataColumn{{Title: "a"}, {Title: "b"}}, 0},
		{"the first is left out", []DataColumn{{Title: "id", CardHidden: true}, {Title: "b"}, {Title: "c"}}, 1},
		{"one marked", []DataColumn{{Title: "a"}, {Title: "b", Primary: true}}, 1},
		{"every column left out", []DataColumn{{Title: "a", CardHidden: true}}, -1},
		{"no columns", nil, -1},
	} {
		if got := (DataTableProps{Columns: test.columns}).primary(); got != test.want {
			t.Errorf("%s: the title is column %d, want %d", test.name, got, test.want)
		}
	}
}

// cardHeight draws one row as a card and says how tall it came out.
func cardHeight(t *testing.T, p DataTableProps, row Row) int {
	t.Helper()
	c, _ := field(t, theme.Light, 360)
	state := &DataTable{}
	state.rows = make([]Button, 1)
	state.checks = make([]Toggle, 1)
	return p.card(c, state, row, 0, false).Size.Y
}

// A cell with nothing in it is left out of the card: the name of its column is
// not drawn in front of nothing.
func TestACardLeavesAnEmptyCellOut(t *testing.T) {
	p := DataTableProps{Columns: []DataColumn{{Title: "Placa"}, {Title: "Câmera"}, {Title: "Lida em"}}}
	full := cardHeight(t, p, Row{Key: "1", Cells: []string{"ABC1D23", "Garagem", "hoje"}})
	blank := cardHeight(t, p, Row{Key: "2", Cells: []string{"ABC1D23", "  ", "hoje"}})
	short := cardHeight(t, p, Row{Key: "3", Cells: []string{"ABC1D23"}})

	if blank >= full {
		t.Errorf("a card with an empty cell is %d tall, as tall as the one with its value (%d): the name of the column stands alone", blank, full)
	}
	if short >= blank {
		t.Errorf("a card whose row has one cell is %d tall, as tall as one with a cell left empty (%d)", short, blank)
	}
}

// The title is drawn even when it is empty, so a card is never headless.
func TestACardKeepsItsTitleEvenWhenItIsEmpty(t *testing.T) {
	p := DataTableProps{Columns: []DataColumn{{Title: "Placa"}, {Title: "Câmera"}}}
	empty := cardHeight(t, p, Row{Key: "1", Cells: []string{"", "Garagem"}})
	filled := cardHeight(t, p, Row{Key: "2", Cells: []string{"ABC1D23", "Garagem"}})
	if empty != filled {
		t.Errorf("a card with an empty title is %d tall and %d with one", empty, filled)
	}
}

// The column marked Actions is the foot of the card: under a rule, at the end, and
// absent from a row that has nothing there.
func TestTheActionsColumnIsTheFootOfTheCard(t *testing.T) {
	p := DataTableProps{Columns: []DataColumn{
		{Title: "Placa"},
		{Title: "Ações", Actions: true, Cell: func(row Row) ayra.Widget {
			if row.Key != "with" {
				return nil
			}
			return func(c ayra.Context) ayra.Dimensions { return ayra.Dimensions{Size: image.Pt(60, 30)} }
		}},
	}}
	without := cardHeight(t, p, Row{Key: "without", Cells: []string{"ABC1D23", ""}})
	with := cardHeight(t, p, Row{Key: "with", Cells: []string{"ABC1D23", ""}})
	bare := cardHeight(t, DataTableProps{Columns: p.Columns[:1]}, Row{Key: "bare", Cells: []string{"ABC1D23"}})

	if without != bare {
		t.Errorf("a row with no action is %d tall, and %d without the column: it draws a foot or a label for nothing", without, bare)
	}
	if with < bare+30 {
		t.Errorf("a row with an action is %d tall, want at least %d: the foot is missing", with, bare+30)
	}
}

// The actions of a row are the foot too: a rule above them, and room around it.
func TestTheActionsOfARowStandUnderARule(t *testing.T) {
	columns := []DataColumn{{Title: "Placa"}}
	plain := cardHeight(t, DataTableProps{Columns: columns}, Row{Key: "1", Cells: []string{"ABC1D23"}})
	acted := cardHeight(t, DataTableProps{Columns: columns, Actions: []RowAction{{Key: "open", Label: "Abrir"}}}, Row{Key: "1", Cells: []string{"ABC1D23"}})

	// The rule is one hairline and a gap of 8 points on each side of it, over the
	// button: without it the first icon is drawn against the last line of text.
	if min := plain + 8 + 8 + 1; acted < min+24 {
		t.Errorf("a card with an action is %d tall against %d without; with a rule and its gaps it is at least %d", acted, plain, min+24)
	}
}

// The box in the heading says what is so: ticked when every row of the page is,
// and not once a row is not.
func TestTheHeadingBoxFollowsTheSelection(t *testing.T) {
	c, _ := field(t, theme.Light, 700)
	p := DataTableProps{Selectable: true, Columns: []DataColumn{{Key: "a", Title: "A"}}}
	state := &DataTable{}
	result := Result{Rows: []Row{{Key: "1", Cells: []string{"x"}}, {Key: "2", Cells: []string{"y"}}}, Total: 2}

	p.headings(c, state, result)
	if state.everyone.On() {
		t.Fatal("the box is ticked with nothing selected")
	}
	state.Select("1", true)
	state.Select("2", true)
	p.headings(c, state, result)
	if !state.everyone.On() {
		t.Error("every row is ticked and the box in the heading is not")
	}
	state.Select("2", false)
	p.headings(c, state, result)
	if state.everyone.On() {
		t.Error("a row was unticked and the box in the heading is still ticked")
	}
	p.headings(c, state, Result{})
	if state.everyone.On() {
		t.Error("a page with no rows has its box ticked")
	}
}

// An error that comes after the first answer is said: the rows on screen belong
// to the question before, and a table that keeps drawing them under a failure
// nobody reads is a table that lies.
func TestAFailureAfterTheFirstAnswerIsSaid(t *testing.T) {
	c, _ := field(t, theme.Light, 700)
	drawn := 0
	p := DataTableProps{Columns: []DataColumn{{Key: "a", Title: "A", Cell: func(Row) ayra.Widget {
		return func(c ayra.Context) ayra.Dimensions { drawn++; return ayra.Dimensions{Size: image.Pt(10, 10)} }
	}}}}
	state := &DataTable{}
	state.rows = make([]Button, 2)
	result := Result{Rows: []Row{{Key: "1", Cells: []string{"x"}}, {Key: "2", Cells: []string{"y"}}}, Total: 2}

	p.table(c, state, result, true, nil, false)
	if drawn != 2 {
		t.Fatalf("the rows were drawn %d times with no error, want 2", drawn)
	}
	drawn = 0
	p.table(c, state, result, true, errors.New("503"), false)
	if drawn != 0 {
		t.Errorf("the rows of the question before were drawn %d times under a failure", drawn)
	}
}

// A heading is given the width its title needs, and the rest of the row is shared
// by the columns' Flex: "Data e hora" is not broken over two lines to make room
// for a value, and a column that would be narrower takes the title's width.
func TestAHeadingIsGivenTheWidthItNeeds(t *testing.T) {
	for _, test := range []struct {
		name   string
		total  int
		shares []float32
		mins   []int
		want   []int
	}{
		{"every share is wide enough", 300, []float32{1, 1, 1}, []int{10, 10, 10}, []int{100, 100, 100}},
		{"one title needs more", 300, []float32{1, 1, 1}, []int{10, 160, 10}, []int{70, 160, 70}},
		{"the rest shares what is left by its shares", 300, []float32{1, 1, 2}, []int{10, 150, 10}, []int{50, 150, 100}},
		{"a column that is off takes no room", 200, []float32{1, -1, 1}, []int{10, 10, 10}, []int{100, 0, 100}},
		{"titles that do not fit are scaled", 100, []float32{1, 1}, []int{80, 80}, []int{50, 50}},
	} {
		got := shareWidths(test.total, test.shares, test.mins)
		sum := 0
		for i := range got {
			sum += got[i]
			if got[i] != test.want[i] {
				t.Errorf("%s: widths %v, want %v", test.name, got, test.want)
				break
			}
		}
		if sum != test.total {
			t.Errorf("%s: the columns add up to %d of %d", test.name, sum, test.total)
		}
	}
}

// The table's heading never breaks: with a narrow row it is as tall as with a
// wide one.
func TestTheHeadingOfATableDoesNotBreakOverTwoLines(t *testing.T) {
	c, _ := field(t, theme.Light, 360)
	p := DataTableProps{Columns: []DataColumn{{Key: "a", Title: "Placa", Flex: 2}, {Key: "b", Title: "Câmera", Flex: 2}, {Key: "c", Title: "Data e hora", Align: End}}}
	state := &DataTable{}
	state.headers = make([]Button, 3)
	narrow := p.headings(c, state, Result{}).Size.Y
	wide, _ := field(t, theme.Light, 900)
	if got := p.headings(wide, state, Result{}).Size.Y; narrow != got {
		t.Errorf("the heading is %d tall in 360 points and %d in 900: it breaks over two lines", narrow, got)
	}
	widths := p.columnWidths(c, 360)
	if need := headingWidth(c, "Data e hora"); widths[2] < need {
		t.Errorf("the last column is %d wide, its title needs %d", widths[2], need)
	}
}

// An empty list says so in a box: centred across and down, 96 points tall, and the
// same in the table and in the cards, for a sentence of any length.
func TestAnEmptyListIsCentredInABox(t *testing.T) {
	short := DataTableProps{Words: DataTableWords{Empty: "Nada."}}
	long := DataTableProps{Words: DataTableWords{Empty: "Nenhuma placa da lista de bloqueados foi vista ainda, volte quando houver passagens."}}
	c, _ := field(t, theme.Light, 360)

	a, b := short.emptyState(c), long.emptyState(c)
	if a.Size.X != 360 || b.Size.X != 360 {
		t.Errorf("the box is %d and %d wide, want the whole 360", a.Size.X, b.Size.X)
	}
	if a.Size.Y < 96 {
		t.Errorf("the box of a short sentence is %d tall, want at least 96", a.Size.Y)
	}
	if b.Size.Y < a.Size.Y {
		t.Errorf("a sentence that wraps drew a box %d tall, shorter than the short one's %d", b.Size.Y, a.Size.Y)
	}
}

// A column switched off is not drawn, in the table or in the cards, and comes
// back when it is switched on.
func TestAColumnSwitchedOffIsNotDrawn(t *testing.T) {
	drawn := 0
	column := func(key string) DataColumn {
		return DataColumn{Key: key, Title: key, Hideable: true, Cell: func(Row) ayra.Widget {
			if key == "b" {
				return func(c ayra.Context) ayra.Dimensions { drawn++; return ayra.Dimensions{Size: image.Pt(10, 10)} }
			}
			return nil
		}}
	}
	p := DataTableProps{Columns: []DataColumn{{Key: "a", Title: "a"}, column("b")}}
	row := Row{Key: "1", Cells: []string{"x", "y"}}
	c, _ := field(t, theme.Light, 700)
	state := &DataTable{}
	state.rows, state.checks = make([]Button, 1), make([]Toggle, 1)

	p.cells(c, row)
	if drawn == 0 {
		t.Fatal("the column is not drawn while it is showing")
	}
	drawn = 0
	state.HideColumn("b", true)
	hidden := p.shown(state)
	hidden.cells(c, row)
	hidden.card(c, state, row, 0, false)
	if drawn != 0 {
		t.Errorf("a column switched off was drawn %d times", drawn)
	}
	if !hidden.Columns[1].off || p.Columns[1].off {
		t.Error("the table marked the caller's columns, or not its own")
	}
	state.HideColumn("b", false)
	if shown := p.shown(state); shown.Columns[1].off {
		t.Error("a column switched back on is still off")
	}
	if got := len(p.hideable()); got != 1 {
		t.Errorf("the menu lists %d columns, want the one that is Hideable", got)
	}
}

// The cards have the bar of order controls: one for each column that can be
// ordered, so a phone orders the list without opening anything.
func TestTheCardsCarryTheBarOfOrderControls(t *testing.T) {
	columns := func(sortable bool) []DataColumn {
		return []DataColumn{{Key: "a", Title: "Nome", Sortable: sortable}, {Key: "b", Title: "Local", Sortable: sortable}}
	}
	with, without := DataTableProps{Columns: columns(true)}, DataTableProps{Columns: columns(false)}
	if !with.hasBar() || without.hasBar() {
		t.Fatalf("the bar is drawn for %v and for %v, want sortable columns only", with.hasBar(), without.hasBar())
	}
	menu := DataTableProps{Columns: []DataColumn{{Key: "a", Title: "Nome", Hideable: true}}}
	if !menu.hasBar() {
		t.Error("a table with columns to switch off has no bar for their menu")
	}
	c, _ := field(t, theme.Light, 360)
	state := &DataTable{}
	state.headers = make([]Button, 2)
	if h := with.cardsBar(c, state).Size.Y; h < 20 {
		t.Errorf("the bar is %d tall, too short to hold the controls", h)
	}
	if got := with.sorts(); len(got) != 2 {
		t.Errorf("the bar orders by %v, want both columns", got)
	}
}
