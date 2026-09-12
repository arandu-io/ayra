package f32color

import "image/color"

// What follows tints a colour to say what state a control is in, and it works
// on the bytes without straightening them first.
//
// That is a deliberate trade and not an oversight. These run on colours a
// person chose, to produce a shade that only has to read as "not now" or "under
// the pointer"; the arithmetic is integer, exact, and costs nothing, where the
// correct version is two curve evaluations per channel to land somewhere the
// eye cannot tell from this. The price is that the shift is not perceptually
// even across the range -- a dark colour moves further than a light one for the
// same step -- and [Hovered] is built around that rather than against it.
//
// Nothing here belongs on the rasteriser's path. The moment one of these is
// used to compute a pixel rather than to pick a colour, it is the wrong tool.

// MulAlpha scales a colour's alpha by another, as a fraction of full.
//
// Both are bytes and the result is a byte, so it is always a fade: applying it
// twice can only make a colour fainter, which is what lets a disabled control
// inside a fading panel be both without either knowing about the other.
func MulAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	c.A = uint8(uint32(c.A) * uint32(alpha) / 0xFF)
	return c
}

// Disabled drains a colour of both its saturation and its solidity.
//
// Two changes rather than one, because either alone is ambiguous: a colour that
// only faded is a colour on a lighter ground, and a colour that only greyed is
// a control that is simply grey. Together they read as unavailable.
//
// Blending towards the colour's own brightness is what drains it, and it is
// the reason no palette here has to name a disabled colour for every variant.
// A palette that did would be one entry per variant, each a chance to be wrong,
// and a grey that means "not now" only in the design somebody wrote it for.
func Disabled(c color.NRGBA) color.NRGBA {
	const towardsGrey = 80

	lum := approxLuminance(c)
	greyed := mix(c, color.NRGBA{R: lum, G: lum, B: lum, A: c.A}, towardsGrey)
	return MulAlpha(greyed, 128+32)
}

// Hovered moves a colour away from where it already is: dark colours towards
// white, light colours towards black.
//
// One direction would not do. A fixed lightening is invisible on a colour that
// is nearly white and a fixed darkening is invisible on one that is nearly
// black, and a palette has both -- so the direction is chosen per colour, from
// the colour.
func Hovered(c color.NRGBA) color.NRGBA {
	if c.A == 0 {
		// A control with nothing painted behind it has no colour to move, and
		// moving nothing is a hover that never appears. A grey wash is the only
		// answer that works over whatever it turns out to be sitting on.
		return color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0x44}
	}

	const step = 0x20

	towards := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: c.A}
	if approxLuminance(c) > 128 {
		towards = color.NRGBA{A: c.A}
	}
	return mix(towards, c, step)
}

// mix blends c1 into c2, where a is how much of c1 to take out of 256.
//
// The weight is out of 256 and a byte stops at 255, so the far end is not
// reachable and the near end is the one that reads backwards: a weight of zero
// is all of c2. Callers here pass small weights, which is the only shape this
// is used in.
func mix(c1, c2 color.NRGBA, a uint8) color.NRGBA {
	ai := int(a)
	blend := func(v1, v2 uint8) uint8 {
		return uint8((int(v1)*ai + int(v2)*(256-ai)) / 256)
	}
	return color.NRGBA{
		R: blend(c1.R, c2.R),
		G: blend(c1.G, c2.G),
		B: blend(c1.B, c2.B),
		A: blend(c1.A, c2.A),
	}
}

// approxLuminance is the brightness of a colour taken from its bytes as
// written, without straightening them first.
//
// It is not [RGBA.Luminance] with a rounding error: on a mid grey the two are
// most of a hundred apart, because one weights the light a colour stands for
// and this weights the number that stands for it. It is right for what it is
// used for -- which of two directions to tint in -- and wrong for anything a
// person is told, such as a contrast figure.
//
// The weights are the same three, scaled up so the whole sum is integer, and
// the divisor is their total rather than 65536 so that white comes back as
// exactly 255.
func approxLuminance(c color.NRGBA) uint8 {
	const (
		r     = 13933 // 0.2126 * 256 * 256
		g     = 46871 // 0.7152 * 256 * 256
		b     = 4732  // 0.0722 * 256 * 256
		total = r + g + b
	)
	return uint8((r*int(c.R) + g*int(c.G) + b*int(c.B)) / total)
}
