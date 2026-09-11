package widget

import (
	"testing"
	"time"

	"github.com/arandu-io/ayra/theme"
)

// TestTheGridPutsTheFirstOfTheMonthOnItsOwnWeekday keeps the offset from being
// applied to the wrong end of the row.
func TestTheGridPutsTheFirstOfTheMonthOnItsOwnWeekday(t *testing.T) {
	// September 2026 opens on a Tuesday.
	september := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)

	sunday := CalendarProps{}.grid(september)
	if got := sunday[2]; got.Day() != 1 {
		t.Errorf("with the week starting on Sunday the first lands at index 2, and index 2 holds %v", got)
	}
	for index := 0; index < 2; index++ {
		if !sunday[index].IsZero() {
			t.Errorf("index %d should be blank, and holds %v", index, sunday[index])
		}
	}

	monday := CalendarProps{Monday: true}.grid(september)
	if got := monday[1]; got.Day() != 1 {
		t.Errorf("with the week starting on Monday the first lands at index 1, and index 1 holds %v", got)
	}
}

// TestSundayIsTheLastDayOfAWeekThatStartsOnMonday is the one weekday where the
// rotation and a plain subtraction disagree.
//
// For the other six, "one place earlier" and "rotate by six" answer the same
// number, so a grid built by subtracting reads correctly all week and then puts
// the first of a Sunday month before the start of the row.
func TestSundayIsTheLastDayOfAWeekThatStartsOnMonday(t *testing.T) {
	// February 2026 opens on a Sunday.
	february := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)

	days := CalendarProps{Monday: true}.grid(february)
	if got := days[6]; got.IsZero() || got.Day() != 1 {
		t.Errorf("with the week starting on Monday a Sunday first lands at index 6, and index 6 holds %v", got)
	}
	for index := 0; index < 6; index++ {
		if !days[index].IsZero() {
			t.Errorf("index %d should be blank, and holds %v", index, days[index])
		}
	}
}

// TestAMonthOpeningOnTheFirstDayOfTheWeekHasNoBlanks fixes the off-by-one that
// a modulo written the other way round produces.
func TestAMonthOpeningOnTheFirstDayOfTheWeekHasNoBlanks(t *testing.T) {
	// February 2026 opens on a Sunday.
	february := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)

	if first := (CalendarProps{}.grid(february))[0]; first.IsZero() || first.Day() != 1 {
		t.Errorf("a month opening on Sunday starts the Sunday-first grid at index 0, and index 0 holds %v", first)
	}

	// June 2026 opens on a Monday.
	june := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	if first := (CalendarProps{Monday: true}.grid(june))[0]; first.IsZero() || first.Day() != 1 {
		t.Errorf("a month opening on Monday starts the Monday-first grid at index 0, and index 0 holds %v", first)
	}
}

// TestTheGridHoldsEveryDayOfTheMonthAndNoOther keeps a neighbouring month's
// numbers out of the grid, which is where a wrong thirty-first is chosen.
func TestTheGridHoldsEveryDayOfTheMonthAndNoOther(t *testing.T) {
	for _, month := range []time.Time{
		time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.February, 10, 0, 0, 0, 0, time.UTC), // a leap year
		time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
	} {
		days := CalendarProps{}.grid(month)

		seen := map[int]bool{}
		for _, day := range days {
			if day.IsZero() {
				continue
			}
			if day.Month() != month.Month() {
				t.Errorf("%v carries a day of %v", month.Month(), day.Month())
			}
			seen[day.Day()] = true
		}

		last := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location()).AddDate(0, 1, -1).Day()
		if len(seen) != last {
			t.Errorf("%v %d has %d days and the grid holds %d", month.Month(), month.Year(), last, len(seen))
		}
	}
}

// TestTheWeekdayInitialsRotateRatherThanShift keeps Sunday from being dropped
// when the week starts on Monday.
func TestTheWeekdayInitialsRotateRatherThanShift(t *testing.T) {
	sunday := CalendarProps{}.weekdayNames()
	monday := CalendarProps{Monday: true}.weekdayNames()

	if len(monday) != 7 {
		t.Fatalf("a week has seven days and the row has %d", len(monday))
	}
	if monday[0] != sunday[1] || monday[6] != sunday[0] {
		t.Errorf("the row was shifted rather than rotated: %v", monday)
	}
}

// TestABoundWithAnHourOnItDoesNotExcludeItsOwnDay fixes the comparison that a
// caller passing time.Now() as the minimum would otherwise hit: every hour of
// today is before this afternoon.
func TestABoundWithAnHourOnItDoesNotExcludeItsOwnDay(t *testing.T) {
	afternoon := time.Date(2026, time.September, 10, 15, 30, 0, 0, time.UTC)
	morning := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)

	if !(CalendarProps{Min: afternoon}).allowed(morning) {
		t.Error("the day of the minimum was refused")
	}
	if !(CalendarProps{Max: morning}).allowed(afternoon) {
		t.Error("the day of the maximum was refused")
	}
	if (CalendarProps{Min: afternoon}).allowed(morning.AddDate(0, 0, -1)) {
		t.Error("the day before the minimum was allowed")
	}
}

// TestTheMonthShownDoesNotComeFromTheClock keeps the control drawing the same
// picture tomorrow, and in another timezone.
func TestTheMonthShownDoesNotComeFromTheClock(t *testing.T) {
	today := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)

	var state Calendar
	if got := state.Showing(today); !got.Equal(today) {
		t.Errorf("with nothing chosen the month is the one passed in, and it is %v", got)
	}

	chosen := time.Date(2025, time.March, 3, 0, 0, 0, 0, time.UTC)
	state.Select(chosen)
	if got := state.Showing(today); got.Month() != time.March {
		t.Errorf("choosing a day shows its month, and the month shown is %v", got.Month())
	}

	state.Show(time.Date(2027, time.July, 1, 0, 0, 0, 0, time.UTC))
	if got := state.Showing(today); got.Month() != time.July {
		t.Errorf("moving the month shows it, and the month shown is %v", got.Month())
	}
	if !state.Selected().Equal(chosen) {
		t.Error("moving the month unchose the day")
	}
}

// TestTwoTimesOnTheSameDayAreTheSameDay keeps the chosen day's highlight from
// depending on the hour it was chosen at.
func TestTwoTimesOnTheSameDayAreTheSameDay(t *testing.T) {
	morning := time.Date(2026, time.September, 10, 1, 0, 0, 0, time.UTC)
	evening := time.Date(2026, time.September, 10, 23, 0, 0, 0, time.UTC)

	if !sameDay(morning, evening) {
		t.Error("two hours of one day were read as two days")
	}
	if sameDay(morning, morning.AddDate(1, 0, 0)) {
		t.Error("the same day of two years was read as one day")
	}
	if sameDay(time.Time{}, time.Time{}) {
		t.Error("nothing chosen matched nothing chosen, which highlights every blank cell")
	}
}

// TestABlankCellTakesTheWidthItWasGiven fixes the fault that put the first of
// September under Monday.
//
// Each cell of a week is handed an equal share of the row as both the least and
// the most it may take, and the row advances by what the cell reports. A blank
// that reported nothing pulled every day after it one column to the left, so a
// month drew against the wrong weekday with every number right.
func TestABlankCellTakesTheWidthItWasGiven(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	c.Constraints.Min.X = 46
	c.Constraints.Max.X = 46

	var state Calendar
	blank := CalendarProps{}.day(c, &state, time.Time{}, 0)

	if blank.Size.X != 46 {
		t.Errorf("a blank cell took %d of the 46 it was given, which shifts the month left by a column", blank.Size.X)
	}
	if blank.Size.Y <= 0 {
		t.Errorf("a blank cell took %d down, so its row is shorter than the ones beside it", blank.Size.Y)
	}
}

// TestAMonthStartingMidWeekFillsTheWholeFirstRow is the same fault read from
// the row rather than from the cell.
func TestAMonthStartingMidWeekFillsTheWholeFirstRow(t *testing.T) {
	// September 2026 opens on a Tuesday, so a Monday-first grid has one blank
	// before it; February 2026 opens on a Sunday, which is six.
	for _, month := range []time.Time{
		time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
	} {
		var state Calendar
		props := CalendarProps{Today: month, Monday: true}
		days := props.grid(month)

		c, _ := field(t, theme.Light, 350)
		c.Constraints.Min.X = 350
		row := props.weeks(c, &state, days)

		if row.Size.X != 350 {
			t.Errorf("%v laid its weeks out %d wide of 350, so the days are packed to one side", month.Month(), row.Size.X)
		}
	}
}
