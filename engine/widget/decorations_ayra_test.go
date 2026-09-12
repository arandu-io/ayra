package widget

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
)

func TestDecorationsMapEveryWindowAction(t *testing.T) {
	for _, action := range []system.Action{
		system.ActionMinimize,
		system.ActionMaximize,
		system.ActionUnmaximize,
		system.ActionFullscreen,
		system.ActionRaise,
		system.ActionCenter,
		system.ActionClose,
		system.ActionMove,
	} {
		t.Run(action.String(), func(t *testing.T) {
			decorations := Decorations{Maximized: action == system.ActionUnmaximize}
			click := decorations.Clickable(action)
			if click != decorations.Clickable(action) {
				t.Fatal("the same action returned two clickables")
			}
			click.Click()
			if got := decorations.Update(layout.Context{}); got != action {
				t.Fatalf("activating %v returned %v", action, got)
			}
		})
	}
}

func TestDecorationsChooseTheActionForTheWindowState(t *testing.T) {
	for _, test := range []struct {
		name      string
		maximized bool
		requested system.Action
		want      system.Action
	}{
		{"maximize a restored window", false, system.ActionMaximize, system.ActionMaximize},
		{"restore a maximized window", true, system.ActionMaximize, system.ActionUnmaximize},
		{"restore request on a restored window", false, system.ActionUnmaximize, system.ActionMaximize},
		{"restore request on a maximized window", true, system.ActionUnmaximize, system.ActionUnmaximize},
	} {
		t.Run(test.name, func(t *testing.T) {
			decorations := Decorations{Maximized: test.maximized}
			decorations.Clickable(test.requested).Click()
			if got := decorations.Update(layout.Context{}); got != test.want {
				t.Fatalf("activated %v, want %v", got, test.want)
			}
		})
	}
}

func TestDecorationsReturnActionsFromTheSameFrameTogether(t *testing.T) {
	var decorations Decorations
	decorations.Clickable(system.ActionMinimize).Click()
	decorations.Clickable(system.ActionClose).Click()

	want := system.ActionMinimize | system.ActionClose
	if got := decorations.Update(layout.Context{}); got != want {
		t.Fatalf("activated %v, want %v", got, want)
	}
}

func TestDecorationsRejectActionSets(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("an action set was accepted as one button")
		}
	}()
	new(Decorations).Clickable(system.ActionMinimize | system.ActionClose)
}

func TestLayoutMoveMarksTheTitlebarAsMovable(t *testing.T) {
	var router input.Router
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(80, 24)),
		Ops:         new(op.Ops),
		Source:      router.Source(),
	}

	new(Decorations).LayoutMove(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	})
	router.Frame(gtx.Ops)

	action, ok := router.ActionAt(f32.Pt(40, 12))
	if !ok || action != system.ActionMove {
		t.Fatalf("the titlebar reported %v, %t; want ActionMove, true", action, ok)
	}
}
