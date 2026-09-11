package widget

import (
	"image"
	"strings"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// TreeNode is one node of a hierarchy, and the nodes under it.
//
// It is data the caller builds each frame, not state: a node the caller stopped
// supplying is gone, and one it supplies under a new parent has moved. Nothing
// here is remembered, which is what lets a caller rebuild the whole shape from
// a fresh listing without reconciling anything.
type TreeNode struct {
	// Label is the text on the row.
	Label string
	// Detail is the muted text after it, and may be empty.
	Detail string
	// Children are the nodes under this one. A node with none is a leaf.
	Children []TreeNode
}

// Tree is the state of a hierarchy: what is open, what is marked, and the two
// presses each row answers to.
//
// Everything is addressed by path -- the child index at each level, from the
// root down -- and never by the row's position on screen. A flat row number
// names a different node the moment a branch above it closes, so a mark kept
// that way jumps to a neighbour when something unrelated is collapsed.
type Tree struct {
	// Open is the set of paths whose children are drawn, keyed by the path
	// written as text. A map rather than a field on the node, because the node
	// is the caller's and is rebuilt every frame.
	open map[string]bool
	// Marked is the path of the chosen row, and nil when none is.
	marked []int
	// Rows are the two presses of each row, keyed the same way. Keyed rather
	// than indexed for the same reason the open set is: the row at position
	// four is a different node after a branch above it closes, and a press
	// target kept by position then fires for whatever moved into it.
	rows map[string]*treeRow
	// Chosen is the mark waiting to be read.
	chosen   []int
	reported bool
}

// treeRow is what one row remembers between frames.
type treeRow struct {
	turn Button
	pick Button
}

// Selected is the path of the marked node, and nil when none is.
//
// The slice is a copy. Handing back the one the control holds would let a
// caller that kept it see the mark change under it, or change the mark by
// writing to it.
func (t *Tree) Selected() []int { return append([]int(nil), t.marked...) }

// Select marks a node.
func (t *Tree) Select(path []int) { t.marked = append([]int(nil), path...) }

// Chosen reports the path a press marked, once, and consumes it.
func (t *Tree) Chosen() ([]int, bool) {
	if !t.reported {
		return nil, false
	}
	t.reported = false
	return append([]int(nil), t.chosen...), true
}

// Expand draws the children of a node, and of every node above it -- a branch
// opened inside a closed parent is a branch nobody can see.
func (t *Tree) Expand(path []int) {
	t.ensure()
	for depth := 1; depth <= len(path); depth++ {
		t.open[pathKey(path[:depth])] = true
	}
}

// Collapse hides the children of a node.
func (t *Tree) Collapse(path []int) {
	t.ensure()
	delete(t.open, pathKey(path))
}

// Expanded reports whether a node's children are drawn.
func (t *Tree) Expanded(path []int) bool { return t.open[pathKey(path)] }

// Toggle opens a node that is closed and closes one that is open.
func (t *Tree) Toggle(path []int) {
	if t.Expanded(path) {
		t.Collapse(path)
		return
	}
	t.Expand(path)
}

// ensure makes the maps, which a zero Tree does not have.
func (t *Tree) ensure() {
	if t.open == nil {
		t.open = map[string]bool{}
	}
	if t.rows == nil {
		t.rows = map[string]*treeRow{}
	}
}

// row answers the press state of a path, making it on first sight.
func (t *Tree) row(path []int) *treeRow {
	t.ensure()

	key := pathKey(path)
	found, known := t.rows[key]
	if !known {
		found = &treeRow{}
		t.rows[key] = found
	}
	return found
}

// mark records a press and holds it for the caller.
func (t *Tree) mark(path []int) {
	t.marked = append([]int(nil), path...)
	t.chosen, t.reported = t.marked, true
}

// pathKey writes a path as text, so it can key a map.
//
// The separator is a character no index can contain, and the path is written
// with one after every element rather than between them: without the trailing
// separator the paths 1,2 and 12 write the same key, and two unrelated nodes
// then share a mark and a press target.
func pathKey(path []int) string {
	var key strings.Builder
	for _, index := range path {
		writeInt(&key, index)
		key.WriteByte('/')
	}
	return key.String()
}

// writeInt appends a non-negative index without allocating a string for it.
func writeInt(to *strings.Builder, value int) {
	if value >= 10 {
		writeInt(to, value/10)
	}
	to.WriteByte(byte('0' + value%10))
}

// TreeProps is a hierarchy of rows that open and close.
type TreeProps struct {
	// Roots are the top-level nodes, in order.
	Roots []TreeNode
	// Indent is how far each level is set in. Zero takes a step a row's own
	// marker is legible against.
	Indent unit.Dp
	// Disabled stops every row answering: neither the markers nor the labels
	// react, and nothing is reported.
	Disabled bool
}

// Layout draws the hierarchy and returns the room it took.
func (p TreeProps) Layout(c ayra.Context, state *Tree) ayra.Dimensions {
	state.ensure()

	rows := make([]layout.FlexChild, 0, len(p.Roots)*2)
	rows = p.branch(c, state, p.Roots, nil, rows)

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
}

// branch appends the rows of one level, and of every open node in it.
//
// Recursion follows the caller's own structure, which is a tree and therefore
// finite: a node cannot contain itself, because containing it would mean
// copying it, and the copy is what the child holds.
func (p TreeProps) branch(c ayra.Context, state *Tree, nodes []TreeNode, at []int, into []layout.FlexChild) []layout.FlexChild {
	for index, node := range nodes {
		path := append(append([]int(nil), at...), index)
		node := node

		if !p.Disabled {
			row := state.row(path)
			if len(node.Children) > 0 && row.turn.Clicked(c) {
				state.Toggle(path)
			}
			if row.pick.Clicked(c) {
				state.mark(path)
			}
		}

		into = append(into, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.row(c.With(gtx), state, node, path)
		}))

		if len(node.Children) > 0 && state.Expanded(path) {
			into = p.branch(c, state, node.Children, path, into)
		}
	}
	return into
}

// row draws one line of the hierarchy.
//
// The label is a list row rather than a button, and the difference is where the
// text sits: a button centres its label, because a button takes the width of
// the column it is in and left-aligned text in a full-width one reads as a
// heading with a box round it. A tree is the opposite -- every label has to
// begin at the same place as the labels of its level, and centring them makes
// the indent move the middle of each word instead of its start, so a hierarchy
// draws as a column of text with no shape at all.
func (p TreeProps) row(c ayra.Context, state *Tree, node TreeNode, path []int) ayra.Dimensions {
	indent := p.Indent
	if indent == 0 {
		indent = 16
	}

	state.ensure()
	row := state.row(path)
	marked := samePath(path, state.marked)

	return layout.Inset{Left: unit.Dp(float32(indent) * float32(len(path)-1))}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.turn(inner.With(gtx), state, row, node, path)
			}),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ItemProps{
					Title:     node.Label,
					Pressable: !p.Disabled,
					Selected:  marked,
				}.Layout(inner.With(gtx), &row.pick, func(c ayra.Context) ayra.Dimensions {
					if node.Detail == "" {
						return ayra.Dimensions{}
					}
					return drawText(c, node.Detail, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.End, plain())
				})
			}),
		)
	})
}

// turn draws the marker that opens a branch, or the room where a leaf's would
// be.
//
// The width is kept rather than removed, and that is deliberate: without it a
// leaf's label starts where its siblings' markers do, so one level draws at two
// different offsets and the branches look broken. What is not kept is the press
// -- a leaf has nothing to open, and a target that answers nothing swallows the
// press that would otherwise have marked the row.
//
// The height is the marker's own and decides nothing: the row is as tall as the
// label beside it, which is taller.
func (p TreeProps) turn(c ayra.Context, state *Tree, row *treeRow, node TreeNode, path []int) ayra.Dimensions {
	side := c.Dp(unit.Dp(10))
	if len(node.Children) == 0 {
		return ayra.Dimensions{Size: image.Pt(side, side)}
	}

	open := state.Expanded(path)
	return row.turn.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return marker(c.With(gtx), open)
	})
}

// samePath reports whether two paths name the same node.
func samePath(a, b []int) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}
