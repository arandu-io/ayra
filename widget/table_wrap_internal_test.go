package widget

import (
	"testing"

	"github.com/arandu-io/ayra/theme"
)

// The text of a cell, and of a row of a list, is said whole: it wraps instead of
// ending in an ellipsis that hides what tells one row from the next.
func TestTheTextOfACellAndOfARowWrapsInsteadOfBeingCut(t *testing.T) {
	long := "Câmera da entrada principal do estacionamento subterrâneo do bloco B, corredor leste"
	c, _ := field(t, theme.Light, 200)

	short := TableProps{Columns: []Column{{Title: "Câmera"}}, Rows: [][]string{{"Garagem"}}}.Layout(c).Size.Y
	wrapped := TableProps{Columns: []Column{{Title: "Câmera"}}, Rows: [][]string{{long}}}.Layout(c).Size.Y
	if wrapped < short+30 {
		t.Errorf("a table row with a long cell is %d tall against %d with a short one: the text is cut to one line", wrapped, short)
	}

	var state Button
	one := ItemProps{Title: "Garagem", Body: "Online"}.Layout(c, &state, nil).Size.Y
	title := ItemProps{Title: long, Body: "Online"}.Layout(c, &state, nil).Size.Y
	body := ItemProps{Title: "Garagem", Body: long}.Layout(c, &state, nil).Size.Y
	if title <= one || body <= one {
		t.Errorf("a list row of %d (long title) and %d (long body) is no taller than %d: the text is cut to one line", title, body, one)
	}
}
