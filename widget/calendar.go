package widget

import (
	"image"
	"strconv"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// dayCell is the height of a day in the grid, in dp: the least a finger can be
// asked to hit, which is also what a row of it has to be to stay one row.
const dayCell = 44

// Calendar is the state half of a grid of days: which month is shown, what is
// chosen, and the press on each day.
type Calendar struct {
	shown    time.Time
	selected time.Time
	previous Button
	next     Button
	days     [42]Button
	today    Button
	clear    Button
	// pressed is the day the last press landed on, held until it is read. The
	// selection moves with it, but a press on the day that is already selected
	// moves nothing -- and a period is chosen by pressing the same day twice.
	pressed time.Time
}

// Pressed reports the day a press landed on since the last frame, once, and the
// zero time when there was none.
func (c *Calendar) Pressed() time.Time {
	day := c.pressed
	c.pressed = time.Time{}
	return day
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
//
// The consequence is that a caller must supply one of Today, a day chosen on
// the state, or a month shown on it. With none of the three there is no month
// to draw and nothing is drawn, rather than a month invented here: an invented
// one is a working-looking grid of the wrong year.
type CalendarProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Today is the day to mark as the current one, and the month to open on
	// when nothing has been chosen. The zero time marks none, and with nothing
	// chosen or shown either the control draws nothing at all.
	Today time.Time
	// Min and Max bound what can be chosen. Zero times leave that end open.
	Min, Max time.Time
	// Monday starts the week on Monday rather than Sunday.
	Monday bool
	// Range marks the days from the first to the second, ends included, as a
	// period. A zero end leaves the period open: the start alone is marked as
	// the one day it is.
	Range [2]time.Time
	// MonthNames and WeekdayNames are what the header and the column heads say,
	// in the language of the application: twelve names starting at January, and
	// seven starting at Sunday. They are data because a calendar is where a
	// locale is least negotiable, and a library that guessed would draw the
	// wrong one in every country but its own. Empty draws English.
	MonthNames   []string
	WeekdayNames []string
	// PreviousLabel and NextLabel are what the two steps say. Empty says the
	// arrows.
	PreviousLabel, NextLabel string
	// TodayLabel and ClearLabel are the two controls under the grid: one chooses
	// Today, the other empties the choice. Empty draws neither.
	TodayLabel, ClearLabel string

	// owned is the component that draws this calendar as a part of itself, by
	// the name of its Kyse pair. Kyse's date-range-picker lays out two arrows of
	// its own over a month, and the arrows are then that component's.
	owned string

	// bare says the calendar sits in a widget that has its own controls under
	// it, as the date range does, and draws none.
	bare bool

	// open says a zero end of Range is no limit and not a day still to be
	// chosen: the days on that side of the other end are inside the period.
	open bool
}

// localised fills what the caller left empty from the application's locale, in
// the theme: the names, the first day of the week and the words on the
// controls. What the caller set is kept, so a locale is a default and never an
// override; with no locale the calendar is the English it always was.
func (p CalendarProps) localised(c ayra.Context) CalendarProps {
	locale := c.Theme.Locale
	if len(p.MonthNames) != 12 {
		p.MonthNames = locale.MonthNames()
	}
	if len(p.WeekdayNames) != 7 {
		p.WeekdayNames = locale.WeekdayNames()
	}
	p.Monday = p.Monday || locale.Monday
	p.PreviousLabel = orDefault(p.PreviousLabel, locale.Previous)
	p.NextLabel = orDefault(p.NextLabel, locale.Next)
	if !p.bare {
		p.TodayLabel = orDefault(p.TodayLabel, locale.Today)
		p.ClearLabel = orDefault(p.ClearLabel, locale.Clear)
	}
	return p
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p CalendarProps) PartNames() []string { return []string{"root"} }

// owner is the component whose icons the month's two arrows are.
func (p CalendarProps) owner() string { return orDefault(p.owned, "calendar") }

// Layout draws the month and returns the room it took.
func (p CalendarProps) Layout(c ayra.Context, state *Calendar) ayra.Dimensions {
	return withRoot(c, "calendar", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p CalendarProps) layout(c ayra.Context, state *Calendar) ayra.Dimensions {
	p = p.localised(c)
	month := state.Showing(p.Today)
	if month.IsZero() {
		// Nothing chosen, nothing shown, and no day passed in. Drawing a fixed
		// month here was what this did, and it drew the same January every time
		// -- a grid that looks like a working calendar and is one year in the
		// past by the time anybody sees it. A blank space says what is true:
		// this control was given no day to work from.
		return ayra.Dimensions{}
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
			state.pressed = day
		}
	}

	rows := []layout.FlexChild{
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
	}
	if p.TodayLabel != "" || p.ClearLabel != "" {
		rows = append(rows,
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.controls(c.With(gtx), state)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
}

// controls draws the two buttons under the grid, the clear one on the left and
// the today one on the right, as Kyse's calendar has them.
func (p CalendarProps) controls(c ayra.Context, state *Calendar) ayra.Dimensions {
	if p.ClearLabel != "" && state.clear.Clicked(c) {
		state.selected = time.Time{}
		state.pressed = time.Time{}
	}
	if p.TodayLabel != "" && state.today.Clicked(c) && !p.Today.IsZero() && p.allowed(p.Today) {
		state.selected = p.Today
		state.shown = p.Today
		state.pressed = p.Today
	}

	button := func(label string, state *Button, disabled bool) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if label == "" {
				return layout.Dimensions{}
			}
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return ButtonProps{Label: label, Variant: Ghost, Size: Small, Disabled: disabled}.Layout(inner, state)
		})
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
		button(p.ClearLabel, &state.clear, false),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
		}),
		button(p.TodayLabel, &state.today, p.Today.IsZero() || !p.allowed(p.Today)),
	)
}

// header draws the month's name between the two steps.
func (p CalendarProps) header(c ayra.Context, state *Calendar, month time.Time) ayra.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return innerButton(c.With(gtx), &state.previous, innerIcon(p.owner(), "CaretLeft"), orDefault(p.PreviousLabel, "Previous month"), Ghost, false)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := month.Format("January 2006")
			if len(p.MonthNames) == 12 {
				label = p.MonthNames[month.Month()-1] + " " + strconv.Itoa(month.Year())
			}
			return drawText(c.With(gtx), label, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Middle, semibold())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return innerButton(c.With(gtx), &state.next, innerIcon(p.owner(), "CaretRight"), orDefault(p.NextLabel, "Next month"), Ghost, false)
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
	if len(p.WeekdayNames) == 7 {
		names = append([]string(nil), p.WeekdayNames...)
	}
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
		//
		// It still takes the width it was given. A cell of the grid is told how
		// wide it is -- the row hands each one an equal share as both the least
		// and the most it may take -- and the row then advances by whatever
		// each cell reports. A blank that reported nothing packed the month
		// leftwards by one column for every day before the first, so a month
		// beginning on a Tuesday drew its first day under Monday. Nothing
		// failed: the numbers were right, the grid was square, and it was the
		// wrong weekday.
		return ayra.Dimensions{Size: image.Pt(c.Constraints.Min.X, c.Dp(unit.Dp(dayCell)))}
	}

	allowed := p.allowed(day)
	selected := sameDay(day, state.selected)
	today := sameDay(day, p.Today)

	variant := Ghost
	switch {
	case selected, p.edge(day):
		variant = Default
	case p.within(day):
		variant = Secondary
	case today:
		variant = Outline
	}

	props := ButtonProps{
		Label:    strconv.Itoa(day.Day()),
		Variant:  variant,
		Size:     Small,
		Disabled: !allowed,
	}

	if variant == Secondary {
		// The inside of a period is one band, as Kyse draws it, and not a tile for
		// each day: the cells touch, so a square corner on each is what makes them
		// join, and the ends of the period keep theirs rounded.
		props.ComponentProps = ComponentProps{Parts: theme.Parts{"root": {Radius: theme.Pt(0)}}}
	}

	// The cell is as tall as a finger needs, 44 dp, padding included: the
	// button's own room above and below the number is part of it, and what is
	// asked of the label is the rest.
	padding := props.style(c.Theme).Padding
	c.Constraints.Min.Y = max(c.Dp(unit.Dp(dayCell))-c.Dp(unit.Dp(padding.Top+padding.Bottom)), 0)
	return props.Layout(c, &state.days[index])
}

// edge reports whether a day is one end of the marked period.
func (p CalendarProps) edge(day time.Time) bool {
	return sameDay(day, p.Range[0]) || sameDay(day, p.Range[1])
}

// within reports whether a day is inside the marked period, ends excluded.
func (p CalendarProps) within(day time.Time) bool {
	from, to := p.Range[0], p.Range[1]
	at := startOfDay(day)
	if p.open {
		switch {
		case from.IsZero() && !to.IsZero():
			return at.Before(startOfDay(to))
		case to.IsZero() && !from.IsZero():
			return at.After(startOfDay(from))
		}
	}
	if from.IsZero() || to.IsZero() {
		return false
	}
	return at.After(startOfDay(from)) && at.Before(startOfDay(to))
}

// orDefault is the text, or the fallback when it is empty.
func orDefault(text, fallback string) string {
	if text == "" {
		return fallback
	}
	return text
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
