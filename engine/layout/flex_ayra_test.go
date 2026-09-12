package layout

import (
	"image"
	"math/rand/v2"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	internalops "github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/op"
)

// axes is both directions, run over by every property below.
//
// A layout that is right horizontally and wrong vertically is the common fault
// here, because the vertical case is the one nobody looks at while writing the
// code: main and cross are swapped by a conversion, and a test that only ever
// asks for a row never exercises the swap.
var axes = []Axis{Horizontal, Vertical}

// exactly returns a context whose available room is fixed at main by cross.
func exactly(axis Axis, main, cross int) Context {
	size := axis.Convert(image.Pt(main, cross))
	return Context{
		Ops:         new(op.Ops),
		Constraints: Exact(size),
	}
}

// atMost returns a context with room up to main by cross and no lower bound.
func atMost(axis Axis, main, cross int) Context {
	size := axis.Convert(image.Pt(main, cross))
	return Context{
		Ops:         new(op.Ops),
		Constraints: Constraints{Max: size},
	}
}

// obedient is a child that takes the main size it was told to take.
//
// It is what a well behaved flexed child does, and it is written here rather
// than measured from a real control so that the arithmetic under test is the
// only thing that can move the number.
func obedient(axis Axis, cross int, taken *int) Widget {
	return func(gtx Context) Dimensions {
		main, _ := axis.mainConstraint(gtx.Constraints)
		if taken != nil {
			*taken = main
		}
		return Dimensions{Size: axis.Convert(image.Pt(main, cross))}
	}
}

// fixed is a child of a size of its own, ignoring what it was offered.
func fixed(axis Axis, main, cross int) Widget {
	return func(Context) Dimensions {
		return Dimensions{Size: axis.Convert(image.Pt(main, cross))}
	}
}

// placements reports where each child was put, in the order they were put
// there.
//
// It reads the drawing rather than the return value, because where a child
// lands is the half of a layout that Dimensions does not carry: a Flex that
// reports the right total size and stacks every child at the origin returns
// exactly what a correct one returns. The children used with this helper draw
// nothing, so every offset in the list belongs to the layout under test.
func placements(t *testing.T, axis Axis, o *op.Ops) []int {
	t.Helper()

	points := decodedOffsets(t, o)
	out := make([]int, 0, len(points))
	for _, point := range points {
		out = append(out, axis.Convert(point).X)
	}
	return out
}

// decodedOffsets reports the origin established by every pushed transform.
func decodedOffsets(t *testing.T, o *op.Ops) []image.Point {
	t.Helper()

	var reader internalops.Reader
	reader.Reset(&o.Internal)

	var out []image.Point
	for {
		encoded, ok := reader.Decode()
		if !ok {
			return out
		}
		if internalops.OpType(encoded.Data[0]) != internalops.TypeTransform {
			continue
		}
		transform, pushed := internalops.DecodeTransform(encoded.Data)
		if !pushed {
			continue
		}
		at := transform.Transform(f32.Point{})
		out = append(out, image.Pt(int(at.X), int(at.Y)))
	}
}

// TestAFlexedChildIsToldItsSizeByTheMinimumConstraint fixes the one fact about
// this layout that a control has to know.
//
// A flexed child is not offered room to choose from: it is handed a decision
// already made, as a minimum and a maximum that are the same number on the main
// axis. A rigid child is the other case, and the difference is the whole of it
// -- rigid is asked how big it wants to be, flexed is told how big it is.
//
// The defect this guards is the one that keeps coming back: a control that
// wants to match a sibling reads the constraint it was given instead of
// measuring that sibling, and the two agree only as long as the control happens
// to be flexed. Made rigid, the same control reads the space left over for
// everything after it and draws itself the width of the rest of the row.
func TestAFlexedChildIsToldItsSizeByTheMinimumConstraint(t *testing.T) {
	for _, axis := range axes {
		t.Run(axis.String(), func(t *testing.T) {
			var flexedMin, flexedMax, rigidMin, rigidMax int

			Flex{Axis: axis}.Layout(atMost(axis, 100, 50),
				Rigid(func(gtx Context) Dimensions {
					rigidMin, rigidMax = axis.mainConstraint(gtx.Constraints)
					return Dimensions{Size: axis.Convert(image.Pt(30, 10))}
				}),
				Flexed(1, func(gtx Context) Dimensions {
					flexedMin, flexedMax = axis.mainConstraint(gtx.Constraints)
					return Dimensions{Size: axis.Convert(image.Pt(flexedMin, 10))}
				}),
			)

			if flexedMin != 70 || flexedMax != 70 {
				t.Errorf("a flexed child was told [%d, %d] and should have been told [70, 70]; "+
					"its size is the minimum constraint, not a range to choose from", flexedMin, flexedMax)
			}
			if rigidMin != 0 {
				t.Errorf("a rigid child was given a minimum of %d and should have been given 0; "+
					"a rigid child chooses its own size", rigidMin)
			}
			if rigidMax != 100 {
				t.Errorf("a rigid child was offered %d and should have been offered 100", rigidMax)
			}
		})
	}
}

// TestFlexedChildrenDivideTheAxisDeterministicallyWithoutCrossingIt fixes the
// arithmetic contract without pretending every weighted division is integral.
//
// Rounding is carried from one child to the next, so a child may leave a short
// tail. The tail is strictly smaller than one pixel per child, never becomes an
// overflow, and the same weights always choose the same pixels. That last fact
// matters to a raster: moving a remainder between two equal cells moves every
// centred mark in one of them even though the total is unchanged.
func TestFlexedChildrenDivideTheAxisDeterministicallyWithoutCrossingIt(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))

	for _, axis := range axes {
		t.Run(axis.String(), func(t *testing.T) {
			for range 500 {
				count := 1 + random.IntN(8)
				total := 1 + random.IntN(400)

				weights := make([]float32, count)
				for i := range weights {
					weights[i] = .01 + random.Float32()*10
				}

				first, dims := weightedSizes(axis, total, weights)
				second, _ := weightedSizes(axis, total, weights)
				if !intsEqual(first, second) {
					t.Fatalf("same weights %v in %d produced %v and then %v", weights, total, first, second)
				}

				var sum int
				for _, size := range first {
					if size < 0 {
						t.Fatalf("a child was given a negative size %d for weights %v in %d", size, weights, total)
					}
					sum += size
				}
				remaining := total - sum
				if remaining < 0 {
					t.Fatalf("children crossed the axis by %d pixels for weights %v in %d", -remaining, weights, total)
				}
				if remaining >= count {
					t.Fatalf("children left %d pixels for %d weights %v in %d; rounding may leave less than one pixel per child",
						remaining, count, weights, total)
				}
				if got := axis.Convert(dims.Size).X; got != total {
					t.Fatalf("the layout reported %d of %d for weights %v", got, total, weights)
				}
			}
		})
	}
}

// TestSevenEqualWeightsLeaveTheNonDivisiblePixelAtTheEnd pins the visible case
// used by calendar rows. The cells remain equal and the single indivisible
// pixel stays after them instead of moving into an arbitrary cell.
func TestSevenEqualWeightsLeaveTheNonDivisiblePixelAtTheEnd(t *testing.T) {
	weights := []float32{1, 1, 1, 1, 1, 1, 1}
	want := []int{48, 48, 48, 48, 48, 48, 48}
	for _, axis := range axes {
		t.Run(axis.String(), func(t *testing.T) {
			got, dims := weightedSizes(axis, 337, weights)
			if !intsEqual(got, want) {
				t.Errorf("seven equal weights in 337 pixels received %v, want %v with one pixel left at the end", got, want)
			}
			if main := axis.Convert(dims.Size).X; main != 337 {
				t.Errorf("layout reported %d pixels, want its constrained width 337", main)
			}
		})
	}
}

func weightedSizes(axis Axis, total int, weights []float32) ([]int, Dimensions) {
	taken := make([]int, len(weights))
	children := make([]FlexChild, len(weights))
	for i, weight := range weights {
		children[i] = Flexed(weight, obedient(axis, 10, &taken[i]))
	}
	dims := Flex{Axis: axis}.Layout(exactly(axis, total, 50), children...)
	return taken, dims
}

// TestAWeightOfZeroTakesNoSpace fixes the value that turns a child off.
//
// Zero is the only weight with a meaning of its own, and it is the one a caller
// computes: a row whose children are weighted by a quantity has a child whose
// quantity is nothing, and it has to disappear rather than be rounded up to a
// sliver. The sliver is what a caller sees as a stripe of colour with no
// content in it.
func TestAWeightOfZeroTakesNoSpace(t *testing.T) {
	for _, axis := range axes {
		t.Run(axis.String(), func(t *testing.T) {
			var silent, loud int

			Flex{Axis: axis}.Layout(exactly(axis, 100, 50),
				Flexed(0, obedient(axis, 10, &silent)),
				Flexed(1, obedient(axis, 10, &loud)),
			)

			if silent != 0 {
				t.Errorf("a child of weight zero took %d and should have taken nothing", silent)
			}
			if loud != 100 {
				t.Errorf("the only weighted child took %d of 100", loud)
			}
		})
	}
}

// TestOneFlexedChildTakesWhatTheRigidsLeave is the ordinary case, and the one
// every screen in this repository is built out of.
//
// The rigid children are measured first and keep what they asked for; the
// single flexed child is the rest, whatever the rest turned out to be. The gap
// is reserved before the division rather than taken out of the flexed child
// afterwards, which is the difference between a row that adds up and a row that
// is one gap too wide.
func TestOneFlexedChildTakesWhatTheRigidsLeave(t *testing.T) {
	for _, axis := range axes {
		t.Run(axis.String(), func(t *testing.T) {
			for _, tc := range []struct {
				name     string
				gap      int
				expected int
			}{
				{name: "no gap", gap: 0, expected: 100 - 30 - 20},
				{name: "two gaps", gap: 7, expected: 100 - 30 - 20 - 14},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var taken int

					dims := Flex{Axis: axis, Gap: tc.gap}.Layout(exactly(axis, 100, 50),
						Rigid(fixed(axis, 30, 10)),
						Flexed(1, obedient(axis, 10, &taken)),
						Rigid(fixed(axis, 20, 10)),
					)

					if taken != tc.expected {
						t.Errorf("the flexed child took %d and should have taken %d", taken, tc.expected)
					}
					if got := axis.Convert(dims.Size).X; got != 100 {
						t.Errorf("the row came to %d of 100", got)
					}
				})
			}
		})
	}
}

// TestSpacingPutsTheLeftoverWhereItSaysItDoes walks the six modes.
//
// Each one is checked by where the children landed, because the name of a
// spacing mode is a claim about the gaps and about nothing else: all six return
// the same size, and five of them are wrong in a way the returned Dimensions
// cannot show.
//
// The room left over divides evenly here. That is deliberate: this test says
// what the modes mean, and the case where the division does not come out even
// is a different question, asked below.
func TestSpacingPutsTheLeftoverWhereItSaysItDoes(t *testing.T) {
	// Two children of twenty in a hundred leave sixty to place.
	for _, tc := range []struct {
		spacing  Spacing
		expected []int
	}{
		{SpaceEnd, []int{0, 20}},
		{SpaceStart, []int{60, 80}},
		{SpaceSides, []int{30, 50}},
		{SpaceBetween, []int{0, 80}},
		{SpaceEvenly, []int{20, 60}},
		{SpaceAround, []int{15, 65}},
	} {
		t.Run(tc.spacing.String(), func(t *testing.T) {
			for _, axis := range axes {
				t.Run(axis.String(), func(t *testing.T) {
					gtx := exactly(axis, 100, 50)
					dims := Flex{Axis: axis, Spacing: tc.spacing}.Layout(gtx,
						Rigid(fixed(axis, 20, 10)),
						Rigid(fixed(axis, 20, 10)),
					)

					got := placements(t, axis, gtx.Ops)
					if len(got) != len(tc.expected) {
						t.Fatalf("%d children were placed and %d were laid out", len(got), len(tc.expected))
					}
					for i, at := range got {
						if at != tc.expected[i] {
							t.Errorf("child %d was placed at %d and belongs at %d", i, at, tc.expected[i])
						}
					}
					if got := axis.Convert(dims.Size).X; got != 100 {
						t.Errorf("the row came to %d of 100", got)
					}
				})
			}
		})
	}
}

// TestSpacingNeverRunsPastTheAxis is the uneven case.
//
// When the room left over does not divide evenly the modes round down, so the
// last child can end short of the far edge. Short is a pixel of background
// nobody notices; past the edge is a child drawn outside the row, and it is
// clipped or it overlaps whatever is next. This fixes the direction of the
// error rather than pretending there is none.
func TestSpacingNeverRunsPastTheAxis(t *testing.T) {
	for _, spacing := range []Spacing{SpaceEnd, SpaceStart, SpaceSides, SpaceAround, SpaceBetween, SpaceEvenly} {
		t.Run(spacing.String(), func(t *testing.T) {
			for main := 40; main <= 100; main++ {
				for count := 1; count <= 5; count++ {
					children := make([]FlexChild, count)
					for i := range children {
						children[i] = Rigid(fixed(Horizontal, 7, 10))
					}

					gtx := exactly(Horizontal, main, 50)
					Flex{Spacing: spacing}.Layout(gtx, children...)

					previous := -1
					for i, at := range placements(t, Horizontal, gtx.Ops) {
						if at < previous {
							t.Fatalf("%s in %d with %d children placed child %d at %d, behind the one before it",
								spacing, main, count, i, at)
						}
						if at+7 > main {
							t.Fatalf("%s in %d with %d children ran child %d from %d past the edge",
								spacing, main, count, i, at)
						}
						previous = at
					}
				}
			}
		})
	}
}

// TestEverySpacingNamesItself guards a set of names that is only ever read by a
// person.
//
// A mode that answers with another mode's name is invisible until the day
// somebody is looking at a layout that is placing children somewhere
// unexpected, and then it costs that whole day: the report says the row is
// doing what it should be doing, so the search moves on to the children, which
// are innocent.
func TestEverySpacingNamesItself(t *testing.T) {
	for name, spacing := range map[string]Spacing{
		"SpaceEnd":     SpaceEnd,
		"SpaceStart":   SpaceStart,
		"SpaceSides":   SpaceSides,
		"SpaceAround":  SpaceAround,
		"SpaceBetween": SpaceBetween,
		"SpaceEvenly":  SpaceEvenly,
	} {
		if got := spacing.String(); got != name {
			t.Errorf("%s answers to %q", name, got)
		}
	}
}

// TestAFlexWithNoChildrenIsItsMinimum keeps the empty case from collapsing.
//
// A row whose children are all behind a condition that is currently false is
// still the row: it holds its place until one of them comes back. Collapsing to
// nothing moves everything below it up and then down again, which reads as the
// screen jumping.
func TestAFlexWithNoChildrenIsItsMinimum(t *testing.T) {
	for _, axis := range axes {
		t.Run(axis.String(), func(t *testing.T) {
			dims := Flex{Axis: axis}.Layout(exactly(axis, 100, 50))
			if got, expected := dims.Size, image.Pt(100, 50); axis.Convert(got) != expected {
				t.Errorf("an empty row came to %v and should have come to %v", axis.Convert(got), expected)
			}
		})
	}
}

// TestTheWeightSumOverridesTheChildrensOwn is how a caller leaves room on
// purpose.
//
// Weights are a fraction of a sum, and by default the sum is the weights
// themselves, so children always fill the axis. Naming a larger sum is the way
// to say that the children are two thirds of the row and the last third is
// nobody's -- which is a layout a caller cannot otherwise write without adding
// an empty child to stand in for the hole.
func TestTheWeightSumOverridesTheChildrensOwn(t *testing.T) {
	for _, axis := range axes {
		t.Run(axis.String(), func(t *testing.T) {
			var first, second int

			Flex{Axis: axis, WeightSum: 4}.Layout(exactly(axis, 100, 50),
				Flexed(1, obedient(axis, 10, &first)),
				Flexed(1, obedient(axis, 10, &second)),
			)

			if first != 25 || second != 25 {
				t.Errorf("children of weight one in a sum of four took %d and %d, and each is a quarter of 100",
					first, second)
			}
		})
	}
}

// TestTheCrossAxisIsTheLargestChild fixes the other dimension.
//
// A row is as tall as its tallest child, and every child is offered that much
// room in the cross axis rather than its own -- which is what lets a short
// child centre itself against a tall one instead of sitting at the top of a
// space it does not know the size of.
func TestTheCrossAxisIsTheLargestChild(t *testing.T) {
	for _, axis := range axes {
		t.Run(axis.String(), func(t *testing.T) {
			dims := Flex{Axis: axis}.Layout(atMost(axis, 100, 50),
				Rigid(fixed(axis, 10, 12)),
				Rigid(fixed(axis, 10, 31)),
				Rigid(fixed(axis, 10, 7)),
			)

			if got := axis.Convert(dims.Size).Y; got != 31 {
				t.Errorf("the cross axis came to %d and the largest child is 31", got)
			}
		})
	}
}

// TestRigidChildrenAreMeasuredFirstButDrawnWhereTheyWereDeclared separates the
// two orders a flex has to preserve.
//
// Measurement order determines how much room is left, so rigid children go
// first. Drawing order determines which overlapping child is visible, so it
// remains the caller's order. Conflating the two either gives a flexed child too
// much room or silently changes which child is on top.
func TestRigidChildrenAreMeasuredFirstButDrawnWhereTheyWereDeclared(t *testing.T) {
	for _, axis := range axes {
		t.Run(axis.String(), func(t *testing.T) {
			var measured []string
			child := func(name string, main int) Widget {
				return func(Context) Dimensions {
					measured = append(measured, name)
					return Dimensions{Size: axis.Convert(image.Pt(main, 10))}
				}
			}
			flexed := func(gtx Context) Dimensions {
				measured = append(measured, "flexed")
				main, _ := axis.mainConstraint(gtx.Constraints)
				return Dimensions{Size: axis.Convert(image.Pt(main, 10))}
			}

			gtx := exactly(axis, 100, 50)
			Flex{Axis: axis}.Layout(gtx,
				Flexed(1, flexed),
				Rigid(child("first rigid", 10)),
				Rigid(child("second rigid", 20)),
			)

			if got, want := measured, []string{"first rigid", "second rigid", "flexed"}; !slicesEqual(got, want) {
				t.Fatalf("measurement order was %v and should have been %v", got, want)
			}
			if got, want := placements(t, axis, gtx.Ops), []int{0, 70, 80}; !intsEqual(got, want) {
				t.Fatalf("drawing positions were %v and should have followed declaration order at %v", got, want)
			}
		})
	}
}

// TestCrossAlignmentMovesOnlyTheCrossAxis fixes the three geometric alignment
// modes in both orientations.
func TestCrossAlignmentMovesOnlyTheCrossAxis(t *testing.T) {
	for _, alignment := range []struct {
		value Alignment
		cross int
	}{
		{Start, 0},
		{End, 20},
		{Middle, 10},
	} {
		t.Run(alignment.value.String(), func(t *testing.T) {
			for _, axis := range axes {
				t.Run(axis.String(), func(t *testing.T) {
					gtx := atMost(axis, 100, 50)
					Flex{Axis: axis, Alignment: alignment.value}.Layout(gtx,
						Rigid(fixed(axis, 10, 10)),
						Rigid(fixed(axis, 10, 30)),
					)

					points := decodedOffsets(t, gtx.Ops)
					if len(points) != 2 {
						t.Fatalf("got %d placements for two children", len(points))
					}
					first := axis.Convert(points[0])
					if first != image.Pt(0, alignment.cross) {
						t.Errorf("first child was placed at %v and should have been at main 0, cross %d", first, alignment.cross)
					}
					second := axis.Convert(points[1])
					if second != image.Pt(10, 0) {
						t.Errorf("second child was placed at %v and should have been at (10,0)", second)
					}
				})
			}
		})
	}
}

// TestBaselineAlignmentSharesOneTextLine fixes both the child offsets and the
// baseline reported to a parent.
func TestBaselineAlignmentSharesOneTextLine(t *testing.T) {
	gtx := atMost(Horizontal, 100, 50)
	dims := Flex{Alignment: Baseline}.Layout(gtx,
		Rigid(func(Context) Dimensions {
			return Dimensions{Size: image.Pt(10, 10), Baseline: 2}
		}),
		Rigid(func(Context) Dimensions {
			return Dimensions{Size: image.Pt(10, 20), Baseline: 8}
		}),
	)

	if got, want := decodedOffsets(t, gtx.Ops), []image.Point{image.Pt(0, 4), image.Pt(10, 0)}; !pointsEqual(got, want) {
		t.Errorf("baseline placements were %v and should have been %v", got, want)
	}
	if dims.Baseline != 8 {
		t.Errorf("reported baseline was %d and should have been 8 pixels from the bottom", dims.Baseline)
	}
}

// TestSpacingDoesNotInflateAnEmptyFlex fixes the zero-child boundary. Spacing
// distributes room around children; without a child there is nothing to
// distribute around, and the layout remains at its minimum size.
func TestSpacingDoesNotInflateAnEmptyFlex(t *testing.T) {
	for _, spacing := range []Spacing{SpaceEnd, SpaceStart, SpaceSides, SpaceAround, SpaceBetween, SpaceEvenly} {
		t.Run(spacing.String(), func(t *testing.T) {
			gtx := Context{
				Ops: new(op.Ops),
				Constraints: Constraints{
					Min: image.Pt(40, 20),
					Max: image.Pt(100, 80),
				},
			}
			dims := Flex{Spacing: spacing}.Layout(gtx)
			if dims.Size != gtx.Constraints.Min {
				t.Errorf("empty flex with %s measured %v and should have stayed at its minimum %v", spacing, dims.Size, gtx.Constraints.Min)
			}
		})
	}
}

func intsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func pointsEqual(a, b []image.Point) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
