package fling

import "testing"

// The fit answers the curve the points lie on.
//
// Three points and a curve with three coefficients is an exact fit, so the
// answer is the curve itself and there is nothing to round away.
func TestTheFitAnswersTheCurveThePointsLieOn(t *testing.T) {
	cases := []struct {
		name string
		X, Y []float32
		want coefficients
	}{
		{
			name: "a parabola through three points",
			X:    []float32{-1, 0, 1},
			Y:    []float32{2, 0, 2},
			want: coefficients{0, 0, 2},
		},
		{
			name: "a straight line has no second order term",
			X:    []float32{-2, -1, 0, 1, 2},
			Y:    []float32{-3, -1, 1, 3, 5},
			want: coefficients{1, 2, 0},
		},
		{
			name: "every term at once",
			X:    []float32{-2, -1, 0, 1, 2, 3},
			Y:    []float32{3, 2, 3, 6, 11, 18},
			want: coefficients{3, 2, 1},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := polyFit(c.X, c.Y)
			if !ok {
				t.Fatal("the points did not determine a curve")
			}
			for i := range got {
				if !closeTo(got[i], c.want[i], 0.0001) {
					t.Fatalf("the fit is %v and %v was expected", got, c.want)
				}
			}
		})
	}
}

// The first order coefficient is the slope at zero, which is what the estimate
// reads off as the velocity.
//
// The window is measured backwards from the newest sample, so zero is the
// instant the finger left and this coefficient is the speed at that instant --
// not the average across the window, which is what a straight line through the
// same points would have answered. The curve here accelerates: its average
// slope over the window is 2 and its slope at zero is 4.
func TestTheFirstOrderCoefficientIsTheSlopeAtZero(t *testing.T) {
	// y = x*x + 4*x, sampled over the window's own sign: backwards from zero.
	X := []float32{0, -1, -2, -3}
	Y := []float32{0, -3, -4, -3}

	got, ok := polyFit(X, Y)
	if !ok {
		t.Fatal("the points did not determine a curve")
	}
	if !closeTo(got[1], 4, 0.0001) {
		t.Errorf("the slope at zero is %v and 4 was expected; the fit is %v", got[1], got)
	}
	if average := (Y[0] - Y[len(Y)-1]) / (X[0] - X[len(X)-1]); closeTo(got[1], average, 0.5) {
		t.Errorf("the slope at zero is %v, which is the average slope %v: the curve is being read as a line",
			got[1], average)
	}
}

// Fewer points than coefficients determines nothing.
func TestTooFewPointsDetermineNoCurve(t *testing.T) {
	for count := range degree + 1 {
		X := make([]float32, count)
		Y := make([]float32, count)
		for i := range X {
			X[i], Y[i] = float32(i), float32(i*i)
		}
		if _, ok := polyFit(X, Y); ok {
			t.Errorf("%d points determined a curve of degree %d", count, degree)
		}
	}
}

// Points stacked at one instant determine nothing either.
//
// This is the case that has to fail rather than answer: the columns of the
// system collapse onto each other, and a solver that pressed on would divide by
// very nearly nothing and hand back an enormous velocity. A device that
// reported a batch of samples with one timestamp would throw the content off
// the screen.
func TestPointsAtOneInstantDetermineNoCurve(t *testing.T) {
	X := []float32{1, 1, 1, 1}
	Y := []float32{1, 2, 3, 4}

	if got, ok := polyFit(X, Y); ok {
		t.Errorf("four points at one instant determined the curve %v", got)
	}
}

// The decomposition rebuilds the matrix it was given.
//
// The check is the definition: each of A's vectors is the sum of Q's vectors
// weighted by a column of R. Written against the stored layout rather than
// through a transpose and a multiply, so that a failure says which vector went
// wrong.
func TestTheDecompositionRebuildsTheMatrix(t *testing.T) {
	A := &matrix{
		rows: 3, cols: 3,
		data: []float32{
			12, 6, -4,
			-51, 167, 24,
			4, -68, -41,
		},
	}

	Q, Rt, ok := decomposeQR(A)
	if !ok {
		t.Fatal("the matrix was not decomposed")
	}

	for j := range A.rows {
		for k := range A.cols {
			var v float32
			for i := range Q.rows {
				v += Rt.get(i, j) * Q.get(i, k)
			}
			if !closeTo(v, A.get(j, k), 0.001) {
				t.Errorf("rebuilding vector %d element %d gave %v and %v was expected", j, k, v, A.get(j, k))
			}
		}
	}
}

// Q comes out orthonormal, which is the property the solve leans on.
//
// Every coefficient is read as a dot product against one of Q's vectors, and
// that is only the right answer because the vectors have unit length and no
// component along each other. Losing it is not a failure that shows up as a
// wrong shape -- it shows up as a velocity that is slightly wrong, always.
func TestTheDecompositionAnswersAnOrthonormalBasis(t *testing.T) {
	A := &matrix{
		rows: 3, cols: 4,
		data: []float32{
			1, 1, 1, 1,
			-3, -2, -1, 0,
			9, 4, 1, 0,
		},
	}

	Q, _, ok := decomposeQR(A)
	if !ok {
		t.Fatal("the matrix was not decomposed")
	}

	for i := range Q.rows {
		for j := range Q.rows {
			var d float32
			for k := range Q.cols {
				d += Q.get(i, k) * Q.get(j, k)
			}
			want := float32(0)
			if i == j {
				want = 1
			}
			if !closeTo(d, want, 0.0001) {
				t.Errorf("vectors %d and %d have dot product %v and %v was expected", i, j, d, want)
			}
		}
	}
}

// A collapsed matrix is refused rather than solved.
func TestACollapsedMatrixIsRefused(t *testing.T) {
	A := &matrix{
		rows: 3, cols: 4,
		data: []float32{
			1, 1, 1, 1,
			1, 1, 1, 1,
			1, 0, 0, 0,
		},
	}

	if _, _, ok := decomposeQR(A); ok {
		t.Error("a matrix with a repeated direction was decomposed")
	}
}

// Mismatched inputs are a mistake in the caller, and it is reported where it
// happened rather than fitted around.
func TestMismatchedInputsAreRefusedLoudly(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a fit over four instants and three values was attempted")
		}
	}()

	polyFit([]float32{0, 1, 2, 3}, []float32{0, 1, 2})
}
