package widget

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

func ayraLabelShaper() *text.Shaper {
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
}

func ayraLabelContext(constraints layout.Constraints) layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Constraints: constraints,
	}
}

// ayraFirstPaintOffset reads the drawing stream at the same boundary as the
// renderer. The first pushed transform is the origin chosen for the first
// visible line; its horizontal coordinate is where alignment became geometry.
func ayraFirstPaintOffset(t *testing.T, operations *op.Ops) f32.Point {
	t.Helper()
	var reader ops.Reader
	reader.Reset(&operations.Internal)
	for {
		encoded, more := reader.Decode()
		if !more {
			t.Fatal("label produced no line transform")
		}
		if ops.OpType(encoded.Data[0]) != ops.TypeTransform {
			continue
		}
		transform, pushed := ops.DecodeTransform(encoded.Data)
		if pushed {
			return transform.Transform(f32.Point{})
		}
	}
}

// TestAyraLabelEmptyTextIsDefined states that an empty label remains a valid
// layout at every useful width, including no room at all. Empty text is common
// while data is loading; it is not an exceptional input and must neither draw
// outside the offer nor invent horizontal content.
func TestAyraLabelEmptyTextIsDefined(t *testing.T) {
	for _, constraints := range []layout.Constraints{
		{},
		{Max: image.Pt(200, 100)},
		layout.Exact(image.Pt(80, 24)),
	} {
		gtx := ayraLabelContext(constraints)
		dims, info := (Label{}).LayoutDetailed(gtx, ayraLabelShaper(), font.Font{}, unit.Sp(16), "", op.CallOp{})

		if got, want := dims.Size, constraints.Constrain(dims.Size); got != want {
			t.Errorf("constraints %+v: empty label returned %v outside its constraints; constrained value is %v", constraints, got, want)
		}
		if constraints.Min.X == 0 && dims.Size.X != 0 {
			t.Errorf("constraints %+v: empty label invented a width of %d", constraints, dims.Size.X)
		}
		if info.Truncated != 0 {
			t.Errorf("constraints %+v: empty label reported %d truncated runes", constraints, info.Truncated)
		}
	}
}

// TestAyraLabelAlignmentUsesTheOfferedWidth states that alignment is applied
// inside the complete width, rather than around the text's natural width. A
// centred or trailing label in a column only has meaning if the free room is
// part of the calculation.
func TestAyraLabelAlignmentUsesTheOfferedWidth(t *testing.T) {
	const width = 180
	positions := make(map[text.Alignment]float32)
	for _, alignment := range []text.Alignment{text.Start, text.Middle, text.End} {
		gtx := ayraLabelContext(layout.Exact(image.Pt(width, 40)))
		dims := (Label{Alignment: alignment}).Layout(gtx, ayraLabelShaper(), font.Font{}, unit.Sp(16), "short", op.CallOp{})
		if dims.Size.X != width {
			t.Fatalf("alignment %v returned width %d, wanted the offered %d", alignment, dims.Size.X, width)
		}
		positions[alignment] = ayraFirstPaintOffset(t, gtx.Ops).X
	}

	if start, middle, end := positions[text.Start], positions[text.Middle], positions[text.End]; !(start < middle && middle < end) {
		t.Errorf("line origins do not follow start < middle < end within %d px: %.2f, %.2f, %.2f", width, start, middle, end)
	}
}

// TestAyraLabelLongTextElidesInsideTheOffer states that a one-line label uses
// a truncator when its content is wider than the room it was given, and that
// the resulting box never exceeds that room.
func TestAyraLabelLongTextElidesInsideTheOffer(t *testing.T) {
	constraints := layout.Constraints{Max: image.Pt(72, 200)}
	gtx := ayraLabelContext(constraints)
	label := Label{MaxLines: 1}
	dims, info := label.LayoutDetailed(
		gtx,
		ayraLabelShaper(),
		font.Font{},
		unit.Sp(16),
		"this sentence is much wider than the label",
		op.CallOp{},
	)

	if info.Truncated == 0 {
		t.Fatal("long one-line label did not report any elided runes")
	}
	if dims.Size.X > constraints.Max.X || dims.Size.Y > constraints.Max.Y {
		t.Errorf("elided label returned %v outside maximum %v", dims.Size, constraints.Max)
	}
	if dims.Size.X == 0 || dims.Size.Y == 0 {
		t.Errorf("elided label returned an empty box %v", dims.Size)
	}
}
