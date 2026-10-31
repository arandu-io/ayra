package main

import (
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
	"time"

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

// comboboxEntry draws the field that searches a server as it is typed in.
func comboboxEntry() Entry {
	return Entry{
		Control: "ComboboxProps",
		Summary: "A field that searches a server and holds the option chosen, never the text typed.",
		Build: func() ayra.Widget {
			var state widget.Combobox
			props := widget.ComboboxProps{
				Placeholder: "Search a city",
				Source: widget.LookupFunc(func(ctx context.Context, query string) ([]widget.Option, error) {
					var found []widget.Option
					for code, name := range map[string]string{"SP": "Sao Paulo", "RJ": "Rio de Janeiro", "BH": "Belo Horizonte", "SJ": "Sao Jose dos Campos"} {
						if strings.Contains(strings.ToLower(name), strings.ToLower(query)) || strings.EqualFold(code, query) {
							found = append(found, widget.Option{Value: code, Label: name})
						}
					}
					sort.Slice(found, func(a, b int) bool { return found[a].Label < found[b].Label })
					return found, nil
				}),
			}
			return sized(320, func(c ayra.Context) ayra.Dimensions { return props.Layout(c, &state) })
		},
	}
}

// fileUploadEntry draws the field that chooses files and sends them.
func fileUploadEntry() Entry {
	return Entry{
		Control: "FileUploadProps",
		Summary: "A field that chooses files and sends them, with a bar for each and the rules checked first.",
		Build: func() ayra.Widget {
			var state widget.FileUpload
			props := widget.FileUploadProps{
				Accept:  []string{".pdf", ".png"},
				MaxSize: 5 << 20,
				Chooser: widget.ChooserFunc(func(ctx context.Context) ([]widget.File, error) {
					return []widget.File{demoFile("contract.pdf", 3<<20), demoFile("notes.txt", 900)}, nil
				}),
				Uploader: widget.UploaderFunc(func(ctx context.Context, f widget.File, body io.Reader) error {
					buf := make([]byte, 64<<10)
					for {
						_, err := body.Read(buf)
						if err == io.EOF {
							return nil
						}
						if err != nil {
							return err
						}
						select {
						case <-time.After(30 * time.Millisecond):
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				}),
			}
			return sized(480, func(c ayra.Context) ayra.Dimensions { return props.Layout(c, &state) })
		},
	}
}

func demoFile(name string, size int) widget.File {
	return widget.File{Name: name, Size: int64(size), Open: func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(make([]byte, size))), nil
	}}
}
