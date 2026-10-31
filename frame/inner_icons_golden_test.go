package frame_test

import (
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// The icons a component draws inside itself are Kyse's, and these are the
// pictures that show it: every component whose inside changed to the icon Kyse
// draws there, in both schemes at two densities. The colour of each is the
// theme's, and the dark picture is what shows one that was written as a value.
//
// What the icons are is checked by icons_parity_test.go in the root package,
// against Kyse's source. What they look like is checked here.

func innerScreens() []screen {
	return []screen{
		{name: "inner-structure", width: 380, height: 560, draw: innerStructure},
		{name: "inner-inputs", width: 380, height: 540, draw: innerInputs},
		{name: "inner-overlays", width: 380, height: 580, draw: innerOverlays},
		{name: "inner-palette", width: 380, height: 260, draw: innerPalette},
	}
}

func TestTheInnerIconsLookLikeKyses(t *testing.T) {
	for _, screen := range innerScreens() {
		for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
			for _, density := range []float32{1, 2} {
				name := pictureName(screen.name, scheme, density)

				t.Run(name, func(t *testing.T) {
					got, err := frame.Draw(frame.Options{
						Width: screen.width, Height: screen.height, Density: density, Scheme: scheme, Fonts: fonts(),
					}, screen.draw())
					if err != nil {
						if noSurface(err) {
							t.Skipf("no drawing surface on this machine: %v", err)
						}
						t.Fatalf("the screen did not draw: %v", err)
					}
					compare(t, name, got)
				})
			}
		}
	}
}

// innerStructure draws what opens and what leads: an accordion with its second
// section open, a collapsible, a trail and a tree with a branch open.
func innerStructure() ayra.Widget {
	var accordion widget.Accordion
	accordion.Open(1)
	var section widget.Disclosure
	var crumbs widget.Crumbs
	var tree widget.Tree
	tree.Expand([]int{0})

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 340,
			line("Structure", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.AccordionProps{Sections: []string{"Is it accessible?", "Is it styled?", "Is it animated?"}}.
					Layout(c, &accordion, func(int) ayra.Widget { return line("Yes. It follows the pattern.", widget.Caption, false) })
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.CollapsibleProps{Summary: "Advanced"}.Layout(c, &section, line("", widget.Caption, false))
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.BreadcrumbProps{Steps: []string{"Home", "Cameras", "Gate"}}.Layout(c, &crumbs)
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions {
				return widget.TreeProps{Roots: []widget.TreeNode{
					{Label: "app", Children: []widget.TreeNode{{Label: "Http"}, {Label: "Models"}}},
					{Label: "resources", Children: []widget.TreeNode{{Label: "views"}}},
					{Label: "go.mod"},
				}}.Layout(c, &tree)
			},
		)
	}
}

// innerInputs draws the fields: a list, a count, a password hidden and shown, a
// rating, a ticked box, a combobox and the two buttons that carry a caret.
func innerInputs() ayra.Widget {
	var picked widget.Select
	picked.Choose(1)
	var count widget.Stepper
	count.SetValue(3)
	var hidden, shown widget.Secret
	hidden.SetText("correct horse")
	shown.SetText("correct horse")
	shown.Reveal(true)
	var stars widget.Rating
	stars.SetValue(3)
	var ticked, unticked widget.Toggle
	ticked.Set(true)
	var lookup widget.Combobox
	var split widget.Split
	var removal widget.DeleteRow
	var edit widget.EditInPlace
	var themeButton widget.Button

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 340,
			line("Inputs", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SelectProps{Options: []string{"Free", "Team", "Enterprise"}, Placeholder: "Choose one"}.Layout(c, &picked)
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions { return widget.NumberProps{Min: 1, Max: 10}.Layout(c, &count) },
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.PasswordProps{Placeholder: "Your password"}.Layout(c, &hidden)
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.PasswordProps{Placeholder: "Your password"}.Layout(c, &shown)
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions { return widget.RatingProps{}.Layout(c, &stars) },
			spacer(10),
			check(widget.CheckboxProps{Label: "Remember this device"}, &ticked),
			spacer(6),
			check(widget.CheckboxProps{Label: "Send me the receipt"}, &unticked),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ComboboxProps{Placeholder: "Search a repository"}.Layout(c, &lookup)
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.SplitProps{Label: "Save"}.Layout(c, &split)
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions { return widget.DeleteRowProps{}.Layout(c, &removal) },
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.EditInPlaceProps{Value: "Gate camera"}.Layout(c, &edit)
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions { return widget.ThemeToggleProps{}.Layout(c, &themeButton) },
		)
	}
}

// innerOverlays draws the dates and what sits over the screen: a month, the
// three closed date fields, a stack of messages and the palette.
func innerOverlays() ayra.Widget {
	today := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)
	var month widget.Calendar
	var day widget.DatePicker
	var period widget.DateRange
	var moment widget.DateTime
	var toasts widget.Toasts
	toasts.Show(widget.Toast{Title: "Saved."})
	toasts.Show(widget.Toast{Title: "The camera did not answer.", Severity: widget.Failure})

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 340,
			line("Overlays", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions { return widget.CalendarProps{Today: today}.Layout(c, &month) },
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return widget.DatePickerProps{Today: today, Placeholder: "Pick a date"}.Layout(c, &day)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.DateRangePickerProps{Today: today}.Layout(c, &period)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.DateTimePickerProps{Today: today, Kind: widget.DateAndTime}.Layout(c, &moment)
			},
			spacer(14),
			func(c ayra.Context) ayra.Dimensions { return widget.ToastProps{}.Layout(c, &toasts) },
		)
	}
}

// innerPalette draws the command palette open, with the magnifying glass at the
// start of its field.
func innerPalette() ayra.Widget {
	var commands widget.Palette
	commands.Open()

	return func(c ayra.Context) ayra.Dimensions {
		return widget.PaletteProps{Commands: []string{"Open cameras", "Open reads", "Open alerts"}}.Layout(c, &commands)
	}
}
