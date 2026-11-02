package widget

import (
	"testing"
	"time"

	"github.com/arandu-io/ayra/theme"
)

// TestADayAndTimeAreWrittenInTheOrderOfTheLocale is the field of a Brazilian
// application reading 2026-11-02 09:30: the day in the ISO order whatever the
// theme said, which half the people looking at it read as the wrong month.
func TestADayAndTimeAreWrittenInTheOrderOfTheLocale(t *testing.T) {
	moment := time.Date(2026, time.November, 2, 9, 30, 0, 0, time.UTC)

	var state DateTime
	state.Select(moment, false)

	props := DateTimePickerProps{Today: moment}
	if got, want := props.text(theme.Locale{}, &state), "2026-11-02 09:30"; got != want {
		t.Errorf("with no locale the field says %q, want %q", got, want)
	}
	if got, want := props.text(theme.PortugueseBrazil(), &state), "02/11/2026 09:30"; got != want {
		t.Errorf("under pt-BR the field says %q, want %q", got, want)
	}

	date := DateTimePickerProps{Today: moment, Kind: DateOnly}
	if got, want := date.text(theme.PortugueseBrazil(), &state), "02/11/2026"; got != want {
		t.Errorf("under pt-BR a day alone is written %q, want %q", got, want)
	}

	given := DateTimePickerProps{Today: moment, Format: "2006/01/02 15h04"}
	if got, want := given.text(theme.PortugueseBrazil(), &state), "2026/11/02 09h30"; got != want {
		t.Errorf("the layout the caller gave was overridden by the locale: %q", got)
	}
}

// TestAChosenTimeIsOnTheFieldBeforeThePanelHasBeenOpened fixes a closed picker
// that arrived with 09:30 chosen and said 00:00: Select wrote the digits 0930
// into the time field, which is not a time until the mask has formatted it, and
// the mask only runs when the panel is drawn. A picker of a time alone said
// nothing at all.
func TestAChosenTimeIsOnTheFieldBeforeThePanelHasBeenOpened(t *testing.T) {
	moment := time.Date(2026, time.November, 2, 9, 30, 0, 0, time.UTC)

	var state DateTime
	state.Select(moment, false)

	if got := state.Selected(false); !got.Equal(moment) {
		t.Errorf("a picker chosen at %v answers %v", moment, got)
	}
	if got := state.Value(TimeOnly, false); got.Hour() != 9 || got.Minute() != 30 {
		t.Errorf("a time of day chosen at 09:30 answers %v", got)
	}
}

// TestEveryMonthIsThreeLetters keeps "Maio" from standing among "Abr" and "Jun".
func TestEveryMonthIsThreeLetters(t *testing.T) {
	for name, want := range map[string]string{
		"Janeiro": "Jan", "Março": "Mar", "Maio": "Mai", "Junho": "Jun",
		"May": "May", "Dez": "Dez", "": "",
	} {
		if got := shortMonth(name); got != want {
			t.Errorf("%q is cut to %q, want %q", name, got, want)
		}
	}
}
