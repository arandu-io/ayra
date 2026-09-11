package widget_test

import (
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// september is the month these tests work in, and it is written down rather
// than read from the clock so that they draw the same picture next year.
var september = time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)

// TestSelectDoesNotReadAsAChoiceByTheUser keeps a form that fills a field in
// from firing whatever a choice fires -- a save, a request, a step forward.
func TestSelectDoesNotReadAsAChoiceByTheUser(t *testing.T) {
	var state widget.DatePicker
	state.Select(september)

	c, _ := frame(t, theme.Light, 400)
	widget.DatePickerProps{Today: september}.Layout(c, &state)

	if state.Changed() {
		t.Error("filling the field in reported a choice")
	}
}

// TestTheFieldTakesRoomWithNothingChosen keeps the closed control from
// collapsing to nothing before a day is picked, which reads as a missing field
// rather than an empty one.
func TestTheFieldTakesRoomWithNothingChosen(t *testing.T) {
	var state widget.DatePicker

	c, _ := frame(t, theme.Light, 400)
	dims := widget.DatePickerProps{Today: september}.Layout(c, &state)

	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("an empty date field took %v", dims.Size)
	}
}

// TestTheDayIsWrittenInAnOrderNobodyReadsWrongly is the one formatting choice
// this control makes for the caller, and it is worth a test because the two
// common local orders put the day and the month in opposite places: the same
// eight characters name two different days to two different readers.
func TestTheDayIsWrittenInAnOrderNobodyReadsWrongly(t *testing.T) {
	written := september.Format(widget.DateFormat)

	if written != "2026-09-10" {
		t.Errorf("a day is written %q", written)
	}
	if parts := strings.Split(written, "-"); len(parts) != 3 || len(parts[0]) != 4 {
		t.Errorf("the year does not lead, so the order is one somebody reads wrongly: %q", written)
	}
}
