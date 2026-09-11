package widget

import (
	"image"
	"image/color"
	"math"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/unit"
	engine "github.com/arandu-io/ayra/engine/widget"
)

// Slider is the state half of a control that picks a value along a line.
type Slider struct {
	value engine.Float
}

// Value is where it sits, between 0 and 1.
func (s *Slider) Value() float32 { return s.value.Value }

// SetValue moves it, for the state a screen arrives with.
func (s *Slider) SetValue(value float32) { s.value.Value = min(max(value, 0), 1) }

// SliderProps is a value picked along a line.
//
// The range is the caller's: this answers a fraction, and turning that into
// pounds, pixels or minutes is a decision with a unit attached, which is not
// something a control should hold.
type SliderProps struct {
	// Disabled draws it as unavailable and refuses to move.
	Disabled bool
}

// Layout draws the track, the filled part and the handle.
func (p SliderProps) Layout(c ayra.Context, state *Slider) ayra.Dimensions {
	if p.Disabled {
		c.Context = c.Context.Disabled()
	}

	height := c.Dp(unit.Dp(20))
	width := c.Constraints.Max.X
	track := c.Dp(unit.Dp(6))
	knob := c.Dp(unit.Dp(16))

	// The float owns the dragging: it reads the pointer over the area it is
	// given and writes the value. The margin is half a knob, so a press just
	// past the end of the track still takes hold of the handle rather than
	// jumping it.
	c.Constraints.Min = image.Pt(width, height)
	state.value.Layout(c.Context, layout.Horizontal, unit.Dp(8))

	fill, ink := c.Theme.Colours.Muted, c.Theme.Colours.Primary
	if p.Disabled {
		fill, ink = fade(fill), fade(ink)
	}

	middle := height / 2
	rail := image.Rect(0, middle-track/2, width, middle+track/2)
	paint.FillShape(c.Ops, fill, clip.UniformRRect(rail, track/2).Op(c.Ops))

	travel := width - knob
	at := knob/2 + int(float32(travel)*state.value.Value)

	filled := image.Rect(0, middle-track/2, at, middle+track/2)
	paint.FillShape(c.Ops, ink, clip.UniformRRect(filled, track/2).Op(c.Ops))

	handle := image.Rect(at-knob/2, middle-knob/2, at+knob/2, middle+knob/2)
	paint.FillShape(c.Ops, ink, clip.UniformRRect(handle, knob/2).Op(c.Ops))

	return ayra.Dimensions{Size: image.Pt(width, height)}
}

// Rating is the state half of a row of marks somebody chooses from.
type Rating struct {
	click   gesture.Click
	value   int
	hovered int
}

// Value is how many marks are set.
func (r *Rating) Value() int { return r.value }

// SetValue writes it, for the state a screen arrives with.
func (r *Rating) SetValue(value int) { r.value = value }

// RatingProps is a row of marks.
type RatingProps struct {
	// Of is how many marks there are. Zero is five.
	Of int
	// ReadOnly draws the value without accepting a press, for a rating
	// somebody else left.
	ReadOnly bool
}

// Layout draws the marks and returns the room they took.
func (p RatingProps) Layout(c ayra.Context, state *Rating) ayra.Dimensions {
	of := p.Of
	if of == 0 {
		of = 5
	}

	side := c.Dp(unit.Dp(20))
	width := side * of
	area := image.Pt(width, side)

	if !p.ReadOnly {
		defer clip.Rect(image.Rectangle{Max: area}).Push(c.Ops).Pop()
		state.click.Add(c.Ops)
		event.Op(c.Ops, state)

		for {
			press, ok := state.click.Update(c.Source)
			if !ok {
				break
			}
			if press.Kind == gesture.KindClick {
				// Which mark was pressed, from where the pointer landed. A
				// rating drawn as marks and read as a slider would set four
				// and a half stars, which is not a value this can hold.
				chosen := int(press.Position.X)/side + 1
				state.value = min(max(chosen, 0), of)
			}
		}
	}

	for index := 0; index < of; index++ {
		ink := c.Theme.Colours.Border
		if index < state.value {
			ink = c.Theme.Colours.Foreground
		}
		// Offset, not a clip: a clip restricts where the drawing may land and
		// moves nothing, so five stars drawn this way are five stars at the
		// same place, four of them clipped away. The picture showed one.
		offset := op.Offset(image.Pt(index*side, 0)).Push(c.Ops)
		star(c, side, ink)
		offset.Pop()
	}

	return ayra.Dimensions{Size: area}
}

// star draws one mark, filled.
//
// A five-pointed outline rather than a glyph, because a glyph comes from
// whichever font the screen happens to carry, and on a platform missing it the
// rating renders as five empty boxes.
func star(c ayra.Context, side int, ink color.NRGBA) {
	centre := float32(side) / 2
	outer := float32(side) * 0.42
	inner := outer * 0.42

	var shape clip.Path
	shape.Begin(c.Ops)

	for point := 0; point < 10; point++ {
		radius := outer
		if point%2 == 1 {
			radius = inner
		}
		// Ten points alternating between the two radii, starting at the top:
		// that is what a five-pointed star is, and the half-turn offset is what
		// puts a point upwards rather than an edge.
		angle := float64(point)*math.Pi/5 - math.Pi/2
		at := f32.Pt(
			centre+radius*float32(math.Cos(angle)),
			centre+radius*float32(math.Sin(angle)),
		)
		if point == 0 {
			shape.MoveTo(at)
			continue
		}
		shape.LineTo(at)
	}
	shape.Close()

	paint.FillShape(c.Ops, ink, clip.Outline{Path: shape.End()}.Op())
}
