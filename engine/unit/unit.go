/*
Package unit is the vocabulary every size on a screen is written in.

A size here is a size on a person's eye, not a count of the display's pixels.
Two devices of the same physical size with different pixel counts draw the
same control at the same apparent size, and neither the control nor the screen
that placed it has to know which device it is on.

There are three units, and only the first two are ever written by hand.

[Dp] is a length: a padding, a corner, a thickness, the height of a bar.

[Sp] is a text size. It is a length with whatever text scaling the person asked
their system for applied, so that somebody who asked for larger text gets it in
every control that draws a string, and does not get a layout whose boxes grew
with it.

The third is a device pixel, which is a plain int and a result rather than an
input. It is what a length becomes once a [Metric] has been applied, and it is
the only one of the three whose meaning changes from one device to the next: a
screen written in pixels is a screen that is correct on the display it was
written on.
*/
package unit

import "math"

type (
	// Dp is a length, in device independent pixels.
	//
	// One Dp has the same apparent size on every display. It is what a
	// control's padding, thickness, corner radius and height are written in,
	// and it is what keeps a control drawn on a dense display the size it is
	// on a sparse one rather than a fraction of it.
	Dp float32

	// Sp is a text size, in scaled pixels.
	//
	// It is a length plus the text scaling the person set on their device. It
	// is a type of its own, rather than the same one under a second name, so
	// that the compiler refuses a text size where a length belongs: the two
	// are the same number on every device where nobody touched the setting,
	// which is what would make the mistake invisible until it reached
	// somebody who did.
	Sp float32
)

// Metric converts lengths and text sizes to the pixels of one display.
//
// It travels by value. Two displays are two Metrics, and a window dragged from
// one to the other is handed a new one -- anything reading a single current
// density instead would see it change underneath a frame that was halfway laid
// out, and half the screen would come out at the old density.
//
// The zero Metric converts one to one, which is what makes it usable
// unconfigured: a test, or a tool that measures a frame without opening a
// window, has no density to supply and does not need one.
type Metric struct {
	// PxPerDp is how many device pixels one Dp is. Zero means one.
	PxPerDp float32
	// PxPerSp is how many device pixels one Sp is. Zero means one.
	PxPerSp float32
}

// Dp converts a length to whole device pixels, rounded to the nearest.
func (c Metric) Dp(v Dp) int {
	return pixels(density(c.PxPerDp), float32(v))
}

// Sp converts a text size to whole device pixels, rounded to the nearest.
func (c Metric) Sp(v Sp) int {
	return pixels(density(c.PxPerSp), float32(v))
}

// DpToSp converts a length to the text size that covers the same distance.
//
// It divides one density by the other rather than going by way of pixels, so
// that the answer keeps its fraction. A caller halves this, insets by it or
// hands it to a further conversion; a value already snapped to a whole pixel
// of this one display would carry that snap into everything computed from it.
//
// Converting a length straight to pixels and converting it through here first
// land on the same pixel or on the one beside it, and not always the same one.
// Each route rounds once at the end, so a value that falls exactly on a half
// after one route's arithmetic falls just under it after the other's.
func (c Metric) DpToSp(v Dp) Sp {
	return Sp(float32(v) * density(c.PxPerDp) / density(c.PxPerSp))
}

// SpToDp converts a text size to the length that covers the same distance. It
// is the inverse of [Metric.DpToSp], to the precision a single-precision float
// holds a density in.
func (c Metric) SpToDp(v Sp) Dp {
	return Dp(float32(v) * density(c.PxPerSp) / density(c.PxPerDp))
}

// PxToSp converts a pixel count to the text size that covers it.
//
// It keeps the fraction, for the reason [Metric.PxToDp] gives.
func (c Metric) PxToSp(v int) Sp {
	return Sp(float32(v) / density(c.PxPerSp))
}

// PxToDp converts a pixel count to the length that covers it.
//
// This is what reads a size the platform reports -- a window's width, a pointer
// position, a rectangle measured off the screen -- back into the vocabulary the
// rest of a screen is written in.
//
// It does not round, and the fraction it keeps is the part that carries a
// length back to the pixel it came from: [Metric.Dp] of this answer is the
// count that was passed in. Rounding here would cost up to half a pixel per
// trip, and a size that goes out and back several times in one frame would
// walk.
func (c Metric) PxToDp(v int) Dp {
	return Dp(float32(v) / density(c.PxPerDp))
}

// pixels converts a logical size at a given density to whole device pixels.
//
// Rounding rather than truncating, because these are sizes and not indices.
// Truncation biases every conversion towards zero, and a hairline written as a
// fraction of a unit truncates to no pixels at all -- a border that disappears
// on exactly the displays where it was thinnest.
//
// Halves go away from zero, which is what keeps a size and its negation the
// same size. Offsets and insets are signed: the gap above a control and the
// gap below it are one number with two signs, and a rule that sent both halves
// the same direction would give one of them a pixel the other does not get,
// putting the control off centre in code that says the two gaps are equal.
func pixels(pxPerUnit, v float32) int {
	return int(math.Round(float64(pxPerUnit) * float64(v)))
}

// density answers the pixels per unit to convert at.
//
// A stored zero is not a density but a field nobody filled in, and reading it
// as the number it is would multiply every size by nothing. A screen whose
// every control measures nought pixels is not a visibly wrong screen; it is an
// empty window, which reads as a fault in the drawing rather than in the
// Metric it was handed.
//
// Each field falls back on its own. A zero text density that borrowed the
// length density would make text scaling follow display density, and somebody
// on a dense display would get large text they never asked for.
func density(pxPerUnit float32) float32 {
	if pxPerUnit == 0 {
		return 1
	}
	return pxPerUnit
}
