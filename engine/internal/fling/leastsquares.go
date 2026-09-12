package fling

import "math"

// coefficients are a fitted polynomial, from the constant term upwards.
type coefficients [degree + 1]float32

// degenerate is the length below which a direction is treated as having none.
//
// It is not a tolerance on the answer. A vector this short means the points do
// not determine the curve -- every sample at one instant, or a device that
// reported the same instant twice -- and normalising it would divide by very
// nearly nothing and answer with an enormous velocity. Refusing is the only
// answer that leaves the screen where the hand left it.
const degenerate = 0.000001

// polyFit fits a polynomial of the package's degree through the points in X and
// Y by least squares, and answers its coefficients.
//
// It reports false when the points do not determine one: fewer of them than the
// curve has coefficients, or points that lie along a single direction and leave
// the system without a unique answer.
func polyFit(X, Y []float32) (coefficients, bool) {
	if len(X) != len(Y) {
		panic("an instant without a value, or a value without an instant")
	}
	if len(X) <= degree {
		return coefficients{}, false
	}

	// Every point counts the same. Weighting the recent ones more heavily is
	// the other way to lean the answer towards the moment of release, and the
	// curve already leans it there: what is read off the fit is the slope at
	// the newest point, not an average over the window.
	//
	// The design matrix is held as one vector per power of x -- vector j is
	// x^j across the points -- because every step below reads a whole one of
	// those at a time, and a vector laid out end to end is a slice rather than
	// a walk with a stride.
	A := newMatrix(degree+1, len(X))
	for i, x := range X {
		A.set(0, i, 1)
		for j := 1; j < A.rows; j++ {
			A.set(j, i, A.get(j-1, i)*x)
		}
	}

	Q, Rt, ok := decomposeQR(A)
	if !ok {
		return coefficients{}, false
	}

	// Solve R*B = transpose(Q)*Y for B, which is then the coefficients. R is
	// upper triangular, so the last one is read straight off and each earlier
	// one is corrected by those already known.
	var B coefficients
	for i := Q.rows - 1; i >= 0; i-- {
		B[i] = dot(Q.vector(i), Y)
		for j := Q.rows - 1; j > i; j-- {
			B[i] -= Rt.get(i, j) * B[j]
		}
		B[i] /= Rt.get(i, i)
	}
	return B, true
}

// decomposeQR factors A into an orthonormal Q and an upper triangular R.
//
// R is answered transposed, and only its square part, because that is the shape
// the solve reads it in. What the pair means is that A's j-th vector is the sum
// of Q's vectors weighted by Rt's j-th column, and that is what a test of this
// should check.
//
// It reports false when A has a direction it has already used, which is the
// case that must not be solved: the vector left after the projections is
// nothing but rounding error, and normalising nothing gives a basis that points
// anywhere.
func decomposeQR(A *matrix) (*matrix, *matrix, bool) {
	Q := newMatrix(A.rows, A.cols)
	Rt := newMatrix(A.rows, A.rows)

	for i := range Q.rows {
		q := Q.vector(i)
		copy(q, A.vector(i))

		// Take out whatever this vector has in common with the ones already
		// settled. Against a vector of unit length the projection is just the
		// dot product, which is why they are normalised as they are made and
		// not at the end.
		for j := range i {
			settled := Q.vector(j)
			d := dot(settled, q)
			for k := range q {
				q[k] -= d * settled[k]
			}
		}

		length := norm(q)
		if length < degenerate {
			return nil, nil, false
		}
		scale := 1 / length
		for k := range q {
			q[k] *= scale
		}

		// R's row, which is this vector measured against the ones it was made
		// from. Everything below the diagonal is zero and is never written.
		for j := i; j < Rt.cols; j++ {
			Rt.set(i, j, dot(q, A.vector(j)))
		}
	}
	return Q, Rt, true
}

// norm is the length of V.
func norm(V []float32) float32 {
	var n float32
	for _, v := range V {
		n += v * v
	}
	return float32(math.Sqrt(float64(n)))
}

// dot is the dot product of two vectors of the same length.
func dot(V1, V2 []float32) float32 {
	var d float32
	for i, v1 := range V1 {
		d += v1 * V2[i]
	}
	return d
}

// matrix is a set of equal-length vectors laid out one after another.
//
// rows is how many vectors there are and cols is how long each one is. The
// first index chooses a vector and the second walks along it, so a whole vector
// is a slice of the storage and not a stride through it -- which is the layout
// everything here wants, because every operation in this file works on one
// vector at a time.
type matrix struct {
	rows, cols int
	data       []float32
}

func newMatrix(rows, cols int) *matrix {
	return &matrix{
		rows: rows,
		cols: cols,
		data: make([]float32, rows*cols),
	}
}

// vector answers the i-th vector, which aliases the matrix rather than copying
// it: the decomposition writes through it.
func (m *matrix) vector(i int) []float32 {
	return m.data[i*m.cols : (i+1)*m.cols]
}

func (m *matrix) set(vector, at int, v float32) {
	if vector < 0 || vector >= m.rows {
		panic("vector out of range")
	}
	if at < 0 || at >= m.cols {
		panic("index out of range")
	}
	m.data[vector*m.cols+at] = v
}

func (m *matrix) get(vector, at int) float32 {
	if vector < 0 || vector >= m.rows {
		panic("vector out of range")
	}
	if at < 0 || at >= m.cols {
		panic("index out of range")
	}
	return m.data[vector*m.cols+at]
}
