package widget

import (
	"image"
	"strconv"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Calendar is the state half of a grid of days: which month is shown, what is
// chosen, and the press on each day.
type Calendar struct {
	shown    time.Time
	selected time.Time
	previous Button
	next     Button
	days     [42]Button
}

// Selected is the day chosen, and the zero time when none is.
func (c *Calendar) Selected() time.Time { return c.selected }

// Select chooses a day and shows its month.
func (c *Calendar) Select(day time.Time) {
	c.selected = day
	c.shown = day
}

// Show moves to a month without choosing anything in it.
func (c *Calendar) Show(month time.Time) { c.shown = month }

// Showing is the month on screen.
//
// It defaults to the month of whatever is selected, and then to the month the
// caller passed as today -- never to the machine's own clock, which a test
// cannot pin and which is a different day in two timezones.
func (c *Calendar) Showing(today time.Time) time.Time {
	switch {
	case !c.shown.IsZero():
		return c.shown
	case !c.selected.IsZero():
		return c.selected
	}
	return today
}

// CalendarProps is a month of days.
//
// Today is passed in rather than read from the clock, and that is the whole
// difference between a control a test can pin and one that draws a different
// picture tomorrow. It is also correct: what "today" is depends on where
// somebody is, and a control has no business deciding that.
type CalendarProps struct {
	// Today is the day to mark as the current one. The zero time marks none.
	Today time.Time
	// Min and Max bound what can be chosen. Zero times leave that end open.
	Min, Max time.Time
	// Monday starts the week on Monday rather than Sunday.
	Monday bool
}

// Layout draws the month and returns the room it took.
func (p CalendarProps) Layout(c ayra.Context, state *Calendar) ayra.Dimensions {
	month := state.Showing(p.Today)
	if month.IsZero() {
		month = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	if state.previous.Clicked(c) {
		state.shown = month.AddDate(0, -1, 0)
		month = state.shown
	}
	if state.next.Clicked(c) {
		state.shown = month.AddDate(0, 1, 0)
		month = state.shown
	}

	days := p.grid(month)
	for index, day := range days {
		if day.IsZero() || !p.allowed(day) {
			continue
		}
		if state.days[index].Clicked(c) {
			state.selected = day
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.header(c.With(gtx), state, month)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.weekdays(c.With(gtx))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.weeks(c.With(gtx), state, days)
		}),
	)
}

// header draws the month's name between the two steps.
func (p CalendarProps) header(c ayra.Context, state *Calendar, month time.Time) ayra.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return ButtonProps{Label: "<", Variant: Ghost, Size: Small}.Layout(inner, &state.previous)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := month.Format("January 2006")
			return drawText(c.With(gtx), label, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Middle, semibold())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return ButtonProps{Label: ">", Variant: Ghost, Size: Small}.Layout(inner, &state.next)
		}),
	)
}

// weekdays draws the seven initials above the grid.
func (p CalendarProps) weekdays(c ayra.Context) ayra.Dimensions {
	children := make([]layout.FlexChild, 0, 7)
	for _, name := range p.weekdayNames() {
		name := name
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), name, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Middle, plain())
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context, children...)
}

// weekdayNames answers the seven initials, starting on the right day.
func (p CalendarProps) weekdayNames() []string {
	names := []string{"S", "M", "T", "W", "T", "F", "S"}
	if p.Monday {
		return append(names[1:], names[0])
	}
	return names
}

// weeks draws the six rows of days.
func (p CalendarProps) weeks(c ayra.Context, state *Calendar, days [42]time.Time) ayra.Dimensions {
	rows := make([]layout.FlexChild, 0, 6)

	for week := 0; week < 6; week++ {
		week := week
		if days[week*7].IsZero() && days[week*7+6].IsZero() {
			// A month that fits in five rows draws five. The sixth, left
			// empty, is a block of white the eye reads as the grid being
			// broken.
			continue
		}

		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cells := make([]layout.FlexChild, 0, 7)
			for index := week * 7; index < week*7+7; index++ {
				index := index
				cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return p.day(c.With(gtx), state, days[index], index)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
}

// day draws one cell.
func (p CalendarProps) day(c ayra.Context, state *Calendar, day time.Time, index int) ayra.Dimensions {
	if day.IsZero() {
		// The days before the first and after the last are blank rather than
		// showing the neighbouring month's numbers: a grid with two months of
		// numbers in it is one where somebody picks the wrong thirty-first.
		return ayra.Dimensions{Size: image.Pt(0, c.Dp(unit.Dp(36)))}
	}

	allowed := p.allowed(day)
	selected := sameDay(day, state.selected)
	today := sameDay(day, p.Today)

	variant := Ghost
	switch {
	case selected:
		variant = Default
	case today:
		variant = Outline
	}

	c.Constraints.Min.Y = c.Dp(unit.Dp(36))
	return ButtonProps{
		Label:    strconv.Itoa(day.Day()),
		Variant:  variant,
		Size:     Small,
		Disabled: !allowed,
	}.Layout(c, &state.days[index])
}

// allowed reports whether a day is inside the bounds.
//
// Every one of the three times is taken down to its own midnight first. A
// bound is given with an hour on it whenever a caller passes the current time,
// which is the ordinary case, and comparing that against an hour of the day
// being tested answers by the clock rather than by the date: with a maximum of
// this morning, this afternoon is out of range.
func (p CalendarProps) allowed(day time.Time) bool {
	at := startOfDay(day)
	if !p.Min.IsZero() && at.Before(startOfDay(p.Min)) {
		return false
	}
	if !p.Max.IsZero() && at.After(startOfDay(p.Max)) {
		return false
	}
	return true
}

// grid answers the six weeks of a month, with zero times where the month has
// no day.
func (p CalendarProps) grid(month time.Time) [42]time.Time {
	var days [42]time.Time

	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	offset := int(first.Weekday())
	if p.Monday {
		// Monday-first shifts every day back one and wraps Sunday to the end.
		offset = (offset + 6) % 7
	}

	last := first.AddDate(0, 1, -1).Day()
	for day := 1; day <= last; day++ {
		at := offset + day - 1
		if at >= len(days) {
			break
		}
		days[at] = first.AddDate(0, 0, day-1)
	}
	return days
}

// sameDay reports whether two times fall on the same day.
func sameDay(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// startOfDay answers midnight of a time's own day, so a bound given with an
// hour on it does not exclude the day it names.
func startOfDay(at time.Time) time.Time {
	year, month, day := at.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, at.Location())
}
