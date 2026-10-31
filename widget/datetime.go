package widget

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/mask"
)

// DateTimeFormat is how a chosen day and time are written on the control: the ISO
// order, for the reason [DateFormat] gives, and the twenty-four-hour clock.
const DateTimeFormat = "2006-01-02 15:04"

// DateTimeKind is what a [DateTimePickerProps] asks for: the kinds Kyse's
// DateTimePicker takes -- "date", "time", "datetime", "month" and "week".
type DateTimeKind uint8

const (
	// DateAndTime asks for a day and the time of it. Zero, because it is what the
	// control is named for.
	DateAndTime DateTimeKind = iota
	// DateOnly asks for a day.
	DateOnly
	// TimeOnly asks for a time of day.
	TimeOnly
	// MonthOnly asks for a month of a year, and the value is its first day.
	MonthOnly
	// WeekOnly asks for a week, written as the ISO week 2026-W37, and the value is
	// its Monday.
	WeekOnly
)

// DateTime is the state of a field that chooses a day and a time of day.
type DateTime struct {
	year        int
	chosenMonth time.Month
	chosenYear  int
	months      [12]Button
	earlier     Button
	later       Button
	popover     Menu
	month       Calendar
	clock       Masked
	done        Button
	seen        time.Time
	changed     bool
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

// Open shows the panel, for a screen that opens it by some other means than the
// press on the field.
func (d *DateTime) Open() { d.popover.Open() }

// Showing reports whether the panel is on screen.
func (d *DateTime) Showing() bool { return d.popover.Showing() }

// Value is what the field holds for the kind it was drawn as: the day and the
// time for [DateAndTime]; the day at midnight for [DateOnly]; the first of the
// month for [MonthOnly]; the Monday of the week for [WeekOnly]; and for
// [TimeOnly] the time on the zero day, whose hour and minute are what matter.
// It is the zero time when nothing was chosen.
func (d *DateTime) Value(kind DateTimeKind, seconds bool) time.Time {
	switch kind {
	case DateOnly:
		return startOfDay(d.month.Selected())
	case TimeOnly:
		hour, minute, second, valid := parseClock(d.clock.Text(), seconds)
		if !valid {
			return time.Time{}
		}
		return time.Date(1, time.January, 1, hour, minute, second, 0, time.UTC)
	case MonthOnly:
		if d.chosenMonth == 0 {
			return time.Time{}
		}
		return time.Date(d.chosenYear, d.chosenMonth, 1, 0, 0, 0, 0, time.UTC)
	case WeekOnly:
		day := d.month.Selected()
		if day.IsZero() {
			return time.Time{}
		}
		back := (int(day.Weekday()) + 6) % 7
		return startOfDay(day).AddDate(0, 0, -back)
	}
	return d.Selected(seconds)
}

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
	// Kind is what is asked for. Zero is a day and a time.
	Kind DateTimeKind
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

// layoutFor is the layout a value of this kind is written in, unless the caller
// gave one.
func (p DateTimePickerProps) layoutFor() string {
	if p.Format != "" {
		return p.Format
	}
	clock := "15:04"
	if p.Seconds {
		clock = "15:04:05"
	}
	switch p.Kind {
	case DateOnly:
		return DateFormat
	case TimeOnly:
		return clock
	case MonthOnly:
		return "2006-01"
	}
	return DateFormat + " " + clock
}

// Text writes the value the way the field shows it: the ISO week for a week, and
// the format of the kind for the others. It is empty when nothing was chosen.
func (p DateTimePickerProps) Text(state *DateTime) string {
	value := state.Value(p.Kind, p.Seconds)
	if value.IsZero() {
		return ""
	}
	if p.Kind == WeekOnly {
		year, week := value.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	}
	if p.Kind == MonthOnly && len(p.MonthNames) == 12 {
		return p.MonthNames[value.Month()-1] + " " + strconv.Itoa(value.Year())
	}
	return value.Format(p.layoutFor())
}

// Layout draws the field and, while it is open, the panel under it.
func (p DateTimePickerProps) Layout(c ayra.Context, state *DateTime) ayra.Dimensions {
	placeholder := map[DateTimeKind]string{
		DateAndTime: "Pick a date and time", DateOnly: "Pick a date", TimeOnly: "Pick a time",
		MonthOnly: "Pick a month", WeekOnly: "Pick a week",
	}[p.Kind]
	label, ink := orDefault(p.Placeholder, placeholder), c.Theme.Colours.MutedForeground
	if text := p.Text(state); text != "" {
		label, ink = text, c.Theme.Colours.Foreground
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

	if at := state.Value(p.Kind, p.Seconds); !at.Equal(state.seen) {
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

	calendar := func(gtx layout.Context) layout.Dimensions {
		return CalendarProps{
			Today: p.Today, Min: p.Min, Max: p.Max, Monday: p.Monday,
			MonthNames: p.MonthNames, WeekdayNames: p.WeekdayNames,
		}.Layout(c.With(gtx), &state.month)
	}
	clock := func(gtx layout.Context) layout.Dimensions {
		return FieldProps{Label: orDefault(p.TimeLabel, "Time")}.Layout(c.With(gtx), func(c ayra.Context) ayra.Dimensions {
			_, _, _, valid := parseClock(state.clock.Text(), p.Seconds)
			return MaskedProps{
				Mask: clockMask(p.Seconds), Kind: Number, Placeholder: "00:00",
				Invalid: state.clock.Text() != "" && !valid && state.clock.Complete(clockMask(p.Seconds)),
			}.Layout(c, &state.clock)
		})
	}
	done := func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		inner.Constraints.Min.X = inner.Constraints.Max.X
		return ButtonProps{Label: orDefault(p.DoneLabel, "Done")}.Layout(inner, &state.done)
	}

	rows := []layout.FlexChild{}
	switch p.Kind {
	case TimeOnly:
		rows = append(rows, layout.Rigid(clock))
	case MonthOnly:
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.months(c.With(gtx), state) }))
	case DateOnly, WeekOnly:
		rows = append(rows, layout.Rigid(calendar))
	default:
		rows = append(rows, layout.Rigid(calendar), layout.Rigid(layout.Spacer{Height: 10}.Layout), layout.Rigid(clock))
	}
	rows = append(rows, layout.Rigid(layout.Spacer{Height: 10}.Layout), layout.Rigid(done))
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
}

// months draws the year and its twelve months, three to a row.
func (p DateTimePickerProps) months(c ayra.Context, state *DateTime) ayra.Dimensions {
	if state.year == 0 {
		state.year = p.Today.Year()
		if !state.month.Selected().IsZero() {
			state.year = state.month.Selected().Year()
		}
	}
	if state.earlier.Clicked(c) {
		state.year--
	}
	if state.later.Clicked(c) {
		state.year++
	}
	for i := range state.months {
		if state.months[i].Clicked(c) {
			state.chosenMonth, state.chosenYear = time.Month(i+1), state.year
		}
	}

	rows := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					i := inner.With(gtx)
					i.Constraints.Min = image.Point{}
					return ButtonProps{Label: "<", Variant: Ghost, Size: Small}.Layout(i, &state.earlier)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return drawText(inner.With(gtx), strconv.Itoa(state.year), unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Middle, semibold())
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					i := inner.With(gtx)
					i.Constraints.Min = image.Point{}
					return ButtonProps{Label: ">", Variant: Ghost, Size: Small}.Layout(i, &state.later)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
	}
	for row := 0; row < 4; row++ {
		row := row
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cells := make([]layout.FlexChild, 0, 3)
			for col := 0; col < 3; col++ {
				index := row*3 + col
				cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					name := time.Month(index + 1).String()[:3]
					if len(p.MonthNames) == 12 {
						name = p.MonthNames[index]
						if r := []rune(name); len(r) > 4 {
							name = string(r[:3])
						}
					}
					variant := Ghost
					if state.chosenMonth == time.Month(index+1) && state.year == state.chosenYear {
						variant = Default
					}
					inner := c.With(gtx)
					inner.Constraints.Min.X = inner.Constraints.Max.X
					return ButtonProps{Label: name, Variant: variant, Size: Small}.Layout(inner, &state.months[index])
				}))
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
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
