package layout

import (
	"image"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
)

// Constraints describe the smallest and largest size available to a widget.
type Constraints struct {
	Min, Max image.Point
}

// Dimensions describe the space occupied by a widget.
// Baseline is measured upward from the bottom edge and is zero for widgets
// without a text baseline.
type Dimensions struct {
	Size     image.Point
	Baseline int
}

// Axis identifies the direction in which a layout advances.
type Axis uint8

const (
	// Horizontal advances from left to right.
	Horizontal Axis = iota
	// Vertical advances from top to bottom.
	Vertical
)

// Alignment places siblings across their layout axis.
type Alignment uint8

const (
	// Start places children at the leading cross-axis edge.
	Start Alignment = iota
	// End places children at the trailing cross-axis edge.
	End
	// Middle centers children on the cross axis.
	Middle
	// Baseline aligns children by their text baselines.
	Baseline
)

// Direction places a widget inside a larger rectangle.
type Direction uint8

const (
	// NW places a widget at the top-left corner.
	NW Direction = iota
	// N places a widget at the center of the top edge.
	N
	// NE places a widget at the top-right corner.
	NE
	// E places a widget at the center of the right edge.
	E
	// SE places a widget at the bottom-right corner.
	SE
	// S places a widget at the center of the bottom edge.
	S
	// SW places a widget at the bottom-left corner.
	SW
	// W places a widget at the center of the left edge.
	W
	// Center places a widget at the center of its bounds.
	Center
)

// Widget lays out one user interface element.
type Widget func(gtx Context) Dimensions

// Exact returns constraints that admit only size.
func Exact(size image.Point) Constraints {
	return Constraints{Min: size, Max: size}
}

// FPt converts an integer point to a floating-point point.
func FPt(p image.Point) f32.Point {
	return f32.Pt(float32(p.X), float32(p.Y))
}

// Constrain clamps size to the inclusive range from Min to Max.
func (c Constraints) Constrain(size image.Point) image.Point {
	size.X = constrainDimension(size.X, c.Min.X, c.Max.X)
	size.Y = constrainDimension(size.Y, c.Min.Y, c.Max.Y)
	return size
}

func constrainDimension(value, minimum, maximum int) int {
	if value < minimum {
		value = minimum
	}
	if value > maximum {
		value = maximum
	}
	return value
}

// AddMin adjusts Min by delta, keeping both coordinates between zero and Max.
func (c Constraints) AddMin(delta image.Point) Constraints {
	minimum := c.Min.Add(delta)
	minimum.X = max(0, minimum.X)
	minimum.Y = max(0, minimum.Y)
	c.Min = minimum
	c.Min = c.Constrain(c.Min)
	return c
}

// SubMax reduces Max by delta without allowing negative coordinates. Min is
// reduced as needed so the resulting interval remains valid.
func (c Constraints) SubMax(delta image.Point) Constraints {
	c.Max = c.Max.Sub(delta)
	c.Max.X = max(0, c.Max.X)
	c.Max.Y = max(0, c.Max.Y)
	c.Min = c.Constrain(c.Min)
	return c
}

// Inset reserves space around a widget.
type Inset struct {
	Top, Bottom, Left, Right unit.Dp
}

// Layout lays out w inside the inset and adds the reserved space to its result.
func (in Inset) Layout(gtx Context, w Widget) Dimensions {
	top, right := gtx.Dp(in.Top), gtx.Dp(in.Right)
	bottom, left := gtx.Dp(in.Bottom), gtx.Dp(in.Left)

	childConstraints := gtx.Constraints
	childConstraints.Max.X, left, right = insetDimension(childConstraints.Max.X, left, right)
	childConstraints.Max.Y, top, bottom = insetDimension(childConstraints.Max.Y, top, bottom)
	childConstraints.Min.X = min(childConstraints.Min.X, childConstraints.Max.X)
	childConstraints.Min.Y = min(childConstraints.Min.Y, childConstraints.Max.Y)

	gtx.Constraints = childConstraints
	offset := op.Offset(image.Pt(left, top)).Push(gtx.Ops)
	child := w(gtx)
	offset.Pop()

	return Dimensions{
		Size:     child.Size.Add(image.Pt(left+right, top+bottom)),
		Baseline: child.Baseline + bottom,
	}
}

// insetDimension returns the room and edges that remain active on one axis.
// An inset that consumes the axis has no drawable edge, so both edges are
// discarded together instead of reporting padding outside the available room.
func insetDimension(available, leading, trailing int) (room, keptLeading, keptTrailing int) {
	room = available - leading - trailing
	if room <= 0 {
		return 0, 0, 0
	}
	return room, leading, trailing
}

// UniformInset returns an inset with v on every edge.
func UniformInset(v unit.Dp) Inset {
	return Inset{Top: v, Bottom: v, Left: v, Right: v}
}

// Layout lays out w at the named direction within the minimum available size.
func (d Direction) Layout(gtx Context, w Widget) Dimensions {
	required := gtx.Constraints.Min
	gtx.Constraints.Min = d.childMinimum(required)

	recording := op.Record(gtx.Ops)
	child := w(gtx)
	drawChild := recording.Stop()

	occupied := image.Pt(
		max(required.X, child.Size.X),
		max(required.Y, child.Size.Y),
	)
	position := d.Position(child.Size, occupied)
	offset := op.Offset(position).Push(gtx.Ops)
	drawChild.Add(gtx.Ops)
	offset.Pop()

	return Dimensions{
		Size:     occupied,
		Baseline: child.Baseline + occupied.Y - child.Size.Y - position.Y,
	}
}

func (d Direction) childMinimum(parent image.Point) image.Point {
	switch d {
	case N, S:
		parent.Y = 0
	case E, W:
		parent.X = 0
	default:
		parent = image.Point{}
	}
	return parent
}

// Position returns the top-left position of widget inside bounds.
func (d Direction) Position(widget, bounds image.Point) image.Point {
	space := bounds.Sub(widget)
	var position image.Point

	switch d {
	case N, S, Center:
		position.X = space.X / 2
	case NE, E, SE:
		position.X = space.X
	}
	switch d {
	case W, Center, E:
		position.Y = space.Y / 2
	case SW, S, SE:
		position.Y = space.Y
	}
	return position
}

// Spacer occupies a fixed amount of otherwise empty space.
type Spacer struct {
	Width, Height unit.Dp
}

// Layout resolves the spacer size within the context constraints.
func (s Spacer) Layout(gtx Context) Dimensions {
	requested := image.Pt(gtx.Dp(s.Width), gtx.Dp(s.Height))
	return Dimensions{Size: gtx.Constraints.Constrain(requested)}
}

// String returns the alignment name.
func (a Alignment) String() string {
	switch a {
	case Start:
		return "Start"
	case End:
		return "End"
	case Middle:
		return "Middle"
	case Baseline:
		return "Baseline"
	default:
		panic("invalid Alignment")
	}
}

// Convert maps a point between screen coordinates and main/cross coordinates.
func (a Axis) Convert(point image.Point) image.Point {
	if a == Vertical {
		return image.Pt(point.Y, point.X)
	}
	return point
}

// FConvert is the floating-point form of Convert.
func (a Axis) FConvert(point f32.Point) f32.Point {
	if a == Vertical {
		return f32.Pt(point.Y, point.X)
	}
	return point
}

func (a Axis) mainConstraint(c Constraints) (minimum, maximum int) {
	if a == Vertical {
		return c.Min.Y, c.Max.Y
	}
	return c.Min.X, c.Max.X
}

func (a Axis) crossConstraint(c Constraints) (minimum, maximum int) {
	if a == Vertical {
		return c.Min.X, c.Max.X
	}
	return c.Min.Y, c.Max.Y
}

func (a Axis) constraints(mainMin, mainMax, crossMin, crossMax int) Constraints {
	minimum, maximum := image.Pt(mainMin, crossMin), image.Pt(mainMax, crossMax)
	if a == Vertical {
		minimum, maximum = image.Pt(crossMin, mainMin), image.Pt(crossMax, mainMax)
	}
	return Constraints{Min: minimum, Max: maximum}
}

// String returns the axis name.
func (a Axis) String() string {
	switch a {
	case Horizontal:
		return "Horizontal"
	case Vertical:
		return "Vertical"
	default:
		panic("invalid Axis")
	}
}

// String returns the direction name.
func (d Direction) String() string {
	names := [...]string{"NW", "N", "NE", "E", "SE", "S", "SW", "W", "Center"}
	if int(d) >= len(names) {
		panic("invalid Direction")
	}
	return names[d]
}
