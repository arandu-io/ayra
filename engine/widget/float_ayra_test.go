package widget

import (
	"image"
	"math"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
)

func TestFloatAlwaysClampsItsValue(t *testing.T) {
	for _, test := range []struct {
		name  string
		value float32
		want  float32
	}{
		{"below the range", -2, 0},
		{"above the range", 3, 1},
		{"not a number", float32(math.NaN()), 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := Float{Value: test.value}
			if !f.Update(layout.Context{}) {
				t.Error("clamping the value was not reported as a change")
			}
			if f.Value != test.want {
				t.Fatalf("clamping %v produced %v, want %v", test.value, f.Value, test.want)
			}
		})
	}
}

func TestFloatDoesNotChangeAtTheSamePoint(t *testing.T) {
	f := Float{Value: .5}
	if changed := pressFloat(t, &f, image.Pt(100, 20), f32.Pt(50, 10)); changed {
		t.Error("pressing the point already represented by the value reported a change")
	}
	if f.Value != .5 {
		t.Fatalf("pressing the same point moved the value to %v", f.Value)
	}
}

func TestFloatClampsPointerPositions(t *testing.T) {
	for _, test := range []struct {
		name     string
		position float32
		want     float32
	}{
		{"before the start", -20, 0},
		{"after the end", 120, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := Float{Value: .5}
			if !pressFloat(t, &f, image.Pt(100, 20), f32.Pt(test.position, 10)) {
				t.Error("moving to a different point was not reported as a change")
			}
			if f.Value != test.want {
				t.Fatalf("a press at %v produced %v, want %v", test.position, f.Value, test.want)
			}
		})
	}
}

func TestFloatZeroLengthRangeDoesNotDivideByZero(t *testing.T) {
	f := Float{Value: .75}
	if changed := pressFloat(t, &f, image.Point{}, f32.Point{}); changed {
		t.Error("a zero-length range reported a change")
	}
	if f.Value != .75 || math.IsNaN(float64(f.Value)) || math.IsInf(float64(f.Value), 0) {
		t.Fatalf("a zero-length range produced %v, want 0.75", f.Value)
	}
}

func pressFloat(t *testing.T, f *Float, size image.Point, position f32.Point) bool {
	t.Helper()
	var router input.Router
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Ops:         new(op.Ops),
		Source:      router.Source(),
	}
	// The oversized pointer margin keeps the test positions inside the input
	// area while leaving the selected range itself at exactly size.X.
	f.Layout(gtx, layout.Horizontal, 100)
	router.Frame(gtx.Ops)
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: position,
	})
	return f.Update(gtx)
}
