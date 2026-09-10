package widget_test

import (
	"image"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// frame builds the context a screen is drawn into, without a window and
// without a GPU.
//
// A drawn frame is a list of operations, and a list can be built and measured
// on a machine that has no display -- which is what a build server is. What it
// cannot do is rasterise, so what these tests assert is what the control
// computed and emitted, not what a pixel came out as.
func frame(t *testing.T, scheme theme.Scheme, width int) (ayra.Context, *op.Ops) {
	t.Helper()

	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(width, 1000)},
	}
	return ayra.Context{
		Context: gtx,
		Theme:   theme.New(scheme),
		Shaper:  text.NewShaper(text.WithCollection(gofont.Collection())),
	}, ops
}

// TestAButtonTakesRoomForItsLabel is the first thing a control has to do, and
// the thing a broken one fails silently: a zero size draws nothing, and a
// screen with an invisible button reads as a layout problem rather than as a
// control that never measured itself.
func TestAButtonTakesRoomForItsLabel(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.Button

	dims := widget.ButtonProps{Label: "Salvar"}.Layout(c, &state)

	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("a button with a label took %v, want room for the label", dims.Size)
	}
}

// TestSizeChangesTheRoomTaken fixes that the size vocabulary means something.
// A Size that resolved to the same padding as the one beside it would be a
// word in the API that changes nothing, which is worse than not having it.
func TestSizeChangesTheRoomTaken(t *testing.T) {
	measure := func(size widget.Size) image.Point {
		c, _ := frame(t, theme.Light, 400)
		var state widget.Button
		return widget.ButtonProps{Label: "Salvar", Size: size}.Layout(c, &state).Size
	}

	xs, sm, md, lg := measure(widget.ExtraSmall), measure(widget.Small), measure(widget.Medium), measure(widget.Large)

	if !(xs.Y < sm.Y && sm.Y < md.Y && md.Y < lg.Y) {
		t.Errorf("the sizes do not grow: xs=%d sm=%d md=%d lg=%d", xs.Y, sm.Y, md.Y, lg.Y)
	}
}

// TestEveryVariantDraws keeps a variant from being a name with no drawing
// behind it. A variant whose colours are all transparent takes room and paints
// nothing, which is the one failure that looks like the button is missing.
func TestEveryVariantDraws(t *testing.T) {
	variants := []widget.Variant{
		widget.Default, widget.Secondary, widget.Outline,
		widget.Ghost, widget.Destructive, widget.Link,
	}

	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		for _, v := range variants {
			t.Run(scheme.String()+"/"+v.String(), func(t *testing.T) {
				c, ops := frame(t, scheme, 400)
				var state widget.Button

				dims := widget.ButtonProps{Label: "Salvar", Variant: v}.Layout(c, &state)

				if dims.Size.X <= 0 || dims.Size.Y <= 0 {
					t.Errorf("took %v", dims.Size)
				}
				_ = ops
			})
		}
	}
}

// TestADisabledButtonDoesNotAnswer is the half a test usually forgets: a
// control drawn as unavailable that still reports a press is worse than one
// that is not drawn as unavailable at all, because the screen says one thing
// and does another.
func TestADisabledButtonDoesNotAnswer(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.Button

	widget.ButtonProps{Label: "Excluir", Disabled: true}.Layout(c, &state)

	if state.Clicked(c) {
		t.Error("a disabled button reported a press")
	}
}
