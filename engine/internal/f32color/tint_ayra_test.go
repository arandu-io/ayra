package f32color

import (
	"image/color"
	"testing"
)

func TestMulAlphaOnlyTouchesAlpha(t *testing.T) {
	in := color.NRGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}
	got := MulAlpha(in, 0x80)
	if got.R != in.R || got.G != in.G || got.B != in.B {
		t.Errorf("MulAlpha(%v, 0x80) = %v, which moved a channel", in, got)
	}
	if got.A >= in.A {
		t.Errorf("MulAlpha(%v, 0x80).A = %v, want it below %v", in, got.A, in.A)
	}
}

func TestMulAlphaKeepsItsEnds(t *testing.T) {
	for a := 0; a <= 0xFF; a++ {
		in := color.NRGBA{R: 0x40, A: uint8(a)}

		if got := MulAlpha(in, 0xFF); got != in {
			t.Errorf("MulAlpha(%v, 0xFF) = %v, want it unchanged", in, got)
		}
		if got := MulAlpha(in, 0); got.A != 0 {
			t.Errorf("MulAlpha(%v, 0).A = %v, want 0", in, got.A)
		}
		// Never above what it started at, which is what makes it safe to apply
		// twice: a control that is both disabled and behind a fading panel must
		// not come back more solid than either asked for.
		if got := MulAlpha(in, 0x80); got.A > in.A {
			t.Errorf("MulAlpha(%v, 0x80).A = %v, want no more than %v", in, got.A, in.A)
		}
	}
}

func TestDisabledDrainsTheColourAndTheAlpha(t *testing.T) {
	red := color.NRGBA{R: 0xE0, G: 0x20, B: 0x20, A: 0xFF}
	got := Disabled(red)

	if spread, was := int(got.R)-int(got.B), int(red.R)-int(red.B); spread >= was {
		t.Errorf("Disabled(%v) = %v, whose channels are %d apart; they were %d apart", red, got, spread, was)
	}
	if got.A >= red.A {
		t.Errorf("Disabled(%v).A = %v, want it below %v", red, got.A, red.A)
	}
}

func TestDisabledLeavesGreyGrey(t *testing.T) {
	// Blending towards its own luminance is what desaturates, so a colour that
	// is already its own luminance has nothing left to give up. Only the alpha
	// moves.
	for v := 0; v <= 0xFF; v++ {
		grey := color.NRGBA{R: uint8(v), G: uint8(v), B: uint8(v), A: 0xFF}
		got := Disabled(grey)
		if got.R != got.G || got.G != got.B {
			t.Fatalf("Disabled(%v) = %v, which is no longer grey", grey, got)
		}
	}
}

func TestHoveredMovesEachWayFromWhereItStarted(t *testing.T) {
	dark := color.NRGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xFF}
	if got := Hovered(dark); approxLuminance(got) <= approxLuminance(dark) {
		t.Errorf("Hovered(%v) = %v, want something lighter", dark, got)
	}

	light := color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF}
	if got := Hovered(light); approxLuminance(got) >= approxLuminance(light) {
		t.Errorf("Hovered(%v) = %v, want something darker", light, got)
	}
}

func TestHoveredKeepsTheAlphaItWasGiven(t *testing.T) {
	for a := 1; a <= 0xFF; a++ {
		in := color.NRGBA{R: 0x30, G: 0x60, B: 0x90, A: uint8(a)}
		if got := Hovered(in); got.A != in.A {
			t.Errorf("Hovered(%v).A = %v, want %v", in, got.A, in.A)
		}
	}
}

func TestHoveredGivesAnInvisibleControlSomethingToShow(t *testing.T) {
	// A control with nothing painted behind it has no colour to move, and
	// moving nothing is a hover that does not appear. The answer is a grey
	// wash, and it has to be visible: a zero alpha here is the bug.
	got := Hovered(color.NRGBA{})
	if got.A == 0 {
		t.Fatalf("Hovered of a transparent control = %v, which is still invisible", got)
	}
}

func TestMixRunsFromOneEndToTheOther(t *testing.T) {
	a := color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x00}
	b := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

	// The weight names the first argument, and 256 is not reachable in a byte,
	// so the far end is the one that reads oddly: a weight of zero is all of
	// the second colour, and 0xFF is nearly all of the first.
	if got := mix(a, b, 0); got != b {
		t.Errorf("mix(a, b, 0) = %v, want %v", got, b)
	}
	if got := mix(a, b, 0xFF); got.R > 0 {
		t.Errorf("mix(a, b, 0xFF) = %v, want it near %v", got, a)
	}

	// Nothing it produces can leave the byte it came from, which is what stops
	// a blend from wrapping to the opposite end of the range.
	for w := 0; w <= 0xFF; w++ {
		got := mix(color.NRGBA{R: 0x40, A: 0x40}, color.NRGBA{R: 0xC0, A: 0xC0}, uint8(w))
		if got.R < 0x40 || got.R > 0xC0 || got.A < 0x40 || got.A > 0xC0 {
			t.Fatalf("mix at weight %d = %v, which is outside the two colours", w, got)
		}
	}
}

func TestApproxLuminanceKeepsTheEndsAndTheOrder(t *testing.T) {
	if got := approxLuminance(color.NRGBA{A: 0xFF}); got != 0 {
		t.Errorf("approxLuminance(black) = %v, want 0", got)
	}
	if got := approxLuminance(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}); got != 0xFF {
		t.Errorf("approxLuminance(white) = %v, want 255", got)
	}

	// The same weighting as [RGBA.Luminance], and the same order of channels.
	// It is not the same number and is not meant to be: this one weights the
	// bytes as written, and Luminance weights the light they stand for. On a
	// mid grey the two are most of a hundred apart, and that is the price of
	// not decoding a curve to decide whether a label goes black or white.
	green := approxLuminance(color.NRGBA{G: 0xFF, A: 0xFF})
	red := approxLuminance(color.NRGBA{R: 0xFF, A: 0xFF})
	blue := approxLuminance(color.NRGBA{B: 0xFF, A: 0xFF})
	if !(green > red && red > blue) {
		t.Errorf("approxLuminance weights are green %v, red %v, blue %v, want them in that order", green, red, blue)
	}

	// Rising in every channel, so that no pair of colours compares backwards.
	for v := 1; v <= 0xFF; v++ {
		lo := approxLuminance(color.NRGBA{R: uint8(v - 1), G: uint8(v - 1), B: uint8(v - 1), A: 0xFF})
		hi := approxLuminance(color.NRGBA{R: uint8(v), G: uint8(v), B: uint8(v), A: 0xFF})
		if hi < lo {
			t.Fatalf("approxLuminance fell from %v to %v between grey %d and %d", lo, hi, v-1, v)
		}
	}
}
