package widget

import (
	"image"
	"image/color"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// DateFormat is how a chosen day is written on the control.
//
// It is the ISO order rather than a local one, and that is a decision the
// control has to make for itself: the two common local orders put the day and
// the month in opposite places, so a date written in either is read wrongly by
// half the people who see it. Year, month, day is ambiguous to nobody.
const DateFormat = "2006-01-02"

// DatePicker is the state of a field that opens a month to choose from.
type DatePicker struct {
	// Popover is the open state, and the press that toggles it.
	popover Menu
	// Month is the grid inside it.
	month Calendar
	// Seen is what was chosen the last time this was drawn, which is how the
	// panel knows a day was picked and closes itself. The calendar reports the
	// press to its own state, not upwards, and a control that never closed
	// would need a second press somewhere else to dismiss it.
	seen time.Time
	// Picked is that press, held for one frame, for the caller to read.
	picked bool
}

// Selected is the day chosen, and the zero time when none is.
func (d *DatePicker) Selected() time.Time { return d.month.Selected() }

// Select chooses a day without opening the panel.
func (d *DatePicker) Select(day time.Time) {
	d.month.Select(day)
	d.seen = day
}

// Changed reports that a day was chosen, once, and consumes it.
func (d *DatePicker) Changed() bool {
	picked := d.picked
	d.picked = false
	return picked
}

// Showing reports whether the month is on screen.
func (d *DatePicker) Showing() bool { return d.popover.Showing() }

// DatePickerProps is a field that chooses one day.
type DatePickerProps struct {
	// Placeholder is what stands in the field before a day is chosen.
	Placeholder string
	// Today, Min, Max and Monday are the month's, and mean what they mean
	// there.
	Today    time.Time
	Min, Max time.Time
	Monday   bool
	// Disabled draws it as unavailable and stops it opening.
	Disabled bool
	// Above opens the month upwards, for a field near the foot of a page.
	Above bool
}

// Layout draws the field and, while it is open, the month under it.
func (p DatePickerProps) Layout(c ayra.Context, state *DatePicker) ayra.Dimensions {
	label := p.Placeholder
	if label == "" {
		label = "Pick a date"
	}
	ink := c.Theme.Colours.MutedForeground
	if day := state.month.Selected(); !day.IsZero() {
		label = day.Format(DateFormat)
		ink = c.Theme.Colours.Foreground
	}

	if p.Disabled {
		// Closed as well as unpressable: a panel left open over a field that
		// has since been disabled is a month whose days do nothing.
		state.popover.Close()
	}

	dims := PopoverProps{Width: 320, Above: p.Above}.Layout(c, &state.popover,
		func(c ayra.Context) ayra.Dimensions {
			return p.field(c, label, ink)
		},
		func(c ayra.Context) ayra.Dimensions {
			return CalendarProps{
				Today:  p.Today,
				Min:    p.Min,
				Max:    p.Max,
				Monday: p.Monday,
			}.Layout(c, &state.month)
		},
	)

	if chosen := state.month.Selected(); !chosen.Equal(state.seen) {
		state.seen = chosen
		state.picked = true
		state.popover.Close()
		// The frame that closes the panel is already drawn with it open. Asking
		// for another is what takes it off the screen now rather than when the
		// pointer next moves.
		c.Redraw()
	}

	return dims
}

// field draws the closed control: the day, or what stands in for it, with the
// border a text field has.
func (p DatePickerProps) field(c ayra.Context, label string, ink color.NRGBA) ayra.Dimensions {
	fill, border := c.Theme.Colours.Background, c.Theme.Colours.Border
	if p.Disabled {
		fill, border, ink = fade(fill), fade(border), fade(ink)
		c.Context = c.Context.Disabled()
	}

	return surface(c, fill, border, controlRadius(c), func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return drawText(inner.With(gtx), label, unit.Sp(c.Theme.Type.Body), ink, 1, text.Start, plain())
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return calendarMark(inner.With(gtx), ink)
				}),
			)
		})
	})
}

// calendarMark draws the small grid that says this field opens a month.
//
// Drawn rather than set in text: the glyph for it is not in every font, and a
// missing glyph is a hollow box in the corner of a form.
func calendarMark(c ayra.Context, ink color.NRGBA) ayra.Dimensions {
	size := c.Dp(unit.Dp(14))
	line := c.Dp(unit.Dp(1))
	if line < 1 {
		line = 1
	}

	// The page, the rule across its head, and the two posts above it.
	head := size / 3
	page := image.Rect(0, head, size, size)
	paint.FillShape(c.Ops, ink, clip.Stroke{
		Path:  clip.UniformRRect(page, line).Path(c.Ops),
		Width: float32(line),
	}.Op())
	paint.FillShape(c.Ops, ink, clip.Rect(image.Rect(0, head, size, head+line*2)).Op())
	for _, at := range []int{size / 4, size - size/4 - line} {
		paint.FillShape(c.Ops, ink, clip.Rect(image.Rect(at, 0, at+line, head)).Op())
	}

	return ayra.Dimensions{Size: image.Pt(size, size)}
}
