package widget

import (
	"testing"
	"time"

	"github.com/arandu-io/ayra/theme"
)

// press is what a press on a day leaves in the calendar, which is the only way
// to drive it without a pointer.
func (d *DateRange) press(day time.Time) { d.month.pressed = day }

func drawRange(t *testing.T, props DateRangePickerProps, state *DateRange) {
	t.Helper()
	c, _ := field(t, theme.Light, 400)
	props.Layout(c, state)
}

func TestThePeriodIsTheFirstDayAndTheSecond(t *testing.T) {
	var state DateRange
	props := DateRangePickerProps{Today: pickerDay}

	drawRange(t, props, &state)
	state.popover.open.Open()
	drawRange(t, props, &state)

	state.press(pickerDay.AddDate(0, 0, 5))
	drawRange(t, props, &state)

	if from, to := state.Range(); !from.IsZero() || !to.IsZero() {
		t.Errorf("a period with only its start reported %v to %v; a period is both ends or neither", from, to)
	}
	if state.Changed() {
		t.Error("the first press announced a change")
	}
	if !state.Showing() {
		t.Error("the panel closed on the first press, with the end still to choose")
	}

	// The second day is before the first: the period is the same either way.
	state.press(pickerDay.AddDate(0, 0, 2))
	drawRange(t, props, &state)

	from, to := state.Range()
	if !sameDay(from, pickerDay.AddDate(0, 0, 2)) || !sameDay(to, pickerDay.AddDate(0, 0, 5)) {
		t.Errorf("the period is %v to %v, want the 12th to the 15th", from, to)
	}
	if !state.Changed() || state.Changed() {
		t.Error("a completed period must be announced once")
	}
	if state.Showing() {
		t.Error("the panel stayed open over a finished period")
	}
}

func TestAPresetPicksBothEnds(t *testing.T) {
	var state DateRange
	props := DateRangePickerProps{Today: pickerDay, Presets: []DatePreset{LastDays("7 days", pickerDay, 7)}}

	drawRange(t, props, &state)
	state.popover.open.Open()
	drawRange(t, props, &state)

	// A press on the preset's own button is the one thing driven by the
	// engine's clickable; what is asserted is what the preset says.
	preset := props.Presets[0]
	state.Set(preset.From, preset.To)

	from, to := state.Range()
	if !sameDay(from, pickerDay.AddDate(0, 0, -6)) || !sameDay(to, pickerDay) {
		t.Errorf("the last seven days are %v to %v", from, to)
	}
}

func TestThePresets(t *testing.T) {
	today := time.Date(2026, time.September, 10, 15, 30, 0, 0, time.UTC)

	if got := LastDays("today", today, 1); !sameDay(got.From, today) || !sameDay(got.To, today) {
		t.Errorf("today is %v to %v", got.From, got.To)
	}
	month := ThisMonth("month", today)
	if month.From.Day() != 1 || month.To.Day() != 30 || month.To.Month() != time.September {
		t.Errorf("this month is %v to %v", month.From, month.To)
	}
	previous := PreviousMonth("last month", today)
	if previous.From.Month() != time.August || previous.To.Day() != 31 {
		t.Errorf("the previous month is %v to %v", previous.From, previous.To)
	}
}

func TestADateAndATimeAreOneValue(t *testing.T) {
	var state DateTime
	props := DateTimePickerProps{Today: pickerDay}

	c, _ := field(t, theme.Light, 400)
	props.Layout(c, &state)
	state.popover.open.Open()

	state.month.Select(pickerDay)
	state.clock.SetValue(nil, "1430")
	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)

	got := state.Selected(false)
	if want := time.Date(2026, time.September, 10, 14, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("the value is %v, want %v", got, want)
	}
	if !state.Changed() {
		t.Error("choosing a moment was not reported")
	}
	if !state.Showing() {
		t.Error("the panel closed while the time was being typed")
	}
}

func TestATimeThatIsNotATimeDoesNotMoveTheValue(t *testing.T) {
	for _, text := range []string{"", "25:00", "12:61", "9:30", "12", "ab:cd"} {
		if _, _, _, valid := parseClock(text, false); valid {
			t.Errorf("%q was read as a time", text)
		}
	}
	if h, m, _, valid := parseClock("23:59", false); !valid || h != 23 || m != 59 {
		t.Error("23:59 was not read")
	}
	if _, _, s, valid := parseClock("10:20:30", true); !valid || s != 30 {
		t.Error("10:20:30 was not read with seconds")
	}
}

func TestTheCalendarSpeaksTheApplicationsLanguage(t *testing.T) {
	var state Calendar
	c, _ := field(t, theme.Light, 400)
	english := CalendarProps{Today: pickerDay}.Layout(c, &state).Size

	names := []string{"janeiro", "fevereiro", "marco", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}
	c, _ = field(t, theme.Light, 400)
	portuguese := CalendarProps{Today: pickerDay, MonthNames: names, WeekdayNames: []string{"D", "S", "T", "Q", "Q", "S", "S"}}.Layout(c, &state).Size

	if english.X <= 0 || portuguese.X <= 0 {
		t.Errorf("the calendar took %v and %v", english, portuguese)
	}
}

func TestTheKindsOfDateTimeEachHoldTheirOwnValue(t *testing.T) {
	var state DateTime
	c, _ := field(t, theme.Light, 400)
	state.month.Select(pickerDay.AddDate(0, 0, 3)) // Sunday the 13th
	state.clock.SetValue(clockMask(false), "0930")
	state.chosenMonth, state.chosenYear = time.October, 2026

	for _, c := range []struct {
		kind DateTimeKind
		text string
		want time.Time
	}{
		{DateAndTime, "2026-09-13 09:30", time.Date(2026, 9, 13, 9, 30, 0, 0, time.UTC)},
		{DateOnly, "2026-09-13", time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)},
		{TimeOnly, "09:30", time.Date(1, 1, 1, 9, 30, 0, 0, time.UTC)},
		{MonthOnly, "2026-10", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		// The 13th is a Sunday, which belongs to the week that began on the 7th.
		{WeekOnly, "2026-W37", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
	} {
		props := DateTimePickerProps{Kind: c.kind, Today: pickerDay}
		if got := state.Value(c.kind, false); !got.Equal(c.want) {
			t.Errorf("kind %d holds %v, want %v", c.kind, got, c.want)
		}
		if got := props.Text(&state); got != c.text {
			t.Errorf("kind %d writes %q, want %q", c.kind, got, c.text)
		}
		ctx, _ := field(t, theme.Light, 400)
		if dims := props.Layout(ctx, &state); dims.Size.X <= 0 {
			t.Errorf("kind %d drew nothing", c.kind)
		}
	}
	_ = c
}

// TestAPeriodWithOneEndIsOpenOnTheOtherSide is "Até 31/12/2026": a period set
// with only its last day. Set took the missing end for a day before the start
// and swapped the two, so the period came back as nothing to 31 December, and
// the field -- which drew a period only when it had a start -- said "choose a
// period".
func TestAPeriodWithOneEndIsOpenOnTheOtherSide(t *testing.T) {
	last := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)

	var until DateRange
	until.Set(time.Time{}, last)
	if from, to := until.Range(); !from.IsZero() || !to.Equal(last) {
		t.Errorf("a period until %v answered %v to %v", last, from, to)
	}

	var since DateRange
	since.Set(last, time.Time{})
	if from, to := since.Range(); !from.Equal(last) || !to.IsZero() {
		t.Errorf("a period from %v answered %v to %v", last, from, to)
	}

	var both DateRange
	both.Set(last, last.AddDate(0, 0, -10))
	if from, to := both.Range(); !from.Before(to) {
		t.Errorf("a period given backwards was not put right: %v to %v", from, to)
	}
}

func TestAPeriodWithOneEndIsWrittenWithItsWord(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	c.Theme.Locale = theme.PortugueseBrazil()
	layout := "02/01/2006"
	first := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		from, to time.Time
		want     string
	}{
		{time.Time{}, last, "Até 31/12/2026"},
		{first, time.Time{}, "A partir de 01/01/2026"},
		{first, last, "01/01/2026 – 31/12/2026"},
	} {
		if got := periodText(c, tc.from, tc.to, layout); got != tc.want {
			t.Errorf("the period %v to %v is written %q, want %q", tc.from, tc.to, got, tc.want)
		}
	}

	english, _ := field(t, theme.Light, 400)
	if got := periodText(english, time.Time{}, last, layout); got != "Until 31/12/2026" {
		t.Errorf("with no locale the period is written %q", got)
	}
}

// TestTheDaysOnTheOpenSideAreInsideThePeriod is what the calendar shows of
// "until 31 December": every day before it, as a band. While the end of a period
// is still being chosen, the start stands alone and nothing around it is marked.
func TestTheDaysOnTheOpenSideAreInsideThePeriod(t *testing.T) {
	last := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	early := time.Date(2026, time.December, 3, 0, 0, 0, 0, time.UTC)

	until := CalendarProps{Range: [2]time.Time{{}, last}, open: true}
	if !until.within(early) {
		t.Error("a day before the end of an open period is not marked as inside it")
	}
	if until.within(last.AddDate(0, 0, 1)) {
		t.Error("a day after the end of the period is marked as inside it")
	}

	since := CalendarProps{Range: [2]time.Time{early, {}}, open: true}
	if !since.within(last) {
		t.Error("a day after the start of an open period is not marked as inside it")
	}

	choosing := CalendarProps{Range: [2]time.Time{early, {}}}
	if choosing.within(last) {
		t.Error("the days after a start that is still waiting for its end are marked")
	}
}
