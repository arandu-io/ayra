package f32_test

import (
	"image"
	"math"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
)

// tolerance is how far apart two coordinates may be and still count as the same
// place.
//
// Exact equality is the wrong question for anything that went through a sine or
// a division. These are float32, and a quarter turn lands a hair off the axis it
// should land on. The value is far below one device pixel on any display, so two
// points that pass this draw on top of each other.
const tolerance = 1e-5

// near reports whether two points are within tolerance of each other.
func near(a, b f32.Point) bool {
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	return math.Hypot(dx, dy) < tolerance
}

func TestPt(t *testing.T) {
	p := f32.Pt(3, -4)
	if p.X != 3 || p.Y != -4 {
		t.Errorf("Pt(3, -4) = %v, want {3 -4}", p)
	}
	if p != (f32.Point{X: 3, Y: -4}) {
		t.Errorf("Pt and a literal disagree: %v", p)
	}
}

// TestPointIsFloatNotInt fixes the one thing a caller has to know before using
// this package: the coordinates are float32, and a fraction survives.
//
// The name is one letter away from the integer point of package image, and the
// mistake it guards is silent -- integer coordinates would round the half away
// and nothing would report it.
func TestPointIsFloatNotInt(t *testing.T) {
	p := f32.Pt(0.5, 0.25)
	if p.X != 0.5 || p.Y != 0.25 {
		t.Errorf("Pt(0.5, 0.25) = %v, want the fractions kept", p)
	}
	if got := p.Add(p); got != (f32.Point{X: 1, Y: 0.5}) {
		t.Errorf("two halves did not add to a whole: %v", got)
	}
}

func TestPointAddSubClose(t *testing.T) {
	points := []f32.Point{{}, {X: 1}, {Y: 1}, {X: -3.5, Y: 7.25}, {X: 1e6, Y: -1e6}}
	for _, p := range points {
		for _, q := range points {
			if got := p.Add(q).Sub(q); !near(got, p) {
				t.Errorf("%v.Add(%v).Sub(%v) = %v, want %v", p, q, q, got, p)
			}
			if p.Add(q) != q.Add(p) {
				t.Errorf("Add is not commutative for %v and %v", p, q)
			}
		}
	}
}

// TestPointSubIsAddOfNegation checks the two spellings of the same step agree,
// because a layout that walks back up a tree writes it both ways.
func TestPointSubIsAddOfNegation(t *testing.T) {
	p, q := f32.Pt(2, -3), f32.Pt(-5, 7)
	if got, want := p.Sub(q), p.Add(q.Mul(-1)); got != want {
		t.Errorf("Sub and Add of the negation disagree: %v, %v", got, want)
	}
}

func TestPointMulDivClose(t *testing.T) {
	p := f32.Pt(3, -4)
	for _, s := range []float32{1, 2, 0.5, -1, 1000} {
		if got := p.Mul(s).Div(s); !near(got, p) {
			t.Errorf("%v.Mul(%v).Div(%v) = %v, want %v", p, s, s, got, p)
		}
	}
}

// TestPointMulByOneAndZero fixes the two scales with an answer of their own: one
// is the point itself, and zero collapses it to the origin rather than to
// something merely small.
func TestPointMulByOneAndZero(t *testing.T) {
	p := f32.Pt(3, -4)
	if got := p.Mul(1); got != p {
		t.Errorf("%v.Mul(1) = %v, want the point unchanged", p, got)
	}
	if got := p.Mul(0); got.X != 0 || got.Y != 0 {
		t.Errorf("%v.Mul(0) = %v, want the origin", p, got)
	}
}

// TestPointDivByZero records that division does not guard its divisor.
//
// The answer is an infinity rather than a panic, which is what a float division
// gives, and a caller that can produce a zero scale has to say what it means
// before it gets here.
func TestPointDivByZero(t *testing.T) {
	got := f32.Pt(1, -1).Div(0)
	if !math.IsInf(float64(got.X), 1) || !math.IsInf(float64(got.Y), -1) {
		t.Errorf("Pt(1, -1).Div(0) = %v, want the two infinities", got)
	}
}

// TestPointRound checks the crossing from this package to the integer grid a
// window actually addresses.
//
// Halves go away from zero on both sides. The alternative, rounding both halves
// up, moves a shape by a whole pixel when it crosses the origin, and a control
// centred on zero is drawn a pixel off centre on one side only.
func TestPointRound(t *testing.T) {
	for _, test := range []struct {
		in   f32.Point
		want image.Point
	}{
		{f32.Pt(0, 0), image.Pt(0, 0)},
		{f32.Pt(1, 2), image.Pt(1, 2)},
		{f32.Pt(1.4, 2.4), image.Pt(1, 2)},
		{f32.Pt(1.5, 2.5), image.Pt(2, 3)},
		{f32.Pt(1.6, 2.6), image.Pt(2, 3)},
		{f32.Pt(-1.4, -2.4), image.Pt(-1, -2)},
		{f32.Pt(-1.5, -2.5), image.Pt(-2, -3)},
		{f32.Pt(-1.6, -2.6), image.Pt(-2, -3)},
	} {
		if got := test.in.Round(); got != test.want {
			t.Errorf("%v.Round() = %v, want %v", test.in, got, test.want)
		}
	}
}

// TestRoundOfAnIntegerPointIsItself is the property under the table above: a
// point already on the grid does not move, so a value that made the crossing
// once can make it again without drifting.
func TestRoundOfAnIntegerPointIsItself(t *testing.T) {
	for _, p := range []image.Point{{}, {X: 1, Y: 2}, {X: -7, Y: 9}, {X: 1 << 20}} {
		round := f32.Pt(float32(p.X), float32(p.Y)).Round()
		if round != p {
			t.Errorf("round trip through Point moved %v to %v", p, round)
		}
	}
}

func TestPointString(t *testing.T) {
	for _, test := range []struct {
		in   f32.Point
		want string
	}{
		{f32.Point{}, "(0,0)"},
		{f32.Pt(1, 2), "(1,2)"},
		{f32.Pt(-1.5, 2.25), "(-1.5,2.25)"},
		{f32.Pt(0.1, -0.1), "(0.1,-0.1)"},
	} {
		if got := test.in.String(); got != test.want {
			t.Errorf("%#v.String() = %q, want %q", test.in, got, test.want)
		}
	}
}

// TestPointStringKeepsFloat32Precision checks the printed form is read back at
// the width the value is stored at.
//
// Printed as a float64 the same number gains the digits float32 never held, and
// a coordinate written into a log or a golden file is then a number nobody can
// match against the one in memory.
func TestPointStringKeepsFloat32Precision(t *testing.T) {
	if got, want := f32.Pt(0.1, 0).String(), "(0.1,0)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
