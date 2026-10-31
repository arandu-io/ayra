package theme

// Locale is the words and the calendar conventions the widgets draw with when
// the caller did not supply their own.
//
// It is a field of [Theme] and not of each Props because a locale is the
// application's, not a screen's: the product says once that it is in Portuguese
// and every calendar, date picker and date range it draws is. The zero Locale
// draws English, with the week starting on Sunday, which is what the widgets
// drew before this existed; a Props field that is set still wins over it.
//
// It holds arrays and not slices so that the zero Theme stays comparable with
// ==.
type Locale struct {
	// Tag names the locale, "pt-BR". It is for a diagnostic and for a test; no
	// widget reads it.
	Tag string
	// Months are the twelve names, from January. Any empty one makes the whole
	// list unused, since a calendar with one month missing is worse than one in
	// English.
	Months [12]string
	// Weekdays head the columns of a calendar, from Sunday.
	Weekdays [7]string
	// Monday starts the week on Monday. Brazil's calendar starts on Sunday, as
	// the web panel's does.
	Monday bool
	// Today and Clear are the words on the two controls under the grid. Empty
	// draws neither.
	Today, Clear string
	// Previous and Next name the two steps either side of the month, and
	// PreviousYear and NextYear the two of the month picker. They are the names
	// a screen reader says: the steps are drawn as Kyse's carets, so the words
	// are not on screen. Empty says the English of the widget, "Previous month"
	// and "Next month".
	Previous, Next, PreviousYear, NextYear string

	// The words of the pickers that open a calendar. Each is what a Props field
	// of the same meaning says when the caller left it empty: the placeholder of
	// a field before anything is chosen, the line that asks for the end of a
	// period, the label of the time field and the button that puts the panel
	// away. Empty leaves the English the widget has.
	PickDate, PickPeriod, PickDateTime, PickTime, PickMonth, PickWeek string
	PendingEnd                                                        string
	Time, Done                                                        string
}

// IsZero reports whether the locale says nothing, which draws English.
func (l Locale) IsZero() bool { return l == Locale{} }

// MonthNames answers the twelve names, and nil when the locale does not carry
// all of them.
func (l Locale) MonthNames() []string {
	for _, name := range l.Months {
		if name == "" {
			return nil
		}
	}
	return append([]string(nil), l.Months[:]...)
}

// WeekdayNames answers the seven column heads from Sunday, and nil when the
// locale does not carry all of them.
func (l Locale) WeekdayNames() []string {
	for _, name := range l.Weekdays {
		if name == "" {
			return nil
		}
	}
	return append([]string(nil), l.Weekdays[:]...)
}

// PortugueseBrazil is pt-BR, with the same words the web panel's date pickers
// carry: the month and weekday names of the Kyse calendar, "Hoje" and "Limpar"
// under the grid, and "Mês anterior" and "Próximo mês" as the names of the two
// steps, which is what the web panel's calendar says. The week starts on
// Sunday, as the web panel's does.
func PortugueseBrazil() Locale {
	return Locale{
		Tag: "pt-BR",
		Months: [12]string{
			"Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
			"Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro",
		},
		Weekdays:     [7]string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"},
		Today:        "Hoje",
		Clear:        "Limpar",
		Previous:     "Mês anterior",
		Next:         "Próximo mês",
		PreviousYear: "Ano anterior",
		NextYear:     "Próximo ano",

		PickDate:     "Escolher data",
		PickPeriod:   "Escolher período",
		PickDateTime: "Escolher data e hora",
		PickTime:     "Escolher hora",
		PickMonth:    "Escolher mês",
		PickWeek:     "Escolher semana",
		PendingEnd:   "Escolha a data final",
		Time:         "Hora",
		Done:         "Concluir",
	}
}
