package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra"
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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Disabled draws it as unavailable and refuses to move.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p SliderProps) PartNames() []string { return []string{"root"} }

// Layout draws the track, the filled part and the handle.
func (p SliderProps) Layout(c ayra.Context, state *Slider) ayra.Dimensions {
	return withRoot(c, "slider", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p SliderProps) layout(c ayra.Context, state *Slider) ayra.Dimensions {
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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Of is how many marks there are. Zero is five.
	Of int
	// ReadOnly draws the value without accepting a press, for a rating
	// somebody else left.
	ReadOnly bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p RatingProps) PartNames() []string { return []string{"root"} }

// Layout draws the marks and returns the room they took.
func (p RatingProps) Layout(c ayra.Context, state *Rating) ayra.Dimensions {
	return withRoot(c, "rating", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p RatingProps) layout(c ayra.Context, state *Rating) ayra.Dimensions {
	of := p.Of
	if of == 0 {
		of = 5
	}

	side := innerPixels(c, innerSideStar)
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
		// Kyse's stylesheet: text-primary up to the chosen star, and the muted
		// foreground at 40% beyond it.
		ink := c.Theme.Colours.MutedForeground
		ink.A = uint8(int(ink.A) * 2 / 5)
		if index < state.value {
			ink = c.Theme.Colours.Primary
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

// star draws one mark: Kyse's Star, the same picture lit and unlit, which differ
// only in the ink.
func star(c ayra.Context, side int, ink color.NRGBA) {
	drawInner(c, innerIcon("rating", "Star"), side, ink, 0)
}
