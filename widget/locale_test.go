package widget_test

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// The words a screen reader says for the calendar's steps are the locale's, and
// they were English: the steps are Kyse's carets, so the label is not on screen
// and a picture cannot show it. They are read off the semantic tree.

// treeIn is tree with the theme of the application, whose locale is under test.
func treeIn(t *testing.T, th theme.Theme, draw ayra.Widget) []string {
	t.Helper()
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(400, 600)},
	}
	draw(ayra.Context{Context: gtx, Theme: th, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))})
	router.Frame(ops)
	return spoken(router.AppendSemantics(nil))
}

func localised(locale theme.Locale) theme.Theme {
	th := theme.New(theme.Light)
	th.Locale = locale
	return th
}

var theDay = time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)

func TestTheStepsOfTheCalendarAreNamedInTheLocale(t *testing.T) {
	var state widget.Calendar
	screen := func(props widget.CalendarProps) ayra.Widget {
		return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, &state) }
	}

	said := treeIn(t, localised(theme.PortugueseBrazil()), screen(widget.CalendarProps{Today: theDay}))
	for _, want := range []string{"Mês anterior", "Próximo mês", "Hoje", "Limpar"} {
		if !slices.Contains(said, want) {
			t.Errorf("under pt-BR the calendar says %q, want %q among them", said, want)
		}
	}

	said = treeIn(t, theme.New(theme.Light), screen(widget.CalendarProps{Today: theDay}))
	for _, want := range []string{"Previous month", "Next month"} {
		if !slices.Contains(said, want) {
			t.Errorf("with no locale the calendar says %q, want %q among them", said, want)
		}
	}

	said = treeIn(t, localised(theme.PortugueseBrazil()), screen(widget.CalendarProps{Today: theDay, PreviousLabel: "Voltar"}))
	if !slices.Contains(said, "Voltar") || slices.Contains(said, "Mês anterior") {
		t.Errorf("a label the caller wrote lost to the locale: %q", said)
	}
}

func TestTheStepsOfTheMonthPickerAreNamedInTheLocale(t *testing.T) {
	var state widget.DateTime
	state.Open()
	said := treeIn(t, localised(theme.PortugueseBrazil()), func(c ayra.Context) ayra.Dimensions {
		return widget.DateTimePickerProps{Today: theDay, Kind: widget.MonthOnly}.Layout(c, &state)
	})
	for _, want := range []string{"Ano anterior", "Próximo ano"} {
		if !slices.Contains(said, want) {
			t.Errorf("under pt-BR the month picker says %q, want %q among them", said, want)
		}
	}
}
