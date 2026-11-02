package widget_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// TestADisabledFieldInTheDarkIsHalfwayToTheGround: its colours were halved in
// alpha, and the compositor blends in linear light, so the typed text of an
// unavailable field was four fifths as bright as an available one's.
func TestADisabledFieldInTheDarkIsHalfwayToTheGround(t *testing.T) {
	var state widget.Input
	state.SetText("MMMMMMMM")
	brightest := func(disabled bool) uint8 {
		screen := func(c ayra.Context) ayra.Dimensions {
			return layout.UniformInset(16).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return widget.InputProps{Disabled: disabled}.Layout(c.With(gtx), &state)
			})
		}
		r := newRig(t, 300, 80, 2, theme.Dark)
		r.settle(screen)
		if err := r.win.Frame(&r.ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rectangle{Max: r.size})
		if err := r.win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		var top uint8
		for y := 40; y < 100; y++ {
			for x := 50; x < 200; x++ {
				if got := img.At(x, y).(color.RGBA); got.G > top {
					top = got.G
				}
			}
		}
		return top
	}
	on, off := brightest(false), brightest(true)
	if off > 160 || off < 100 || on < 230 {
		t.Errorf("the typed text is %d enabled and %d disabled, want about 255 and 135", on, off)
	}
}
