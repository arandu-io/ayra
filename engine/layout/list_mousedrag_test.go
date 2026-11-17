package layout

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
)

// A touch panel that reaches the system as a mouse: the window sets
// Context.MouseDragScrolls, every list in it follows, and the item under the
// finger is not opened by the drag that moved the list.
//
// The first item holds a click, the way a row of a table does. Its height is
// three times the viewport's so that it is still on screen after the scroll,
// which is what lets it be told it was cancelled.
func dragAList(t *testing.T, mouseDragScrolls bool) (offset int, got []gesture.ClickKind) {
	t.Helper()
	var router input.Router
	gtx := Context{
		Ops:              new(op.Ops),
		Constraints:      Constraints{Max: image.Pt(10, 20)},
		Source:           router.Source(),
		MouseDragScrolls: mouseDragScrolls,
	}
	var click gesture.Click
	row := func(gtx Context, index int) Dimensions {
		size := image.Pt(10, 60)
		if index == 0 {
			defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
			click.Add(gtx.Ops)
			for {
				e, ok := click.Update(gtx.Source)
				if !ok {
					break
				}
				got = append(got, e.Kind)
			}
		}
		return Dimensions{Size: size}
	}

	l := List{Axis: Vertical}
	step := func(evts ...pointer.Event) {
		for _, e := range evts {
			router.Queue(e)
		}
		gtx.Ops.Reset()
		l.Layout(gtx, 4, row)
		router.Frame(gtx.Ops)
	}
	mouse := func(kind pointer.Kind, y float32) pointer.Event {
		e := pointer.Event{Kind: kind, Source: pointer.Mouse, Position: f32.Pt(5, y)}
		if kind != pointer.Release {
			e.Buttons = pointer.ButtonPrimary
		}
		return e
	}

	step()
	step(mouse(pointer.Press, 15))
	step(mouse(pointer.Move, 10)) // past the slop: the pointer is claimed
	step(mouse(pointer.Move, 5))
	step(mouse(pointer.Release, 5))
	step()
	return l.Position.Offset, got
}

func TestListScrollsWithAMouseDragWhenTheWindowAsks(t *testing.T) {
	offset, clicks := dragAList(t, true)
	if offset != 10 {
		t.Errorf("offset %d after a mouse drag of 10 with the option on; want 10", offset)
	}
	for _, k := range clicks {
		if k == gesture.KindClick {
			t.Errorf("the drag ended in a click: %v", clicks)
		}
	}
	if len(clicks) == 0 || clicks[len(clicks)-1] != gesture.KindCancel {
		t.Errorf("the click under the drag was not cancelled: %v", clicks)
	}
}

func TestListIgnoresAMouseDragByDefault(t *testing.T) {
	offset, clicks := dragAList(t, false)
	if offset != 0 {
		t.Errorf("offset %d after a mouse drag with the option off; want 0", offset)
	}
	if len(clicks) == 0 || clicks[len(clicks)-1] != gesture.KindClick {
		t.Errorf("the mouse press was not left a click: %v", clicks)
	}
}
