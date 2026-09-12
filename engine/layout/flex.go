package layout

import (
	"image"

	"github.com/arandu-io/ayra/engine/op"
)

// Flex arranges children one after another on a horizontal or vertical axis.
//
// Rigid children choose their size first. Flexed children divide what remains,
// and Spacing places any room required by the minimum constraint around them.
type Flex struct {
	// Axis is the direction children follow.
	Axis Axis
	// Spacing chooses where unused room on the main axis goes.
	Spacing Spacing
	// Alignment places children within the cross axis.
	Alignment Alignment
	// WeightSum is the denominator used to divide room between Flexed children.
	// Zero uses the sum of their weights.
	WeightSum float32
	// Gap is the number of pixels inserted between adjacent children.
	Gap int
}

// FlexChild is a child and the rule Flex uses to measure it.
type FlexChild struct {
	flex   bool
	weight float32
	widget Widget
}

// Spacing describes where Flex puts room left on its main axis.
type Spacing uint8

const (
	// SpaceEnd puts all unused room after the last child.
	SpaceEnd Spacing = iota
	// SpaceStart puts all unused room before the first child.
	SpaceStart
	// SpaceSides splits unused room between the two ends.
	SpaceSides
	// SpaceAround puts equal room around every child, making the two outer
	// spaces half the size of an inner one.
	SpaceAround
	// SpaceBetween puts equal room between children and none at the ends.
	SpaceBetween
	// SpaceEvenly makes every inner and outer space equal.
	SpaceEvenly
)

// Rigid makes a child choose its main-axis size from the room still available.
func Rigid(widget Widget) FlexChild {
	return FlexChild{widget: widget}
}

// Flexed makes a child take weight parts of the room left after rigid children.
//
// The denominator is Flex.WeightSum when it is non-zero, and otherwise the sum
// of all flexed weights. A flexed child receives the chosen size as both its
// minimum and maximum main-axis constraint.
func Flexed(weight float32, widget Widget) FlexChild {
	return FlexChild{flex: true, weight: weight, widget: widget}
}

type flexMeasurement struct {
	call op.CallOp
	dims Dimensions
}

// flexShare carries one child's rounding error into the next. Keeping the
// correction local makes the distribution deterministic without letting
// rounding push the final child past the available axis.
type flexShare struct {
	room      int
	weightSum float32
	carry     float32
}

func (s *flexShare) take(weight float32, remaining int) int {
	if remaining == 0 || weight <= 0 || s.weightSum <= 0 {
		return 0
	}

	base := float32(s.room) * weight / s.weightSum
	exact := base + s.carry
	share := int(exact + .5)
	s.carry = base - float32(share)
	if share < 0 {
		return 0
	}
	if share > remaining {
		return remaining
	}
	return share
}

// Layout measures and places children while preserving their declared drawing
// order. Rigid children are measured before flexed children, regardless of that
// order, so every flexed child sees the same pool of remaining room.
func (f Flex) Layout(gtx Context, children ...FlexChild) Dimensions {
	constraints := gtx.Constraints
	mainMin, mainMax := f.Axis.mainConstraint(constraints)
	crossMin, crossMax := f.Axis.crossConstraint(constraints)

	gap := f.totalGap(len(children))
	remaining := subtractFloor(mainMax, gap)
	weightSum := f.childWeightSum(children)

	var local [32]flexMeasurement
	measurements := local[:]
	if len(children) > len(measurements) {
		measurements = make([]flexMeasurement, len(children))
	} else {
		measurements = measurements[:len(children)]
	}

	childContext := gtx
	mainSize := 0
	for i, child := range children {
		if child.flex {
			continue
		}
		childContext.Constraints = f.Axis.constraints(0, remaining, crossMin, crossMax)
		measurements[i] = measureFlexChild(gtx, childContext, child.widget)
		size := f.Axis.Convert(measurements[i].dims.Size).X
		mainSize += size
		remaining = subtractFloor(remaining, size)
	}

	shares := flexShare{room: remaining, weightSum: weightSum}
	for i, child := range children {
		if !child.flex {
			continue
		}
		main := shares.take(child.weight, remaining)
		childContext.Constraints = f.Axis.constraints(main, main, crossMin, crossMax)
		measurements[i] = measureFlexChild(gtx, childContext, child.widget)
		size := f.Axis.Convert(measurements[i].dims.Size).X
		mainSize += size
		remaining = subtractFloor(remaining, size)
	}

	mainSize += gap
	crossSize, baselineLine := f.crossSize(crossMin, measurements)
	unused := 0
	if mainSize < mainMin {
		unused = mainMin - mainSize
	}

	cursor := f.leadingSpace(unused, len(children))
	between := f.betweenSpace(unused, len(children))
	for i, measured := range measurements {
		cross := f.crossOffset(crossSize, baselineLine, measured.dims)
		position := f.Axis.Convert(image.Pt(cursor, cross))
		transform := op.Offset(position).Push(gtx.Ops)
		measured.call.Add(gtx.Ops)
		transform.Pop()

		cursor += f.Axis.Convert(measured.dims.Size).X
		if i+1 < len(measurements) {
			cursor += f.Gap + between
		}
	}
	cursor += f.trailingSpace(unused, len(children))

	size := constraints.Constrain(f.Axis.Convert(image.Pt(cursor, crossSize)))
	return Dimensions{Size: size, Baseline: size.Y - baselineLine}
}

func measureFlexChild(parent, child Context, widget Widget) flexMeasurement {
	recording := op.Record(parent.Ops)
	dims := widget(child)
	return flexMeasurement{call: recording.Stop(), dims: dims}
}

func (f Flex) childWeightSum(children []FlexChild) float32 {
	if f.WeightSum != 0 {
		return f.WeightSum
	}
	var sum float32
	for _, child := range children {
		if child.flex && child.weight > 0 {
			sum += child.weight
		}
	}
	return sum
}

func (f Flex) totalGap(children int) int {
	if children < 2 || f.Gap <= 0 {
		return 0
	}
	return f.Gap * (children - 1)
}

func subtractFloor(available, used int) int {
	if used >= available {
		return 0
	}
	return available - used
}

func (f Flex) crossSize(minimum int, children []flexMeasurement) (size, baselineLine int) {
	size = minimum
	for _, child := range children {
		cross := f.Axis.Convert(child.dims.Size).Y
		if cross > size {
			size = cross
		}
		line := child.dims.Size.Y - child.dims.Baseline
		if line > baselineLine {
			baselineLine = line
		}
	}
	return size, baselineLine
}

func (f Flex) crossOffset(crossSize, baselineLine int, dims Dimensions) int {
	childCross := f.Axis.Convert(dims.Size).Y
	switch f.Alignment {
	case End:
		return crossSize - childCross
	case Middle:
		return (crossSize - childCross) / 2
	case Baseline:
		if f.Axis == Horizontal {
			return baselineLine - (dims.Size.Y - dims.Baseline)
		}
	}
	return 0
}

func (f Flex) leadingSpace(unused, children int) int {
	if children == 0 {
		return 0
	}
	switch f.Spacing {
	case SpaceStart:
		return unused
	case SpaceSides:
		return unused / 2
	case SpaceAround:
		return unused / (children * 2)
	case SpaceEvenly:
		return unused / (children + 1)
	default:
		return 0
	}
}

func (f Flex) betweenSpace(unused, children int) int {
	switch f.Spacing {
	case SpaceAround:
		if children > 0 {
			return unused / children
		}
	case SpaceBetween:
		if children > 1 {
			return unused / (children - 1)
		}
	case SpaceEvenly:
		if children > 0 {
			return unused / (children + 1)
		}
	}
	return 0
}

func (f Flex) trailingSpace(unused, children int) int {
	if children == 0 {
		return 0
	}
	switch f.Spacing {
	case SpaceEnd:
		return unused

	case SpaceSides:
		return unused / 2
	case SpaceAround:
		return unused / (children * 2)
	case SpaceEvenly:
		return unused / (children + 1)
	default:
		return 0
	}
}

// String returns the spacing name used in source.
func (s Spacing) String() string {
	switch s {
	case SpaceEnd:
		return "SpaceEnd"
	case SpaceStart:
		return "SpaceStart"
	case SpaceSides:
		return "SpaceSides"
	case SpaceAround:
		return "SpaceAround"
	case SpaceBetween:
		return "SpaceBetween"
	case SpaceEvenly:
		return "SpaceEvenly"
	default:
		panic("invalid Spacing")
	}
}
