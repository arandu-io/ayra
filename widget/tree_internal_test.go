package widget

import (
	"testing"

	"github.com/arandu-io/ayra/theme"
)

// forest is the hierarchy these tests walk. Two roots, one of them three levels
// deep, and a leaf beside a branch at every level.
func forest() []TreeNode {
	return []TreeNode{
		{Label: "app", Children: []TreeNode{
			{Label: "Http", Children: []TreeNode{
				{Label: "Controllers"},
				{Label: "Middleware"},
			}},
			{Label: "Models"},
		}},
		{Label: "go.mod", Detail: "1.2 kB"},
	}
}

// TestAPathIsWrittenSoThatNoTwoNodesShareIt is the one arithmetic in this
// control, and getting it wrong makes two unrelated nodes share a mark.
func TestAPathIsWrittenSoThatNoTwoNodesShareIt(t *testing.T) {
	seen := map[string][]int{}

	for _, path := range [][]int{
		{1, 2}, {12}, {1, 2, 3}, {123}, {0}, {0, 0}, {}, {10, 1}, {1, 0, 1},
	} {
		key := pathKey(path)
		if other, taken := seen[key]; taken {
			t.Errorf("the paths %v and %v are both written %q", other, path, key)
			continue
		}
		seen[key] = path
	}
}

// TestOpeningANodeDeepInsideOpensWhatIsAboveIt fixes a branch opened inside a
// closed parent, which is a branch nobody can see.
func TestOpeningANodeDeepInsideOpensWhatIsAboveIt(t *testing.T) {
	var state Tree
	state.Expand([]int{0, 0, 1})

	for _, path := range [][]int{{0}, {0, 0}, {0, 0, 1}} {
		if !state.Expanded(path) {
			t.Errorf("%v is closed, so what was opened under it cannot be seen", path)
		}
	}
}

// TestClosingANodeLeavesWhatIsAboveItOpen keeps one collapse from folding the
// whole branch back to the root.
func TestClosingANodeLeavesWhatIsAboveItOpen(t *testing.T) {
	var state Tree
	state.Expand([]int{0, 0, 1})
	state.Collapse([]int{0, 0})

	if state.Expanded([]int{0, 0}) {
		t.Error("the node did not close")
	}
	if !state.Expanded([]int{0}) {
		t.Error("closing a node closed its parent")
	}
}

// TestAMarkSurvivesACollapseAboveIt is why the mark is a path and not a row
// number.
//
// Closing a branch removes rows from the screen. A mark kept by position then
// names whatever moved into that position, which is a neighbour the person
// never touched.
func TestAMarkSurvivesACollapseAboveIt(t *testing.T) {
	var state Tree
	state.Expand([]int{0, 0})
	state.Select([]int{1})

	state.Collapse([]int{0})

	if got := state.Selected(); !samePath(got, []int{1}) {
		t.Errorf("the mark moved to %v when a branch above it closed", got)
	}
}

// TestTheMarkHandedBackIsACopy keeps a caller that kept the slice from writing
// to the control's own state.
func TestTheMarkHandedBackIsACopy(t *testing.T) {
	var state Tree
	state.Select([]int{0, 1})

	got := state.Selected()
	got[0] = 9

	if again := state.Selected(); !samePath(again, []int{0, 1}) {
		t.Errorf("writing to the answer changed the mark to %v", again)
	}
}

// TestSelectDoesNotReadAsAPressByTheUser keeps a screen that arrives with a
// node marked from firing whatever a press fires.
func TestSelectDoesNotReadAsAPressByTheUser(t *testing.T) {
	var state Tree
	state.Select([]int{0})

	if _, pressed := state.Chosen(); pressed {
		t.Error("marking a node from the screen reported a press")
	}
}

// TestAPressIsReportedOnceAndConsumed keeps one press from being acted on every
// frame for as long as the row stays marked.
func TestAPressIsReportedOnceAndConsumed(t *testing.T) {
	var state Tree
	state.mark([]int{0, 1})

	path, pressed := state.Chosen()
	if !pressed {
		t.Fatal("the press was not reported")
	}
	if !samePath(path, []int{0, 1}) {
		t.Errorf("the press reported %v", path)
	}
	if _, again := state.Chosen(); again {
		t.Error("the press was reported twice, and whatever it fires fires twice")
	}
}

// TestTogglingIsOpeningAndClosing keeps the one press a marker answers to from
// only working one way.
func TestTogglingIsOpeningAndClosing(t *testing.T) {
	var state Tree
	path := []int{0, 0}

	state.Toggle(path)
	if !state.Expanded(path) {
		t.Fatal("the first press did not open it")
	}
	state.Toggle(path)
	if state.Expanded(path) {
		t.Error("the second press did not close it")
	}
}

// TestARowKeepsItsPressTargetAcrossAReshape is why the targets are keyed by
// path rather than held in a slice.
//
// A slice indexed by the row's position hands the press state of row four to
// whatever is at position four next frame, and after a collapse that is a
// different node.
func TestARowKeepsItsPressTargetAcrossAReshape(t *testing.T) {
	var state Tree
	deep := []int{0, 0, 1}

	state.Expand(deep)
	before := state.row(deep)

	state.Collapse([]int{0})
	state.Expand(deep)

	if state.row(deep) != before {
		t.Error("the row was handed a different press target after the branch closed and opened")
	}
}

// TestTwoNodesNeverShareAPressTarget is the other half of the path being
// written the way it is.
//
// A key that took only part of the path would hand one press target to every
// node under a root: pressing one row would mark, and redraw, a different one.
func TestTwoNodesNeverShareAPressTarget(t *testing.T) {
	var state Tree

	targets := map[*treeRow][]int{}
	for _, path := range [][]int{{0}, {1}, {0, 0}, {0, 1}, {0, 0, 0}, {0, 0, 1}} {
		row := state.row(path)
		if other, taken := targets[row]; taken {
			t.Errorf("%v and %v are the same press target", other, path)
			continue
		}
		targets[row] = path
	}
}

// TestAZeroTreeDrawsWithoutBeingPrepared keeps a caller from having to call a
// constructor the API does not have.
func TestAZeroTreeDrawsWithoutBeingPrepared(t *testing.T) {
	var state Tree

	c, _ := field(t, theme.Light, 400)
	dims := TreeProps{Roots: forest()}.Layout(c, &state)

	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("a tree with two roots took %v", dims.Size)
	}
}

// TestOnlyTheOpenBranchesAreDrawn is what makes a deep hierarchy cheap: a tree
// that laid out every node would pay for a thousand rows to show ten.
func TestOnlyTheOpenBranchesAreDrawn(t *testing.T) {
	var state Tree
	props := TreeProps{Roots: forest()}

	c, _ := field(t, theme.Light, 400)
	closed := props.Layout(c, &state)

	state.Expand([]int{0})
	c, _ = field(t, theme.Light, 400)
	opened := props.Layout(c, &state)

	if opened.Size.Y <= closed.Size.Y {
		t.Errorf("opening a branch took %d and closed took %d; the children were drawn either way", opened.Size.Y, closed.Size.Y)
	}

	state.Expand([]int{0, 0})
	c, _ = field(t, theme.Light, 400)
	deeper := props.Layout(c, &state)

	if deeper.Size.Y <= opened.Size.Y {
		t.Errorf("opening a branch inside took %d and one level took %d", deeper.Size.Y, opened.Size.Y)
	}
}

// TestALeafKeepsTheRoomItsMarkerWouldTake fixes a level that reads as two.
//
// Without the room, a leaf's label starts where its siblings' markers do, so
// the leaves and the branches of one level line up differently and the
// hierarchy looks broken.
func TestALeafKeepsTheRoomItsMarkerWouldTake(t *testing.T) {
	var withLeaf, withBranch Tree

	leaf := TreeProps{Roots: []TreeNode{{Label: "go.mod"}}}
	branch := TreeProps{Roots: []TreeNode{{Label: "go.mod", Children: []TreeNode{{Label: "x"}}}}}

	c, _ := field(t, theme.Light, 400)
	leafRow := leaf.Layout(c, &withLeaf)

	c, _ = field(t, theme.Light, 400)
	branchRow := branch.Layout(c, &withBranch)

	if leafRow.Size.Y != branchRow.Size.Y {
		t.Errorf("a leaf row is %d tall and a branch row is %d", leafRow.Size.Y, branchRow.Size.Y)
	}
}

// TestADisabledTreeAnswersNothing keeps a hierarchy that is drawn unavailable
// from opening and marking under the finger.
func TestADisabledTreeAnswersNothing(t *testing.T) {
	var state Tree
	state.Expand([]int{0})

	c, _ := field(t, theme.Light, 400)
	TreeProps{Roots: forest(), Disabled: true}.Layout(c, &state)

	if _, pressed := state.Chosen(); pressed {
		t.Error("a disabled tree reported a press")
	}
}

// TestNothingMarkedIsNotTheFirstNode keeps a fresh tree from drawing a row as
// chosen before anybody chose one.
func TestNothingMarkedIsNotTheFirstNode(t *testing.T) {
	var state Tree

	if got := state.Selected(); got != nil {
		t.Errorf("a fresh tree has %v marked", got)
	}
	if samePath(nil, nil) {
		t.Error("nothing marked matched nothing marked, which draws every row as chosen")
	}
	if samePath([]int{}, []int{}) {
		t.Error("an empty path matched an empty path, which is the root nobody drew")
	}
}
