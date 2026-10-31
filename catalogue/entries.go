package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/icons"
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

// iconEntry draws a sample of the Phosphor set, in the weights of ink a screen
// uses, and an icon-only button.
func iconEntry() Entry {
	return Entry{
		Control: "IconProps",
		Summary: "The Phosphor icons under Kyse's names, drawn in the colour of the text around them.",
		Build: func() ayra.Widget {
			var only widget.Button
			set := []icons.Icon{
				icons.Trash(), icons.Gear(), icons.Camera(), icons.MagnifyingGlass(), icons.Check(), icons.Warning(),
				icons.Car(), icons.Bell(), icons.CloudArrowUp(), icons.Funnel(), icons.House(), icons.User(),
			}
			return func(c ayra.Context) ayra.Dimensions {
				return stack(14,
					func(c ayra.Context) ayra.Dimensions {
						children := make([]layout.FlexChild, 0, len(set)*2)
						for _, icon := range set {
							icon := icon
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return widget.IconProps{Icon: icon, Size: 28}.Layout(c.With(gtx))
							}), layout.Rigid(layout.Spacer{Width: 10}.Layout))
						}
						return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context, children...)
					},
					labelled("in a button, and alone", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Icon: icons.Gear(), Size: widget.Icon, Variant: widget.Outline, Label: ""}.Layout(c, &only)
					})),
				)(c)
			}
		},
	}
}

// imageEntry draws a picture in each of its fits, and an avatar with a photo.
func imageEntry() Entry {
	return Entry{
		Control: "ImageProps",
		Summary: "A picture loaded once from a source, with a skeleton while it loads and the alternative text if it fails.",
		Build: func() ayra.Widget {
			var wide, failed widget.Image
			wide.Set(demoPicture(240, 90))
			var avatar widget.Image
			avatar.Set(demoPicture(64, 64))
			return func(c ayra.Context) ayra.Dimensions {
				return stack(12,
					labelled("cover", sized(240, func(c ayra.Context) ayra.Dimensions {
						return widget.ImageProps{Height: 90, Alt: "A gate camera"}.Layout(c, &wide)
					})),
					labelled("contain", sized(240, func(c ayra.Context) ayra.Dimensions {
						return widget.ImageProps{Height: 90, Fit: widget.Contain}.Layout(c, &wide)
					})),
					labelled("failed", sized(240, func(c ayra.Context) ayra.Dimensions {
						return widget.ImageProps{
							Height: 60, Alt: "Camera offline",
							Source: widget.ImageSourceFunc(func(context.Context) (image.Image, error) { return nil, errors.New("offline") }),
						}.Layout(c, &failed)
					})),
					labelled("avatar with a photo", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.AvatarProps{Initials: "PL", Photo: &avatar}.Layout(c)
					})),
				)(c)
			}
		},
	}
}

func demoPicture(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 140, A: 255})
		}
	}
	return img
}

// relativeTimeEntry draws moments written the way a person says them.
func relativeTimeEntry() Entry {
	return Entry{
		Control: "RelativeTimeProps",
		Summary: "A moment as a person says it, relative to the time of the frame.",
		Build: func() ayra.Widget {
			now := today()
			ago := []time.Duration{5 * time.Second, 3 * time.Minute, 2 * time.Hour, 3 * 24 * time.Hour, 20 * 24 * time.Hour, -2 * time.Hour}
			return func(c ayra.Context) ayra.Dimensions {
				rows := make([]ayra.Widget, 0, len(ago))
				for _, d := range ago {
					d := d
					rows = append(rows, func(c ayra.Context) ayra.Dimensions {
						return widget.RelativeTimeProps{At: now.Add(-d), Now: now}.Layout(c)
					})
				}
				return stack(6, rows...)(c)
			}
		},
	}
}

// loadMoreEntry draws the control at the foot of a list.
func loadMoreEntry() Entry {
	return Entry{
		Control: "LoadMoreProps",
		Summary: "The button that asks for the next page, and says so while it is coming.",
		Build: func() ayra.Widget {
			var ready, busy widget.LoadMore
			return func(c ayra.Context) ayra.Dimensions {
				return stack(10,
					tight(func(c ayra.Context) ayra.Dimensions { return widget.LoadMoreProps{}.Layout(c, &ready) }),
					tight(func(c ayra.Context) ayra.Dimensions { return widget.LoadMoreProps{IsLoading: true}.Layout(c, &busy) }),
				)(c)
			}
		},
	}
}

// editInPlaceEntry draws a text that becomes a field when it is pressed.
func editInPlaceEntry() Entry {
	return Entry{
		Control: "EditInPlaceProps",
		Summary: "A text edited where it stands: press to open a field, return to save.",
		Build: func() ayra.Widget {
			var state widget.EditInPlace
			value := "Gate camera 1"
			return func(c ayra.Context) ayra.Dimensions {
				if saved, ok := state.Saved(); ok {
					value = saved
				}
				return sized(320, func(c ayra.Context) ayra.Dimensions {
					return widget.EditInPlaceProps{Value: value, Placeholder: "Name"}.Layout(c, &state)
				})(c)
			}
		},
	}
}

// inputGroupEntry draws a field and a button sharing one border.
func inputGroupEntry() Entry {
	return Entry{
		Control: "InputGroupProps",
		Summary: "A row of controls under one border and one set of corners.",
		Build: func() ayra.Widget {
			var field widget.Input
			var send widget.Button
			return sized(360, func(c ayra.Context) ayra.Dimensions {
				return widget.InputGroupProps{}.Layout(c,
					func(c ayra.Context) ayra.Dimensions {
						return widget.InputProps{Placeholder: "Plate", ComponentProps: widget.GroupedParts()}.Layout(c, &field)
					},
					func(c ayra.Context) ayra.Dimensions {
						return widget.ButtonProps{Label: "Search", Variant: widget.Ghost, ComponentProps: widget.GroupedParts()}.Layout(c, &send)
					},
				)
			})
		},
	}
}

// themeToggleEntry draws the button that switches the palette.
func themeToggleEntry() Entry {
	return Entry{
		Control: "ThemeToggleProps",
		Summary: "The button that switches between the light and the dark palette: a moon in one, a sun in the other.",
		Build: func() ayra.Widget {
			var state widget.Button
			return tight(func(c ayra.Context) ayra.Dimensions { return widget.ThemeToggleProps{}.Layout(c, &state) })
		},
	}
}
