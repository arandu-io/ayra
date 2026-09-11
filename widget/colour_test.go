package widget_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// device is a frame with a pointer attached, which the plain frame has not got.
//
// A drag is the only way into this control that is not typing, so a test that
// cannot press cannot say whether the areas answer at all. The router is what
// a window would hold: the operations are handed to it after each frame, and
// the events queued against it are delivered on the next one -- which is also
// why every assertion here is about the frame after the press.
type device struct {
	router input.Router
	ops    *op.Ops
	size   image.Point
	shaper *text.Shaper
}

func newDevice(width, height int) *device {
	return &device{
		ops:    new(op.Ops),
		size:   image.Pt(width, height),
		shaper: text.NewShaper(text.WithCollection(gofont.Collection())),
	}
}

// draw runs one frame and hands the operations to the router.
func (d *device) draw(props widget.ColourPickerProps, state *widget.ColourPicker) ayra.Dimensions {
	d.ops.Reset()

	gtx := layout.Context{
		Ops:         d.ops,
		Source:      d.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: d.size},
	}
	dims := props.Layout(ayra.Context{
		Context: gtx,
		Theme:   theme.New(theme.Light),
		Shaper:  d.shaper,
	}, state)

	d.router.Frame(d.ops)
	return dims
}

// press queues a primary press at a point.
func (d *device) press(x, y int) {
	d.router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(float32(x), float32(y)),
	})
}

// release queues the letting go, which is what completes a click. A press
// alone moves the dragged areas and finishes nothing that is pressed.
func (d *device) release(x, y int) {
	d.router.Queue(pointer.Event{
		Kind:     pointer.Release,
		Source:   pointer.Mouse,
		Position: f32.Pt(float32(x), float32(y)),
	})
}

// TestAPickerTakesTheWidthItIsOffered is the first thing a control has to do,
// and the thing a broken one fails silently.
//
// Every part of this stacks in a column, and a child of a column is handed a
// minimum size of nothing. A control that measured itself from the minimum
// would take no room, draw nothing, and read as a layout problem rather than
// as a missing measurement.
func TestAPickerTakesTheWidthItIsOffered(t *testing.T) {
	for _, width := range []int{200, 400, 800} {
		c, _ := frame(t, theme.Light, width)
		var state widget.ColourPicker

		dims := widget.ColourPickerProps{}.Layout(c, &state)

		if dims.Size.X != width {
			t.Errorf("in %d of room the picker took %d across", width, dims.Size.X)
		}
		if dims.Size.Y <= 0 {
			t.Errorf("in %d of room the picker took %d down", width, dims.Size.Y)
		}
	}
}

// TestEveryPartAddsToTheRoomTaken keeps a prop from being a word that changes
// nothing.
//
// An alpha bar that drew nothing, or a row of presets that took no room, is a
// field in the API with no drawing behind it -- and the one failure that looks
// like the control is simply missing.
func TestEveryPartAddsToTheRoomTaken(t *testing.T) {
	measure := func(props widget.ColourPickerProps) int {
		c, _ := frame(t, theme.Light, 400)
		var state widget.ColourPicker
		return props.Layout(c, &state).Size.Y
	}

	plain := measure(widget.ColourPickerProps{})
	withAlpha := measure(widget.ColourPickerProps{Alpha: true})
	withPresets := measure(widget.ColourPickerProps{
		Swatches: []color.NRGBA{{R: 255, A: 255}, {G: 255, A: 255}},
	})

	if withAlpha <= plain {
		t.Errorf("the alpha bar added %d to the height", withAlpha-plain)
	}
	if withPresets <= plain {
		t.Errorf("the row of presets added %d to the height", withPresets-plain)
	}
}

// TestEveryPartDrawsInBothSchemes keeps a control from being written against
// one palette.
func TestEveryPartDrawsInBothSchemes(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		t.Run(scheme.String(), func(t *testing.T) {
			c, _ := frame(t, scheme, 400)
			var state widget.ColourPicker
			state.SetColour(color.NRGBA{R: 120, G: 200, B: 40, A: 180})

			dims := widget.ColourPickerProps{
				Alpha:    true,
				Swatches: []color.NRGBA{{R: 255, A: 255}},
			}.Layout(c, &state)

			if dims.Size.X <= 0 || dims.Size.Y <= 0 {
				t.Errorf("took %v", dims.Size)
			}
		})
	}
}

// TestAPickerInNoRoomDrawsNothingRatherThanCrashing fixes the division a plane
// of no width would do.
//
// A column that has run out of room hands its next child nothing, and the
// fractions this control computes are a position over a size. Over a size of
// nothing they are not numbers, and a handle placed from one is drawn at a
// coordinate no clamp catches.
func TestAPickerInNoRoomDrawsNothingRatherThanCrashing(t *testing.T) {
	for _, size := range []image.Point{{X: 0, Y: 0}, {X: 0, Y: 500}, {X: 400, Y: 0}} {
		ops := new(op.Ops)
		gtx := layout.Context{
			Ops:         ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: size},
		}
		c := ayra.Context{
			Context: gtx,
			Theme:   theme.New(theme.Light),
			Shaper:  text.NewShaper(text.WithCollection(gofont.Collection())),
		}

		var state widget.ColourPicker
		dims := widget.ColourPickerProps{Alpha: true}.Layout(c, &state)

		if dims.Size.X < 0 || dims.Size.Y < 0 {
			t.Errorf("in %v the picker took %v", size, dims.Size)
		}
		if got := state.Colour(); got != (color.NRGBA{A: 255}) {
			t.Errorf("in %v the colour became %v", size, got)
		}
	}
}

// TestPressingThePlaneSetsSaturationAndValue is the control's whole point, and
// it needs a pointer to say anything at all.
//
// Across is saturation and down is value, inverted. A plane that ran the other
// way would be right in the arithmetic and upside down on the screen, and no
// test that only checks the numbers moved would notice.
func TestPressingThePlaneSetsSaturationAndValue(t *testing.T) {
	for _, test := range []struct {
		name string
		x, y int
		want color.NRGBA
	}{
		{"the top right corner is the hue at full", 399, 0, color.NRGBA{R: 255, A: 255}},
		{"the top left corner is white", 0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		{"the bottom left corner is black", 0, 139, color.NRGBA{A: 255}},
		{"the bottom right corner is black too", 399, 139, color.NRGBA{A: 255}},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := newDevice(400, 2000)
			var state widget.ColourPicker

			// A half-bright red: the hue is known, and it sits in the middle
			// of the plane so that every corner is somewhere else. Started at
			// a corner, the press for that corner would move nothing and the
			// change it reports would be the one thing not under test.
			state.SetColour(color.NRGBA{R: 128, A: 255})

			d.draw(widget.ColourPickerProps{}, &state) // registers the handlers
			d.press(test.x, test.y)
			d.draw(widget.ColourPickerProps{}, &state) // reads the press

			if got := state.Colour(); got != test.want {
				t.Errorf("a press at %d,%d left the picker on %v, want %v", test.x, test.y, got, test.want)
			}
			if !state.Changed() {
				t.Error("the press was not reported as a change")
			}
		})
	}
}

// TestAPressIsReportedOnceAndNotAgain is the defect this package writes
// whenever a control has something to say.
//
// A picker that raised its event from its own drawing would send the same
// colour for as long as the frames kept coming, which from the far end is a
// client that will not stop talking.
func TestAPressIsReportedOnceAndNotAgain(t *testing.T) {
	d := newDevice(400, 2000)
	var state widget.ColourPicker

	props := widget.ColourPickerProps{Alpha: true}
	d.draw(props, &state)
	d.press(200, 40)
	d.draw(props, &state)

	if !state.Changed() {
		t.Fatal("the press was not reported")
	}
	for frame := 0; frame < 4; frame++ {
		d.draw(props, &state)

		if state.Changed() {
			t.Fatalf("frame %d after the press reported it again", frame)
		}
	}
}

// TestPressingTheHueBarMovesTheHue fixes that the bar under the plane is the
// one that turns the plane a different colour.
func TestPressingTheHueBarMovesTheHue(t *testing.T) {
	d := newDevice(360, 2000)
	var state widget.ColourPicker
	state.SetColour(color.NRGBA{R: 255, A: 255}) // red, hue nought

	props := widget.ColourPickerProps{}
	d.draw(props, &state)

	// The bar sits below the plane and a gap. Rather than counting them, the
	// press walks down the column until the hue moves: what is being fixed is
	// that a press on the bar changes the hue, not where the bar happens to be
	// laid out this week.
	//
	// Each press is let go of before the next. A second press while the first
	// is still down is not a second press to the router -- the pointer is
	// already held -- so a walk that only presses tries one position and then
	// talks to nothing.
	moved := false
	for y := 140; y < 200 && !moved; y++ {
		d.press(180, y)
		d.release(180, y)
		d.draw(props, &state)

		if state.Colour() != (color.NRGBA{R: 255, A: 255}) {
			moved = true
		}
		state.Changed()
	}

	if !moved {
		t.Fatal("no press below the plane moved the hue")
	}

	// Half way along the bar is cyan, which is the hue at a hundred and
	// eighty. A bar drawn from a table with two arms swapped still runs red to
	// red and is wrong exactly here.
	got := state.Colour()
	if got.R > 60 || got.G < 180 || got.B < 180 {
		t.Errorf("half way along the hue bar is %v, want something near cyan", got)
	}
}

// TestPressingAPresetTakesItsColour fixes that the row is a shortcut rather
// than a decoration.
func TestPressingAPresetTakesItsColour(t *testing.T) {
	presets := []color.NRGBA{
		{R: 255, A: 255},
		{R: 0, G: 128, B: 255, A: 255},
	}

	d := newDevice(400, 2000)
	var state widget.ColourPicker

	props := widget.ColourPickerProps{Swatches: presets}
	dims := d.draw(props, &state)

	// The row is the last thing in the column, so the second square is
	// somewhere along its bottom edge. Walking it is the same argument as
	// above: the assertion is about what a press does, not about the spacing.
	taken := false
	for y := dims.Size.Y - 22; y < dims.Size.Y && !taken; y++ {
		// The second square: past the first one and past the gap.
		d.press(22+6+11, y)
		d.release(22+6+11, y)
		d.draw(props, &state)

		if state.Colour() == presets[1] {
			taken = true
		}
	}

	if !taken {
		t.Errorf("no press on the row took the second preset; the picker is on %v", state.Colour())
	}
}

// TestADisabledColourPickerDoesNotAnswer is the half a test usually forgets: a
// control drawn as unavailable that still moves is worse than one not drawn as
// unavailable at all, because the screen says one thing and does another.
func TestADisabledColourPickerDoesNotAnswer(t *testing.T) {
	d := newDevice(400, 2000)
	var state widget.ColourPicker
	state.SetColour(color.NRGBA{R: 255, A: 255})

	props := widget.ColourPickerProps{Disabled: true, Alpha: true}
	d.draw(props, &state)
	d.press(200, 40)
	d.draw(props, &state)

	if got := state.Colour(); got != (color.NRGBA{R: 255, A: 255}) {
		t.Errorf("a disabled picker moved to %v", got)
	}
	if state.Changed() {
		t.Error("a disabled picker reported a change")
	}
}
