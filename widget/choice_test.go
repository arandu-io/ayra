package widget_test

import (
	"fmt"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// TestASelectionPastTheOptionsIsNoSelection is a list that got shorter under a
// choice: a role picked from three, and the server now sending one.
//
// The control drew the placeholder, because the index named nothing, while
// Selected still answered the index -- so a form built from it submitted a
// choice the screen said had not been made.
func TestASelectionPastTheOptionsIsNoSelection(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.Select
	state.Choose(2)

	widget.SelectProps{Options: []string{"viewer"}, Placeholder: "Pick a role"}.Layout(c, &state)

	if got := state.Selected(); got != -1 {
		t.Errorf("the screen shows the placeholder and Selected answers %d", got)
	}
}

// TestASelectionInsideTheOptionsIsKept keeps the reset to the case it is for.
func TestASelectionInsideTheOptionsIsKept(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.Select
	state.Choose(1)

	widget.SelectProps{Options: []string{"viewer", "admin"}}.Layout(c, &state)

	if got := state.Selected(); got != 1 {
		t.Errorf("a choice inside the options was dropped: %d", got)
	}
}

// formOf draws a column of the given widgets, one under the other with no room
// between them, which is how a form is laid out.
func formOf(children ...ayra.Widget) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		items := make([]layout.FlexChild, 0, len(children))
		for _, child := range children {
			child := child
			items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return child(c.With(gtx))
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, items...)
	}
}

// TestTheOpenListIsDrawnOverTheFieldsAfterIt is the select in the middle of a
// form. The list was drawn where the control is, so the next field of the form
// -- drawn later -- painted over the options, and took the presses meant for
// them: a list nobody could choose from. It is put off to the end of the frame.
//
// The same list is drawn with a field after it and without one, and the part of
// the picture that is the list has to be the same in both.
func TestTheOpenListIsDrawnOverTheFieldsAfterIt(t *testing.T) {
	plans := []string{"Gratuito", "Equipe", "Empresarial"}

	list := func(after bool) *image.RGBA {
		var open, below widget.Select
		open.Open()
		children := []ayra.Widget{func(c ayra.Context) ayra.Dimensions {
			return widget.SelectProps{Options: plans, Placeholder: "Plano"}.Layout(c, &open)
		}}
		if after {
			children = append(children, func(c ayra.Context) ayra.Dimensions {
				return widget.SelectProps{Options: plans, Placeholder: "Outro plano"}.Layout(c, &below)
			})
		}
		return drawn(t, 360, 260, theme.Light, formOf(children...))
	}

	alone, crowded := list(false), list(true)

	// Under the control (about 40 px) and inside the list (about 110 px tall), a
	// little in from its edge, which is a hairline that blends with whatever is
	// beneath it and so is never the same twice.
	area := image.Rect(8, 48, 352, 105)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if alone.RGBAAt(x, y) != crowded.RGBAAt(x, y) {
				t.Fatalf("the list is not what is on top at %d,%d: the field after it was drawn over the options", x, y)
			}
		}
	}
}

// TestALongListScrollsInsteadOfRunningOffTheWindow is a list of the states of
// Brazil: twenty-seven rows of 36 points, which the window does not have, so the
// last options were somewhere nobody could reach. The list is as tall as eight
// rows and scrolls inside that.
func TestALongListScrollsInsteadOfRunningOffTheWindow(t *testing.T) {
	var options []string
	for i := 1; i <= 40; i++ {
		options = append(options, fmt.Sprintf("Opção %d", i))
	}
	var state widget.Select
	state.Open()

	picture := drawn(t, 360, 700, theme.Light, func(c ayra.Context) ayra.Dimensions {
		return widget.SelectProps{Options: options, Placeholder: "Estado"}.Layout(c, &state)
	})

	if !inked(picture, image.Rect(0, 60, 360, 100)) {
		t.Fatal("the list is not drawn under the control")
	}
	if inked(picture, image.Rect(0, 420, 360, 700)) {
		t.Error("the list runs on past eight rows: forty options are 1440 points of list")
	}
}

// TestADisabledSelectFadesItsCaret fixes the one part of it that stayed at full
// strength: the text went pale and the arrow beside it did not.
func TestADisabledSelectFadesItsCaret(t *testing.T) {
	strength := func(disabled bool) int {
		var state widget.Select
		picture := drawn(t, 360, 60, theme.Light, func(c ayra.Context) ayra.Dimensions {
			return widget.SelectProps{Options: []string{"Equipe"}, Placeholder: "Plano", Disabled: disabled}.Layout(c, &state)
		})
		darkest := 255
		for y := 8; y < 32; y++ {
			for x := 322; x < 346; x++ {
				darkest = min(darkest, luminance(picture.RGBAAt(x, y)))
			}
		}
		return darkest
	}

	if on, off := strength(false), strength(true); off <= on {
		t.Errorf("the caret of a disabled select is as dark as an enabled one's: %d against %d", off, on)
	}
}

// TestFieldsOfAFormAreAsTallAsEachOther is the Input, the Select and the dates in
// one row of a filter: the Select was drawn 37 points tall and the rest 33, and
// a row of them did not sit on one line.
func TestFieldsOfAFormAreAsTallAsEachOther(t *testing.T) {
	c, _ := frame(t, theme.Light, 360)
	var in widget.Input
	var pick widget.Select
	var date widget.DatePicker
	var period widget.DateRange
	var moment widget.DateTime

	want := widget.InputProps{}.Layout(c, &in).Size.Y
	for name, got := range map[string]int{
		"select":    widget.SelectProps{}.Layout(c, &pick).Size.Y,
		"date":      widget.DatePickerProps{}.Layout(c, &date).Size.Y,
		"period":    widget.DateRangePickerProps{}.Layout(c, &period).Size.Y,
		"date-time": widget.DateTimePickerProps{}.Layout(c, &moment).Size.Y,
	} {
		if got != want {
			t.Errorf("the %s is %d tall and the text field %d", name, got, want)
		}
	}
}

// TestFieldsTakeTheSurfaceTheySitOnInBothSchemes is the fill of Kyse's input: the
// text field, the select and the date fields draw the same inside, which is
// nothing in the light scheme and the same tint in the dark one. The Select and
// the dates were filled with the page's colour and stood out from the Input
// beside them.
func TestFieldsTakeTheSurfaceTheySitOnInBothSchemes(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		inside := func(w ayra.Widget) color.RGBA {
			picture := drawn(t, 360, 40, scheme, w)
			// Away from the text, the caret and the corners.
			return picture.RGBAAt(200, 16)
		}
		var pick widget.Select
		var date widget.DatePicker
		var period widget.DateRange
		var moment widget.DateTime

		// What Kyse's input is: the surface in the light scheme, and 30% of the
		// input colour over the card in the dark one, mixed and opaque.
		th := theme.New(scheme)
		want := color.RGBA{R: th.Colours.Background.R, G: th.Colours.Background.G, B: th.Colours.Background.B, A: 255}
		if scheme == theme.Dark {
			mix := func(over, under uint8) uint8 {
				alpha := float32(th.Colours.Input.A) / 255 * 0.3
				return uint8(float32(over)*alpha + float32(under)*(1-alpha) + 0.5)
			}
			want = color.RGBA{R: mix(th.Colours.Input.R, th.Colours.Card.R), G: mix(th.Colours.Input.G, th.Colours.Card.G), B: mix(th.Colours.Input.B, th.Colours.Card.B), A: 255}
		}

		for name, w := range map[string]ayra.Widget{
			"select": func(c ayra.Context) ayra.Dimensions { return widget.SelectProps{}.Layout(c, &pick) },
			"date":   func(c ayra.Context) ayra.Dimensions { return widget.DatePickerProps{}.Layout(c, &date) },
			"period": func(c ayra.Context) ayra.Dimensions { return widget.DateRangePickerProps{}.Layout(c, &period) },
			"time":   func(c ayra.Context) ayra.Dimensions { return widget.DateTimePickerProps{}.Layout(c, &moment) },
		} {
			if got := inside(w); got != want {
				t.Errorf("%v: the inside of the %s is %v and Kyse's input is %v", scheme, name, got, want)
			}
		}
	}
}

// TestAPeriodsPanelIsOverTheListBelowIt is the period of a screen of readings, in
// a list with a table after it: the panel of the field was behind the next item,
// which is drawn later.
func TestAPeriodsPanelIsOverTheListBelowIt(t *testing.T) {
	today := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)

	screen := func(table bool) *image.RGBA {
		var period widget.DateRange
		period.Open()
		var list layout.List
		list.Axis = layout.Vertical
		count := 1
		if table {
			count = 2
		}
		return drawn(t, 360, 500, theme.Light, func(c ayra.Context) ayra.Dimensions {
			return list.Layout(c.Context, count, func(gtx layout.Context, index int) layout.Dimensions {
				c := c.With(gtx)
				if index == 0 {
					return widget.DateRangePickerProps{Today: today}.Layout(c, &period)
				}
				return widget.CardProps{}.Layout(c, func(c ayra.Context) ayra.Dimensions {
					return widget.TextProps{Content: "ABC1D23 Portaria principal 09:30 ABC1D24 Garagem 09:31"}.Layout(c)
				})
			})
		})
	}

	alone, crowded := screen(false), screen(true)
	for y := 60; y < 360; y++ {
		for x := 24; x < 300; x++ {
			if alone.RGBAAt(x, y) != crowded.RGBAAt(x, y) {
				t.Fatalf("the panel is not on top at %d,%d: the item after it was drawn over the month", x, y)
			}
		}
	}
}

// TestAFieldOnTheKeyboardFocusHasKyses3PointRing is the focus-visible ring of the
// select and the dates: a 3 point ring at half strength outside the field, and the
// ring colour on its border. A field on the focus drew as one off it.
func TestAFieldOnTheKeyboardFocusHasKyses3PointRing(t *testing.T) {
	today := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)
	var pick widget.Select
	var date widget.DatePicker
	var period widget.DateRange
	var moment widget.DateTime

	for name, draw := range map[string]func(c ayra.Context) ayra.Dimensions{
		"select": func(c ayra.Context) ayra.Dimensions { return widget.SelectProps{}.Layout(c, &pick) },
		"date":   func(c ayra.Context) ayra.Dimensions { return widget.DatePickerProps{Today: today}.Layout(c, &date) },
		"period": func(c ayra.Context) ayra.Dimensions {
			return widget.DateRangePickerProps{Today: today}.Layout(c, &period)
		},
		"time": func(c ayra.Context) ayra.Dimensions {
			return widget.DateTimePickerProps{Today: today}.Layout(c, &moment)
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := newPanel()
			screen := func(c ayra.Context) ayra.Dimensions {
				return layout.Inset{Top: 20, Left: 20}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.X = 300
					return draw(c.With(gtx))
				})
			}
			p.draw(screen)
			before := p.raster(t)
			p.router.MoveFocus(key.FocusForward)
			p.draw(screen)
			p.draw(screen)
			after := p.raster(t)

			// One point above the field's top edge, away from the corners.
			above := image.Pt(120, 20-1)
			if before.RGBAAt(above.X, above.Y) != before.RGBAAt(5, 5) {
				t.Fatalf("a field off the focus draws something above its edge")
			}
			if after.RGBAAt(above.X, above.Y) == after.RGBAAt(5, 5) {
				t.Errorf("a field on the keyboard focus has no ring outside its edge")
			}
		})
	}
}
