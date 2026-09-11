package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/semantic"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// What a control says about itself is read off the drawn frame, not off the
// call that made it.
//
// A test that checked the source for a function call would pass on a control
// that announced itself outside the shape it drew, or after the shape was
// popped -- and both of those produce a tree where the description belongs to
// the wrong node, or to none. The platform that reads a screen aloud walks that
// tree. So these ask the router what the tree came out as.

// tree draws a widget into a router and answers the semantic tree it produced.
func tree(t *testing.T, draw ayra.Widget) []input.SemanticNode {
	t.Helper()

	var router input.Router
	ops := new(op.Ops)

	gtx := layout.Context{
		Ops:         ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(400, 600)},
	}
	draw(ayra.Context{
		Context: gtx,
		Theme:   theme.New(theme.Light),
		Shaper:  text.NewShaper(text.WithCollection(gofont.Collection())),
	})

	router.Frame(ops)
	return router.AppendSemantics(nil)
}

// walk answers every node of a tree.
//
// The list the router hands back is already every node, and each entry also
// carries its children. Walking into those counts each one twice, which is a
// test that reports a control announced twice when it was announced once --
// and it hid a real duplicate behind a made-up one.
func walk(nodes []input.SemanticNode) []input.SemanticNode { return nodes }

// spoken answers every label in a tree.
func spoken(nodes []input.SemanticNode) []string {
	var said []string
	for _, node := range walk(nodes) {
		if node.Desc.Label != "" {
			said = append(said, node.Desc.Label)
		}
	}
	return said
}

// ofClass answers every node announced as one kind of control.
//
// Asked by class rather than by label, because the label is not this library's
// alone: every piece of text announces itself, so the same words appear again
// as ordinary prose beside the control that carries them. What has to be
// exactly one is the control.
func ofClass(nodes []input.SemanticNode, class semantic.ClassOp) []input.SemanticNode {
	var found []input.SemanticNode
	for _, node := range walk(nodes) {
		if node.Desc.Class == class {
			found = append(found, node)
		}
	}
	return found
}

// TestAButtonSaysWhatItIs is the first thing anything reading the screen needs.
//
// Before this, nothing in the library announced anything at all: the one
// platform of nine that walks the tree walked an empty one, so the boundary the
// documentation declared was wider than the one that had been written down.
func TestAButtonSaysWhatItIs(t *testing.T) {
	var state widget.Button

	nodes := tree(t, func(c ayra.Context) ayra.Dimensions {
		return widget.ButtonProps{Label: "Salvar"}.Layout(c, &state)
	})

	controls := ofClass(nodes, semantic.Button)
	if len(controls) != 1 {
		t.Fatalf("the tree holds %d buttons, want one; what is in it: %v", len(controls), spoken(nodes))
	}
	if controls[0].Desc.Label != "Salvar" {
		t.Errorf("the button is announced as %q", controls[0].Desc.Label)
	}
	if controls[0].Desc.Disabled {
		t.Error("a live button is announced as unavailable")
	}
}

// TestADisabledControlSaysSo keeps somebody who cannot see the fade from
// pressing a control that will not answer.
func TestADisabledControlSaysSo(t *testing.T) {
	var state widget.Button

	nodes := tree(t, func(c ayra.Context) ayra.Dimensions {
		return widget.ButtonProps{Label: "Salvar", Disabled: true}.Layout(c, &state)
	})

	controls := ofClass(nodes, semantic.Button)
	if len(controls) != 1 {
		t.Fatalf("the tree holds %d buttons, want one; what is in it: %v", len(controls), spoken(nodes))
	}
	if !controls[0].Desc.Disabled {
		t.Error("a disabled button is announced as available")
	}
}

// TestAControlWithNoNameSaysNothing keeps a silence out of the tree.
//
// A node with an empty label is something a reader stops on and cannot act on,
// which is worse than a control it passes over.
func TestAControlWithNoNameSaysNothing(t *testing.T) {
	var state widget.Button

	nodes := tree(t, func(c ayra.Context) ayra.Dimensions {
		return widget.ButtonProps{Label: ""}.Layout(c, &state)
	})

	for _, node := range walk(nodes) {
		if node.Desc.Class == semantic.Button {
			t.Errorf("a control with no name was announced as a button")
		}
	}
}

// TestTheDescriptionBelongsToTheShapeItWasDrawnIn is why the announcement is
// made where it is.
//
// The node a description attaches to is the clip in force when it is made. Two
// buttons drawn one after the other must be two nodes with one label each; an
// announcement made outside the shape, or after it was popped, puts both labels
// on whatever was in force instead.
func TestTheDescriptionBelongsToTheShapeItWasDrawnIn(t *testing.T) {
	var first, second widget.Button

	nodes := tree(t, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.ButtonProps{Label: "Salvar"}.Layout(c.With(gtx), &first)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.ButtonProps{Label: "Cancelar"}.Layout(c.With(gtx), &second)
			}),
		)
	})

	controls := ofClass(nodes, semantic.Button)
	if len(controls) != 2 {
		t.Fatalf("two buttons produced %d nodes: %v", len(controls), spoken(nodes))
	}
	if controls[0].Desc.Bounds == controls[1].Desc.Bounds {
		t.Errorf("both buttons attached to one shape: %v", controls[0].Desc.Bounds)
	}
	if controls[0].Desc.Label == controls[1].Desc.Label {
		t.Errorf("both buttons are announced as %q", controls[0].Desc.Label)
	}
}

// TestTheAnnouncedShapeIsTheControlsOwn keeps what reads a screen aloud from
// being told the wrong place.
//
// The bounds on a node are how a reader says where a control is, how a touch
// exploration finds it under a finger, and how a magnifier decides what to
// follow. A node whose shape is not the control's is a control somebody cannot
// reach by touch even though it is announced correctly.
func TestTheAnnouncedShapeIsTheControlsOwn(t *testing.T) {
	var state widget.Button
	var drawn ayra.Dimensions

	nodes := tree(t, func(c ayra.Context) ayra.Dimensions {
		drawn = widget.ButtonProps{Label: "Salvar"}.Layout(c, &state)
		return drawn
	})

	controls := ofClass(nodes, semantic.Button)
	if len(controls) != 1 {
		t.Fatalf("the tree holds %d buttons", len(controls))
	}

	bounds := controls[0].Desc.Bounds
	if bounds.Dx() != drawn.Size.X || bounds.Dy() != drawn.Size.Y {
		t.Errorf("the button drew %v and is announced at %v", drawn.Size, bounds.Size())
	}
}
