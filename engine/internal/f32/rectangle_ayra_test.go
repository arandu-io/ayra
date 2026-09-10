// SPDX-License-Identifier: Unlicense OR MIT

package f32

import "testing"

// The file under test carries no tests of its own, and the packages that
// import it exercise none of these methods, so a wrong sign here reaches a
// drawn frame before it reaches a failure. The name of this file keeps it out
// of the way of a test file the original may gain later.

func TestRectOrdersItsCorners(t *testing.T) {
	want := Rectangle{Point{X: 1, Y: 2}, Point{X: 3, Y: 4}}
	for _, args := range [][4]float32{
		{1, 2, 3, 4},
		{3, 2, 1, 4},
		{1, 4, 3, 2},
		{3, 4, 1, 2},
	} {
		if got := Rect(args[0], args[1], args[2], args[3]); got != want {
			t.Errorf("Rect%v = %v, want %v", args, got, want)
		}
	}
}

func TestRectKeepsEachArgumentOnItsOwnAxis(t *testing.T) {
	// Distinct values on every corner, so swapping X with Y is visible.
	got := Rect(1, 2, 3, 5)
	if got.Min.X != 1 || got.Min.Y != 2 || got.Max.X != 3 || got.Max.Y != 5 {
		t.Errorf("Rect(1, 2, 3, 5) = %v, want (1,2)-(3,5)", got)
	}
}

func TestAddAndSubOffsetBothCorners(t *testing.T) {
	r := Rect(1, 2, 3, 5)
	p := Point{X: 10, Y: 20}

	add := r.Add(p)
	if want := Rect(11, 22, 13, 25); add != want {
		t.Errorf("Add(%v) = %v, want %v", p, add, want)
	}

	sub := r.Sub(p)
	if want := Rect(-9, -18, -7, -15); sub != want {
		t.Errorf("Sub(%v) = %v, want %v", p, sub, want)
	}

	if back := add.Sub(p); back != r {
		t.Errorf("Add then Sub = %v, want %v", back, r)
	}
}

func TestSizeReportsWidthThenHeight(t *testing.T) {
	got := Rect(1, 2, 4, 12).Size()
	if want := (Point{X: 3, Y: 10}); got != want {
		t.Errorf("Size() = %v, want %v", got, want)
	}
}
