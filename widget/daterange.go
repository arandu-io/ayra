package widget

import (
	"image"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// DatePreset is a shortcut above the calendar: a name and the period it picks.
type DatePreset struct {
	// Label is what the shortcut is called.
	Label string
	// From and To are the first and the last day, ends included.
	From, To time.Time
}

// LastDays is the preset for the last n days, counting today: 1 is today and 7
// is the week that ends today.
//
// It takes the day as an argument and does not read the clock, for the reason
// every date control here takes one: what today is depends on where somebody
// is, and a control that decided would be a different picture tomorrow.
func LastDays(label string, today time.Time, n int) DatePreset {
	today = startOfDay(today)
	return DatePreset{Label: label, From: today.AddDate(0, 0, -(n - 1)), To: today}
}

// ThisMonth is the preset for the month that today is in.
func ThisMonth(label string, today time.Time) DatePreset {
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	return DatePreset{Label: label, From: first, To: first.AddDate(0, 1, -1)}
}

// PreviousMonth is the preset for the month before the one today is in.
func PreviousMonth(label string, today time.Time) DatePreset {
	first := time.Date(today.Year(), today.Month()-1, 1, 0, 0, 0, 0, today.Location())
	return DatePreset{Label: label, From: first, To: first.AddDate(0, 1, -1)}
}

// DateRange is the state of a field that chooses a period: the start and the end
// of it, the month in the panel, and the shortcuts.
type DateRange struct {
	popover Menu
	month   Calendar
	from    time.Time
	to      time.Time
	// waiting is true between the press that chose the start and the one that
	// will choose the end.
	waiting bool
	presets []Button
	clear   Button
	changed bool
}

// Range is the period chosen, ends included, and two zero times when none is. A
// period is both ends or neither: while the end is still to be chosen this
// answers neither, so a caller that asks the server on change never asks with
// half of one.
func (d *DateRange) Range() (from, to time.Time) {
	if d.waiting {
		return time.Time{}, time.Time{}
	}
	return d.from, d.to
}

// Set chooses a period without opening the panel.
func (d *DateRange) Set(from, to time.Time) {
	if to.Before(from) {
		from, to = to, from
	}
	d.from, d.to, d.waiting = from, to, false
	d.month.Show(from)
}

// Clear empties the period.
func (d *DateRange) Clear() { d.from, d.to, d.waiting = time.Time{}, time.Time{}, false }

// Changed reports that a period was completed or cleared, once, and consumes it.
func (d *DateRange) Changed() bool {
	changed := d.changed
	d.changed = false
	return changed
}

// Showing reports whether the panel is on screen.
func (d *DateRange) Showing() bool { return d.popover.Showing() }

// DateRangePickerProps is a field that chooses a period -- the counterpart of
// Kyse's DateRangePicker. The first day pressed is the start and the second is
// the end, in either order; a shortcut picks both at once.
type DateRangePickerProps struct {
	// ComponentProps is what the caller changes on this one instance. The field
	// publishes "root"; the panel's calendar and buttons are the ones of their
	// own widgets.
	ComponentProps
	// Placeholder is what stands in the field before a period is chosen.
	Placeholder string
	// Today, Min, Max and Monday are the calendar's, and mean what they mean
	// there.
	Today    time.Time
	Min, Max time.Time
	Monday   bool
	// Presets are the shortcuts above the calendar. None draws none.
	Presets []DatePreset
	// DateFormat is the Go reference layout the field writes a day in:
	// "02/01/2006" for the Brazilian one. Empty writes [DateFormat].
	DateFormat string
	// MonthNames and WeekdayNames are the calendar's words, in the application's
	// language. Empty draws English.
	MonthNames, WeekdayNames []string
	// PendingLabel is said while the start is chosen and the end is not.
	PendingLabel string
	// ClearLabel is the control that empties the period.
	ClearLabel string
	// Disabled draws it as unavailable and stops it opening.
	Disabled bool
	// Above opens the panel upwards.
	Above bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p DateRangePickerProps) PartNames() []string { return []string{"root"} }

// Layout draws the field and, while it is open, the panel under it.
func (p DateRangePickerProps) Layout(c ayra.Context, state *DateRange) ayra.Dimensions {
	layoutName := orDefault(p.DateFormat, DateFormat)

	label, ink := orDefault(p.Placeholder, "Choose a period"), c.Theme.Colours.MutedForeground
	if from, to := state.Range(); !from.IsZero() {
		label = from.Format(layoutName) + " – " + to.Format(layoutName)
		ink = c.Theme.Colours.Foreground
	}

	field := func(c ayra.Context) ayra.Dimensions {
		return dateField(c, p.ComponentProps, "daterangepicker", label, ink, p.Disabled)
	}
	if p.Disabled {
		state.popover.Close()
		return field(c)
	}

	dims := PopoverProps{Width: 340, Above: p.Above}.Layout(c, &state.popover, field, func(c ayra.Context) ayra.Dimensions {
		return p.panel(c, state)
	})

	state.apply(p, c)
	return dims
}

// apply reads what was pressed in the panel and moves the period with it.
func (d *DateRange) apply(p DateRangePickerProps, c ayra.Context) {
	if !d.popover.Showing() {
		return
	}

	for index := range p.Presets {
		if index < len(d.presets) && d.presets[index].Clicked(c) {
			d.Set(p.Presets[index].From, p.Presets[index].To)
			d.finish(c)
			return
		}
	}
	if d.clear.Clicked(c) {
		d.Clear()
		d.changed = true
		c.Redraw()
		return
	}

	day := d.month.Pressed()
	switch {
	case day.IsZero():
	case !d.waiting:
		d.from, d.to, d.waiting = day, time.Time{}, true
		c.Redraw()
	default:
		if day.Before(d.from) {
			d.from, d.to = day, d.from
		} else {
			d.to = day
		}
		d.waiting = false
		d.finish(c)
	}
}

// finish is a completed period: it is announced and the panel is put away.
func (d *DateRange) finish(c ayra.Context) {
	d.changed = true
	d.popover.Close()
	c.Redraw()
}

// panel draws the shortcuts, the month and the line under it.
func (p DateRangePickerProps) panel(c ayra.Context, state *DateRange) ayra.Dimensions {
	for len(state.presets) < len(p.Presets) {
		state.presets = append(state.presets, Button{})
	}

	rows := []layout.FlexChild{}
	for index := 0; index < len(p.Presets); index += 2 {
		index := index
		rows = append(rows,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				cells := []layout.FlexChild{}
				for at := index; at < min(index+2, len(p.Presets)); at++ {
					at := at
					cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						inner := c.With(gtx)
						inner.Constraints.Min.X = inner.Constraints.Max.X
						return ButtonProps{Label: p.Presets[at].Label, Variant: Outline, Size: Small}.Layout(inner, &state.presets[at])
					}))
					cells = append(cells, layout.Rigid(layout.Spacer{Width: 6}.Layout))
				}
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
		)
	}

	to := state.to
	if state.waiting {
		to = time.Time{}
	}
	rows = append(rows,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return CalendarProps{
				Today: p.Today, Min: p.Min, Max: p.Max, Monday: p.Monday,
				Range:      [2]time.Time{state.from, to},
				MonthNames: p.MonthNames, WeekdayNames: p.WeekdayNames,
			}.Layout(c.With(gtx), &state.month)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if !state.waiting {
						return layout.Dimensions{}
					}
					return drawText(c.With(gtx), orDefault(p.PendingLabel, "Choose the end date"), unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min = image.Point{}
					return ButtonProps{Label: orDefault(p.ClearLabel, "Clear"), Variant: Ghost, Size: Small}.Layout(inner, &state.clear)
				}),
			)
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
}
