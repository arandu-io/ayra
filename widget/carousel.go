package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// The words on the two steps. They are the words the pager already uses, so
// that the two controls a screen puts under a row of things say the same thing
// -- and so that a translation has one string to find rather than two.
const (
	previousStep = "Previous"
	nextStep     = "Next"
)

// Carousel is the state half: which slide is at the front, the presses that
// move it, and the drag in progress.
//
// It also remembers the shape of the last frame -- how many positions there
// are and whether stepping wraps. That is what lets [Carousel.Next] be called
// from outside a frame, which is where most of its calls come from: a timer
// advancing the window, a key the screen handled itself. Without it the count
// would have to be passed in at every call site, and the same rule written
// twice comes apart the first time one copy learns about wrapping.
type Carousel struct {
	first    int
	previous Button
	next     Button
	dots     []Button

	// Where the finger went down and how far it has come since, in pixels.
	// The step is decided from the distance rather than from the position,
	// because the position is meaningless once the window has scrolled.
	drag    gesture.Drag
	grabbed int
	travel  int

	moved bool

	// The shape of the last frame. Nothing reads these before a frame has
	// been drawn, and until then a step finds nowhere to go, which is the
	// truth: nobody has said how many slides there are.
	positions int
	loop      bool
}

// At is the index of the slide at the front of the window.
func (c *Carousel) At() int { return c.first }

// Show moves the window, for the state a screen arrives with.
//
// Out of range is clamped rather than refused, so a position restored from
// somewhere else -- a saved session, a link, a list that has since got shorter
// -- opens on the nearest slide there is instead of on nothing.
//
// It does not report a move. A screen writing the position it was opened with
// and then being told the position changed would be answering its own write,
// and what it does with that answer is another write.
func (c *Carousel) Show(index int) { c.first = clampFirst(index, c.positions) }

// Next moves the window on by one slide.
func (c *Carousel) Next() { c.move(1) }

// Previous moves the window back by one slide.
func (c *Carousel) Previous() { c.move(-1) }

// Changed reports that the window moved, once, and consumes that.
//
// Consumed rather than left standing, because what a caller does with it --
// fetch the full-size image, tell the server which slide was reached -- would
// otherwise be done on every frame for as long as nobody moved again. That is
// thirty requests a second for a carousel nobody is touching.
func (c *Carousel) Changed() bool {
	moved := c.moved
	c.moved = false
	return moved
}

// move steps the window and remembers that it moved.
//
// A step that lands where it started is not a move: at the end of a carousel
// that does not wrap, pressing next changes nothing, and reporting it would
// have the caller load again the slide it is already showing.
func (c *Carousel) move(delta int) {
	at := stepTo(c.first, delta, c.positions, c.loop)
	if at == c.first {
		return
	}
	c.first, c.moved = at, true
}

// jump moves the window to a position, for a press on one of the dots.
func (c *Carousel) jump(index int) {
	at := clampFirst(index, c.positions)
	if at == c.first {
		return
	}
	c.first, c.moved = at, true
}

// CarouselProps is a row of slides shown a window at a time, with a step
// either side and a mark per position underneath.
//
// The slides themselves belong to the caller, the way the panel under a row of
// tabs does: a props that owned them would have every slide built on every
// frame, including the ninety-odd nobody is looking at.
type CarouselProps struct {
	// Count is how many slides there are.
	Count int
	// PerView is how many are on screen at once. Zero is one.
	PerView int
	// Loop wraps: stepping past the last position reaches the first, and
	// stepping back from the first reaches the last.
	Loop bool
	// Dots draws the row of position marks under the window.
	Dots bool
	// Arrows draws the two steps either side of the window.
	Arrows bool
	// Gap is the room between one slide and the next, and between the window
	// and the steps. Zero takes eight points: slides with nothing between them
	// read as one wide picture rather than as three.
	Gap unit.Dp
	// Disabled draws the controls as unavailable and stops them answering.
	Disabled bool
}

// Layout draws the window, the steps and the dots, and returns the room they
// took.
//
// The slide function is called once for each slide on screen and no more.
// Laying out all of them and clipping is what turns a carousel over a hundred
// images into a frame that misses: the ninety-nine that are not on screen get
// measured, shaped and recorded, every frame, to be thrown away.
func (p CarouselProps) Layout(c ayra.Context, state *Carousel, slide func(c ayra.Context, index int) ayra.Dimensions) ayra.Dimensions {
	pages := positions(p.Count, p.view())

	state.positions, state.loop = pages, p.Loop
	state.first = clampFirst(state.first, pages)

	if pages == 0 || slide == nil {
		// A carousel over nothing is nothing. Steps around an empty window say
		// there is something to reach and a mark underneath says how much
		// there is: both would be controls that do nothing, and both take
		// room from whatever is beside them.
		return ayra.Dimensions{}
	}

	// The dots are rebuilt rather than grown when the number of positions
	// changes, because a dot is identified by where it is in the row. Kept
	// across a change, the button that was position seven of forty answers for
	// position seven of three, and the press lands on a slide nobody pointed
	// at.
	if len(state.dots) != pages {
		state.dots = make([]Button, pages)
	}

	// Asked before anything is drawn, so that what a press changes is on the
	// screen in the same frame. Asked afterwards it costs a frame, and a frame
	// of lag on a step reads as a carousel that ignored the press.
	//
	// Clicked comes first in each condition because it is what drains the
	// event: put behind the guard, a press made while the control was
	// unavailable would still be waiting when it was not.
	if state.previous.Clicked(c) && !p.Disabled {
		state.move(-1)
	}
	if state.next.Clicked(c) && !p.Disabled {
		state.move(1)
	}
	for index := range state.dots {
		if state.dots[index].Clicked(c) && !p.Disabled && p.Dots {
			state.jump(index)
		}
	}

	back := canStep(state.first, -1, pages, p.Loop)
	forward := canStep(state.first, 1, pages, p.Loop)
	gap := c.Dp(p.gap())

	// The steps are measured before the row is built, because whether they are
	// drawn at all depends on what they leave behind. Two buttons either side
	// of a sliver is a control that has taken the room of the thing it exists
	// to show, and in a narrow column that is exactly what a word on each of
	// them costs.
	room := 0
	if p.Arrows {
		measure := op.Record(c.Ops)
		inner := c
		inner.Constraints.Min = image.Point{}
		room = p.arrow(previousStep, back).Layout(inner, &state.previous).Size.X
		room += p.arrow(nextStep, forward).Layout(inner, &state.next).Size.X
		measure.Stop()
	}
	steps := p.Arrows && arrowsFit(c.Constraints.Max.X, room+2*gap)

	c.Constraints.Min = image.Point{}

	row := make([]layout.FlexChild, 0, 3)
	if steps {
		row = append(row, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return p.arrow(previousStep, back).Layout(inner, &state.previous)
		}))
	}
	row = append(row, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return p.window(c.With(gtx), state, slide)
	}))
	if steps {
		row = append(row, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return p.arrow(nextStep, forward).Layout(inner, &state.next)
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gap}.Layout(gtx, row...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.marks(c.With(gtx), state, pages)
		}),
	)
}

// window draws the slides that are on screen and returns the room they took.
func (p CarouselProps) window(c ayra.Context, state *Carousel, slide func(c ayra.Context, index int) ayra.Dimensions) ayra.Dimensions {
	view := p.view()
	gap := c.Dp(p.gap())
	width := c.Constraints.Max.X

	shared := width - gap*(view-1)
	if shared < view {
		// Less than a pixel per slide. There is no stride to be made out of
		// that, and every offset computed from it would land on top of the
		// last one.
		return ayra.Dimensions{}
	}
	slideWidth := shared / view
	stride := slideWidth + gap

	p.readDrag(c, state, stride)

	// The travel is held to one slide either way, which is what keeps the
	// promise that only the slides on screen are laid out: past a whole slide
	// the window would need a second one coming in behind the first.
	travel := min(max(state.travel, -stride), stride)
	from, howMany := shown(state.first, view, p.Count, travel)

	// The slides are measured before any of them is drawn, because the window
	// is as tall as the tallest one on it and a child cannot ask its siblings
	// how tall they came out. Taken from the constraints instead, the height
	// is nought in a row -- the window disappears and the screen reads as a
	// layout fault -- and the whole page in a column, which draws the carousel
	// down the length of the screen.
	calls := make([]op.CallOp, howMany)
	height := 0

	inner := c
	inner.Constraints = layout.Constraints{
		Min: image.Pt(slideWidth, 0),
		Max: image.Pt(slideWidth, c.Constraints.Max.Y),
	}
	for n := range calls {
		record := op.Record(c.Ops)
		height = max(height, slide(inner, from+n).Size.Y)
		calls[n] = record.Stop()
	}
	if height <= 0 {
		return ayra.Dimensions{}
	}

	area := image.Pt(width, height)
	defer clip.Rect(image.Rectangle{Max: area}).Push(c.Ops).Pop()

	// The drag area goes down before the slides, so that a control inside a
	// slide is the one that answers a press on it: the handler added last is
	// the one on top. A carousel that swallowed every press would be a row of
	// pictures with dead buttons on them.
	if !p.Disabled && state.positions > 1 {
		state.drag.Add(c.Ops)
	}

	for n, call := range calls {
		// Offset rather than a clip per slide: a clip says where drawing may
		// land and moves nothing, so the slides would all be drawn at the left
		// edge with every one but the first clipped away.
		offset := op.Offset(image.Pt((from+n-state.first)*stride+travel, 0)).Push(c.Ops)
		call.Add(c.Ops)
		offset.Pop()
	}

	return ayra.Dimensions{Size: area}
}

// readDrag turns the pointer into a distance, and commits the move when the
// finger comes off.
//
// The step is decided on release rather than while the finger is down, so a
// drag taken back before it ends leaves the window where it was -- which is
// the whole reason somebody drags rather than presses.
func (p CarouselProps) readDrag(c ayra.Context, state *Carousel, stride int) {
	if p.Disabled || state.positions <= 1 {
		state.travel = 0
		return
	}

	for {
		event, ok := state.drag.Update(c.Metric, c.Source, gesture.Horizontal)
		if !ok {
			break
		}
		switch event.Kind {
		case pointer.Press:
			state.grabbed, state.travel = int(event.Position.X), 0
		case pointer.Drag:
			state.travel = int(event.Position.X) - state.grabbed
		case pointer.Release, pointer.Cancel:
			state.move(dragged(state.travel, stride))
			state.travel = 0
		}
	}
}

// marks draws the row of position dots under the window.
func (p CarouselProps) marks(c ayra.Context, state *Carousel, pages int) ayra.Dimensions {
	if !p.Dots || pages <= 1 {
		// One position is not a choice. A single dot under a picture is read
		// as a mark on the picture, not as a place to press.
		return ayra.Dimensions{}
	}

	side := c.Dp(unit.Dp(8))
	children := make([]layout.FlexChild, 0, pages)

	for _, page := range p.dotPages(state.first, pages) {
		page := page
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.mark(c.With(gtx), state, page, side)
		}))
	}

	return layout.Inset{Top: 8}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		// The row is told to fill the width before it is told to share what is
		// left at both ends, because that is the only thing it can centre
		// against: given room to shrink into, it shrinks and sits on the left.
		inner.Constraints.Min = image.Pt(inner.Constraints.Max.X, 0)
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceSides, Gap: inner.Dp(unit.Dp(4))}.
			Layout(inner.Context, children...)
	})
}

// mark draws one dot, or the smaller one that stands for a run of them.
func (p CarouselProps) mark(c ayra.Context, state *Carousel, page, side int) ayra.Dimensions {
	if page == 0 {
		// The run the pager collapsed. Quieter and smaller than a position,
		// and not pressable: there is no single slide it means.
		small := max(side/2, 1)
		dot := image.Rectangle{Max: image.Pt(small, small)}
		paint.FillShape(c.Ops, c.Theme.Colours.MutedForeground, clip.UniformRRect(dot, small/2).Op(c.Ops))
		return ayra.Dimensions{Size: dot.Max}
	}

	ink := p.dotInk(c.Theme, page, state.first)
	if p.Disabled {
		ink = fade(ink)
		c.Context = c.Context.Disabled()
	}

	return state.dots[page-1].click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		// The press target is larger than the mark it holds. Eight points of
		// dot is smaller than a fingertip, and a row of them at that size is a
		// row of misses -- each one landing on the dot beside the one meant.
		target := c.Dp(unit.Dp(20))
		at := (target - side) / 2
		dot := image.Rect(at, at, at+side, at+side)
		paint.FillShape(gtx.Ops, ink, clip.UniformRRect(dot, side/2).Op(gtx.Ops))
		return layout.Dimensions{Size: image.Pt(target, target)}
	})
}

// dotPages answers which positions get a mark, with nought standing for a run
// of them.
//
// The choice is the pager's question and it is asked of the pager: a carousel
// over forty pictures has forty positions, and forty marks in a row is not an
// indicator, it is a texture. Both ends are kept and the run between them
// collapses to a single gap, which is what the pager already decides -- and a
// second answer to that question would be a second one to keep right.
//
// The position counts from nought and the pager counts from one, which is why
// the argument is moved on by one here rather than at the call site: done
// there, the row of marks sits one position behind the window.
func (p CarouselProps) dotPages(first, pages int) []int {
	return PaginationProps{Total: pages}.visible(first+1, 2)
}

// dotInk answers the colour a mark is drawn in.
//
// The page numbers count from one and the positions count from nought, and
// this is where the two meet. Comparing them without moving one of them lights
// the mark before the window on every position but the first, and on the last
// position lights nothing at all.
func (p CarouselProps) dotInk(t theme.Theme, page, first int) color.NRGBA {
	if page-1 == first {
		return t.Colours.Foreground
	}
	return t.Colours.Border
}

// arrow answers the props for one of the two steps.
//
// Unavailable rather than absent when there is nowhere to go: a step that
// disappears at the end of a carousel moves everything beside it, so a list
// growing from one slide to two would shift the whole row as it arrived. A
// disabled step says there is no more, in the place the step was.
func (p CarouselProps) arrow(label string, available bool) ButtonProps {
	return ButtonProps{Label: label, Variant: Outline, Size: Small, Disabled: p.Disabled || !available}
}

// view is how many slides are on screen at once.
func (p CarouselProps) view() int {
	if p.PerView < 1 {
		return 1
	}
	return p.PerView
}

// gap is the room between one slide and the next.
func (p CarouselProps) gap() unit.Dp {
	if p.Gap <= 0 {
		return 8
	}
	return p.Gap
}

// positions is how many different slides the window can start at.
//
// It is Count-PerView+1 rather than Count, because the window stops when its
// right edge reaches the last slide. Counting to Count instead puts the final
// positions past the end, where the row is a slide or two and then blank
// space -- which reads as a carousel that has lost its pictures rather than as
// one that has reached its last.
func positions(count, view int) int {
	if count <= 0 || view <= 0 {
		return 0
	}
	if view >= count {
		// Everything is on screen already, so there is one place to be and
		// nowhere to step to.
		return 1
	}
	return count - view + 1
}

// clampFirst keeps a position inside the ones there are.
//
// No positions at all leaves the index where it is rather than forcing it to
// the start. That is what a carousel holds before its first frame, when nobody
// has yet said how many slides there are, and forcing it then would throw away
// the position the screen was opened on.
func clampFirst(index, positions int) int {
	if index < 0 {
		return 0
	}
	if positions <= 0 {
		return index
	}
	return min(index, positions-1)
}

// stepTo answers the position delta steps away from this one.
//
// Clamped when it does not loop and wrapped when it does. The wrap takes the
// remainder twice because Go's is signed: one step back from the first slide
// is -1 mod n, which is -1, and a carousel wrapping only forwards loses its
// first slide the moment somebody steps back from it.
func stepTo(first, delta, positions int, loop bool) int {
	if positions <= 0 {
		return 0
	}
	at := first + delta
	if loop {
		return ((at % positions) + positions) % positions
	}
	return min(max(at, 0), positions-1)
}

// canStep reports whether there is anywhere to go in a direction.
//
// It answers by taking the step rather than by comparing the position against
// the ends, so that the rule a control is drawn from and the rule a press
// follows cannot come apart. Written twice, they do, and what that looks like
// is a step drawn as available that refuses to move.
func canStep(first, delta, positions int, loop bool) bool {
	if positions <= 1 {
		return false
	}
	return stepTo(first, delta, positions, loop) != first
}

// shown answers which slides to lay out: the first of them, and how many.
//
// It is the window, plus -- while a drag is in progress -- the one slide
// coming in from the side being dragged towards. That slide is on screen:
// part of it is under the finger. Left out, the drag draws as a picture
// sliding away from an empty space.
func shown(first, view, count, travel int) (from, howMany int) {
	if count <= 0 || view <= 0 {
		return 0, 0
	}

	from = max(first, 0)
	howMany = view
	if travel > 0 && from > 0 {
		from--
		howMany++
	}
	if travel < 0 {
		howMany++
	}
	if from+howMany > count {
		howMany = count - from
	}
	return from, max(howMany, 0)
}

// dragged answers how many positions a drag of this many pixels moves the
// window, given the room one slide and its gap take.
//
// Half a slide rather than a fixed number of points: the distance that means
// "the next one" has to grow with the slide, or the same flick advances a
// window of thumbnails and leaves a window of photographs exactly where it
// was. The sign turns over, because dragging to the left brings the next slide
// in from the right.
func dragged(travel, stride int) int {
	if stride <= 0 {
		return 0
	}
	if travel < 0 {
		return (-travel + stride/2) / stride
	}
	return -((travel + stride/2) / stride)
}

// arrowsFit reports whether the two steps leave the window worth drawing.
//
// Half the room rather than a number of points: the window is what the
// carousel is for, and a slide narrower than the controls beside it is a
// picture squeezed out by its own navigation. A fixed minimum would be right
// at one density and wrong at the next.
//
// Nothing to measure against -- no width given at all -- keeps them, because
// the alternative is a carousel that silently drops its steps in every
// measuring pass and every test.
func arrowsFit(available, arrows int) bool {
	if available <= 0 {
		return true
	}
	return arrows*2 <= available
}
