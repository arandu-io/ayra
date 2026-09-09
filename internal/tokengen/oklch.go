package main

import "math"

// Colour is one resolved colour, in the 8-bit sRGB the drawing layer takes.
type Colour struct {
	R, G, B, A uint8
}

// oklchToRGB converts one colour from OKLCH to 8-bit sRGB.
//
// The conversion is written out rather than taken from a library for the
// reason the rest of this module has one dependency: it is forty lines of
// arithmetic with a known answer, and a table of known answers guards it.
//
// The three steps are OKLCH to OKLab, OKLab to linear sRGB through the
// published matrices, and linear to gamma-encoded sRGB. Each is reversible on
// paper, which is what makes the fixture table meaningful: a wrong constant
// moves every colour, and moving every colour is what the table sees.
//
// l is 0..1, c is chroma in the same scale, h is degrees. A value outside the
// sRGB gamut is clipped per channel, because the target is a screen and a
// screen has no answer for a colour it cannot show.
func oklchToRGB(l, c, h, alpha float64) Colour {
	hr := h * math.Pi / 180
	a := c * math.Cos(hr)
	b := c * math.Sin(hr)

	// OKLab to the intermediate cone responses, then cubed.
	lc := l + 0.3963377774*a + 0.2158037573*b
	mc := l - 0.1055613458*a - 0.0638541728*b
	sc := l - 0.0894841775*a - 1.2914855480*b

	lc, mc, sc = lc*lc*lc, mc*mc*mc, sc*sc*sc

	// Cone responses to linear sRGB.
	r := +4.0767416621*lc - 3.3077115913*mc + 0.2309699292*sc
	g := -1.2684380046*lc + 2.6097574011*mc - 0.3413193965*sc
	bl := -0.0041960863*lc - 0.7034186147*mc + 1.7076147010*sc

	return Colour{
		R: encode(r),
		G: encode(g),
		B: encode(bl),
		A: uint8(math.Round(clamp(alpha) * 255)),
	}
}

// encode applies the sRGB transfer function and lands on a byte.
//
// The piecewise form is the standard one: linear below the knee, a power curve
// above it. Using the power curve everywhere is the common shortcut, and it is
// visibly wrong in the darkest few values -- which is exactly where a border
// against a near-black background lives.
func encode(v float64) uint8 {
	v = clamp(v)
	if v <= 0.0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return uint8(math.Round(clamp(v) * 255))
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
