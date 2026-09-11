package widget

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/unit"
)

// ColourPicker is the state half of a control that picks a colour: a field for
// saturation and value, a bar for hue, a bar for alpha and a field holding the
// digits.
//
// The colour is kept as hue, saturation and value rather than as three bytes,
// because that is what the dragged areas move. Kept as bytes it would have to
// be converted back every frame, and the conversion cannot answer what it was
// not told: every grey is at every hue and black is at every saturation, so a
// drag to the dark edge of the field would lose the hue it started from and
// the handle would jump to red when it came back.
type ColourPicker struct {
	hue        float32 // degrees around the wheel, 0 to 360
	saturation float32 // 0 to 1
	value      float32 // 0 to 1
	alpha      float32 // 0 to 1, where 1 is opaque

	// field takes the two-axis drag over the saturation and value area. It is
	// a drag rather than two of the sliders below because a slider reads one
	// axis and pins the other to where the press landed: two of them over the
	// same area would each claim the pointer, and a diagonal drag would move
	// whichever one was added second.
	field gesture.Drag

	// spectrum and opacity are sliders, so that the hue bar and the alpha bar
	// get the drag, the clamping and the margin past the ends that the slider
	// already has. Only the paint differs, and a second drag written for them
	// would be a second set of edge cases to get wrong.
	spectrum Slider
	opacity  Slider

	hex      Input
	swatches []Button

	// written is the last string this control put in the hex field, and it is
	// what tells a person's typing apart from the control's own writing.
	// Without it the field is rewritten from the handles every frame, and a
	// half-typed "#ff" disappears under the next keystroke.
	written string

	changed bool
	settled bool
}

// settle fixes the one field whose zero value is wrong.
//
// A state struct starts with every field at zero, and an alpha of zero is a
// colour nobody can see: a screen that drew the zero value would show nothing
// and read as a control that failed to paint. Hue, saturation and value at
// zero are opaque black, which is a colour and a reasonable place to start.
func (p *ColourPicker) settle() {
	if !p.settled {
		p.settled = true
		p.alpha = 1
	}
}

// Colour answers the colour the control is on.
func (p *ColourPicker) Colour() color.NRGBA {
	p.settle()

	colour := hsvToRGB(p.hue, p.saturation, p.value)
	colour.A = round8(p.alpha)
	return colour
}

// SetColour moves the whole control to a colour, and rewrites the hex field to
// match.
//
// It does not report a change, because the change is the caller's own: a
// screen that set a colour and then read [ColourPicker.Changed] would send
// itself back what it had just sent, which is the shape of an update loop that
// never settles.
func (p *ColourPicker) SetColour(colour color.NRGBA) {
	p.settle()
	p.take(colour)

	// Written without asking whether the field was being typed into, unlike
	// the mirroring done while drawing: this is a caller saying what the
	// colour is, and a field left showing the old one would be the control
	// disagreeing with itself.
	digits := hex(p.Colour())
	p.hex.SetText(digits)
	p.written = digits
}

// Changed reports that the colour moved, once, and consumes it.
//
// It takes no context because the events were read while the control was
// drawn; this only reports what that frame concluded. Consumed, so that a
// screen sending the colour somewhere sends it when it moves rather than on
// every frame of a drag -- which is the same value, sixty times a second, and
// looks from the far end like a client that will not stop talking.
func (p *ColourPicker) Changed() bool {
	moved := p.changed
	p.changed = false
	return moved
}

// take moves the state onto a colour, keeping what the colour cannot say.
//
// A colour can sit at a point the conversion has no name for: every grey is at
// every hue, and black is at every saturation. Taking the zero that
// [rgbToHSV] has to answer with in those cases would throw the handle into a
// corner for a colour that is not in that corner -- the picture on screen
// would not change, so it reads as a control moving by itself.
func (p *ColourPicker) take(colour color.NRGBA) {
	h, s, v := rgbToHSV(colour)

	if s > 0 {
		p.hue = h
	}
	if v > 0 {
		p.saturation = s
	}
	p.value = v
	p.alpha = float32(colour.A) / 255
}

// mirror writes the colour into the hex field, unless the field is what last
// changed.
//
// The test is between the colour the field names and the colour the control
// holds, rather than between two strings. Somebody who typed "FF0000" named
// the same red this would spell "#ff0000", and rewriting it into the control's
// own spelling would move their caret to the end of the field on the keystroke
// that completed the colour.
func (p *ColourPicker) mirror() {
	if p.hex.Text() != p.written {
		return // being typed into, and what is there is not yet a colour
	}
	if named, ok := parseHex(p.written); ok && named == p.Colour() {
		return
	}

	digits := hex(p.Colour())
	p.hex.SetText(digits)
	p.written = digits
}

// ColourPickerProps is what a colour picker is drawn from.
type ColourPickerProps struct {
	// Alpha draws a bar for how opaque the colour is.
	//
	// Without it the control still carries an alpha -- whatever the caller
	// set -- and the hex field still accepts eight digits, because hiding a
	// bar is a decision about the screen and not about the colour. What keeps
	// the two from disagreeing is that [hex] writes the eight-digit form
	// whenever the alpha is not opaque, so the field never claims a colour is
	// solid when it is not.
	Alpha bool
	// Swatches is a row of colours offered as a shortcut. Empty draws no row.
	// They are drawn in one line and not wrapped: a set long enough to need a
	// second line is a palette, and a palette is a screen rather than a
	// footnote to a control.
	Swatches []color.NRGBA
	// Disabled draws it as unavailable and refuses to move.
	Disabled bool
}

// Layout draws the picker and returns the room it took.
func (p ColourPickerProps) Layout(c ayra.Context, state *ColourPicker) ayra.Dimensions {
	state.settle()

	if p.Disabled {
		c.Context = c.Context.Disabled()

		// Half opacity over everything this draws, which is what the field says
		// it does and what every other control of this package does. Refusing
		// the drag was the whole of it before: the plane, the spectrum, the
		// opacity bar and the row of swatches were painted at full strength, so
		// a picker nobody could move looked exactly like one they could -- and
		// beside a live one, indistinguishable.
		//
		// The wash goes over rather than each colour being faded in turn.
		// Fading the colours would be a picker misreporting the colour it is
		// on, which is worse than one that looks unavailable.
		defer func() {
			wash, over := p.wash(c)
			if !over {
				return
			}
			defer clip.Rect{Max: c.Constraints.Max}.Push(c.Ops).Pop()
			paint.ColorOp{Color: wash}.Add(c.Ops)
			paint.PaintOp{}.Add(c.Ops)
		}()
	}

	// The typing is read before anything is drawn, so that a hex completed on
	// this frame moves the handles on this frame. Read after the field is laid
	// out it would land a frame later, and a handle that trails the text by a
	// frame is the kind of lag nobody can point at and everybody can feel.
	valid := p.readHex(c, state)

	// Every child is measured from the room it is offered rather than from the
	// room it is required to fill: a child of a column is handed a minimum
	// height of zero and a minimum width of zero, and a control that sized
	// itself from the minimum would be a control of no size at all.
	c.Constraints.Min = image.Point{}

	gap := layout.Rigid(layout.Spacer{Height: 8}.Layout)

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.plane(c.With(gtx), state)
		}),
		gap,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.spectrum(c.With(gtx), state)
		}),
	}

	if p.Alpha {
		children = append(children, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.opacity(c.With(gtx), state)
		}))
	}

	children = append(children, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return p.digits(c.With(gtx), state, valid)
	}))

	if len(p.Swatches) > 0 {
		children = append(children, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.presets(c.With(gtx), state)
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
}

// wash is what is drawn over the whole control, and whether anything is.
//
// Half the page's own colour, over everything, when the control is unavailable.
// Refusing the drag used to be the whole of it -- the plane, the spectrum, the
// opacity bar and the swatches were painted at full strength, so a picker
// nobody could move looked exactly like one they could, and beside a live one
// the two were the same picture.
//
// Over the top rather than each colour faded in turn, because fading the
// colours would be a picker misreporting the colour it is on. A control that
// looks unavailable is a smaller problem than one that lies about its value.
//
// It is a function because it is the whole of one decision, and a decision
// written inside a deferred closure is one nothing can ask about without
// rendering the control and reading pixels.
func (p ColourPickerProps) wash(c ayra.Context) (color.NRGBA, bool) {
	if !p.Disabled {
		return color.NRGBA{}, false
	}
	return fade(c.Theme.Colours.Background), true
}

// readHex takes what was typed into the field, and reports whether it is a
// colour.
//
// The editor is drained here rather than left to its own layout further down
// the frame, so that the string this reads is the one the person has just
// finished typing rather than the one they had finished by the previous frame.
func (p ColourPickerProps) readHex(c ayra.Context, state *ColourPicker) bool {
	for {
		if _, more := state.hex.editor.Update(c.Context); !more {
			break
		}
	}

	typed := state.hex.Text()
	if typed == state.written {
		return true // nothing typed since this control last wrote
	}

	named, ok := parseHex(typed)
	if !ok {
		// Left exactly as typed and marked as rejected. A field that repaired
		// what it could not read would put a colour on screen nobody asked
		// for, and the person would go looking for the mistake somewhere else.
		return false
	}

	// Kept in the spelling they used, so that the next frame does not decide
	// the field needs rewriting and take their caret with it.
	state.written = typed
	if named != state.Colour() {
		state.take(named)
		state.changed = true
	}
	return true
}

// plane draws the saturation and value field, and takes the drag over it.
func (p ColourPickerProps) plane(c ayra.Context, state *ColourPicker) ayra.Dimensions {
	area := image.Pt(c.Constraints.Max.X, c.Dp(unit.Dp(140)))
	if room := c.Constraints.Max.Y; room > 0 && area.Y > room {
		area.Y = room
	}
	if area.X <= 0 || area.Y <= 0 {
		return ayra.Dimensions{}
	}
	defer clip.UniformRRect(image.Rectangle{Max: area}, controlRadius(c)).Push(c.Ops).Pop()

	state.field.Add(c.Ops)
	p.drag(c, state, area)

	// Two fills rather than a grid of them. The field is two gradients laid
	// over each other -- white to the hue across, then clear to black down --
	// and both are brushes the GPU evaluates per pixel, so the whole plane
	// costs two shaded rectangles however large it is. Built instead as an
	// image it would be a texture uploaded again on every frame of a hue
	// drag, which is the one moment it must not stall.
	paint.LinearGradientOp{
		Stop1:  f32.Pt(0, 0),
		Color1: color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		Stop2:  f32.Pt(float32(area.X), 0),
		Color2: hsvToRGB(state.hue, 1, 1),
	}.Add(c.Ops)
	paint.PaintOp{}.Add(c.Ops)

	paint.LinearGradientOp{
		Stop1:  f32.Pt(0, 0),
		Color1: color.NRGBA{},
		Stop2:  f32.Pt(0, float32(area.Y)),
		Color2: color.NRGBA{A: 255},
	}.Add(c.Ops)
	paint.PaintOp{}.Add(c.Ops)

	radius := c.Dp(unit.Dp(7))
	ring(c, markCentre(area, state.saturation, 1-state.value, radius), radius)

	return ayra.Dimensions{Size: area}
}

// drag turns pointer movement over the plane into saturation and value.
func (p ColourPickerProps) drag(c ayra.Context, state *ColourPicker, area image.Point) {
	for {
		e, ok := state.field.Update(c.Metric, c.Source, gesture.Both)
		if !ok {
			return
		}
		if e.Kind != pointer.Press && e.Kind != pointer.Drag {
			continue
		}

		// Across is saturation and down is value, inverted: the bright end is
		// at the top, where every picker puts it. A plane that ran the other
		// way would be right in the arithmetic and upside down on the screen.
		saturation := clamp01(e.Position.X / span(area.X))
		value := 1 - clamp01(e.Position.Y/span(area.Y))

		// Compared before being written, so that a pointer resting still over
		// the plane does not report a change on every frame it is held.
		if saturation != state.saturation || value != state.value {
			state.saturation, state.value = saturation, value
			state.changed = true
		}
	}
}

// spectrum draws the hue bar.
func (p ColourPickerProps) spectrum(c ayra.Context, state *ColourPicker) ayra.Dimensions {
	// The fraction is compared, not the degrees. Hue is carried in degrees and
	// the slider in a fraction of one, and the round trip through float32 does
	// not land on the same degree it started from -- so a comparison in
	// degrees would find a difference on a frame nobody touched, and the
	// control would report a change forever.
	before := clamp01(state.hue / 360)
	state.spectrum.SetValue(before)

	dims := p.bar(c, &state.spectrum, func(c ayra.Context, area image.Point) {
		paintSpectrum(c, area)
	})

	if after := state.spectrum.Value(); after != before {
		state.hue = after * 360
		state.changed = true
	}
	return dims
}

// opacity draws the alpha bar.
func (p ColourPickerProps) opacity(c ayra.Context, state *ColourPicker) ayra.Dimensions {
	before := clamp01(state.alpha)
	state.opacity.SetValue(before)

	solid := state.Colour()
	solid.A = 255

	dims := p.bar(c, &state.opacity, func(c ayra.Context, area image.Point) {
		// The muted surface underneath, then the colour ramped over it. No
		// chequerboard: a chequer at this height is a hundred fills spent on a
		// decoration, and what the ramp has to say -- that the left end is not
		// there -- it says against any ground.
		paint.FillShape(c.Ops, c.Theme.Colours.Muted, clip.Rect(image.Rectangle{Max: area}).Op())

		clear := solid
		clear.A = 0
		paint.LinearGradientOp{
			Stop1:  f32.Pt(0, 0),
			Color1: clear,
			Stop2:  f32.Pt(float32(area.X), 0),
			Color2: solid,
		}.Add(c.Ops)
		paint.PaintOp{}.Add(c.Ops)
	})

	if after := state.opacity.Value(); after != before {
		state.alpha = after
		state.changed = true
	}
	return dims
}

// bar draws one of the two horizontal bars: the track, painted by the caller,
// and the handle over it.
func (p ColourPickerProps) bar(c ayra.Context, slider *Slider, track func(ayra.Context, image.Point)) ayra.Dimensions {
	area := image.Pt(c.Constraints.Max.X, c.Dp(unit.Dp(16)))
	if area.X <= 0 || area.Y <= 0 {
		return ayra.Dimensions{}
	}

	// The slider's own drag, over the whole bar, with the same half-knob
	// margin the slider gives so that a press just past either end takes hold
	// rather than being ignored.
	c.Constraints.Min = area
	slider.value.Layout(c.Context, layout.Horizontal, unit.Dp(8))

	stack := clip.UniformRRect(image.Rectangle{Max: area}, area.Y/2).Push(c.Ops)
	track(c, area)
	stack.Pop()

	width := c.Dp(unit.Dp(6))
	// Clamped, not inset. The handle has to stand over the colour it names,
	// and a handle inset by half its width at either end would point at a hue
	// that is not the hue it is set to.
	centre := markCentre(area, clamp01(slider.Value()), 0.5, width/2)
	handle := image.Rect(centre.X-width/2, 0, centre.X+width/2, area.Y)

	shape := clip.UniformRRect(handle, width/2)
	paint.FillShape(c.Ops, c.Theme.Colours.Background, shape.Op(c.Ops))
	paint.FillShape(c.Ops, c.Theme.Colours.Foreground,
		clip.Stroke{Path: shape.Path(c.Ops), Width: float32(hairline(c))}.Op())

	return ayra.Dimensions{Size: area}
}

// digits draws the hex field, with the colour shown inside its trailing edge.
func (p ColourPickerProps) digits(c ayra.Context, state *ColourPicker, valid bool) ayra.Dimensions {
	state.mirror()

	dims := InputProps{Placeholder: "#000000", Disabled: p.Disabled, Invalid: !valid}.Layout(c, &state.hex)

	// The swatch is sized from what the field came back saying, not from the
	// constraints: a child in a column is given a minimum height of zero, and
	// a swatch taking its side from that would be a square of nothing -- which
	// draws no error and looks like the colour is simply missing.
	//
	// It is painted over the field's trailing edge rather than beside it, so
	// that the field is laid out once. Laid out twice -- once to measure and
	// once for real -- the editor would register its pointer and key handlers
	// twice in the same frame.
	inset := c.Dp(unit.Dp(6))
	side := dims.Size.Y - 2*inset
	if side <= 0 || dims.Size.X < side+2*inset {
		return dims
	}

	patch := image.Rect(dims.Size.X-inset-side, inset, dims.Size.X-inset, inset+side)
	shape := clip.UniformRRect(patch, c.Dp(unit.Dp(3)))
	paint.FillShape(c.Ops, state.Colour(), shape.Op(c.Ops))
	paint.FillShape(c.Ops, c.Theme.Colours.Border,
		clip.Stroke{Path: shape.Path(c.Ops), Width: float32(hairline(c))}.Op())

	return dims
}

// presets draws the row of offered colours.
func (p ColourPickerProps) presets(c ayra.Context, state *ColourPicker) ayra.Dimensions {
	// Grown rather than rebuilt, because each square is a press somebody may
	// be in the middle of: a fresh set every frame never sees the release of
	// its own press, and the row would take no presses at all.
	for len(state.swatches) < len(p.Swatches) {
		state.swatches = append(state.swatches, Button{})
	}

	side := c.Dp(unit.Dp(22))
	radius := c.Dp(unit.Dp(4))

	children := make([]layout.FlexChild, 0, 2*len(p.Swatches))
	for index, preset := range p.Swatches {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 6}.Layout))
		}

		if state.swatches[index].Clicked(c) {
			state.take(preset)
			state.changed = true
		}

		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return state.swatches[index].click.Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
				square := image.Rectangle{Max: image.Pt(side, side)}
				shape := clip.UniformRRect(square, radius)

				paint.FillShape(gtx.Ops, preset, shape.Op(gtx.Ops))
				paint.FillShape(gtx.Ops, inner.Theme.Colours.Border,
					clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(hairline(inner))}.Op())

				return layout.Dimensions{Size: square.Max}
			})
		}))
	}

	c.Constraints.Min = image.Point{}
	return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context, children...)
}

// paintSpectrum fills the current clip with the hue wheel, left to right.
//
// Six gradients, one per sector, and six is exact rather than a sampling: at
// full saturation and value exactly one channel ramps across a sector while
// the other two hold, so a straight line between two corners is the spectrum
// and not an approximation of it. Fewer would run a line through a corner,
// and the colour at the corner -- cyan, yellow, magenta -- would come out
// grey. More would be the same picture drawn in more pieces.
func paintSpectrum(c ayra.Context, area image.Point) {
	const sectors = 6

	for sector := 0; sector < sectors; sector++ {
		// Boundaries in whole pixels, computed the same way from both sides,
		// so that each strip starts exactly where the last one ended. Rounded
		// independently they leave a seam of background between two of them.
		from := area.X * sector / sectors
		to := area.X * (sector + 1) / sectors
		if to <= from {
			continue
		}

		strip := clip.Rect(image.Rect(from, 0, to, area.Y)).Push(c.Ops)
		paint.LinearGradientOp{
			Stop1:  f32.Pt(float32(from), 0),
			Color1: hsvToRGB(float32(sector)*60, 1, 1),
			Stop2:  f32.Pt(float32(to), 0),
			Color2: hsvToRGB(float32(sector+1)*60, 1, 1),
		}.Add(c.Ops)
		paint.PaintOp{}.Add(c.Ops)
		strip.Pop()
	}
}

// ring draws the handle that marks a point on the plane.
//
// An outline rather than a disc, because a filled handle covers the one colour
// the person is looking at. Two circles, pale inside a dark one, so that it
// reads at both ends of the plane: a white ring alone vanishes in the top
// corner and a dark one vanishes at the bottom.
func ring(c ayra.Context, centre image.Point, radius int) {
	stroke := float32(hairline(c))
	bounds := image.Rect(centre.X-radius, centre.Y-radius, centre.X+radius, centre.Y+radius)

	paint.FillShape(c.Ops, color.NRGBA{A: 160},
		clip.Stroke{Path: clip.Ellipse(bounds).Path(c.Ops), Width: stroke * 3}.Op())

	inner := bounds.Inset(int(stroke))
	paint.FillShape(c.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		clip.Stroke{Path: clip.Ellipse(inner).Path(c.Ops), Width: stroke * 2}.Op())
}

// markCentre answers where a handle sits in an area, kept far enough from each
// edge that the whole handle is inside it.
//
// Clamped and not inset, because what the handle points at has to be what is
// under it: an area drawn to its edges and a handle inset by its own radius
// disagree about where saturation nought is, by a radius, at both ends.
//
// An area too small to hold the handle puts it in the middle, which is the
// only place that is wrong by the least amount.
func markCentre(area image.Point, across, down float32, radius int) image.Point {
	return image.Pt(
		clampInt(int(clamp01(across)*float32(area.X)), radius, area.X-radius),
		clampInt(int(clamp01(down)*float32(area.Y)), radius, area.Y-radius),
	)
}

// span is what a coordinate is divided by to become a fraction.
//
// One less than the width, because the range runs from the first pixel to the
// last rather than to one past it. Divided by the width, a plane four hundred
// across answers 0.9975 for its own last column: the pure colour would be
// reachable only by dragging off the edge of it, which on a touch screen is
// not reachable at all.
func span(pixels int) float32 {
	if pixels < 2 {
		return 1
	}
	return float32(pixels - 1)
}

// clampInt holds a number between two bounds, and answers the midpoint when
// the bounds have crossed.
func clampInt(value, low, high int) int {
	if low > high {
		return (low + high) / 2
	}
	return min(max(value, low), high)
}

// clamp01 holds a fraction between nought and one.
func clamp01(value float32) float32 {
	if value != value {
		// A division by a zero-sized area answers this, and a comparison
		// against it is false either way round -- so a handle placed from it
		// would sit at a position no clamp catches.
		return 0
	}
	return min(max(value, 0), 1)
}

// round8 turns a fraction into a byte, to the nearest.
//
// Rounded rather than truncated, because truncation loses the round trip: one
// arrived at from 255 is 254.9999 in float, and truncating answers 254 -- so a
// colour converted and converted back would be a shade darker every time
// anybody touched it.
func round8(value float32) uint8 {
	if !(value > 0) {
		return 0
	}
	if value >= 1 {
		return 255
	}
	return uint8(value*255 + 0.5)
}

// hsvToRGB turns a hue in degrees and a saturation and value between nought
// and one into bytes.
//
// The alpha comes back opaque. Alpha is a fourth number that no hue,
// saturation or value has anything to say about, and carrying it through both
// conversions would be two places holding the same fact, which is two places
// to disagree.
func hsvToRGB(h, s, v float32) color.NRGBA {
	s, v = clamp01(s), clamp01(v)
	h = wrapHue(h)

	if s == 0 {
		// No hue at all. Reading the sector table here would be reading a
		// number that says nothing, and it is what turns a grey into a colour
		// when the number happens to be a leftover.
		grey := round8(v)
		return color.NRGBA{R: grey, G: grey, B: grey, A: 255}
	}

	position := h / 60
	sector := int(position)
	offset := position - float32(sector)

	low := v * (1 - s)
	falling := v * (1 - s*offset)
	rising := v * (1 - s*(1-offset))

	var r, g, b float32
	switch sector {
	case 0:
		r, g, b = v, rising, low
	case 1:
		r, g, b = falling, v, low
	case 2:
		r, g, b = low, v, rising
	case 3:
		r, g, b = low, falling, v
	case 4:
		r, g, b = rising, low, v
	default:
		r, g, b = v, low, falling
	}

	return color.NRGBA{R: round8(r), G: round8(g), B: round8(b), A: 255}
}

// rgbToHSV turns bytes into a hue in degrees and a saturation and value
// between nought and one.
//
// The alpha is not read, for the reason [hsvToRGB] does not write one.
func rgbToHSV(colour color.NRGBA) (h, s, v float32) {
	r := float32(colour.R) / 255
	g := float32(colour.G) / 255
	b := float32(colour.B) / 255

	high := max(r, max(g, b))
	low := min(r, min(g, b))
	v = high

	spread := high - low
	if spread == 0 {
		// Grey, and white and black are greys: the three channels agree, and
		// no hue separates them. Nought is the answer because the function has
		// to give one, and it is a safe one to give because a saturation of
		// nought makes every hue the same grey. The early return is also what
		// keeps the division below from being nought over nought, which
		// answers a value that no comparison afterwards can catch.
		return 0, 0, v
	}
	s = spread / high

	switch high {
	case r:
		h = (g - b) / spread
	case g:
		h = 2 + (b-r)/spread
	default:
		h = 4 + (r-g)/spread
	}

	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, v
}

// wrapHue brings a hue onto the wheel, where 360 is nought.
//
// The wheel closes, and a hue that came from a drag can land exactly on the
// end: without this the sector index would be six, one past the table, and the
// last pixel of the spectrum would be drawn in whatever the default arm says.
func wrapHue(h float32) float32 {
	h = float32(math.Mod(float64(h), 360))
	if h < 0 {
		h += 360
	}
	return h
}

// parseHex reads a colour written as digits, and reports whether it was one.
//
// Three lengths are taken -- three digits, six, or eight carrying an alpha --
// with or without a leading hash and in either case. Everything else is
// refused rather than repaired, including a four-digit form and a string with
// space around it: a field that guessed would put a colour on the screen that
// nobody typed, and the person would go looking for the mistake somewhere
// else.
func parseHex(s string) (color.NRGBA, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 3 && len(s) != 6 && len(s) != 8 {
		return color.NRGBA{}, false
	}

	var digits [8]uint8
	for index := 0; index < len(s); index++ {
		digit, ok := hexDigit(s[index])
		if !ok {
			return color.NRGBA{}, false
		}
		digits[index] = digit
	}

	if len(s) == 3 {
		// Each digit stands for a pair of itself, so that "#fff" is white
		// rather than the shade one step short of it.
		return color.NRGBA{R: digits[0] * 17, G: digits[1] * 17, B: digits[2] * 17, A: 255}, true
	}

	colour := color.NRGBA{
		R: digits[0]<<4 | digits[1],
		G: digits[2]<<4 | digits[3],
		B: digits[4]<<4 | digits[5],
		A: 255,
	}
	if len(s) == 8 {
		colour.A = digits[6]<<4 | digits[7]
	}
	return colour, true
}

// hexDigit answers what one character is worth, and whether it was a digit.
func hexDigit(b byte) (uint8, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	}
	return 0, false
}

// hex writes a colour as digits, in a spelling [parseHex] reads back.
//
// Eight digits only when the alpha has something to say. A field reading
// "#ff0000ff" for an opaque red is two characters of noise in the one place on
// this control where somebody types, and the shorter form names the same
// colour.
func hex(colour color.NRGBA) string {
	const table = "0123456789abcdef"

	digits := []byte("#00000000")
	for index, channel := range []uint8{colour.R, colour.G, colour.B, colour.A} {
		digits[1+2*index] = table[channel>>4]
		digits[2+2*index] = table[channel&0x0f]
	}

	if colour.A == 255 {
		return string(digits[:7])
	}
	return string(digits)
}
