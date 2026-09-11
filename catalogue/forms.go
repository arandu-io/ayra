package main

import (
	"image/color"
	"strconv"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// forms are the controls somebody answers a question with.
func forms() []Entry {
	return []Entry{
		fieldEntry(),
		labelEntry(),
		inputEntry(),
		textareaEntry(),
		passwordEntry(),
		numberEntry(),
		selectEntry(),
		autocompleteEntry(),
		checkboxEntry(),
		radioEntry(),
		switchEntry(),
		sliderEntry(),
		ratingEntry(),
		colourPickerEntry(),
	}
}

// kinds is the closed set of what a field expects, in the order the library
// declares it.
var kinds = []widget.Kind{
	widget.Text,
	widget.Email,
	widget.Password,
	widget.Number,
	widget.Telephone,
	widget.URL,
}

// fieldEntry draws the shape a form is repeated out of, in both of its states.
//
// Both, because the error is not decoration: setting it is what marks the
// control invalid, and a field whose message and whose border disagree is the
// defect this control exists to prevent.
func fieldEntry() Entry {
	return Entry{
		Control: "FieldProps",
		Summary: "A control with its name, its help and its error.",
		Build: func() ayra.Widget {
			var good, bad, off widget.Input
			good.SetText("ayra@example.org")
			bad.SetText("not an address")
			off.SetText("Decided by the plan")

			return func(c ayra.Context) ayra.Dimensions {
				return stack(18,
					sized(360, func(c ayra.Context) ayra.Dimensions {
						props := widget.FieldProps{
							Label:    "Email",
							Help:     "Where the receipt is sent.",
							Required: true,
						}
						return props.Layout(c, func(c ayra.Context) ayra.Dimensions {
							return widget.InputProps{Kind: widget.Email, Invalid: props.Invalid()}.Layout(c, &good)
						})
					}),
					sized(360, func(c ayra.Context) ayra.Dimensions {
						props := widget.FieldProps{
							Label:    "Email",
							Help:     "Where the receipt is sent.",
							Error:    "That is not an address anything can be sent to.",
							Required: true,
						}
						return props.Layout(c, func(c ayra.Context) ayra.Dimensions {
							return widget.InputProps{Kind: widget.Email, Invalid: props.Invalid()}.Layout(c, &bad)
						})
					}),
					sized(360, func(c ayra.Context) ayra.Dimensions {
						props := widget.FieldProps{
							Label:    "Account",
							Help:     "Changed by whoever administers the workspace.",
							Disabled: true,
						}
						return props.Layout(c, func(c ayra.Context) ayra.Dimensions {
							return widget.InputProps{Disabled: true}.Layout(c, &off)
						})
					}),
				)(c)
			}
		},
	}
}

// labelEntry draws the name of a field in each of the three ways it is written.
func labelEntry() Entry {
	return Entry{
		Control: "LabelProps",
		Summary: "The name of a control, at the one size every form uses for it.",
		Build: func() ayra.Widget {
			return func(c ayra.Context) ayra.Dimensions {
				return stack(10,
					func(c ayra.Context) ayra.Dimensions {
						return widget.LabelProps{Text: "Workspace"}.Layout(c)
					},
					func(c ayra.Context) ayra.Dimensions {
						return widget.LabelProps{Text: "Workspace", Required: true}.Layout(c)
					},
					func(c ayra.Context) ayra.Dimensions {
						return widget.LabelProps{Text: "Workspace", Disabled: true}.Layout(c)
					},
				)(c)
			}
		},
	}
}

// inputEntry draws the field in every kind it can be, and in the three states
// it can be in.
func inputEntry() Entry {
	return Entry{
		Control: "InputProps",
		Summary: "A line somebody types into.",
		Build: func() ayra.Widget {
			byKind := make([]widget.Input, len(kinds))
			var many, off, wrong widget.Input

			many.SetText("Two lines, and return makes the second one rather than sending anything.")
			off.SetText("Nothing here answers")
			wrong.SetText("rejected")

			return func(c ayra.Context) ayra.Dimensions {
				rows := make([]ayra.Widget, 0, len(kinds)+3)
				for index, kind := range kinds {
					index, kind := index, kind
					rows = append(rows, labelled(kind.String(), sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.InputProps{Placeholder: kind.String(), Kind: kind}.Layout(c, &byKind[index])
					})))
				}

				rows = append(rows,
					labelled("multiline", sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.InputProps{Multiline: true}.Layout(c, &many)
					})),
					labelled("invalid", sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.InputProps{Invalid: true}.Layout(c, &wrong)
					})),
					labelled("disabled", sized(360, func(c ayra.Context) ayra.Dimensions {
						return widget.InputProps{Disabled: true}.Layout(c, &off)
					})),
				)

				return stack(14, rows...)(c)
			}
		},
	}
}

// textareaEntry draws the field somebody writes a paragraph into.
func textareaEntry() Entry {
	return Entry{
		Control: "TextareaProps",
		Summary: "A field where return makes a line rather than sending the form.",
		Build: func() ayra.Widget {
			var ordinary, short, wrong, off widget.Input
			off.SetText("Locked until the plan is changed.")

			return func(c ayra.Context) ayra.Dimensions {
				return stack(16,
					labelled("the height it opens at", sized(400, func(c ayra.Context) ayra.Dimensions {
						return widget.TextareaProps{Placeholder: "What happened"}.Layout(c, &ordinary)
					})),
					labelled("two rows", sized(400, func(c ayra.Context) ayra.Dimensions {
						return widget.TextareaProps{Placeholder: "A note", Rows: 2}.Layout(c, &short)
					})),
					labelled("invalid", sized(400, func(c ayra.Context) ayra.Dimensions {
						return widget.TextareaProps{Rows: 2, Invalid: true}.Layout(c, &wrong)
					})),
					labelled("disabled", sized(400, func(c ayra.Context) ayra.Dimensions {
						return widget.TextareaProps{Rows: 2, Disabled: true}.Layout(c, &off)
					})),
				)(c)
			}
		},
	}
}

// passwordEntry draws the field whose value can be looked at.
func passwordEntry() Entry {
	return Entry{
		Control: "PasswordProps",
		Summary: "A hidden value, with the control that shows it.",
		Build: func() ayra.Widget {
			var ordinary, wrong, off widget.Secret
			ordinary.SetText("correct horse battery")
			wrong.SetText("wrong")
			off.SetText("locked")

			return func(c ayra.Context) ayra.Dimensions {
				return stack(16,
					labelled("hidden until it is shown", sized(400, func(c ayra.Context) ayra.Dimensions {
						return widget.PasswordProps{Placeholder: "Password"}.Layout(c, &ordinary)
					})),
					labelled("invalid", sized(400, func(c ayra.Context) ayra.Dimensions {
						return widget.PasswordProps{Invalid: true}.Layout(c, &wrong)
					})),
					labelled("disabled", sized(400, func(c ayra.Context) ayra.Dimensions {
						return widget.PasswordProps{Disabled: true}.Layout(c, &off)
					})),
					note("showing the value: "+strconv.FormatBool(ordinary.Shown())),
				)(c)
			}
		},
	}
}

// numberEntry draws the whole number with its two steps, bounded and not.
func numberEntry() Entry {
	return Entry{
		Control: "NumberProps",
		Summary: "A whole number, changed by one press at a time.",
		Build: func() ayra.Widget {
			var bounded, free, off widget.Stepper
			bounded.SetValue(3)
			free.SetValue(0)
			off.SetValue(12)

			return func(c ayra.Context) ayra.Dimensions {
				return stack(16,
					labelled("bounded, and the ends stop answering", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.NumberProps{Min: 1, Max: 8}.Layout(c, &bounded)
					})),
					labelled("unbounded, five at a time", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.NumberProps{Step: 5}.Layout(c, &free)
					})),
					labelled("disabled", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.NumberProps{Disabled: true}.Layout(c, &off)
					})),
					note("bounded is at "+strconv.Itoa(bounded.Value())+", unbounded at "+strconv.Itoa(free.Value())),
				)(c)
			}
		},
	}
}

// selectEntry draws the short list, including the state it opens in.
func selectEntry() Entry {
	return Entry{
		Control: "SelectProps",
		Summary: "One of a list short enough not to need searching.",
		Build: func() ayra.Widget {
			var empty, filled, wrong, off widget.Select
			options := []string{"Owner", "Administrator", "Member", "Guest"}
			filled.Choose(2)
			off.Choose(3)

			return func(c ayra.Context) ayra.Dimensions {
				chosen := "nothing"
				if index := empty.Selected(); index >= 0 {
					chosen = options[index]
				}

				return stack(16,
					labelled("nothing chosen, so the placeholder stands", sized(300, func(c ayra.Context) ayra.Dimensions {
						return widget.SelectProps{Options: options, Placeholder: "Pick a role"}.Layout(c, &empty)
					})),
					labelled("chosen", sized(300, func(c ayra.Context) ayra.Dimensions {
						return widget.SelectProps{Options: options, Placeholder: "Pick a role"}.Layout(c, &filled)
					})),
					labelled("invalid", sized(300, func(c ayra.Context) ayra.Dimensions {
						return widget.SelectProps{Options: options, Placeholder: "Pick a role", Invalid: true}.Layout(c, &wrong)
					})),
					labelled("disabled", sized(300, func(c ayra.Context) ayra.Dimensions {
						return widget.SelectProps{Options: options, Disabled: true}.Layout(c, &off)
					})),
					note("the first one holds: "+chosen),
				)(c)
			}
		},
	}
}

// autocompleteEntry draws the field that suggests as it is typed in.
func autocompleteEntry() Entry {
	return Entry{
		Control: "AutocompleteProps",
		Summary: "A field with the entries it matches under it.",
		Build: func() ayra.Widget {
			var suggesting, narrow, off widget.Autocomplete
			entries := []string{
				"Amsterdam", "Asunción", "Athens", "Auckland",
				"Belfast", "Belgrade", "Berlin", "Bogotá", "Brasília",
			}
			chosen := "nothing yet"

			return func(c ayra.Context) ayra.Dimensions {
				if index := suggesting.Chosen(); index >= 0 {
					chosen = entries[index]
				}
				if index := narrow.Chosen(); index >= 0 {
					chosen = entries[index]
				}

				return stack(16,
					labelled("type a letter", sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.AutocompleteProps{
							Entries:     entries,
							Placeholder: "Where are you",
							Empty:       "Nowhere by that name",
						}.Layout(c, &suggesting)
					})),
					labelled("at most two at a time", sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.AutocompleteProps{
							Entries:     entries,
							Placeholder: "Where are you",
							Limit:       2,
							Empty:       "Nowhere by that name",
						}.Layout(c, &narrow)
					})),
					labelled("disabled", sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.AutocompleteProps{
							Entries:     entries,
							Placeholder: "Where are you",
							Disabled:    true,
						}.Layout(c, &off)
					})),
					note("last chosen: "+chosen),
				)(c)
			}
		},
	}
}

// checkboxEntry draws the box in both of its states and unavailable.
func checkboxEntry() Entry {
	return Entry{
		Control: "CheckboxProps",
		Summary: "A box that is ticked or is not.",
		Build: func() ayra.Widget {
			var on, off, unavailable, unavailableOn widget.Toggle
			on.Set(true)
			unavailableOn.Set(true)

			return func(c ayra.Context) ayra.Dimensions {
				on.Changed(c)
				off.Changed(c)

				return stack(12,
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.CheckboxProps{Label: "Send me the receipt"}.Layout(c, &on)
					}),
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.CheckboxProps{Label: "Send me everything else"}.Layout(c, &off)
					}),
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.CheckboxProps{Label: "Decided by the plan", Disabled: true}.Layout(c, &unavailable)
					}),
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.CheckboxProps{Label: "Decided by the plan, and set", Disabled: true}.Layout(c, &unavailableOn)
					}),
				)(c)
			}
		},
	}
}

// radioEntry draws a set where picking one drops the others.
//
// Exclusivity is wired here rather than by the control, and the demonstration
// shows that: three states, and the loop that clears the two nobody pressed.
func radioEntry() Entry {
	return Entry{
		Control: "RadioProps",
		Summary: "One option of a set, where picking one drops the others.",
		Build: func() ayra.Widget {
			labels := []string{"Monthly", "Yearly", "Once"}
			options := make([]widget.Toggle, len(labels))
			options[0].Set(true)
			var unavailable widget.Toggle

			return func(c ayra.Context) ayra.Dimensions {
				for index := range options {
					if !options[index].Changed(c) {
						continue
					}
					for other := range options {
						options[other].Set(other == index)
					}
				}

				rows := make([]ayra.Widget, 0, len(labels)+1)
				for index, label := range labels {
					index, label := index, label
					rows = append(rows, tight(func(c ayra.Context) ayra.Dimensions {
						return widget.RadioProps{Label: label}.Layout(c, &options[index])
					}))
				}
				rows = append(rows, tight(func(c ayra.Context) ayra.Dimensions {
					return widget.RadioProps{Label: "Not on this plan", Disabled: true}.Layout(c, &unavailable)
				}))

				return stack(12, rows...)(c)
			}
		},
	}
}

// switchEntry draws the control that is on or off.
func switchEntry() Entry {
	return Entry{
		Control: "SwitchProps",
		Summary: "A setting that takes effect as soon as it is moved.",
		Build: func() ayra.Widget {
			var on, off, unavailable widget.Toggle
			on.Set(true)

			return func(c ayra.Context) ayra.Dimensions {
				on.Changed(c)
				off.Changed(c)

				return stack(12,
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.SwitchProps{Label: "Two-factor sign-in"}.Layout(c, &on)
					}),
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.SwitchProps{Label: "Weekly summary"}.Layout(c, &off)
					}),
					tight(func(c ayra.Context) ayra.Dimensions {
						return widget.SwitchProps{Label: "Audit log", Disabled: true}.Layout(c, &unavailable)
					}),
				)(c)
			}
		},
	}
}

// sliderEntry draws the value picked along a line, with the fraction it
// answers with written beside it.
func sliderEntry() Entry {
	return Entry{
		Control: "SliderProps",
		Summary: "A fraction picked along a line; what it means in units is the caller's.",
		Build: func() ayra.Widget {
			var ordinary, off widget.Slider
			ordinary.SetValue(0.35)
			off.SetValue(0.8)

			return func(c ayra.Context) ayra.Dimensions {
				return stack(14,
					sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.SliderProps{}.Layout(c, &ordinary)
					}),
					note("at "+strconv.FormatFloat(float64(ordinary.Value()), 'f', 2, 32)),
					labelled("disabled", sized(320, func(c ayra.Context) ayra.Dimensions {
						return widget.SliderProps{Disabled: true}.Layout(c, &off)
					})),
				)(c)
			}
		},
	}
}

// ratingEntry draws the row of marks, at two lengths and in the state somebody
// else's rating is drawn in.
func ratingEntry() Entry {
	return Entry{
		Control: "RatingProps",
		Summary: "A row of marks somebody sets, or one somebody else already did.",
		Build: func() ayra.Widget {
			var five, three, theirs widget.Rating
			five.SetValue(4)
			three.SetValue(2)
			theirs.SetValue(3)

			return func(c ayra.Context) ayra.Dimensions {
				return stack(14,
					labelled("five, the default", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.RatingProps{}.Layout(c, &five)
					})),
					labelled("three", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.RatingProps{Of: 3}.Layout(c, &three)
					})),
					labelled("somebody else's, so it does not answer", tight(func(c ayra.Context) ayra.Dimensions {
						return widget.RatingProps{ReadOnly: true}.Layout(c, &theirs)
					})),
					note("set to "+strconv.Itoa(five.Value())),
				)(c)
			}
		},
	}
}

// colourPickerEntry draws the picker with and without its two extra pieces.
func colourPickerEntry() Entry {
	return Entry{
		Control: "ColourPickerProps",
		Summary: "A colour chosen from a plane, a spectrum and a field of digits.",
		Build: func() ayra.Widget {
			var plain, full, off widget.ColourPicker
			plain.SetColour(color.NRGBA{R: 0x2f, G: 0x6f, B: 0xed, A: 0xff})
			full.SetColour(color.NRGBA{R: 0xd9, G: 0x3f, B: 0x3f, A: 0xcc})
			off.SetColour(color.NRGBA{R: 0x6b, G: 0x72, B: 0x80, A: 0xff})

			swatches := []color.NRGBA{
				{R: 0x11, G: 0x18, B: 0x27, A: 0xff},
				{R: 0xdc, G: 0x26, B: 0x26, A: 0xff},
				{R: 0xd9, G: 0x77, B: 0x06, A: 0xff},
				{R: 0x16, G: 0xa3, B: 0x4a, A: 0xff},
				{R: 0x25, G: 0x63, B: 0xeb, A: 0xff},
				{R: 0x93, G: 0x33, B: 0xea, A: 0xff},
			}

			return func(c ayra.Context) ayra.Dimensions {
				return flow(20,
					labelled("as it comes", sized(230, func(c ayra.Context) ayra.Dimensions {
						return widget.ColourPickerProps{}.Layout(c, &plain)
					})),
					labelled("with opacity and a row of presets", sized(230, func(c ayra.Context) ayra.Dimensions {
						return widget.ColourPickerProps{Alpha: true, Swatches: swatches}.Layout(c, &full)
					})),
					labelled("disabled", sized(230, func(c ayra.Context) ayra.Dimensions {
						return widget.ColourPickerProps{Disabled: true}.Layout(c, &off)
					})),
				)(c)
			}
		},
	}
}
