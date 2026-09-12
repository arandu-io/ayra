package unit_test

import (
	"testing"

	"github.com/arandu-io/ayra/engine/unit"
)

// What this package promises is a number, and these tests are that number.
//
// Nothing here draws anything, and that is the point: every length and every
// text size on a screen passes through one of six conversions, and a fault in
// one of them is not a control that looks wrong -- it is every control on every
// display that is not the one the fault was written on. The cases below are
// densities real displays report, including the fractional ones, because the
// integer ones hide rounding entirely.

// densities are the pixels-per-unit pairs a window is handed in practice, plus
// the pairs where the two units disagree.
var densities = []struct {
	name   string
	metric unit.Metric
}{
	{"one to one", unit.Metric{PxPerDp: 1, PxPerSp: 1}},
	{"one and a half", unit.Metric{PxPerDp: 1.5, PxPerSp: 1.5}},
	{"double", unit.Metric{PxPerDp: 2, PxPerSp: 2}},
	{"two and five eighths", unit.Metric{PxPerDp: 2.625, PxPerSp: 2.625}},
	{"triple", unit.Metric{PxPerDp: 3, PxPerSp: 3}},
	{"larger text", unit.Metric{PxPerDp: 2, PxPerSp: 3}},
	{"smaller text", unit.Metric{PxPerDp: 3, PxPerSp: 2}},
	{"text scaled a fraction", unit.Metric{PxPerDp: 1.5, PxPerSp: 1.75}},
	{"unset", unit.Metric{}},
	{"lengths only", unit.Metric{PxPerDp: 2}},
	{"text only", unit.Metric{PxPerSp: 2}},
}

// TestAnUnsetMetricConvertsOneToOne fixes the value of the zero Metric, which
// is the one that costs most to get wrong. A zero read as the number it is
// multiplies every length by nothing, and a screen whose every control measures
// nought pixels is an empty window -- which reads as a drawing fault rather
// than as a Metric nobody filled in.
func TestAnUnsetMetricConvertsOneToOne(t *testing.T) {
	var m unit.Metric

	if got := m.Dp(12); got != 12 {
		t.Errorf("Dp(12) = %d, want 12", got)
	}
	if got := m.Sp(14); got != 14 {
		t.Errorf("Sp(14) = %d, want 14", got)
	}
	if got := m.PxToDp(12); got != 12 {
		t.Errorf("PxToDp(12) = %v, want 12", got)
	}
	if got := m.PxToSp(14); got != 14 {
		t.Errorf("PxToSp(14) = %v, want 14", got)
	}
	if got := m.DpToSp(12); got != 12 {
		t.Errorf("DpToSp(12) = %v, want 12", got)
	}
	if got := m.SpToDp(14); got != 14 {
		t.Errorf("SpToDp(14) = %v, want 14", got)
	}
}

// TestAnUnsetFieldFallsBackOnItsOwn is the half-filled Metric: a platform that
// reports a density for lengths and nothing for text, which is every platform
// where the person never touched the text scaling. Each field has to fall back
// by itself, because a fallback that read the other one would make text scaling
// follow display density -- and then somebody on a dense display gets large
// text they never asked for.
func TestAnUnsetFieldFallsBackOnItsOwn(t *testing.T) {
	lengths := unit.Metric{PxPerDp: 2}
	if got := lengths.Dp(10); got != 20 {
		t.Errorf("Dp(10) at two pixels per unit = %d, want 20", got)
	}
	if got := lengths.Sp(10); got != 10 {
		t.Errorf("Sp(10) with text density unset = %d, want 10", got)
	}
	if got := lengths.DpToSp(10); got != 20 {
		t.Errorf("DpToSp(10) with text density unset = %v, want 20", got)
	}

	text := unit.Metric{PxPerSp: 2}
	if got := text.Sp(10); got != 20 {
		t.Errorf("Sp(10) at two pixels per unit = %d, want 20", got)
	}
	if got := text.Dp(10); got != 10 {
		t.Errorf("Dp(10) with length density unset = %d, want 10", got)
	}
	if got := text.SpToDp(10); got != 20 {
		t.Errorf("SpToDp(10) with length density unset = %v, want 20", got)
	}
}

// TestAConversionScalesByItsOwnDensity fixes that the two units are converted
// by the two fields, and not by whichever was read first. Wiring a text size
// through the length density is the defect that passes on every display where
// the two are equal, which is most of them.
func TestAConversionScalesByItsOwnDensity(t *testing.T) {
	m := unit.Metric{PxPerDp: 2, PxPerSp: 3}

	if got := m.Dp(10); got != 20 {
		t.Errorf("Dp(10) = %d, want 20", got)
	}
	if got := m.Sp(10); got != 30 {
		t.Errorf("Sp(10) = %d, want 30", got)
	}
	if got := m.PxToDp(20); got != 10 {
		t.Errorf("PxToDp(20) = %v, want 10", got)
	}
	if got := m.PxToSp(30); got != 10 {
		t.Errorf("PxToSp(30) = %v, want 10", got)
	}
	if got := m.SpToDp(10); got != 15 {
		t.Errorf("SpToDp(10) = %v, want 15", got)
	}
	if got := m.DpToSp(15); got != 10 {
		t.Errorf("DpToSp(15) = %v, want 10", got)
	}
}

// TestPixelsAreRoundedToTheNearest rules out truncation, which is the
// conversion somebody writes when they reach for a cast. Truncation biases
// every length towards zero, and a hairline written as a fraction of a unit
// truncates to no pixels at all -- a border that disappears on exactly the
// displays it was thinnest on.
//
// The halves are here to pin which way they go, because a rounding rule that
// is nearly right is a rule nobody notices is a rule.
func TestPixelsAreRoundedToTheNearest(t *testing.T) {
	m := unit.Metric{PxPerDp: 1, PxPerSp: 1}

	cases := []struct {
		in   float32
		want int
	}{
		{0, 0},
		{0.4, 0},
		{0.5, 1},
		{0.6, 1},
		{1.5, 2},
		{2.5, 3},
		{2.49, 2},
		{9.99, 10},
	}

	for _, c := range cases {
		if got := m.Dp(unit.Dp(c.in)); got != c.want {
			t.Errorf("Dp(%v) = %d, want %d", c.in, got, c.want)
		}
		if got := m.Sp(unit.Sp(c.in)); got != c.want {
			t.Errorf("Sp(%v) = %d, want %d", c.in, got, c.want)
		}
	}

	// Fractional densities are where the rounding actually happens, since a
	// whole one leaves nothing to round.
	fractional := unit.Metric{PxPerDp: 1.5, PxPerSp: 1.5}
	if got := fractional.Dp(1); got != 2 {
		t.Errorf("Dp(1) at one and a half pixels per unit = %d, want 2", got)
	}
	if got := fractional.Dp(3); got != 5 {
		t.Errorf("Dp(3) at one and a half pixels per unit = %d, want 5", got)
	}
	if got := fractional.Sp(3); got != 5 {
		t.Errorf("Sp(3) at one and a half pixels per unit = %d, want 5", got)
	}
}

// TestALengthAndItsNegationTakeTheSameRoom is why the halves round away from
// zero rather than upwards. Insets and offsets are signed: the gap above a
// control and the gap below it are the same number with opposite signs, and a
// rule that sent both halves the same direction would give one of them a pixel
// the other does not get. A control one pixel off centre is the fault nobody
// finds by reading, because the code says the two gaps are equal.
func TestALengthAndItsNegationTakeTheSameRoom(t *testing.T) {
	for _, d := range densities {
		t.Run(d.name, func(t *testing.T) {
			for i := -400; i <= 400; i++ {
				v := float32(i) / 8

				above, below := d.metric.Dp(unit.Dp(v)), d.metric.Dp(unit.Dp(-v))
				if below != -above {
					t.Fatalf("a length of %v is %d px and one of %v is %d px", v, above, -v, below)
				}

				above, below = d.metric.Sp(unit.Sp(v)), d.metric.Sp(unit.Sp(-v))
				if below != -above {
					t.Fatalf("a text size of %v is %d px and one of %v is %d px", v, above, -v, below)
				}
			}
		})
	}
}

// TestAPixelCountSurvivesTheTripThroughALength is the guarantee that lets a
// size measured off the platform -- a window's width, a pointer position, a
// rectangle the compositor handed over -- be read into this vocabulary, carried
// through a layout and converted back without moving.
//
// It is why the conversions out of pixels keep their fraction instead of
// rounding. A conversion that rounded on the way out would lose up to half a
// pixel per trip, and a length that goes out and back several times in one
// frame would walk.
func TestAPixelCountSurvivesTheTripThroughALength(t *testing.T) {
	for _, d := range densities {
		t.Run(d.name, func(t *testing.T) {
			for px := -300; px <= 2000; px++ {
				if got := d.metric.Dp(d.metric.PxToDp(px)); got != px {
					t.Fatalf("%d px through a length came back %d", px, got)
				}
				if got := d.metric.Sp(d.metric.PxToSp(px)); got != px {
					t.Fatalf("%d px through a text size came back %d", px, got)
				}
			}
		})
	}
}

// TestALengthSurvivesTheTripThroughPixels is the other direction, and it is
// the one that cannot be exact: a length becomes a whole pixel on the way out,
// so what comes back is the length the display can actually draw rather than
// the one that was asked for.
//
// What is fixed here is how far that may be. One rounding moves a size by at
// most half a pixel, so that is the whole of what a trip out and back may cost
// -- a conversion out that had lost the density, or applied it twice, is out by
// a factor and lands nowhere near.
//
// At a whole-numbered density a whole length has nothing to lose and comes back
// exactly, which is the case a layout is usually drawn in and the reason a
// padding does not drift a fraction each time it passes through the platform.
func TestALengthSurvivesTheTripThroughPixels(t *testing.T) {
	whole := func(v float32) bool { return v == float32(int(v)) }

	for _, d := range densities {
		t.Run(d.name, func(t *testing.T) {
			for i := 1; i <= 200; i++ {
				length := unit.Dp(i)
				px := d.metric.Dp(length)
				back := d.metric.PxToDp(px)
				if off := abs32(float32(back-length)) * pxPerDp(d.metric); off > bound(px) {
					t.Fatalf("a length of %v came back %v, %v px away", length, back, off)
				}

				size := unit.Sp(i)
				pxSize := d.metric.Sp(size)
				backSize := d.metric.PxToSp(pxSize)
				if off := abs32(float32(backSize-size)) * pxPerSp(d.metric); off > bound(pxSize) {
					t.Fatalf("a text size of %v came back %v, %v px away", size, backSize, off)
				}

				if whole(pxPerDp(d.metric)) && back != length {
					t.Fatalf("a length of %v came back %v at a whole density", length, back)
				}
				if whole(pxPerSp(d.metric)) && backSize != size {
					t.Fatalf("a text size of %v came back %v at a whole density", size, backSize)
				}
			}
		})
	}
}

// TestTheTwoUnitsAreInverseConversions fixes that a length turned into a text
// size is the same length, and that the pair is not two independent formulas
// that happen to look symmetric. One of them written with its densities the
// wrong way round is a conversion that is correct on every display where the
// two densities are equal.
//
// The tolerance is relative, and small: both values are held in a float of
// single precision, and a value that goes through a multiplication and a
// division comes back within the last digit of itself, not on it.
func TestTheTwoUnitsAreInverseConversions(t *testing.T) {
	const tolerance = 1e-5

	for _, d := range densities {
		t.Run(d.name, func(t *testing.T) {
			for i := 1; i <= 200; i++ {
				length := unit.Dp(i)
				if back := d.metric.SpToDp(d.metric.DpToSp(length)); relative(float32(back), float32(length)) > tolerance {
					t.Fatalf("a length of %v came back %v", length, back)
				}

				size := unit.Sp(i)
				if back := d.metric.DpToSp(d.metric.SpToDp(size)); relative(float32(back), float32(size)) > tolerance {
					t.Fatalf("a text size of %v came back %v", size, back)
				}
			}
		})
	}
}

// TestBothRoutesToPixelsAgree is the promise the two units make together: a
// length converted straight to pixels and the same length converted to a text
// size first land on the same pixel, or on the one beside it.
//
// They cannot be made to land on the same one always, and this test says so
// rather than pretending. Each route rounds once at the end, so a value that
// falls exactly on a half after one route's arithmetic falls just under it
// after the other's, and the two go to different sides. One pixel is the whole
// of the disagreement; a route that was wired to the wrong density is out by a
// factor, which is what this catches.
func TestBothRoutesToPixelsAgree(t *testing.T) {
	// Where the arithmetic divides cleanly there is nothing to disagree
	// about, and the two routes land on the pixel together.
	clean := unit.Metric{PxPerDp: 2, PxPerSp: 3}
	if got, want := clean.Sp(clean.DpToSp(5)), clean.Dp(5); got != want {
		t.Errorf("a length of 5 is %d px through a text size and %d px direct", got, want)
	}
	if got, want := clean.Dp(clean.SpToDp(5)), clean.Sp(5); got != want {
		t.Errorf("a text size of 5 is %d px through a length and %d px direct", got, want)
	}

	for _, d := range densities {
		t.Run(d.name, func(t *testing.T) {
			for i := -400; i <= 1600; i++ {
				v := float32(i) / 4

				direct := d.metric.Dp(unit.Dp(v))
				viaText := d.metric.Sp(d.metric.DpToSp(unit.Dp(v)))
				if diff := abs(direct - viaText); diff > 1 {
					t.Fatalf("a length of %v is %d px direct and %d px through a text size", v, direct, viaText)
				}

				direct = d.metric.Sp(unit.Sp(v))
				viaLength := d.metric.Dp(d.metric.SpToDp(unit.Sp(v)))
				if diff := abs(direct - viaLength); diff > 1 {
					t.Fatalf("a text size of %v is %d px direct and %d px through a length", v, direct, viaLength)
				}
			}
		})
	}
}

// TestAMetricIsCopiedRatherThanShared fixes that a Metric is a value. A window
// dragged from one display to the next is handed a new Metric, and anything
// holding the old one has to keep converting at the old density until it is
// handed the new one -- a shared Metric would change under a frame that was
// halfway through being laid out, and half the screen would be at one density.
func TestAMetricIsCopiedRatherThanShared(t *testing.T) {
	original := unit.Metric{PxPerDp: 2, PxPerSp: 2}

	moved := original
	moved.PxPerDp = 3
	moved.PxPerSp = 3

	if got := original.Dp(10); got != 20 {
		t.Errorf("the original converted 10 to %d px after the copy changed, want 20", got)
	}
	if got := moved.Dp(10); got != 30 {
		t.Errorf("the copy converted 10 to %d px, want 30", got)
	}
}

// bound is the most a size may move on a trip out to pixels and back: half a
// pixel for the rounding, plus the seven digits a single-precision float holds
// the answer in.
//
// The second part is not padding. Several of these trips land exactly on the
// half -- 130 units at one and three quarter pixels each is 227.5 -- and a
// bound without it would read the float's last digit as a conversion that
// overshot.
func bound(px int) float32 {
	const precision = 1e-6
	return 0.5 + precision*abs32(float32(px))
}

// pxPerDp and pxPerSp are the densities a Metric actually converts at, with
// the unset field read as one. The tolerance a trip through pixels is allowed
// is a fraction of a pixel, so a test that needs it needs the density, and an
// unset field would otherwise divide by nothing.
func pxPerDp(m unit.Metric) float32 {
	if m.PxPerDp == 0 {
		return 1
	}
	return m.PxPerDp
}

func pxPerSp(m unit.Metric) float32 {
	if m.PxPerSp == 0 {
		return 1
	}
	return m.PxPerSp
}

// relative answers how far apart two values are, as a fraction of the second.
func relative(got, want float32) float32 {
	if want == 0 {
		return abs32(got)
	}
	return abs32(got-want) / abs32(want)
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
