package main

import "testing"

// TestTheConversionLandsOnKnownColours is the one fixture table in this module.
//
// The conversion is forty lines of arithmetic with eighteen constants, and a
// single mistyped digit moves every colour by an amount nobody notices until a
// border sits against the wrong background. Each row below is a value from the
// stylesheet this generator reads, and the answer beside it is what the same
// value resolves to in a browser.
//
// A tolerance of one is allowed per channel, and only that: the browser rounds
// the last step from float to byte, and so does this, and the two can land on
// either side of a half. Two would let a real error through.
func TestTheConversionLandsOnKnownColours(t *testing.T) {
	cases := []struct {
		name       string
		l, c, h, a float64
		want       Colour
	}{
		{"white", 1, 0, 0, 1, Colour{R: 255, G: 255, B: 255, A: 255}},
		{"near black", 0.145, 0, 0, 1, Colour{R: 10, G: 10, B: 10, A: 255}},
		{"the border grey", 0.922, 0, 0, 1, Colour{R: 229, G: 229, B: 229, A: 255}},
		{"destructive red", 0.577, 0.245, 27.325, 1, Colour{R: 231, G: 0, B: 11, A: 255}},
		{"the ring blue", 0.488, 0.243, 264.376, 1, Colour{R: 20, G: 71, B: 230, A: 255}},
		{"white at a tenth", 1, 0, 0, 0.1, Colour{R: 255, G: 255, B: 255, A: 26}},
	}

	// The three primaries anchor the table. They are the values every
	// published conversion agrees on, so a matrix constant off by a digit
	// moves one of them and the rest of the table becomes readable as a
	// consequence rather than as six independent guesses.
	primaries := []struct {
		name    string
		l, c, h float64
		want    Colour
	}{
		{"pure red", 0.6279, 0.2577, 29.23, Colour{R: 255, G: 0, B: 0, A: 255}},
		{"pure green", 0.8664, 0.2948, 142.5, Colour{R: 0, G: 255, B: 0, A: 255}},
		{"pure blue", 0.452, 0.3132, 264.05, Colour{R: 0, G: 0, B: 255, A: 255}},
	}
	for _, p := range primaries {
		t.Run(p.name, func(t *testing.T) {
			if got := oklchToRGB(p.l, p.c, p.h, 1); !within(got, p.want, 1) {
				t.Errorf("oklch(%g %g %g) = %+v, want %+v", p.l, p.c, p.h, got, p.want)
			}
		})
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := oklchToRGB(c.l, c.c, c.h, c.a)
			if !within(got, c.want, 1) {
				t.Errorf("oklch(%g %g %g / %g) = %+v, want %+v", c.l, c.c, c.h, c.a, got, c.want)
			}
		})
	}
}

// TestTheTransferFunctionIsPiecewise keeps the knee.
//
// Using the power curve everywhere is the common shortcut and it is wrong
// below 0.0031308, which is where the darkest few values live -- and the
// darkest few values are a border against a near-black background, which is
// the one place on a dark screen where being one step off is visible.
func TestTheTransferFunctionIsPiecewise(t *testing.T) {
	if got, want := encode(0.0), uint8(0); got != want {
		t.Errorf("encode(0) = %d, want %d", got, want)
	}
	if got, want := encode(1.0), uint8(255); got != want {
		t.Errorf("encode(1) = %d, want %d", got, want)
	}
	// Just under the knee: linear, so 0.003 * 12.92 * 255 rounds to 10.
	if got, want := encode(0.003), uint8(10); got != want {
		t.Errorf("encode(0.003) = %d, want %d -- the linear segment is gone", got, want)
	}
	// Well above it: the power curve, where the shortcut and the real answer
	// agree, so this row is the one that would still pass if the knee were
	// removed. It is here to say that on purpose.
	if got, want := encode(0.5), uint8(188); got != want {
		t.Errorf("encode(0.5) = %d, want %d", got, want)
	}
}

// TestAColourOutsideTheGamutIsClipped fixes the answer for a colour a screen
// cannot show, because the alternative is a channel that wraps and a red that
// comes out green.
func TestAColourOutsideTheGamutIsClipped(t *testing.T) {
	got := oklchToRGB(0.7, 0.4, 27, 1)
	if got.R != 255 {
		t.Errorf("a red past the gamut clipped to R=%d, want 255", got.R)
	}
	if got.G > 255 || got.B > 255 {
		t.Errorf("a channel escaped the byte: %+v", got)
	}
}

func within(got, want Colour, tolerance int) bool {
	return near(int(got.R), int(want.R), tolerance) &&
		near(int(got.G), int(want.G), tolerance) &&
		near(int(got.B), int(want.B), tolerance) &&
		near(int(got.A), int(want.A), tolerance)
}

func near(a, b, tolerance int) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tolerance
}
