package widget

import (
	"strconv"
	"strings"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/mask"
)

// DateTimeFormat is how a chosen day and time are written on the control: the ISO
// order, for the reason [DateFormat] gives, and the twenty-four-hour clock.
const DateTimeFormat = "2006-01-02 15:04"

// DateTime is the state of a field that chooses a day and a time of day.
type DateTime struct {
	popover Menu
	month   Calendar
	clock   Masked
	done    Button
	seen    time.Time
	changed bool
	// location is the zone the value is in: the one of the day the caller
	// passed as today, which is how a caller says which zone it means.
	location *time.Location
}

// Selected is the day and time chosen, and the zero time when no day is. A time
// of day that is not typed yet, or is not a time, is midnight: a day with no
// hour is the start of it.
func (d *DateTime) Selected(seconds bool) time.Time {
	day := d.month.Selected()
	if day.IsZero() {
		return time.Time{}
	}
	hour, minute, second, _ := parseClock(d.clock.Text(), seconds)
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, second, 0, day.Location())
}

// Select chooses a day and a time without opening the panel.
func (d *DateTime) Select(at time.Time, seconds bool) {
	d.month.Select(at)
	layout := "1504"
	if seconds {
		layout = "150405"
	}
	d.clock.SetValue(nil, at.Format(layout))
	d.seen = d.Selected(seconds)
}

// Changed reports that the day or the time moved, once, and consumes it.
func (d *DateTime) Changed() bool {
	changed := d.changed
	d.changed = false
	return changed
}

// Showing reports whether the panel is on screen.
func (d *DateTime) Showing() bool { return d.popover.Showing() }

// DateTimePickerProps is a field that chooses a day and the time of it -- the
// date half of Kyse's DateTimePicker. A time on its own is a [MaskedProps] with
// [mask.Time].
//
// The day is chosen from a calendar and the time is typed into a field that
// keeps the shape 09:30. The panel stays open while either is changed, because a
// person choosing a moment moves between the two, and is put away by the button
// under them.
type DateTimePickerProps struct {
	// ComponentProps is what the caller changes on this one instance. The field
	// publishes "root".
	ComponentProps
	// Placeholder is what stands in the field before a day is chosen.
	Placeholder string
	// Today, Min, Max and Monday are the calendar's, and mean what they mean
	// there. Today also says which zone the value is in.
	Today    time.Time
	Min, Max time.Time
	Monday   bool
	// Seconds asks for seconds as well as minutes.
	Seconds bool
	// Format is the Go reference layout the field writes the value in. Empty
	// writes [DateTimeFormat], or the same with seconds.
	Format string
	// MonthNames and WeekdayNames are the calendar's words, in the application's
	// language. Empty draws English.
	MonthNames, WeekdayNames []string
	// TimeLabel names the time field in the panel, and DoneLabel the button that
	// puts the panel away.
	TimeLabel, DoneLabel string
	// Disabled draws it as unavailable and stops it opening.
	Disabled bool
	// Above opens the panel upwards.
	Above bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p DateTimePickerProps) PartNames() []string { return []string{"root"} }

// Layout draws the field and, while it is open, the panel under it.
func (p DateTimePickerProps) Layout(c ayra.Context, state *DateTime) ayra.Dimensions {
	format := p.Format
	if format == "" {
		format = DateTimeFormat
		if p.Seconds {
			format = "2006-01-02 15:04:05"
		}
	}

	label, ink := orDefault(p.Placeholder, "Pick a date and time"), c.Theme.Colours.MutedForeground
	if at := state.Selected(p.Seconds); !at.IsZero() {
		label, ink = at.Format(format), c.Theme.Colours.Foreground
	}

	field := func(c ayra.Context) ayra.Dimensions {
		return dateField(c, p.ComponentProps, "datetimepicker", label, ink, p.Disabled)
	}
	if p.Disabled {
		state.popover.Close()
		return field(c)
	}

	dims := PopoverProps{Width: 320, Above: p.Above}.Layout(c, &state.popover, field, func(c ayra.Context) ayra.Dimensions {
		return p.panel(c, state)
	})

	if at := state.Selected(p.Seconds); !at.Equal(state.seen) {
		state.seen = at
		state.changed = true
		c.Redraw()
	}
	return dims
}

// clockMask is the shape of the time field.
func clockMask(seconds bool) mask.Pattern {
	if seconds {
		return mask.TimeSeconds
	}
	return mask.Time
}

func (p DateTimePickerProps) panel(c ayra.Context, state *DateTime) ayra.Dimensions {
	if state.done.Clicked(c) {
		state.popover.Close()
		c.Redraw()
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return CalendarProps{
				Today: p.Today, Min: p.Min, Max: p.Max, Monday: p.Monday,
				MonthNames: p.MonthNames, WeekdayNames: p.WeekdayNames,
			}.Layout(c.With(gtx), &state.month)
		}),
		layout.Rigid(layout.Spacer{Height: 10}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return FieldProps{Label: orDefault(p.TimeLabel, "Time")}.Layout(c.With(gtx), func(c ayra.Context) ayra.Dimensions {
				_, _, _, valid := parseClock(state.clock.Text(), p.Seconds)
				return MaskedProps{
					Mask: clockMask(p.Seconds), Kind: Number, Placeholder: "00:00",
					Invalid: state.clock.Text() != "" && !valid && state.clock.Complete(clockMask(p.Seconds)),
				}.Layout(c, &state.clock)
			})
		}),
		layout.Rigid(layout.Spacer{Height: 10}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min.X = inner.Constraints.Max.X
			return ButtonProps{Label: orDefault(p.DoneLabel, "Done")}.Layout(inner, &state.done)
		}),
	)
}

// parseClock reads 09:30 or 09:30:15, and says whether it was a time: hours
// below 24 and minutes and seconds below 60. Anything it cannot read is
// midnight and not valid, so a half-typed time never moves the value.
func parseClock(text string, seconds bool) (hour, minute, second int, valid bool) {
	parts := strings.Split(text, ":")
	want := 2
	if seconds {
		want = 3
	}
	if len(parts) != want {
		return 0, 0, 0, false
	}
	numbers := make([]int, want)
	for i, part := range parts {
		if len(part) != 2 {
			return 0, 0, 0, false
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return 0, 0, 0, false
		}
		numbers[i] = n
	}
	if numbers[0] > 23 || numbers[1] > 59 || (seconds && numbers[2] > 59) {
		return 0, 0, 0, false
	}
	if seconds {
		return numbers[0], numbers[1], numbers[2], true
	}
	return numbers[0], numbers[1], 0, true
}
