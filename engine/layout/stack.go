package layout

import (
	"image"

	"github.com/arandu-io/ayra/engine/op"
)

// Stack places children in the same bounds, one above the previous child.
type Stack struct {
	// Alignment places children that are smaller than the stack bounds.
	Alignment Direction
}

// StackChild is a child and the rule Stack uses to measure it.
type StackChild struct {
	expanded bool
	widget   Widget
}

// Stacked makes a child choose its size without a minimum constraint.
func Stacked(widget Widget) StackChild {
	return StackChild{widget: widget}
}

// Expanded makes a child receive the largest size already established by the
// stack as its minimum constraint.
func Expanded(widget Widget) StackChild {
	return StackChild{expanded: true, widget: widget}
}

type stackMeasurement struct {
	call op.CallOp
	dims Dimensions
}

// Layout measures ordinary stacked children before expanded children, then
// draws every child in argument order. Later children therefore appear above
// earlier children even when they had to be measured first.
func (s Stack) Layout(gtx Context, children ...StackChild) Dimensions {
	var local [32]stackMeasurement
	measurements := local[:]
	if len(children) > len(measurements) {
		measurements = make([]stackMeasurement, len(children))
	} else {
		measurements = measurements[:len(children)]
	}

	childContext := gtx
	childContext.Constraints.Min = image.Point{}
	bounds := image.Point{}
	for i, child := range children {
		if child.expanded {
			continue
		}
		measurements[i] = measureStackChild(gtx, childContext, child.widget)
		bounds = largestPoint(bounds, measurements[i].dims.Size)
	}

	for i, child := range children {
		if !child.expanded {
			continue
		}
		childContext.Constraints.Min = bounds
		measurements[i] = measureStackChild(gtx, childContext, child.widget)
		bounds = largestPoint(bounds, measurements[i].dims.Size)
	}

	bounds = gtx.Constraints.Constrain(bounds)
	baseline := 0
	for _, measured := range measurements {
		position := s.Alignment.Position(measured.dims.Size, bounds)
		transform := op.Offset(position).Push(gtx.Ops)
		measured.call.Add(gtx.Ops)
		transform.Pop()

		if baseline == 0 && measured.dims.Baseline != 0 {
			baseline = measured.dims.Baseline + bounds.Y - measured.dims.Size.Y - position.Y
		}
	}
	return Dimensions{Size: bounds, Baseline: baseline}
}

func measureStackChild(parent, child Context, widget Widget) stackMeasurement {
	recording := op.Record(parent.Ops)
	dims := widget(child)
	return stackMeasurement{call: recording.Stop(), dims: dims}
}

func largestPoint(a, b image.Point) image.Point {
	if b.X > a.X {
		a.X = b.X
	}
	if b.Y > a.Y {
		a.Y = b.Y
	}
	return a
}

// Background lays out one widget over a background sized to contain it.
type Background struct{}

// Layout measures widget first, draws background, then centres and draws the
// widget above it. The returned size is the background size.
func (Background) Layout(gtx Context, background, widget Widget) Dimensions {
	recording := op.Record(gtx.Ops)
	foreground := widget(gtx)
	foregroundCall := recording.Stop()

	backgroundContext := gtx
	backgroundContext.Constraints.Min = gtx.Constraints.Constrain(foreground.Size)
	backdrop := background(backgroundContext)

	position := Center.Position(foreground.Size, backdrop.Size)
	if position == (image.Point{}) {
		foregroundCall.Add(gtx.Ops)
	} else {
		transform := op.Offset(position).Push(gtx.Ops)
		foregroundCall.Add(gtx.Ops)
		transform.Pop()
	}

	return Dimensions{
		Size:     backdrop.Size,
		Baseline: foreground.Baseline + backdrop.Size.Y - foreground.Size.Y - position.Y,
	}
}
