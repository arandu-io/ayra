package layout

import (
	"image"
	"testing"
	"time"

	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
)

// TestStackMeasuresOrdinaryChildrenFirstAndDrawsInArgumentOrder separates the
// sizing pass from the drawing order. Expanded children need the ordinary
// bounds before they can be measured, while overlap still has to follow the
// order the caller wrote.
func TestStackMeasuresOrdinaryChildrenFirstAndDrawsInArgumentOrder(t *testing.T) {
	gtx := atMost(Horizontal, 100, 100)
	var measured []string

	dims := Stack{Alignment: SE}.Layout(gtx,
		Expanded(func(gtx Context) Dimensions {
			measured = append(measured, "first expanded")
			if gtx.Constraints.Min != image.Pt(30, 20) {
				t.Errorf("first expanded child received minimum %v, want (30,20)", gtx.Constraints.Min)
			}
			return Dimensions{Size: image.Pt(35, 25)}
		}),
		Stacked(func(gtx Context) Dimensions {
			measured = append(measured, "stacked")
			if gtx.Constraints.Min != (image.Point{}) {
				t.Errorf("stacked child received minimum %v, want zero", gtx.Constraints.Min)
			}
			return Dimensions{Size: image.Pt(30, 20)}
		}),
		Expanded(func(gtx Context) Dimensions {
			measured = append(measured, "second expanded")
			if gtx.Constraints.Min != image.Pt(35, 25) {
				t.Errorf("second expanded child received minimum %v, want (35,25)", gtx.Constraints.Min)
			}
			return Dimensions{Size: image.Pt(40, 30)}
		}),
	)

	if want := []string{"stacked", "first expanded", "second expanded"}; !slicesEqual(measured, want) {
		t.Errorf("measurement order was %v, want %v", measured, want)
	}
	if dims.Size != image.Pt(40, 30) {
		t.Errorf("stack measured %v, want (40,30)", dims.Size)
	}
	if got, want := decodedOffsets(t, gtx.Ops), []image.Point{image.Pt(5, 5), image.Pt(10, 10), image.Pt(0, 0)}; !pointsEqual(got, want) {
		t.Errorf("drawing positions were %v, want argument order at %v", got, want)
	}
}

// TestStackAlignmentNamesAllNineAnchors fixes the complete Direction surface.
// A corner-only test leaves the centred edges free to drift into a corner.
func TestStackAlignmentNamesAllNineAnchors(t *testing.T) {
	for _, tc := range []struct {
		direction Direction
		position  image.Point
	}{
		{NW, image.Pt(0, 0)},
		{N, image.Pt(40, 0)},
		{NE, image.Pt(80, 0)},
		{E, image.Pt(80, 35)},
		{SE, image.Pt(80, 70)},
		{S, image.Pt(40, 70)},
		{SW, image.Pt(0, 70)},
		{W, image.Pt(0, 35)},
		{Center, image.Pt(40, 35)},
	} {
		t.Run(tc.direction.String(), func(t *testing.T) {
			gtx := Context{Ops: new(op.Ops), Constraints: Exact(image.Pt(100, 80))}
			Stack{Alignment: tc.direction}.Layout(gtx,
				Stacked(func(Context) Dimensions {
					return Dimensions{Size: image.Pt(20, 10)}
				}),
			)

			positions := decodedOffsets(t, gtx.Ops)
			if len(positions) != 1 || positions[0] != tc.position {
				t.Errorf("%s placed child at %v, want %v", tc.direction, positions, tc.position)
			}
		})
	}
}

// TestStackBaselineFollowsTheFirstTextBearingChild fixes the conversion from a
// child's baseline to the stack's distance from its bottom edge.
func TestStackBaselineFollowsTheFirstTextBearingChild(t *testing.T) {
	for _, tc := range []struct {
		direction Direction
		baseline  int
	}{
		{N, 43},
		{Center, 23},
		{S, 3},
	} {
		t.Run(tc.direction.String(), func(t *testing.T) {
			gtx := Context{Ops: new(op.Ops), Constraints: Exact(image.Pt(100, 50))}
			dims := Stack{Alignment: tc.direction}.Layout(gtx,
				Stacked(func(Context) Dimensions {
					return Dimensions{Size: image.Pt(20, 10), Baseline: 3}
				}),
			)
			if dims.Baseline != tc.baseline {
				t.Errorf("%s reported baseline %d, want %d", tc.direction, dims.Baseline, tc.baseline)
			}
		})
	}
}

// TestBackgroundMeasuresForegroundThenDrawsItCentred fixes the two different
// orders of a background layout: foreground first for measurement, background
// first for drawing.
func TestBackgroundMeasuresForegroundThenDrawsItCentred(t *testing.T) {
	gtx := atMost(Horizontal, 100, 100)
	var measured []string

	dims := (Background{}).Layout(gtx,
		func(gtx Context) Dimensions {
			measured = append(measured, "background")
			if gtx.Constraints.Min != image.Pt(20, 10) {
				t.Errorf("background minimum was %v, want foreground size (20,10)", gtx.Constraints.Min)
			}
			return Dimensions{Size: image.Pt(40, 30)}
		},
		func(Context) Dimensions {
			measured = append(measured, "foreground")
			return Dimensions{Size: image.Pt(20, 10), Baseline: 3}
		},
	)

	if want := []string{"foreground", "background"}; !slicesEqual(measured, want) {
		t.Errorf("measurement order was %v, want %v", measured, want)
	}
	if got, want := decodedOffsets(t, gtx.Ops), []image.Point{image.Pt(10, 10)}; !pointsEqual(got, want) {
		t.Errorf("foreground position was %v, want %v", got, want)
	}
	if dims.Size != image.Pt(40, 30) || dims.Baseline != 13 {
		t.Errorf("background dimensions were %+v, want size (40,30) and baseline 13", dims)
	}
}

// TestContextConvertsLengthAndTextWithIndependentMetrics catches the easy
// mistake of routing both wrappers through the same density.
func TestContextConvertsLengthAndTextWithIndependentMetrics(t *testing.T) {
	gtx := Context{Metric: unit.Metric{PxPerDp: 2, PxPerSp: 3}}
	if got := gtx.Dp(unit.Dp(1.25)); got != 3 {
		t.Errorf("1.25dp at density 2 became %d pixels, want 3", got)
	}
	if got := gtx.Sp(unit.Sp(1.5)); got != 5 {
		t.Errorf("1.5sp at density 3 became %d pixels, want 5", got)
	}
}

// TestDisabledContextOnlyDisablesInput fixes Disabled as a copy operation. A
// parent can suppress interaction for a subtree without losing its frame time,
// dimensions, values or drawing list, and without disabling its own context.
func TestDisabledContextOnlyDisablesInput(t *testing.T) {
	var router input.Router
	ops := new(op.Ops)
	now := time.Unix(123, 456)
	original := Context{
		Constraints: Exact(image.Pt(20, 30)),
		Metric:      unit.Metric{PxPerDp: 2, PxPerSp: 3},
		Now:         now,
		Values:      map[string]any{"answer": 42},
		Source:      router.Source(),
		Ops:         ops,
	}

	disabled := original.Disabled()
	if !original.Source.Enabled() {
		t.Fatal("disabling the copy also disabled the original context")
	}
	if disabled.Source.Enabled() {
		t.Fatal("disabled context still has an enabled event source")
	}
	if disabled.Constraints != original.Constraints || disabled.Metric != original.Metric || disabled.Now != now {
		t.Errorf("disabled context changed frame state: got %+v, original %+v", disabled, original)
	}
	if disabled.Ops != ops || disabled.Values["answer"] != 42 {
		t.Error("disabled context did not preserve drawing state and application values")
	}
}
