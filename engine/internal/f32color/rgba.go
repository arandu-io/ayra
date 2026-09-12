// Package f32color is the colour arithmetic underneath a drawn frame.
//
// Two spaces meet here, and telling them apart is the whole reason the package
// exists. A byte in a picture is sRGB: a number bent by a curve so that the
// 256 steps land where an eye can still tell them apart, most of them crowded
// near black. Light does not work that way -- two lamps are twice one lamp --
// and everything that blends, fades, covers or antialiases is adding light. A
// blend computed on the bent numbers is wrong in a way that looks like a
// haloed edge or a grey that reads too dark, and it is wrong quietly: it
// compiles, it draws, and the picture is merely a little off.
//
// So colours arrive as bytes, are straightened on the way in, do their
// arithmetic as float32, and are bent again on the way out. [RGBA] is the
// straightened form. Everything that takes or returns an [image/color] value
// is a boundary, and the conversion happens there rather than wherever
// somebody remembered.
//
// It is premultiplied throughout: each channel already carries its alpha. That
// makes compositing a multiply and an add instead of a special case per
// operation, and it is the form the GPU wants.
package f32color

import (
	"image/color"
	"math"
)

//go:generate go run ./f32colorgen -out tables.go

// RGBA is a colour the rasteriser can do arithmetic on: linear, premultiplied,
// one float32 per channel.
//
// It is a struct of four floats rather than a packed word because it is read
// far more often than it is built -- a shader uniform, a clear colour, a
// gradient stop -- and every read of a packed word would be a shift and a mask
// to get back to the number that was put in.
type RGBA struct {
	R, G, B, A float32
}

// Array returns the channels in the order a graphics API expects them.
//
// It exists because a driver wants a contiguous four, and taking the address
// of the struct would hand it the memory layout as a promise. This makes the
// order a decision written once.
func (c RGBA) Array() [4]float32 {
	return [4]float32{c.R, c.G, c.B, c.A}
}

// Float32 returns the channels one at a time.
func (c RGBA) Float32() (r, g, b, a float32) {
	return c.R, c.G, c.B, c.A
}

// SRGB bends the colour back into the space a picture is written in, undoing
// the premultiply on the way.
//
// An alpha that is not above zero answers nothing at all. It is not only the
// division that has to be avoided: premultiplying by nothing keeps nothing, so
// there are no channels left to recover and any number produced from them
// would be invented.
func (c RGBA) SRGB() color.NRGBA {
	// Written this way round rather than as a comparison against zero, because
	// it also has to catch an alpha that is not a number -- which would divide
	// three channels into nonsense and convert the nonsense to bytes.
	if !(c.A > 0) {
		return color.NRGBA{}
	}
	return color.NRGBA{
		R: encode(linearToSRGB(c.R / c.A)),
		G: encode(linearToSRGB(c.G / c.A)),
		B: encode(linearToSRGB(c.B / c.A)),
		A: encode(c.A),
	}
}

// Luminance is how bright the colour reads, from 0 at black to 1 at white.
//
// The three weights are not a third each: the eye takes most of its brightness
// from green and almost none from blue, and an average would call a saturated
// blue as bright as a saturated green. It is what decides whether a label goes
// black or white over a given ground.
//
// The weights are the ones the accessibility contrast rules are defined with,
// so a contrast figure computed from this is the figure those rules mean.
func (c RGBA) Luminance() float32 {
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

// Opaque returns the colour at full alpha.
//
// The channels are left as they are rather than divided back out, because the
// callers that want this want the colour a surface is cleared to, and a clear
// has nothing behind it to blend with.
func (c RGBA) Opaque() RGBA {
	c.A = 1
	return c
}

// LinearFromSRGB straightens a colour out of the space a picture is written in
// and premultiplies it.
//
// The three channels come from the table rather than from [sRGBToLinear],
// because there are only 256 answers and this runs per colour per frame. The
// table is generated from that same function, and a test in this package fails
// if the two ever disagree.
func LinearFromSRGB(col color.NRGBA) RGBA {
	a := float32(col.A) / 0xFF
	return RGBA{
		R: srgb8ToLinear[col.R] * a,
		G: srgb8ToLinear[col.G] * a,
		B: srgb8ToLinear[col.B] * a,
		A: a,
	}
}

// NRGBAToRGBA premultiplies a colour and writes the result back as bytes in the
// space it came from.
//
// The multiply happens straightened, which is the point: premultiplying the
// bent numbers directly would darken everything that is not fully opaque.
//
// What comes back is not a premultiplied colour by the standard library's
// definition, and it must not be handed to anything that blends. Bending the
// product back lifts it, so half of white leaves here as 188 over an alpha of
// 128 -- a channel above its own alpha, which [image/color] forbids. The one
// caller wants three channels for a platform that names colours in hex and
// carries the alpha separately.
func NRGBAToRGBA(col color.NRGBA) color.RGBA {
	if col.A == 0xFF {
		// Nothing to premultiply, and the two conversions are not exact
		// inverses: running an opaque colour through them both would move most
		// inputs by a byte for no reason.
		return color.RGBA(col)
	}
	c := LinearFromSRGB(col)
	return color.RGBA{
		R: encode(linearToSRGB(c.R)),
		G: encode(linearToSRGB(c.G)),
		B: encode(linearToSRGB(c.B)),
		A: col.A,
	}
}

// NRGBAToLinearRGBA premultiplies a colour and leaves it straightened, written
// back as bytes.
//
// Unlike [NRGBAToRGBA] this is a premultiplied colour in the sense the standard
// library means: no channel comes out above its alpha. It is what a rasteriser
// that composites for itself wants handed to it, at the cost of the eight bits
// now holding linear values, where the steps near black are further apart than
// the eye would like.
func NRGBAToLinearRGBA(col color.NRGBA) color.RGBA {
	if col.A == 0xFF {
		return color.RGBA(col)
	}
	c := LinearFromSRGB(col)
	return color.RGBA{R: encode(c.R), G: encode(c.G), B: encode(c.B), A: col.A}
}

// RGBAToNRGBA undoes the premultiply of a colour whose bytes are in the space a
// picture is written in.
//
// It is the way back from [NRGBAToRGBA] and it inherits that function's idea of
// what premultiplied means, so it is the wrong tool for a colour that came from
// anywhere else -- the standard library's own premultiplied values included.
func RGBAToNRGBA(col color.RGBA) color.NRGBA {
	if col.A == 0xFF {
		return color.NRGBA(col)
	}
	return RGBA{
		R: sRGBToLinear(float32(col.R) / 0xFF),
		G: sRGBToLinear(float32(col.G) / 0xFF),
		B: sRGBToLinear(float32(col.B) / 0xFF),
		A: float32(col.A) / 0xFF,
	}.SRGB()
}

// linearToSRGB bends a straightened channel back for display.
//
// The exponent is 0.41666 and not one over 2.4, which is what [sRGBToLinear]
// undoes. It is the constant the specification this curve is quoted from
// writes out, the two directions are therefore not exact inverses, and the
// worst a round trip loses is far inside the step a byte can hold. Correcting
// it would change every colour this package has ever produced, so it stays and
// a test holds the error to that budget.
//
// The straight segment near black is not a smoothing of the corner. A power
// curve there compresses so hard that the first several steps of the range all
// land on the same byte, and the segment is what keeps them apart.
func linearToSRGB(c float32) float32 {
	switch {
	case c <= 0:
		return 0
	case c < 0.0031308:
		return 12.92 * c
	case c < 1:
		return 1.055*float32(math.Pow(float64(c), 0.41666)) - 0.055
	}
	// Above one, and also whatever is not a number: there is nothing brighter
	// than white to return, and a colour that arrives broken is better seen
	// than hidden.
	return 1
}

// sRGBToLinear straightens a channel out of the space a picture is written in.
func sRGBToLinear(c float32) float32 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return float32(math.Pow(float64((c+0.055)/1.055), 2.4))
}

// encode turns a channel into the byte a picture carries.
//
// The ends are answered rather than computed, and that is the whole reason this
// is a function. Converting a float to an integer is only defined by the
// language when the value fits, so a channel that arithmetic pushed past one
// used to wrap: an alpha of 2 is 510 before the conversion and came back as
// 254, turning a colour twice as solid as white into an almost invisible one.
// Deciding it here makes the answer the same on every machine.
//
// Inside the range it is the same rounding it always was -- scale, add a half,
// truncate -- so nothing that was already valid moves.
func encode(v float32) uint8 {
	switch {
	case v >= 1:
		return 0xFF
	case !(v > 0):
		// Zero, negative, or not a number.
		return 0
	}
	return uint8(v*255 + 0.5)
}
