package f32color

import (
	"image/color"
	"math"
	"testing"
)

// What this package does is arithmetic on the rasteriser's path, and the way it
// fails is quiet: a wrong constant still compiles, still draws, and produces a
// picture that is merely the wrong colour. So these are properties rather than
// examples -- an example pins one number somebody read off the screen, and the
// screen is what is being checked.
//
// The name of this file keeps it out of the way of a test file the original may
// gain later.

// samples is a colour per byte with the three channels far apart, so a
// conversion that swapped two of them is visible.
//
// Sweeping one channel and leaving the others at zero is what the inherited
// test did, and it cannot see a red-blue swap at all.
func samples(v int) (r, g, b uint8) {
	return uint8(v), uint8(0xFF - v), uint8((v * 7) % 0x100)
}

func TestEveryByteComesBackFromLinear(t *testing.T) {
	for a := 0; a <= 0xFF; a++ {
		for v := 0; v <= 0xFF; v++ {
			r, g, b := samples(v)
			want := color.NRGBA{R: r, G: g, B: b, A: uint8(a)}
			if a == 0 {
				// Premultiplying by nothing keeps nothing, and un-premultiplying
				// it back cannot invent what was multiplied away. A fully
				// transparent colour has no channels to compare.
				want = color.NRGBA{}
			}
			if got := LinearFromSRGB(color.NRGBA{R: r, G: g, B: b, A: uint8(a)}).SRGB(); got != want {
				t.Fatalf("LinearFromSRGB(%v).SRGB() = %v, want %v", color.NRGBA{R: r, G: g, B: b, A: uint8(a)}, got, want)
			}
		}
	}
}

func TestATransparentColourForgetsItsChannels(t *testing.T) {
	got := RGBA{R: 0.5, G: 0.25, B: 1, A: 0}.SRGB()
	if want := (color.NRGBA{}); got != want {
		t.Errorf("SRGB() of a transparent colour = %v, want %v", got, want)
	}
}

func TestBlackAndWhiteAreFixedPoints(t *testing.T) {
	for _, c := range []color.NRGBA{
		{A: 0xFF},
		{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	} {
		if got := LinearFromSRGB(c).SRGB(); got != c {
			t.Errorf("LinearFromSRGB(%v).SRGB() = %v, want %v", c, got, c)
		}
		if got := NRGBAToRGBA(c); got != (color.RGBA(c)) {
			t.Errorf("NRGBAToRGBA(%v) = %v, want %v", c, got, color.RGBA(c))
		}
	}

	// The ends of the curve, where a fixed point is a property of the formula
	// and not of a table lookup that happens to be right.
	for _, end := range []float32{0, 1} {
		if got := sRGBToLinear(end); got != end {
			t.Errorf("sRGBToLinear(%v) = %v, want %v", end, got, end)
		}
		if got := linearToSRGB(end); got != end {
			t.Errorf("linearToSRGB(%v) = %v, want %v", end, got, end)
		}
	}
	if srgb8ToLinear[0] != 0 || srgb8ToLinear[0xFF] != 1 {
		t.Errorf("the table runs %v..%v, want 0..1", srgb8ToLinear[0], srgb8ToLinear[0xFF])
	}
}

func TestAlphaNeverGoesThroughTheCurve(t *testing.T) {
	for a := 0; a <= 0xFF; a++ {
		r, g, b := samples(a)
		in := color.NRGBA{R: r, G: g, B: b, A: uint8(a)}

		if got, want := LinearFromSRGB(in).A, float32(a)/0xFF; got != want {
			t.Errorf("LinearFromSRGB(%v).A = %v, want %v", in, got, want)
		}
		if got := NRGBAToRGBA(in).A; got != uint8(a) {
			t.Errorf("NRGBAToRGBA(%v).A = %v, want %v", in, got, a)
		}
		if got := NRGBAToLinearRGBA(in).A; got != uint8(a) {
			t.Errorf("NRGBAToLinearRGBA(%v).A = %v, want %v", in, got, a)
		}
		if got := LinearFromSRGB(in).SRGB().A; got != uint8(a) {
			t.Errorf("LinearFromSRGB(%v).SRGB().A = %v, want %v", in, got, a)
		}
	}
}

func TestNoChannelOutrunsItsAlpha(t *testing.T) {
	for a := 0; a <= 0xFF; a++ {
		for v := 0; v <= 0xFF; v++ {
			r, g, b := samples(v)
			in := color.NRGBA{R: r, G: g, B: b, A: uint8(a)}
			got := NRGBAToLinearRGBA(in)
			if got.R > got.A || got.G > got.A || got.B > got.A {
				t.Fatalf("NRGBAToLinearRGBA(%v) = %v, which is not a premultiplied colour", in, got)
			}
		}
	}
}

// TestPremultiplyingInSRGBOverrunsItsAlpha pins what [NRGBAToRGBA] really
// answers, which is not a premultiplied colour by the standard library's
// definition.
//
// It premultiplies in the linear space and then encodes the product, and the
// encoding curve lifts every value between the ends -- so half of white comes
// back as 188 over an alpha of 128. The one caller wants three channels for a
// hex string and drops the alpha, which is why nothing has broken.
//
// This is pinned rather than fixed because fixing it changes what is drawn, and
// the screens this repository keeps are the record of what is drawn.
func TestPremultiplyingInSRGBOverrunsItsAlpha(t *testing.T) {
	in := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x80}
	got := NRGBAToRGBA(in)
	if got.R <= got.A {
		t.Fatalf("NRGBAToRGBA(%v) = %v; the overrun this test exists to record is gone, "+
			"and the approved screens have to be looked at before it is welcomed", in, got)
	}
}

func TestTheCurveIsItsOwnInverseWithinAByte(t *testing.T) {
	// One eight-bit step is the only accuracy anything downstream can see: the
	// result is written to a byte. The two directions are not exact inverses --
	// the encoding exponent is 0.41666 and the decoding one is 2.4 -- and this
	// is the budget that approximation has to stay inside.
	const step = 1.0 / 255.0

	var worst float64
	var at float32
	for i := 0; i <= 100000; i++ {
		v := float32(i) / 100000
		if d := math.Abs(float64(linearToSRGB(sRGBToLinear(v)) - v)); d > worst {
			worst, at = d, v
		}
	}
	t.Logf("the worst round trip is %g at %g, in a budget of %g", worst, at, step)
	if worst > step {
		t.Errorf("a round trip lost %g at %g, which is more than the %g a byte can hold", worst, at, step)
	}
}

// TestTheTableIsTheFormula is the gate on a duplicated formula.
//
// The generator carries its own copy of [sRGBToLinear], because it cannot
// import the package it is about to write a file into. Two copies of one
// formula drift, and the drift is silent: the table keeps answering, just not
// what the function next to it would have answered.
func TestTheTableIsTheFormula(t *testing.T) {
	for i := range srgb8ToLinear {
		if want := sRGBToLinear(float32(i) / 0xFF); srgb8ToLinear[i] != want {
			t.Errorf("srgb8ToLinear[%d] = %v, want %v; regenerate the table", i, srgb8ToLinear[i], want)
		}
	}
}

func TestTheTableOnlyRises(t *testing.T) {
	for i := 1; i < len(srgb8ToLinear); i++ {
		if srgb8ToLinear[i] <= srgb8ToLinear[i-1] {
			t.Fatalf("srgb8ToLinear[%d] = %v is not above [%d] = %v", i, srgb8ToLinear[i], i-1, srgb8ToLinear[i-1])
		}
	}
}

func TestAChannelPastTheEndsStopsAtThem(t *testing.T) {
	// The curve clamps, so this half has always held. It is asserted because
	// the clamp is the reason the byte conversion below it is safe, and a
	// clamp nobody checks is a clamp somebody simplifies away.
	got := RGBA{R: 8, G: -8, B: 0.5, A: 1}.SRGB()
	if want := (color.NRGBA{R: 0xFF, G: 0x00, B: 0xBC, A: 0xFF}); got != want {
		t.Errorf("a colour past both ends = %v, want %v", got, want)
	}
}

// TestAnAlphaOutsideTheRangeSaturatesInsteadOfWrapping is the one way this
// package could fail loudly.
//
// Converting a float to a byte is only defined by the language when the value
// fits, and alpha is the one channel no curve clamps on the way out: an alpha
// of 2 is 510 before the conversion, and on this machine that came back as 254
// -- a colour twice as solid as white arriving as an almost invisible one. An
// alpha of -1 came back as 2, which is the same failure pointing the other way.
//
// Nothing produces such a value today. That is exactly why nobody would find
// it, and why the answer is decided here rather than by whatever the platform
// happens to do with a conversion the language does not define.
func TestAnAlphaOutsideTheRangeSaturatesInsteadOfWrapping(t *testing.T) {
	if got := (RGBA{R: 1, G: 1, B: 1, A: 2}).SRGB(); got.A != 0xFF {
		t.Errorf("an alpha of 2 came out as %v, want 255", got.A)
	}
	if got := (RGBA{R: -1, G: -1, B: -1, A: -1}).SRGB(); got != (color.NRGBA{}) {
		t.Errorf("an alpha of -1 came out as %v, want nothing at all", got)
	}
}

func TestAnAlphaThatIsNotANumberShowsNothing(t *testing.T) {
	// There is no sensible colour to draw and no way to divide by it, and the
	// answer that matters is that there is an answer: no panic, and no byte
	// that wrapped to whatever the hardware left behind.
	nan := float32(math.NaN())
	if got := (RGBA{R: 1, G: 1, B: 1, A: nan}).SRGB(); got != (color.NRGBA{}) {
		t.Errorf("an alpha of NaN came out as %v, want nothing at all", got)
	}
}

func TestAChannelThatIsNotANumberComesOutVisible(t *testing.T) {
	// White rather than black, so that whoever produced it sees it. It is
	// pinned because either answer is defensible and only one of them is what
	// the curve does today.
	nan := float32(math.NaN())
	got := RGBA{R: nan, G: nan, B: nan, A: 1}.SRGB()
	if want := (color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}); got != want {
		t.Errorf("a colour made of NaN = %v, want %v", got, want)
	}
}

func TestLuminanceRunsBlackToWhite(t *testing.T) {
	if got := (RGBA{A: 1}).Luminance(); got != 0 {
		t.Errorf("black luminance = %v, want 0", got)
	}
	if got := (RGBA{R: 1, G: 1, B: 1, A: 1}).Luminance(); got != 1 {
		t.Errorf("white luminance = %v, want 1", got)
	}
	// Green carries most of it and blue almost none, which is the whole point
	// of weighting the channels rather than averaging them.
	green := RGBA{G: 1, A: 1}.Luminance()
	red := RGBA{R: 1, A: 1}.Luminance()
	blue := RGBA{B: 1, A: 1}.Luminance()
	if !(green > red && red > blue) {
		t.Errorf("luminance weights are green %v, red %v, blue %v, want them in that order", green, red, blue)
	}
}

func TestOpaqueOnlyTouchesAlpha(t *testing.T) {
	in := RGBA{R: 0.25, G: 0.5, B: 0.75, A: 0.1}
	got := in.Opaque()
	if want := (RGBA{R: 0.25, G: 0.5, B: 0.75, A: 1}); got != want {
		t.Errorf("Opaque() = %v, want %v", got, want)
	}
	if in.A != 0.1 {
		t.Errorf("Opaque() changed its receiver to %v", in)
	}
}

func TestArrayAndFloat32ReportTheFieldsInOrder(t *testing.T) {
	in := RGBA{R: 0.1, G: 0.2, B: 0.3, A: 0.4}
	if got := in.Array(); got != [4]float32{0.1, 0.2, 0.3, 0.4} {
		t.Errorf("Array() = %v", got)
	}
	if r, g, b, a := in.Float32(); r != in.R || g != in.G || b != in.B || a != in.A {
		t.Errorf("Float32() = %v %v %v %v, want %v", r, g, b, a, in)
	}
}

func TestAnOpaqueColourSkipsTheArithmeticAltogether(t *testing.T) {
	// Both conversions return the input unchanged at a full alpha, and that is
	// a promise rather than an optimisation: premultiplying by one and encoding
	// the product back would land a byte or two away for most inputs.
	for v := 0; v <= 0xFF; v++ {
		r, g, b := samples(v)
		in := color.NRGBA{R: r, G: g, B: b, A: 0xFF}
		if got := NRGBAToRGBA(in); got != color.RGBA(in) {
			t.Fatalf("NRGBAToRGBA(%v) = %v, want it unchanged", in, got)
		}
		if got := NRGBAToLinearRGBA(in); got != color.RGBA(in) {
			t.Fatalf("NRGBAToLinearRGBA(%v) = %v, want it unchanged", in, got)
		}
		if got := RGBAToNRGBA(color.RGBA(in)); got != in {
			t.Fatalf("RGBAToNRGBA(%v) = %v, want it unchanged", in, got)
		}
	}
}

var sink RGBA

// BenchmarkLinearFromSRGB is the reason tables.go exists.
//
// The conversion runs per colour per frame, and the three cases are the three
// shapes a caller arrives in: nothing to premultiply, something to premultiply,
// and nothing to draw. If the table ever stops paying for itself here, it stops
// being worth generating.
func BenchmarkLinearFromSRGB(b *testing.B) {
	for _, alpha := range []struct {
		name string
		a    uint8
	}{
		{"opaque", 0xFF},
		{"translucent", 0x50},
		{"transparent", 0x00},
	} {
		b.Run(alpha.name, func(b *testing.B) {
			for i := 0; b.Loop(); i++ {
				sink = LinearFromSRGB(color.NRGBA{R: byte(i), G: byte(i >> 8), B: byte(i >> 16), A: alpha.a})
			}
		})
	}
}
