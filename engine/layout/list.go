package layout

import (
	"image"
	"math"

	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
)

type listChild struct {
	size image.Point
	draw op.CallOp
}

type scanDirection uint8

const (
	scanStopped scanDirection = iota
	scanForward
	scanBackward
)

const unboundedListSize = 1_000_000

// List lays out the visible part of a sequence and handles scrolling through it.
type List struct {
	// Axis is the direction in which elements follow one another.
	Axis Axis
	// ScrollToEnd keeps a list at its trailing edge until it is scrolled away.
	ScrollToEnd bool
	// Alignment places elements across the list axis.
	Alignment Alignment
	// ScrollAnyAxis accepts scroll input from either screen axis.
	ScrollAnyAxis bool
	// Gap is the number of pixels between adjacent elements.
	Gap int

	// Position is updated by Layout and may be saved or changed between frames.
	Position Position

	bounds       Constraints
	scroll       gesture.Scroll
	scrollPixels int
	itemCount    int
	mainLength   int
	children     []listChild
	scan         scanDirection
}

// ListElement lays out the element at index.
type ListElement func(gtx Context, index int) Dimensions

// Position describes a list location relative to its first visible element.
type Position struct {
	// BeforeEnd reports whether more content remains beyond the trailing edge.
	// Its inverse makes the zero value useful for a List with ScrollToEnd set.
	BeforeEnd bool
	// First is the index of the first visible element.
	First int
	// Offset is the number of pixels hidden before First's leading edge.
	Offset int
	// OffsetLast is the signed distance from the viewport's trailing edge to
	// the trailing edge of the last visible element.
	OffsetLast int
	// Count is the number of elements intersecting the viewport.
	Count int
	// Length estimates the main-axis length of the complete sequence.
	Length int
}

// Layout lays out elements from w until the viewport and its focus margins are
// covered. It does not visit elements outside that measured window.
func (l *List) Layout(gtx Context, count int, w ListElement) Dimensions {
	l.beginFrame(gtx, count)
	crossMin, crossMax := l.Axis.crossConstraint(gtx.Constraints)
	gtx.Constraints = l.Axis.constraints(0, unboundedListSize, crossMin, crossMax)

	frame := op.Record(gtx.Ops)
	measuredTotal := 0
	for direction := l.nextDirection(); direction != scanStopped; direction = l.nextDirection() {
		index := l.nextIndex(direction)
		childRecording := op.Record(gtx.Ops)
		dimensions := w(gtx, index)
		draw := childRecording.Stop()
		l.remember(direction, dimensions, draw)
		measuredTotal += l.Axis.Convert(dimensions.Size).X
	}

	if measured := len(l.children); measured > 0 {
		l.Position.Length = measuredTotal*l.itemCount/measured + l.Gap*(l.itemCount-1)
	} else {
		l.Position.Length = 0
	}
	return l.compose(gtx.Ops, frame)
}

func (l *List) beginFrame(gtx Context, count int) {
	if l.scan != scanStopped {
		panic("layout: previous list element was not completed")
	}
	if count < 0 {
		count = 0
	}
	l.bounds = gtx.Constraints
	l.children = l.children[:0]
	l.mainLength = 0
	l.itemCount = count
	l.readScroll(gtx)

	if l.Position.First < 0 {
		l.Position.First = 0
		l.Position.Offset = 0
	}
	if l.pinnedToEnd() || l.Position.First > count {
		l.Position.First = count
		l.Position.Offset = 0
	}
}

func (l *List) readScroll(gtx Context) {
	minimum, maximum := -unboundedListSize, unboundedListSize
	if l.Position.First == 0 {
		minimum = min(0, -l.Position.Offset)
	}
	if l.Position.First+l.Position.Count == l.itemCount {
		maximum = max(0, -l.Position.OffsetLast)
	}

	horizontal := pointer.ScrollRange{Min: minimum, Max: maximum}
	vertical := pointer.ScrollRange{}
	axis := gesture.Axis(l.Axis)
	if l.ScrollAnyAxis {
		axis = gesture.Both
		vertical = horizontal
	} else if l.Axis == Vertical {
		horizontal, vertical = vertical, horizontal
	}
	l.scrollPixels = l.scroll.Update(gtx.Metric, gtx.Source, gtx.Now, axis, horizontal, vertical)
	l.Position.Offset += l.scrollPixels
}

func (l *List) pinnedToEnd() bool {
	return l.ScrollToEnd && !l.Position.BeforeEnd
}

func (l *List) nextDirection() scanDirection {
	direction := l.chooseDirection()
	if l.pinnedToEnd() && direction == scanStopped && l.scrollPixels < 0 {
		l.Position.BeforeEnd = true
		l.Position.Offset += l.scrollPixels
		direction = l.chooseDirection()
	}
	l.scan = direction
	return direction
}

func (l *List) chooseDirection() scanDirection {
	_, viewport := l.Axis.mainConstraint(l.bounds)
	end := l.Position.First + len(l.children)

	if end == l.itemCount && l.mainLength-l.Position.Offset < viewport {
		l.Position.Offset = l.mainLength - viewport
	}
	if l.Position.First == 0 && l.Position.Offset < 0 {
		l.Position.Offset = 0
	}

	if len(l.children) == l.itemCount {
		return scanStopped
	}
	leadingMargin, trailingMargin := l.focusMargins(end)
	if end < l.itemCount && l.mainLength-l.Position.Offset-trailingMargin < viewport {
		return scanForward
	}
	if l.Position.First > 0 && l.Position.Offset-leadingMargin < 0 {
		return scanBackward
	}
	return scanStopped
}

func (l *List) focusMargins(end int) (leading, trailing int) {
	if len(l.children) == 0 {
		return 0, 0
	}
	if l.Position.First > 0 {
		leading = l.Axis.Convert(l.children[0].size).X + l.Gap
	}
	if end < l.itemCount {
		trailing = l.Axis.Convert(l.children[len(l.children)-1].size).X + l.Gap
	}
	return leading, trailing
}

func (l *List) nextIndex(direction scanDirection) int {
	switch direction {
	case scanForward:
		return l.Position.First + len(l.children)
	case scanBackward:
		return l.Position.First - 1
	default:
		panic("layout: list index requested without a scan direction")
	}
}

func (l *List) remember(direction scanDirection, dimensions Dimensions, draw op.CallOp) {
	child := listChild{size: dimensions.Size, draw: draw}
	main := l.Axis.Convert(dimensions.Size).X
	if len(l.children) > 0 {
		l.mainLength += l.Gap
	}
	l.mainLength += main

	switch direction {
	case scanForward:
		l.children = append(l.children, child)
	case scanBackward:
		l.children = append(l.children, listChild{})
		copy(l.children[1:], l.children[:len(l.children)-1])
		l.children[0] = child
		l.Position.First--
		l.Position.Offset += main + l.Gap
	default:
		panic("layout: list element completed without a scan direction")
	}
	l.scan = scanStopped
}

func (l *List) compose(ops *op.Ops, frame op.MacroOp) Dimensions {
	if l.scan != scanStopped {
		panic("layout: list element was not completed")
	}
	mainMin, mainMax := l.Axis.mainConstraint(l.bounds)
	visible, leading := l.trimLeading(l.children)
	visible, trailing, mainSize, crossSize := l.visibleWindow(visible, mainMax)

	l.Position.Count = len(visible)
	l.Position.OffsetLast = mainMax - mainSize
	if l.ScrollToEnd && l.Position.OffsetLast > 0 {
		l.Position.Offset -= l.Position.OffsetLast
	}

	cursor := -l.Position.Offset
	draw := func(child listChild) {
		size := l.Axis.Convert(child.size)
		cross := 0
		switch l.Alignment {
		case End:
			cross = crossSize - size.Y
		case Middle:
			cross = (crossSize - size.Y) / 2
		}
		position := l.Axis.Convert(image.Pt(cursor, cross))
		offset := op.Offset(position).Push(ops)
		child.draw.Add(ops)
		offset.Pop()
		cursor += size.X
	}

	if leading != nil {
		cursor -= l.Axis.Convert(leading.size).X + l.Gap
		draw(*leading)
		cursor += l.Gap
	}
	for index, child := range visible {
		if index > 0 {
			cursor += l.Gap
		}
		draw(child)
	}
	if trailing != nil {
		cursor += l.Gap
		draw(*trailing)
	}

	atStart := l.Position.First == 0 && l.Position.Offset <= 0
	atEnd := l.Position.First+len(visible) == l.itemCount && mainMax >= cursor
	if (atStart && l.scrollPixels < 0) || (atEnd && l.scrollPixels > 0) {
		l.scroll.Stop()
	}
	l.Position.BeforeEnd = !atEnd

	mainSize = constrainDimension(cursor, mainMin, mainMax)
	crossMin, crossMax := l.Axis.crossConstraint(l.bounds)
	crossSize = constrainDimension(crossSize, crossMin, crossMax)
	dimensions := l.Axis.Convert(image.Pt(mainSize, crossSize))

	drawFrame := frame.Stop()
	defer clip.Rect(image.Rectangle{Max: dimensions}).Push(ops).Pop()
	l.scroll.Add(ops)
	drawFrame.Add(ops)
	return Dimensions{Size: dimensions}
}

func (l *List) trimLeading(children []listChild) ([]listChild, *listChild) {
	var leading *listChild
	for len(children) > 0 {
		child := children[0]
		main := l.Axis.Convert(child.size).X
		if l.Position.Offset < main {
			break
		}
		l.Position.First++
		l.Position.Offset -= main + l.Gap
		leadingCopy := child
		leading = &leadingCopy
		children = children[1:]
	}
	return children, leading
}

func (l *List) visibleWindow(children []listChild, mainMax int) (visible []listChild, trailing *listChild, mainSize, crossSize int) {
	mainSize = -l.Position.Offset
	for index, child := range children {
		size := l.Axis.Convert(child.size)
		crossSize = max(crossSize, size.Y)
		if index > 0 {
			mainSize += l.Gap
		}
		mainSize += size.X
		if mainSize >= mainMax {
			if index+1 < len(children) {
				trailingCopy := children[index+1]
				trailing = &trailingCopy
			}
			return children[:index+1], trailing, mainSize, crossSize
		}
	}
	return children, nil, mainSize, crossSize
}

// Dragging reports whether pointer input is actively dragging the list.
func (l *List) Dragging() bool {
	return l.scroll.State() == gesture.StateDragging
}

// ScrollBy moves the position by num elements. A fractional part is converted
// using the previous frame's average element length.
func (l *List) ScrollBy(num float32) {
	whole, fraction := math.Modf(float64(num))
	l.Position.First += int(whole)
	if l.itemCount > 0 {
		average := float64(l.Position.Length) / float64(l.itemCount)
		l.Position.Offset += int(math.Round(average * fraction))
	}
	l.Position.BeforeEnd = true
}

// ScrollTo places element n at the leading edge on the next layout.
func (l *List) ScrollTo(n int) {
	l.Position.First = n
	l.Position.Offset = 0
	l.Position.BeforeEnd = true
}
