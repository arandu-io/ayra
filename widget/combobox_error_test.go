package widget_test

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

type failing struct{}

func (failing) Search(context.Context, string) ([]widget.Option, error) {
	return nil, errors.New("sem conexão")
}

// redBelow counts the red pixels under y, in pixels: a pixel is red when its red
// is well above both of its other channels, which is the destructive ink and its
// edges in either scheme and nothing else the field draws.
func (r *rig) redBelow(y int) int {
	img := image.NewRGBA(image.Rectangle{Max: r.size})
	if err := r.win.Screenshot(img); err != nil {
		r.t.Fatal(err)
	}
	n := 0
	for yy := y; yy < r.size.Y; yy++ {
		for x := 0; x < r.size.X; x++ {
			got := img.At(x, yy).(color.RGBA)
			if int(got.R)-int(got.G) > 100 && int(got.R)-int(got.B) > 100 {
				n++
			}
		}
	}
	return n
}

// TestAFailedSearchIsSaidUnderTheField: the error was kept in the state and
// drawn nowhere, so a search that failed looked like one that found nothing --
// or, with no message for "nothing", like a field that did not answer.
func TestAFailedSearchIsSaidUnderTheField(t *testing.T) {
	var state widget.Combobox
	props := widget.ComboboxProps{Source: failing{}, Debounce: -1, Empty: "Nenhum resultado.", Failed: "Não foi possível buscar."}
	screen := func(c ayra.Context) ayra.Dimensions {
		return widget.ComboboxProps{Source: props.Source, Debounce: props.Debounce, Empty: props.Empty, Failed: props.Failed}.Layout(c, &state)
	}
	r := newRig(t, 390, 200, 2, theme.Light)
	r.draw(screen)
	r.tap(screen, 100, 16)
	r.typed(screen, "ma")
	for i := 0; i < 50 && state.Err() == nil; i++ {
		time.Sleep(10 * time.Millisecond)
		r.draw(screen)
	}
	if state.Err() == nil {
		t.Fatal("the search did not fail")
	}
	r.settle(screen)
	if err := r.win.Frame(&r.ops); err != nil {
		t.Fatal(err)
	}
	if n := r.redBelow(120); n < 40 {
		t.Errorf("only %d destructive pixels under the field: the failure is not said", n)
	}
}

// TestAnInvalidComboboxIsDrawnRejected is the field's own look.
func TestAnInvalidComboboxIsDrawnRejected(t *testing.T) {
	var state widget.Combobox
	screen := func(c ayra.Context) ayra.Dimensions {
		return widget.ComboboxProps{Invalid: true}.Layout(c, &state)
	}
	r := newRig(t, 390, 120, 2, theme.Light)
	r.settle(screen)
	if err := r.win.Frame(&r.ops); err != nil {
		t.Fatal(err)
	}
	if n := r.redBelow(0); n < 100 {
		t.Errorf("only %d destructive pixels: the border is not the rejected one", n)
	}
}
