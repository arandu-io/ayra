package f32_test

import (
	"math"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
)

// elems is the transform as the six numbers it is made of.
//
// It is how one transform is compared with another from outside the package:
// the stored form subtracts the identity so that the zero value is the identity,
// and a test that read the fields would be fixing that trick rather than the
// behaviour it exists to give.
func elems(a f32.Affine2D) [6]float32 {
	var e [6]float32
	e[0], e[1], e[2], e[3], e[4], e[5] = a.Elems()
	return e
}

// nearAffine reports whether two transforms agree to within tolerance.
func nearAffine(a, b f32.Affine2D) bool {
	x, y := elems(a), elems(b)
	for i := range x {
		if math.Abs(float64(x[i]-y[i])) >= tolerance {
			return false
		}
	}
	return true
}

// corners is a spread of points to apply a property to: the origin, both axes,
// a negative quadrant and something far enough out that an error in the linear
// part shows up as more than a rounding difference.
var corners = []f32.Point{
	{},
	{X: 1},
	{Y: 1},
	{X: 1, Y: 1},
	{X: -1, Y: -1},
	{X: 10, Y: 20},
	{X: -3.5, Y: 7.25},
}

// TestIdentityMovesNothing is the property the zero value promises, checked on
// the value a caller writes and on the one it gets by declaring the field.
func TestIdentityMovesNothing(t *testing.T) {
	for _, id := range []f32.Affine2D{f32.AffineId(), {}} {
		for _, p := range corners {
			if got := id.Transform(p); got != p {
				t.Errorf("identity moved %v to %v", p, got)
			}
		}
	}
}

// TestZeroValueIsIdentity fixes the reason the stored form is what it is.
//
// A transform is held with the identity subtracted so that a struct nobody
// filled in draws in the place it was given. A field of this type needs no
// constructor, and a tree of them that forgot one does not collapse to the
// origin.
func TestZeroValueIsIdentity(t *testing.T) {
	var zero f32.Affine2D
	if zero != f32.AffineId() {
		t.Errorf("zero value %v is not the identity %v", zero, f32.AffineId())
	}
	if got, want := elems(zero), [6]float32{1, 0, 0, 0, 1, 0}; got != want {
		t.Errorf("zero value elements = %v, want %v", got, want)
	}
}

func TestNewAffine2DElemsRoundTrip(t *testing.T) {
	for _, want := range [][6]float32{
		{1, 0, 0, 0, 1, 0},
		{2, 0, 5, 0, 3, 7},
		{-1, 2, 3, 4, -5, 6},
		{7, 8, 9, 10, 11, 12},
	} {
		a := f32.NewAffine2D(want[0], want[1], want[2], want[3], want[4], want[5])
		if got := elems(a); got != want {
			t.Errorf("NewAffine2D%v.Elems() = %v, want %v", want, got, want)
		}
	}
}

// TestElemsAreRowMajor pins which of the six numbers is which, because every one
// of them is a float32 and a pair swapped by mistake still compiles and still
// draws something.
func TestElemsAreRowMajor(t *testing.T) {
	sx, hx, ox, hy, sy, oy := f32.NewAffine2D(2, 3, 5, 7, 11, 13).Elems()
	if sx != 2 || hx != 3 || ox != 5 || hy != 7 || sy != 11 || oy != 13 {
		t.Errorf("Elems() = (%v, %v, %v, %v, %v, %v), want (2, 3, 5, 7, 11, 13)", sx, hx, ox, hy, sy, oy)
	}

	// The rows read [sx hx ox] and [hy sy oy], so the point (1, 0) picks out the
	// first column and (0, 1) the second, with the offset added to both.
	a := f32.NewAffine2D(2, 3, 5, 7, 11, 13)
	if got := a.Transform(f32.Pt(1, 0)); !near(got, f32.Pt(7, 20)) {
		t.Errorf("first column = %v, want (7,20)", got)
	}
	if got := a.Transform(f32.Pt(0, 1)); !near(got, f32.Pt(8, 24)) {
		t.Errorf("second column = %v, want (8,24)", got)
	}
}

func TestOffsetCloses(t *testing.T) {
	for _, offset := range corners {
		a := f32.AffineId().Offset(offset)
		for _, p := range corners {
			moved := a.Transform(p)
			if !near(moved, p.Add(offset)) {
				t.Errorf("Offset(%v).Transform(%v) = %v, want %v", offset, p, moved, p.Add(offset))
			}
			if back := a.Invert().Transform(moved); !near(back, p) {
				t.Errorf("Offset(%v) then its inverse moved %v to %v", offset, p, back)
			}
		}
	}
}

// TestOffsetAccumulates checks that two steps are one step of the sum, which is
// what a nested layout relies on when it offsets a child of a child.
func TestOffsetAccumulates(t *testing.T) {
	first, second := f32.Pt(3, -4), f32.Pt(-1, 10)
	twice := f32.AffineId().Offset(first).Offset(second)
	once := f32.AffineId().Offset(first.Add(second))
	if !nearAffine(twice, once) {
		t.Errorf("two offsets %v, one offset %v", twice, once)
	}
}

// TestTransformAboutAnOriginFixesIt is the property that says what "around the
// given origin" means: the origin is the one point that does not move.
//
// It is written for all three of scale, rotate and shear because all three are
// defined about zero, and all three reach an origin the same way -- by moving
// that point to zero, acting, and moving it back. An origin whose two
// coordinates are equal cannot tell the two moves apart, so the table carries
// origins that lie on one axis and origins in a negative quadrant.
func TestTransformAboutAnOriginFixesIt(t *testing.T) {
	origins := []f32.Point{{}, {X: 1, Y: 1}, {X: 2}, {Y: 3}, {X: -2, Y: 5}, {X: 4, Y: 5}}

	for _, origin := range origins {
		cases := []struct {
			name string
			a    f32.Affine2D
		}{
			{"scale", f32.AffineId().Scale(origin, f32.Pt(2, 3))},
			{"scale negative", f32.AffineId().Scale(origin, f32.Pt(-1, 2))},
			{"rotate", f32.AffineId().Rotate(origin, math.Pi/2)},
			{"rotate back", f32.AffineId().Rotate(origin, -math.Pi/3)},
			{"shear x", f32.AffineId().Shear(origin, math.Pi/4, 0)},
			{"shear y", f32.AffineId().Shear(origin, 0, math.Pi/4)},
			{"shear both", f32.AffineId().Shear(origin, math.Pi/6, -math.Pi/5)},
		}
		for _, test := range cases {
			if got := test.a.Transform(origin); !near(got, origin) {
				t.Errorf("%s about %v moved the origin to %v", test.name, origin, got)
			}
		}
	}
}

// TestTransformsClose checks every transform against its own inverse, on every
// point, which is the cheapest statement that the matrix and the inverse of the
// matrix were derived from each other rather than each on its own.
func TestTransformsClose(t *testing.T) {
	origin := f32.Pt(4, 5)
	transforms := []struct {
		name string
		a    f32.Affine2D
	}{
		{"offset", f32.AffineId().Offset(f32.Pt(2, -3))},
		{"scale", f32.AffineId().Scale(f32.Point{}, f32.Pt(-1, 2))},
		{"scale about", f32.AffineId().Scale(origin, f32.Pt(2, 3))},
		{"rotate", f32.AffineId().Rotate(f32.Point{}, math.Pi/2)},
		{"rotate about", f32.AffineId().Rotate(origin, -math.Pi/2)},
		{"shear x", f32.AffineId().Shear(f32.Point{}, math.Pi/4, 0)},
		{"shear y about", f32.AffineId().Shear(origin, 0, math.Pi/4)},
		{"compound", f32.AffineId().
			Offset(f32.Pt(2, -3)).
			Scale(f32.Point{}, f32.Pt(-1, 2)).
			Rotate(f32.Point{}, -math.Pi/2).
			Shear(f32.Point{}, math.Pi/4, 0)},
	}

	for _, test := range transforms {
		for _, p := range corners {
			moved := test.a.Transform(p)
			if back := test.a.Invert().Transform(moved); !near(back, p) {
				t.Errorf("%s: %v went to %v and came back as %v", test.name, p, moved, back)
			}
		}
		if twice := test.a.Invert().Invert(); !nearAffine(twice, test.a) {
			t.Errorf("%s: inverting twice gave %v, want %v", test.name, twice, test.a)
		}
	}
}

// TestTransformKnownValues is the arithmetic itself, on numbers chosen so that
// no two of them share a factor: a term added where it should be multiplied, or
// a row read as a column, lands on a different answer rather than on the same
// one by luck.
func TestTransformKnownValues(t *testing.T) {
	xa := f32.NewAffine2D(9, 11, 13, 17, 19, 23)
	xb := f32.NewAffine2D(29, 31, 37, 43, 47, 53)

	for _, test := range []struct {
		a    f32.Affine2D
		p    f32.Point
		want f32.Point
	}{
		{xa, f32.Pt(2, 3), f32.Pt(64, 114)},
		{xa, f32.Pt(5, 7), f32.Pt(135, 241)},
		{xb, f32.Pt(2, 3), f32.Pt(188, 280)},
		{xb, f32.Pt(5, 7), f32.Pt(399, 597)},
	} {
		if got := test.a.Transform(test.p); !near(got, test.want) {
			t.Errorf("%v.Transform(%v) = %v, want %v", test.a, test.p, got, test.want)
		}
	}

	for _, test := range []struct {
		a, want f32.Affine2D
	}{
		{xa, f32.NewAffine2D(-1.1875, 0.6875, -0.375, 1.0625, -0.5625, -0.875)},
		{xb, f32.NewAffine2D(1.5666667, -1.0333333, -3.2000008, -1.4333333, 0.96666664, 1.7999992)},
	} {
		if got := test.a.Invert(); !nearAffine(got, test.want) {
			t.Errorf("%v.Invert() = %v, want %v", test.a, got, test.want)
		}
	}

	if got, want := xa.Mul(xb), f32.NewAffine2D(734, 796, 929, 1310, 1420, 1659); !nearAffine(got, want) {
		t.Errorf("%v.Mul(%v) = %v, want %v", xa, xb, got, want)
	}
}

// TestScaleAndRotateKnownValues fixes the direction each one turns, which no
// round trip can: an inverse closes just as neatly on a rotation that went the
// wrong way.
func TestScaleAndRotateKnownValues(t *testing.T) {
	if got := f32.AffineId().Scale(f32.Point{}, f32.Pt(-1, 2)).Transform(f32.Pt(1, 2)); !near(got, f32.Pt(-1, 4)) {
		t.Errorf("scale = %v, want (-1,4)", got)
	}
	if got := f32.AffineId().Rotate(f32.Point{}, math.Pi/2).Transform(f32.Pt(1, 0)); !near(got, f32.Pt(0, 1)) {
		t.Errorf("quarter turn = %v, want (0,1)", got)
	}
	if got := f32.AffineId().Shear(f32.Point{}, math.Pi/4, 0).Transform(f32.Pt(1, 1)); !near(got, f32.Pt(2, 1)) {
		t.Errorf("shear in x = %v, want (2,1)", got)
	}
	if got := f32.AffineId().Scale(f32.Pt(4, 5), f32.Pt(2, 3)).Transform(f32.Pt(-1, -1)); !near(got, f32.Pt(-6, -13)) {
		t.Errorf("scale about (4,5) = %v, want (-6,-13)", got)
	}
	if got := f32.AffineId().Rotate(f32.Pt(1, 1), -math.Pi/2).Transform(f32.Pt(-1, -1)); !near(got, f32.Pt(-1, 3)) {
		t.Errorf("rotate about (1,1) = %v, want (-1,3)", got)
	}
	if got := f32.AffineId().Shear(f32.Pt(1, 1), math.Pi/4, 0).Transform(f32.Pt(2, 3)); !near(got, f32.Pt(4, 3)) {
		t.Errorf("shear in x about (1,1) = %v, want (4,3)", got)
	}
	if got := f32.AffineId().Shear(f32.Pt(1, 1), 0, math.Pi/4).Transform(f32.Pt(2, 3)); !near(got, f32.Pt(2, 4)) {
		t.Errorf("shear in y about (1,1) = %v, want (2,4)", got)
	}

	// An origin that lies on one axis only. A shear about an origin whose two
	// coordinates are equal gives the same answer whichever coordinate the
	// offset row is built from, so this is the case that tells them apart.
	if got := f32.AffineId().Shear(f32.Pt(2, 0), 0, math.Pi/4).Transform(f32.Pt(3, 1)); !near(got, f32.Pt(3, 2)) {
		t.Errorf("shear in y about (2,0) = %v, want (3,2)", got)
	}
}

// TestNoOpArguments checks the arguments that mean "do nothing" do nothing,
// rather than nearly nothing. A caller that animates a transform passes through
// all three of these on the way to and from rest.
func TestNoOpArguments(t *testing.T) {
	origin := f32.Pt(4, 5)
	for _, test := range []struct {
		name string
		a    f32.Affine2D
	}{
		{"scale by one", f32.AffineId().Scale(origin, f32.Pt(1, 1))},
		{"scale by one about zero", f32.AffineId().Scale(f32.Point{}, f32.Pt(1, 1))},
		{"rotate by nothing", f32.AffineId().Rotate(origin, 0)},
		{"shear by nothing", f32.AffineId().Shear(origin, 0, 0)},
		{"offset by nothing", f32.AffineId().Offset(f32.Point{})},
	} {
		if !nearAffine(test.a, f32.AffineId()) {
			t.Errorf("%s gave %v, want the identity", test.name, test.a)
		}
	}
}

// nearAffineScaled compares two transformations with a tolerance that grows with
// how large they are.
//
// A float32 keeps about seven digits wherever the value sits, so an absolute
// bound is the wrong shape twice over: it fails on a transformation whose terms
// are in the hundreds and passes anything at all on one whose terms are tiny.
func nearAffineScaled(a, b f32.Affine2D) bool {
	x, y := elems(a), elems(b)
	for i := range x {
		size := math.Max(math.Abs(float64(x[i])), math.Abs(float64(y[i])))
		if math.Abs(float64(x[i]-y[i])) >= tolerance*math.Max(1, size) {
			return false
		}
	}
	return true
}

// TestStepsPreMultiply ties each of the four steps to the product, on
// transformations that already scale, shear and turn.
//
// Applied to the identity, all four agree with almost anything: the identity is
// mostly zeros, and half of the terms in each derivation are multiplied by one
// of them, so a sign wrong in that half cannot show. Every other test here
// starts from the identity, and a rotation with its shear term negated passed
// all of them. Building the same step out of [f32.Affine2D.Mul], whose
// arithmetic is pinned on its own against known values, is what reaches the
// other half.
//
// That each step pre-multiplies is also the claim the chained calls rest on: a
// step is applied to the transformation, not composed after it.
func TestStepsPreMultiply(t *testing.T) {
	general := []f32.Affine2D{
		f32.NewAffine2D(9, 11, 13, 17, 19, 23),
		f32.NewAffine2D(2, -1, 5, 3, -4, -7),
		f32.NewAffine2D(-0.5, 0.25, -3, 1.5, 2, 4),
		f32.AffineId().Offset(f32.Pt(2, -3)).Rotate(f32.Point{}, math.Pi/3),
	}
	steps := []struct {
		name string
		step func(f32.Affine2D) f32.Affine2D
	}{
		{"offset", func(a f32.Affine2D) f32.Affine2D { return a.Offset(f32.Pt(3, -5)) }},
		{"scale", func(a f32.Affine2D) f32.Affine2D { return a.Scale(f32.Point{}, f32.Pt(2, -3)) }},
		{"scale about", func(a f32.Affine2D) f32.Affine2D { return a.Scale(f32.Pt(4, 5), f32.Pt(2, -3)) }},
		{"rotate", func(a f32.Affine2D) f32.Affine2D { return a.Rotate(f32.Point{}, math.Pi/3) }},
		{"rotate about", func(a f32.Affine2D) f32.Affine2D { return a.Rotate(f32.Pt(4, 5), math.Pi/3) }},
		{"shear", func(a f32.Affine2D) f32.Affine2D { return a.Shear(f32.Point{}, math.Pi/5, -math.Pi/7) }},
		{"shear about", func(a f32.Affine2D) f32.Affine2D { return a.Shear(f32.Pt(4, 5), math.Pi/5, -math.Pi/7) }},
	}

	for _, a := range general {
		for _, s := range steps {
			got := s.step(a)
			want := s.step(f32.AffineId()).Mul(a)
			if !nearAffineScaled(got, want) {
				t.Errorf("%s applied to %v gave %v, want %v", s.name, a, got, want)
			}
		}
	}
}

// TestTransformOfAGeneralPairComposes is the same reach stated on points rather
// than on matrices, so that a derivation and the product would have to be wrong
// together to pass both.
func TestTransformOfAGeneralPairComposes(t *testing.T) {
	a := f32.NewAffine2D(2, -1, 5, 3, -4, -7)
	for _, p := range corners {
		turned := a.Rotate(f32.Point{}, math.Pi/3)
		want := f32.AffineId().Rotate(f32.Point{}, math.Pi/3).Transform(a.Transform(p))
		if got := turned.Transform(p); !near(got, want) {
			t.Errorf("turning %v through a general transform gave %v, want %v", p, got, want)
		}
	}
}

// TestMulComposes states what the product is for: the transform that does what
// both do, in the order they are applied to a point.
func TestMulComposes(t *testing.T) {
	a := f32.AffineId().Offset(f32.Pt(2, -3)).Rotate(f32.Point{}, math.Pi/3)
	b := f32.AffineId().Scale(f32.Point{}, f32.Pt(2, 0.5)).Offset(f32.Pt(1, 1))

	for _, p := range corners {
		if got, want := a.Mul(b).Transform(p), a.Transform(b.Transform(p)); !near(got, want) {
			t.Errorf("a.Mul(b).Transform(%v) = %v, want %v", p, got, want)
		}
	}
}

// TestMulOrder fixes which side of the product a later step goes on, because the
// two orders differ and both compile.
func TestMulOrder(t *testing.T) {
	offset := f32.AffineId().Offset(f32.Pt(100, 100))
	scale := f32.AffineId().Scale(f32.Point{}, f32.Pt(2, 2))

	chained := f32.AffineId().Offset(f32.Pt(100, 100)).Scale(f32.Point{}, f32.Pt(2, 2))
	if multiplied := scale.Mul(offset); chained != multiplied {
		t.Errorf("chained %v, multiplied %v", chained, multiplied)
	}
}

// TestMulByIdentity checks the identity is the identity of the product too, so
// that a transform accumulated from an empty start is the accumulation alone.
func TestMulByIdentity(t *testing.T) {
	a := f32.AffineId().Offset(f32.Pt(2, -3)).Scale(f32.Point{}, f32.Pt(3, 4)).Rotate(f32.Point{}, math.Pi/5)
	if got := a.Mul(f32.AffineId()); !nearAffine(got, a) {
		t.Errorf("a.Mul(identity) = %v, want %v", got, a)
	}
	if got := f32.AffineId().Mul(a); !nearAffine(got, a) {
		t.Errorf("identity.Mul(a) = %v, want %v", got, a)
	}
}

// TestInvertOfIdentityIsIdentity is the one inverse that has to be exact: it is
// the value a draw tree starts from, and an identity that came back a rounding
// off would drift a shape by the depth of the tree.
func TestInvertOfIdentityIsIdentity(t *testing.T) {
	if got := f32.AffineId().Invert(); got != f32.AffineId() {
		t.Errorf("identity inverted = %v, want the identity", got)
	}
}

// TestInvertOfPureOffsetIsExact checks the offset-only case is negated rather
// than divided out.
//
// It is the common case in a draw tree -- most of what a layout accumulates is
// offset and nothing else -- and going through the general inverse costs six
// divisions by a determinant that is one, each of which can only lose bits.
func TestInvertOfPureOffsetIsExact(t *testing.T) {
	for _, offset := range corners {
		got := f32.AffineId().Offset(offset).Invert()
		want := f32.AffineId().Offset(offset.Mul(-1))
		if elems(got) != elems(want) {
			t.Errorf("Offset(%v).Invert() = %v, want %v", offset, got, want)
		}
	}
}

// TestSplitRecomposes checks the two halves are the whole.
//
// The split exists so that a caller can treat a plain offset differently from
// anything that scales, shears or rotates. That is only safe while putting the
// offset back gives the transform it came from.
func TestSplitRecomposes(t *testing.T) {
	for _, a := range []f32.Affine2D{
		f32.AffineId(),
		f32.NewAffine2D(2, 0, 10, 0, 3, 20),
		f32.NewAffine2D(2, 2, 3, 4, 6, 6),
		f32.AffineId().Offset(f32.Pt(2, -3)).Rotate(f32.Point{}, math.Pi/3),
	} {
		srs, offset := a.Split()
		if got := srs.Offset(offset); got != a {
			t.Errorf("%v split into %v and %v, which recompose to %v", a, srs, offset, got)
		}

		// The first half carries no offset at all, which is the half of the
		// promise a recomposition cannot check on its own: a split that left the
		// offset in both halves would also put it back correctly.
		if _, rest := srs.Split(); rest != (f32.Point{}) {
			t.Errorf("%v.Split() left the offset %v in the first half", a, rest)
		}
	}
}

func TestAffineString(t *testing.T) {
	for _, test := range []struct {
		in   f32.Affine2D
		want string
	}{
		{f32.AffineId(), "[[1 0 0] [0 1 0]]"},
		{f32.Affine2D{}, "[[1 0 0] [0 1 0]]"},
		{f32.NewAffine2D(9, 11, 13, 17, 19, 23), "[[9 11 13] [17 19 23]]"},
		{f32.NewAffine2D(29, 31, 37, 43, 47, 53), "[[29 31 37] [43 47 53]]"},
		{
			f32.NewAffine2D(29.142342, 31.4123412, 37.53152, 43.51324213, 47.123412, 53.14312342),
			"[[29.1423 31.4123 37.5315] [43.5132 47.1234 53.1431]]",
		},
	} {
		if got := test.in.String(); got != test.want {
			t.Errorf("String() = %q, want %q", got, test.want)
		}
	}
}

// TestAffineStringShowsTheMatrixNotTheStoredForm checks the printed rows are the
// transform a reader expects, not the identity-subtracted form it is held in. A
// value printed while debugging is no use if it is off by one on the diagonal.
func TestAffineStringShowsTheMatrixNotTheStoredForm(t *testing.T) {
	if got, want := f32.NewAffine2D(2, 0, 0, 0, 2, 0).String(), "[[2 0 0] [0 2 0]]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func BenchmarkTransformOffset(b *testing.B) {
	p := f32.Pt(1, 2)
	a := f32.AffineId().Offset(f32.Pt(0.5, 0.5))
	for b.Loop() {
		p = a.Transform(p)
	}
	_ = p
}

func BenchmarkTransformScale(b *testing.B) {
	p := f32.Pt(1, 2)
	a := f32.AffineId().Scale(f32.Point{}, f32.Pt(0.5, 0.5))
	for b.Loop() {
		p = a.Transform(p)
	}
	_ = p
}

func BenchmarkTransformRotate(b *testing.B) {
	p := f32.Pt(1, 2)
	a := f32.AffineId().Rotate(f32.Point{}, math.Pi/2)
	for b.Loop() {
		p = a.Transform(p)
	}
	_ = p
}

func BenchmarkOffset(b *testing.B) {
	a := f32.AffineId()
	o := f32.Pt(0.5, 0.5)
	for b.Loop() {
		a = a.Offset(o)
	}
	_ = a
}

// BenchmarkScaleAboutZero and BenchmarkRotateAboutZero measure the path every
// caller in this repository takes: an origin of zero, where the two moves that
// reach an origin are skipped entirely.
func BenchmarkScaleAboutZero(b *testing.B) {
	a := f32.AffineId()
	factor := f32.Pt(1.0001, 0.9999)
	for b.Loop() {
		a = a.Scale(f32.Point{}, factor)
	}
	_ = a
}

func BenchmarkRotateAboutZero(b *testing.B) {
	a := f32.AffineId()
	for b.Loop() {
		a = a.Rotate(f32.Point{}, math.Pi/512)
	}
	_ = a
}

func BenchmarkScaleAboutOrigin(b *testing.B) {
	a := f32.AffineId()
	origin, factor := f32.Pt(4, 5), f32.Pt(1.0001, 0.9999)
	for b.Loop() {
		a = a.Scale(origin, factor)
	}
	_ = a
}

func BenchmarkRotateAboutOrigin(b *testing.B) {
	a := f32.AffineId()
	origin := f32.Pt(4, 5)
	for b.Loop() {
		a = a.Rotate(origin, math.Pi/512)
	}
	_ = a
}

func BenchmarkTransformTranslateMultiply(b *testing.B) {
	a := f32.AffineId().Offset(f32.Pt(1, 1)).Rotate(f32.Point{}, math.Pi/3)
	t := f32.AffineId().Offset(f32.Pt(0.5, 0.5))
	for b.Loop() {
		a = a.Mul(t)
	}
}

func BenchmarkTransformScaleMultiply(b *testing.B) {
	a := f32.AffineId().Offset(f32.Pt(1, 1)).Rotate(f32.Point{}, math.Pi/3)
	t := f32.AffineId().Offset(f32.Pt(0.5, 0.5)).Scale(f32.Point{}, f32.Pt(0.4, -0.5))
	for b.Loop() {
		a = a.Mul(t)
	}
}

func BenchmarkTransformMultiply(b *testing.B) {
	a := f32.AffineId().Offset(f32.Pt(1, 1)).Rotate(f32.Point{}, math.Pi/3)
	t := f32.AffineId().Offset(f32.Pt(0.5, 0.5)).Rotate(f32.Point{}, math.Pi/7)
	for b.Loop() {
		a = a.Mul(t)
	}
}

func BenchmarkInvertOffset(b *testing.B) {
	a := f32.AffineId().Offset(f32.Pt(1, 1))
	for b.Loop() {
		a = a.Invert()
	}
	_ = a
}
