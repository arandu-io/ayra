package widget_test

import (
	"image"
	"testing"
	"time"

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

// hierarchy is what these tests press on.
func hierarchy() []widget.TreeNode {
	return []widget.TreeNode{
		{Label: "app", Children: []widget.TreeNode{
			{Label: "Http"},
			{Label: "Models"},
		}},
		{Label: "go.mod"},
	}
}

// trees is a frame with a pointer attached.
//
// A press is the only way into this control, so a test that cannot press
// cannot say whether the rows answer at all. The router is what a window would
// hold: the operations are handed to it after each frame and the events queued
// against it are delivered on the next, which is why every assertion here is
// about the frame after the press.
type trees struct {
	router input.Router
	ops    *op.Ops
	size   image.Point
	shaper *text.Shaper
}

func newTrees(width, height int) *trees {
	return &trees{
		ops:    new(op.Ops),
		size:   image.Pt(width, height),
		shaper: text.NewShaper(text.WithCollection(gofont.Collection())),
	}
}

// draw runs one frame and hands the operations to the router.
func (d *trees) draw(props widget.TreeProps, state *widget.Tree) ayra.Dimensions {
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

// click queues a press and the letting go that completes it.
func (d *trees) click(x, y int) {
	at := f32.Pt(float32(x), float32(y))
	d.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
}

// TestPressingARowMarksIt is the control working at all.
func TestPressingARowMarksIt(t *testing.T) {
	device := newTrees(300, 400)
	var state widget.Tree
	props := widget.TreeProps{Roots: hierarchy()}

	first := device.draw(props, &state)
	if first.Size.Y <= 0 {
		t.Fatal("the tree drew nothing to press")
	}

	// The second root, which with the first closed is the second row.
	row := first.Size.Y / 2
	device.click(150, row+row/2)
	device.draw(props, &state)

	path, pressed := state.Chosen()
	if !pressed {
		t.Fatal("pressing a row reported nothing")
	}
	if len(path) != 1 || path[0] != 1 {
		t.Errorf("pressing the second row marked %v", path)
	}
}

// TestADisabledTreeDoesNotAnswerAPressThatLandedBeforeIt fixes the one case a
// disabled button does not cover.
//
// A row asks whether it was pressed before it is drawn, so it is reading the
// frame that has already gone. A press that landed while the tree was live is
// therefore waiting when the tree comes back disabled, and without the guard it
// is acted on -- a mark, and whatever the caller does with one, on a control
// the person can see is unavailable.
func TestADisabledTreeDoesNotAnswerAPressThatLandedBeforeIt(t *testing.T) {
	device := newTrees(300, 400)
	var state widget.Tree
	live := widget.TreeProps{Roots: hierarchy()}

	first := device.draw(live, &state)
	row := first.Size.Y / 2
	device.click(150, row+row/2)

	device.draw(widget.TreeProps{Roots: hierarchy(), Disabled: true}, &state)

	if path, pressed := state.Chosen(); pressed {
		t.Errorf("a disabled tree marked %v from a press that landed before it", path)
	}
}

// TestPressingTheMarkerOpensTheBranchAndNotTheRow keeps the two presses a row
// carries from being one.
func TestPressingTheMarkerOpensTheBranchAndNotTheRow(t *testing.T) {
	device := newTrees(300, 400)
	var state widget.Tree
	props := widget.TreeProps{Roots: hierarchy()}

	first := device.draw(props, &state)
	row := first.Size.Y / 2

	// The marker is at the left edge of the first row.
	device.click(6, row/2)
	device.draw(props, &state)

	if !state.Expanded([]int{0}) {
		t.Error("pressing the marker did not open the branch")
	}
	if _, pressed := state.Chosen(); pressed {
		t.Error("pressing the marker also marked the row, so one press did two things")
	}
}

// TestALeafKeepsTheWidthItsMarkerWouldTake fixes a level that draws at two
// offsets.
//
// It is read through a press rather than through a measurement, because a row
// takes the width it is offered either way: what moves is where the label
// begins inside it. A point inside the marker column is blank on a leaf, and
// would be the label if the column were not reserved.
func TestALeafKeepsTheWidthItsMarkerWouldTake(t *testing.T) {
	device := newTrees(300, 200)
	var state widget.Tree
	props := widget.TreeProps{Roots: []widget.TreeNode{{Label: "go.mod"}}}

	row := device.draw(props, &state)
	if row.Size.Y <= 0 {
		t.Fatal("the leaf drew nothing")
	}

	// Between the marker column and the label: the column is ten points wide
	// and the gap after it is six, so twelve is inside the gap while the
	// column is reserved, and inside the label the moment it is not.
	device.click(12, row.Size.Y/2)
	device.draw(props, &state)

	if path, pressed := state.Chosen(); pressed {
		t.Errorf("a press in the marker column of a leaf marked %v, so the label starts where a branch's marker does", path)
	}

	// And past it, which is the label.
	device.click(60, row.Size.Y/2)
	device.draw(props, &state)

	if _, pressed := state.Chosen(); !pressed {
		t.Error("a press on the label of a leaf marked nothing")
	}
}

// TestADisabledTreeDoesNotQueueAPressOnItsMarkers is the other half of a
// disabled control, and the half a target left registered gets wrong.
//
// Refusing to act on a press is not the same as not taking it. A marker that
// still registers its target takes the press, and the branch opens on the first
// frame the tree is enabled again -- which is one frame wide, and is exactly
// the shape of a form that disables its tree while a save is in flight and
// enables it when the answer arrives. A press made on a control somebody could
// see was unavailable is then acted on, with nothing on screen connecting the
// two.
//
// So the enabling frame comes immediately after the press. Drawing one more
// disabled frame in between closes the window and the test proves nothing.
func TestADisabledTreeDoesNotQueueAPressOnItsMarkers(t *testing.T) {
	device := newTrees(300, 400)
	var state widget.Tree
	live := widget.TreeProps{Roots: hierarchy()}
	off := widget.TreeProps{Roots: hierarchy(), Disabled: true}

	first := device.draw(off, &state)
	if first.Size.Y <= 0 {
		t.Fatal("the tree drew nothing to press")
	}

	// On the first root's marker, while the tree is unavailable.
	device.click(6, first.Size.Y/4)

	device.draw(live, &state)
	if state.Expanded([]int{0}) {
		t.Error("the branch opened from a press made while the tree was unavailable")
	}
	if _, pressed := state.Chosen(); pressed {
		t.Error("a row was marked from a press made while the tree was unavailable")
	}
}

// TestPressingAMarkerOnALiveTreeStillOpensIt is what makes the test above mean
// something: the same press, one frame earlier in the tree's life, does work.
func TestPressingAMarkerOnALiveTreeStillOpensIt(t *testing.T) {
	device := newTrees(300, 400)
	var state widget.Tree
	live := widget.TreeProps{Roots: hierarchy()}

	first := device.draw(live, &state)
	device.click(6, first.Size.Y/4)
	device.draw(live, &state)

	if !state.Expanded([]int{0}) {
		t.Error("pressing the marker of a live tree opened nothing")
	}
}

// pickers is a frame with a pointer attached, for a date picker.
type pickers struct {
	router input.Router
	ops    *op.Ops
	size   image.Point
	shaper *text.Shaper
}

func newPickers(width, height int) *pickers {
	return &pickers{
		ops:    new(op.Ops),
		size:   image.Pt(width, height),
		shaper: text.NewShaper(text.WithCollection(gofont.Collection())),
	}
}

// draw runs one frame and hands the operations to the router.
func (d *pickers) draw(props widget.DatePickerProps, state *widget.DatePicker) ayra.Dimensions {
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

// click queues a press and the letting go that completes it.
func (d *pickers) click(x, y int) {
	at := f32.Pt(float32(x), float32(y))
	d.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
}

// TestPressingAnEnabledDateFieldOpensTheMonth is the control working at all,
// and it is what makes the test below mean something.
func TestPressingAnEnabledDateFieldOpensTheMonth(t *testing.T) {
	device := newPickers(400, 600)
	var state widget.DatePicker
	props := widget.DatePickerProps{Today: pickerDay()}

	field := device.draw(props, &state)
	device.click(field.Size.X/2, field.Size.Y/2)
	device.draw(props, &state)

	if !state.Showing() {
		t.Fatal("pressing the field opened nothing")
	}
}

// TestPressingADisabledDateFieldOpensNothing fixes a month shown for one frame.
//
// Closing the panel and laying it out anyway was not enough: the panel's own
// layout reads the press that toggles it, on a context nothing had disabled, so
// a press opened the month for that frame and only the frame after took it
// down. One frame is enough to be seen, and what is seen is a month of days
// that do nothing.
func TestPressingADisabledDateFieldOpensNothing(t *testing.T) {
	device := newPickers(400, 600)
	var state widget.DatePicker
	off := widget.DatePickerProps{Today: pickerDay(), Disabled: true}

	field := device.draw(off, &state)
	device.click(field.Size.X/2, field.Size.Y/2)

	for range 3 {
		device.draw(off, &state)
		if state.Showing() {
			t.Fatal("a disabled field opened its month")
		}
	}

	// And nothing was queued for when it comes back.
	device.draw(widget.DatePickerProps{Today: pickerDay()}, &state)
	if state.Showing() {
		t.Error("the month opened by itself: the press was queued rather than refused")
	}
}

// pickerDay is the day these tests work in, written down rather than read from
// the clock so that they draw the same picture next year.
func pickerDay() time.Time {
	return time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)
}
