package widget

import (
	"testing"
	"time"

	"github.com/arandu-io/ayra/theme"
)

// pickerDay is the day these tests work in, written down rather than read from
// the clock so that they draw the same picture next year.
var pickerDay = time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)

// pressDay is what a press inside the month leaves behind: the calendar knows a
// day was chosen and nothing above it does.
//
// It reaches past the public surface deliberately. Select is the other way in,
// and it is the one that must *not* read as a choice by the person at the
// screen -- so driving these tests through it would prove the opposite of what
// they are for.
func pressDay(state *DatePicker, day time.Time) { state.month.Select(day) }

// TestChoosingADayClosesTheMonth fixes what a panel that stays open costs: the
// calendar reports a press to its own state and not upwards, so a picker that
// did not watch for the change would need a second press somewhere else before
// the form underneath could be read again.
func TestChoosingADayClosesTheMonth(t *testing.T) {
	var state DatePicker
	props := DatePickerProps{Today: pickerDay}

	c, _ := field(t, theme.Light, 400)
	props.Layout(c, &state)
	state.popover.open.Open()

	pressDay(&state, pickerDay.AddDate(0, 0, 4))

	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)

	if state.Showing() {
		t.Error("the month is still open over the form")
	}
}

// TestAChoiceIsReportedOnceAndConsumed keeps one press from being acted on
// every frame for as long as the day stays chosen.
func TestAChoiceIsReportedOnceAndConsumed(t *testing.T) {
	var state DatePicker
	props := DatePickerProps{Today: pickerDay}

	c, _ := field(t, theme.Light, 400)
	props.Layout(c, &state)

	pressDay(&state, pickerDay)

	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)

	if !state.Changed() {
		t.Fatal("the choice was not reported")
	}
	if state.Changed() {
		t.Error("the choice was reported twice, and whatever it fires fires twice")
	}

	// A further frame with the same day chosen reports nothing.
	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)
	if state.Changed() {
		t.Error("a day that was already chosen was reported as chosen again")
	}
}

// TestADisabledPickerCannotBeLeftOpen fixes a month of days that do nothing:
// the panel is drawn over the form, and every press in it is refused.
func TestADisabledPickerCannotBeLeftOpen(t *testing.T) {
	var state DatePicker

	c, _ := field(t, theme.Light, 400)
	DatePickerProps{Today: pickerDay}.Layout(c, &state)
	state.popover.open.Open()

	c, _ = field(t, theme.Light, 400)
	DatePickerProps{Today: pickerDay, Disabled: true}.Layout(c, &state)

	if state.Showing() {
		t.Error("a disabled field is showing a month")
	}
}
