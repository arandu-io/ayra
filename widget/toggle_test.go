package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

const longConsent = "Li e concordo com os termos de uso, a política de privacidade e o tratamento dos meus dados pessoais, inclusive o compartilhamento das leituras de placa com a operadora"

// toggles is the three controls a label sits beside, each drawn from its props
// and the state it holds, so a test can ask all three the same question.
func toggles(label string, mixed bool) map[string]func(c ayra.Context, state *widget.Toggle) ayra.Dimensions {
	return map[string]func(c ayra.Context, state *widget.Toggle) ayra.Dimensions{
		"checkbox": func(c ayra.Context, state *widget.Toggle) ayra.Dimensions {
			return widget.CheckboxProps{Label: label, Mixed: mixed}.Layout(c, state)
		},
		"radio": func(c ayra.Context, state *widget.Toggle) ayra.Dimensions {
			return widget.RadioProps{Label: label}.Layout(c, state)
		},
		"switch": func(c ayra.Context, state *widget.Toggle) ayra.Dimensions {
			return widget.SwitchProps{Label: label}.Layout(c, state)
		},
	}
}

// TestALongLabelGoesOnToASecondLine is the sentence somebody agrees to: cut at
// the edge of the screen with an ellipsis it is a sentence nobody can read to the
// end. Kyse lets the label wrap beside the mark, and so does this.
func TestALongLabelGoesOnToASecondLine(t *testing.T) {
	c, _ := frame(t, theme.Light, 300)
	line := widget.TextProps{Content: "Ag"}.Layout(c).Size.Y

	for name, draw := range toggles(longConsent, false) {
		var state widget.Toggle
		got := draw(c, &state).Size

		if got.Y < 3*line {
			t.Errorf("%s: a label of four lines took %d px, and a line is %d: it was cut short", name, got.Y, line)
		}
		if got.X > 300 {
			t.Errorf("%s: the label ran off the %d px it was given: %d", name, 300, got.X)
		}
	}
}

// TestALabelledControlIsAsTallAsAFinger fixes a column of targets half the
// height of a thumb: the mark is 18 points and the line beside it is not much
// more, and the whole row is the press.
func TestALabelledControlIsAsTallAsAFinger(t *testing.T) {
	c, _ := frame(t, theme.Light, 300)

	for name, draw := range toggles("Lembrar deste dispositivo", false) {
		var state widget.Toggle
		if got := draw(c, &state).Size.Y; got < 44 {
			t.Errorf("%s with a label is %d dp tall, and a finger needs 44", name, got)
		}
	}
}

// TestABoxWithNoLabelStaysTheSizeOfTheBox keeps the rule above to the controls
// it is for: the box in a table's cell has the row around it to be the target,
// and 44 dp of height there is a row twice as tall as it was.
func TestABoxWithNoLabelStaysTheSizeOfTheBox(t *testing.T) {
	c, _ := frame(t, theme.Light, 300)
	var state widget.Toggle

	if got := (widget.CheckboxProps{}).Layout(c, &state).Size; got != image.Pt(18, 18) {
		t.Errorf("a box with no label took %v", got)
	}
}

// TestTheMarkIsLevelWithTheFirstLine is where the eye starts. A mark centred on
// a block of four lines hangs beside the second, and reads as belonging to
// neither.
func TestTheMarkIsLevelWithTheFirstLine(t *testing.T) {
	var state widget.Toggle
	state.Set(true)

	picture := drawn(t, 300, 200, theme.Light, func(c ayra.Context) ayra.Dimensions {
		return widget.CheckboxProps{Label: longConsent}.Layout(c, &state)
	})

	top, bottom := -1, -1
	for y := 0; y < 200; y++ {
		if luminance(picture.RGBAAt(9, y)) < 100 {
			if top < 0 {
				top = y
			}
			bottom = y
		}
	}
	if top < 0 {
		t.Fatal("the ticked box was not drawn")
	}
	if top > 8 {
		t.Errorf("the box starts %d px down: it is centred on the four lines and not on the first", top)
	}
	if bottom > 30 {
		t.Errorf("the box reaches %d px down: it is stretched or centred on the block", bottom)
	}
}

// TestAPartlySetBoxDrawsADash is the "select all" of a list of which some are
// chosen: filled, as a ticked box is, with a dash where the tick goes.
func TestAPartlySetBoxDrawsADash(t *testing.T) {
	var mixed, plain widget.Toggle

	box := func(props widget.CheckboxProps, state *widget.Toggle) *image.RGBA {
		return drawn(t, 40, 40, theme.Light, func(c ayra.Context) ayra.Dimensions {
			return props.Layout(c, state)
		})
	}
	partly := box(widget.CheckboxProps{Mixed: true}, &mixed)
	empty := box(widget.CheckboxProps{}, &plain)

	if got := luminance(partly.RGBAAt(9, 4)); got > 100 {
		t.Errorf("a partly set box is not filled: %d at its top edge", got)
	}
	if got := luminance(partly.RGBAAt(9, 9)); got < 200 {
		t.Errorf("a partly set box has no dash through its middle: %d", got)
	}
	if got := luminance(empty.RGBAAt(9, 4)); got < 200 {
		t.Errorf("an unset box is filled: %d", got)
	}
}

// TestAnOffSwitchHasAKnobInBothSchemes is the switch of an edit screen that read
// as a grey pill: toggleColours gives a control that is off no ink, and the knob
// was drawn in it, so an off switch was a track with nothing on it. The knob is
// the page's colour in the light scheme and the text's in the dark one.
func TestAnOffSwitchHasAKnobInBothSchemes(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		var state widget.Toggle
		picture := drawn(t, 60, 40, scheme, func(c ayra.Context) ayra.Dimensions {
			return widget.SwitchProps{}.Layout(c, &state)
		})

		// The switch is 36 by 20, and the knob sits at the left end of it.
		knob, track := luminance(picture.RGBAAt(10, 10)), luminance(picture.RGBAAt(28, 10))
		if gap := knob - track; gap < 20 && gap > -20 {
			t.Errorf("%v: the knob of an off switch is %d and the track %d: there is no knob to see", scheme, knob, track)
		}
	}
}
