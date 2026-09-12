package widget

import (
	"image"
	"math"

	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/unit"
)

// Float tracks a value selected along a line.
type Float struct {
	// Value is the selected fraction in the inclusive range from zero to one.
	Value float32

	drag   gesture.Drag
	axis   layout.Axis
	length float32
}

// Dragging reports whether a pointer is interacting with the value.
func (f *Float) Dragging() bool { return f.drag.Dragging() }

// Layout updates the value and registers a drag area along axis.
func (f *Float) Layout(gtx layout.Context, axis layout.Axis, pointerMargin unit.Dp) layout.Dimensions {
	f.Update(gtx)

	size := gtx.Constraints.Min
	f.axis = axis
	f.length = float32(axis.Convert(size).X)

	margin := axis.Convert(image.Pt(gtx.Dp(pointerMargin), 0))
	area := image.Rectangle{
		Min: margin.Mul(-1),
		Max: size.Add(margin),
	}
	defer clip.Rect(area).Push(gtx.Ops).Pop()
	f.drag.Add(gtx.Ops)

	return layout.Dimensions{Size: size}
}

// Update applies pending drag events and reports whether Value changed.
func (f *Float) Update(gtx layout.Context) bool {
	changed := f.setValue(f.Value)
	for {
		event, ok := f.drag.Update(gtx.Metric, gtx.Source, gesture.Axis(f.axis))
		if !ok {
			return changed
		}
		if f.length <= 0 || event.Kind != pointer.Press && event.Kind != pointer.Drag {
			continue
		}

		position := event.Position.X
		if f.axis == layout.Vertical {
			position = f.length - event.Position.Y
		}
		changed = f.setValue(position/f.length) || changed
	}
}

func (f *Float) setValue(value float32) bool {
	value = unitFraction(value)
	if value == f.Value {
		return false
	}
	f.Value = value
	return true
}

func unitFraction(value float32) float32 {
	switch {
	case math.IsNaN(float64(value)), value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}
