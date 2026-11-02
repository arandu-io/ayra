package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// TestAFilePickerRowIsAsTallAsAFinger: a row was 36 points, under the 44 a touch
// target needs.
func TestAFilePickerRowIsAsTallAsAFinger(t *testing.T) {
	p := newPanel()
	p.size = image.Pt(400, 2000)
	var state widget.FilePicker
	one := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.FilePickerProps{Entries: []widget.FileEntry{{Name: "a.txt"}}}.Layout(c, &state)
	})
	two := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.FilePickerProps{Entries: []widget.FileEntry{{Name: "a.txt"}, {Name: "b.txt"}}}.Layout(c, &state)
	})
	if row := two.Size.Y - one.Size.Y; row < 44 {
		t.Errorf("a row of the listing is %d tall, want at least 44", row)
	}
}

// TestAFilePickerSpeaksTheLanguageItIsGiven: "Filter", "Empty" and the rest were
// English with no way to change them.
func TestAFilePickerSpeaksTheLanguageItIsGiven(t *testing.T) {
	p := newPanel()
	p.size = image.Pt(160, 800)
	var a, b widget.FilePicker
	english := p.draw(func(c ayra.Context) ayra.Dimensions { return widget.FilePickerProps{}.Layout(c, &a) })
	words := widget.FilePickerWords{Filter: "Filtrar", Empty: "Vazia", EmptyBody: "Não há nada nesta pasta, nem arquivos, nem outras pastas dentro dela para abrir."}
	portuguese := p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.FilePickerProps{Words: words}.Layout(c, &b)
	})
	if english.Size == portuguese.Size {
		t.Error("the sentences given changed nothing that was drawn")
	}
}
