package main

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/mask"
	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/widget"
)

// maskedEntry draws the field that keeps what is typed in the shape of a mask,
// for each of the shapes a Brazilian form asks for and one of the product's own.
func maskedEntry() Entry {
	return Entry{
		Control: "MaskedProps",
		Summary: "A field that formats what is typed and keeps what a program stores.",
		Build: func() ayra.Widget {
			shapes := []struct {
				name        string
				mask        mask.Mask
				placeholder string
				typed       string
			}{
				{"CPF", brazil.CPF, "000.000.000-00", "12345678909"},
				{"CNPJ", brazil.CNPJ, "00.000.000/0000-00", ""},
				{"phone: a landline, or a mobile at the eleventh digit", brazil.Phone, "(00) 00000-0000", "11987654321"},
				{"CEP", brazil.CEP, "00000-000", ""},
				{"licence plate, old or Mercosul", brazil.Plate, "ABC1D23", "ABC1D23"},
				{"real", brazil.Real, "R$ 0,00", "123456"},
				{"date", mask.Date, "00/00/0000", ""},
				{"the product's own", mask.Pattern("AA-9999"), "AB-1234", ""},
			}
			states := make([]widget.Masked, len(shapes))
			for index, shape := range shapes {
				states[index].SetValue(nil, shape.typed)
			}

			return func(c ayra.Context) ayra.Dimensions {
				rows := make([]ayra.Widget, 0, len(shapes))
				for index, shape := range shapes {
					index, shape := index, shape
					rows = append(rows, labelled(shape.name, sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.MaskedProps{Mask: shape.mask, Placeholder: shape.placeholder, Kind: widget.Number}.Layout(c, &states[index])
					})))
				}
				return stack(12, rows...)(c)
			}
		},
	}
}

// oneTimeCodeEntry draws the row of squares a code is typed into.
func oneTimeCodeEntry() Entry {
	return Entry{
		Control: "OneTimeCodeProps",
		Summary: "A code typed one character to a square, that answers once when it is full.",
		Build: func() ayra.Widget {
			var six, wrong, off widget.OneTimeCode
			six.Set("482")
			wrong.Set("482913")
			off.Set("482913")

			return func(c ayra.Context) ayra.Dimensions {
				return stack(14,
					labelled("six digits", func(c ayra.Context) ayra.Dimensions {
						return widget.OneTimeCodeProps{}.Layout(c, &six)
					}),
					labelled("refused", func(c ayra.Context) ayra.Dimensions {
						return widget.OneTimeCodeProps{Invalid: true}.Layout(c, &wrong)
					}),
					labelled("disabled", func(c ayra.Context) ayra.Dimensions {
						return widget.OneTimeCodeProps{Disabled: true}.Layout(c, &off)
					}),
				)(c)
			}
		},
	}
}

// dateRangePickerEntry draws the field that chooses a period, with shortcuts.
func dateRangePickerEntry() Entry {
	return Entry{
		Control: "DateRangePickerProps",
		Summary: "A field that chooses a period: the first day pressed and the second, or a shortcut.",
		Build: func() ayra.Widget {
			now := today()
			var open, set widget.DateRange
			set.Set(now.AddDate(0, 0, -6), now)

			return func(c ayra.Context) ayra.Dimensions {
				presets := []widget.DatePreset{
					widget.LastDays("Today", now, 1),
					widget.LastDays("Last 7 days", now, 7),
					widget.LastDays("Last 30 days", now, 30),
					widget.ThisMonth("This month", now),
				}
				return stack(16,
					labelled("with shortcuts", sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.DateRangePickerProps{Today: now, Monday: true, Presets: presets}.Layout(c, &open)
					})),
					labelled("a period held", sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.DateRangePickerProps{Today: now, Monday: true, Above: true}.Layout(c, &set)
					})),
				)(c)
			}
		},
	}
}

// dateTimePickerEntry draws the field that chooses a moment.
func dateTimePickerEntry() Entry {
	return Entry{
		Control: "DateTimePickerProps",
		Summary: "A field that chooses a day and the time of it.",
		Build: func() ayra.Widget {
			now := today()
			var below, seconds widget.DateTime
			below.Select(now.Add(0), false)
			seconds.Select(now, true)

			return func(c ayra.Context) ayra.Dimensions {
				return stack(16,
					labelled("minutes", sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.DateTimePickerProps{Today: now, Monday: true}.Layout(c, &below)
					})),
					labelled("seconds", sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.DateTimePickerProps{Today: now, Seconds: true, Above: true}.Layout(c, &seconds)
					})),
				)(c)
			}
		},
	}
}
