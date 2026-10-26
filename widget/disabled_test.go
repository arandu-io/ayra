package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// A control drawn as unavailable that still answers is worse than one that is
// not drawn as unavailable at all: the screen says one thing and does another.
// A checkbox for a flag the server sent as not editable flipped on a press, and
// the form built from it submitted the flip.
//
// Asserting this on a frame with no input attached proves nothing, because
// nothing can arrive. Every test here drives a real router -- presses across
// the whole control, and the focus moved onto it with Space and Return pressed
// -- and reads the control the way a screen does: asked before it is drawn.

// panel is a frame with a router attached, drawing whatever it is handed.
type panel struct {
	router input.Router
	ops    *op.Ops
	size   image.Point
	shaper *text.Shaper
}

func newPanel() *panel {
	return &panel{
		ops:    new(op.Ops),
		size:   image.Pt(400, 400),
		shaper: text.NewShaper(text.WithCollection(gofont.Collection())),
	}
}

// draw runs one frame and hands the operations to the router.
func (p *panel) draw(screen func(ayra.Context) ayra.Dimensions) ayra.Dimensions {
	p.ops.Reset()
	gtx := layout.Context{
		Ops:         p.ops,
		Source:      p.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: p.size},
	}
	dims := screen(ayra.Context{Context: gtx, Theme: theme.New(theme.Light), Shaper: p.shaper})
	p.router.Frame(p.ops)
	return dims
}

// assault presses everywhere in area, then moves the focus through the
// controls the way a window does on Tab, pressing Space and Return on each and
// activating it the way a screen reader does. A frame is drawn after every
// step, so whatever was delivered is read.
func (p *panel) assault(area image.Point, screen func(ayra.Context) ayra.Dimensions) {
	step := 4
	for y := 1; y < area.Y; y += step {
		for x := 1; x < area.X; x += step {
			at := f32.Pt(float32(x), float32(y))
			p.router.Queue(
				pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
				pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
			)
		}
		p.draw(screen)
	}
	for range 6 {
		p.router.MoveFocus(key.FocusForward)
		p.draw(screen)
		for _, name := range []key.Name{key.NameSpace, key.NameReturn} {
			p.router.Queue(key.Event{Name: name, State: key.Press}, key.Event{Name: name, State: key.Release})
			p.draw(screen)
		}
		p.router.ClickFocus()
		p.draw(screen)
	}
}

// refuses runs one control through the assault and reports every answer it
// gave. ask is what a screen asks before drawing, and draw draws the control
// with disabled as its flag.
func refuses(t *testing.T, ask func(ayra.Context) bool, draw func(c ayra.Context, disabled bool) ayra.Dimensions) {
	t.Helper()

	p := newPanel()
	answered := 0
	disabled := func(c ayra.Context) ayra.Dimensions {
		if ask(c) {
			answered++
		}
		return draw(c, true)
	}

	area := p.draw(disabled).Size
	if area.X <= 0 || area.Y <= 0 {
		t.Fatalf("the control drew nothing to press: %v", area)
	}
	p.assault(area, disabled)
	if answered > 0 {
		t.Errorf("a disabled control answered %d times", answered)
	}

	// Enabled again, with nothing done to it since. Whatever was queued while
	// it was unavailable must not arrive now, which is the same press acted on
	// later with nothing on screen to connect the two.
	answered = 0
	p.draw(func(c ayra.Context) ayra.Dimensions {
		if ask(c) {
			answered++
		}
		return draw(c, false)
	})
	if answered > 0 {
		t.Errorf("a press made while disabled answered %d times once enabled", answered)
	}
}

// TestADisabledButtonDoesNotAnswer is the button asked the way a screen asks
// it: Clicked first, then drawn disabled.
func TestADisabledButtonDoesNotAnswer(t *testing.T) {
	var state widget.Button
	refuses(t,
		state.Clicked,
		func(c ayra.Context, disabled bool) ayra.Dimensions {
			return widget.ButtonProps{Label: "Delete", Disabled: disabled}.Layout(c, &state)
		})
}

// TestADisabledToggleNeitherAnswersNorChanges covers the three drawings of a
// toggle, and the value a form is built from as well as the report.
func TestADisabledToggleNeitherAnswersNorChanges(t *testing.T) {
	controls := map[string]func(c ayra.Context, state *widget.Toggle, disabled bool) ayra.Dimensions{
		"checkbox": func(c ayra.Context, state *widget.Toggle, disabled bool) ayra.Dimensions {
			return widget.CheckboxProps{Label: "Administrator", Disabled: disabled}.Layout(c, state)
		},
		"radio": func(c ayra.Context, state *widget.Toggle, disabled bool) ayra.Dimensions {
			return widget.RadioProps{Label: "Administrator", Disabled: disabled}.Layout(c, state)
		},
		"switch": func(c ayra.Context, state *widget.Toggle, disabled bool) ayra.Dimensions {
			return widget.SwitchProps{Label: "Administrator", Disabled: disabled}.Layout(c, state)
		},
	}
	for name, control := range controls {
		t.Run(name, func(t *testing.T) {
			var state widget.Toggle
			refuses(t,
				state.Changed,
				func(c ayra.Context, disabled bool) ayra.Dimensions { return control(c, &state, disabled) })
			if state.On() {
				t.Error("a disabled toggle turned on, and a form built from it submits the change")
			}
		})
	}
}

// TestControlsInADisabledRegionDoNotAnswer covers the controls with no flag of
// their own, drawn inside something the screen made unavailable.
func TestControlsInADisabledRegionDoNotAnswer(t *testing.T) {
	region := func(c ayra.Context, disabled bool) ayra.Context {
		if disabled {
			c.Context = c.Context.Disabled()
		}
		return c
	}

	t.Run("tabs", func(t *testing.T) {
		var state widget.Tabs
		refuses(t,
			state.Changed,
			func(c ayra.Context, disabled bool) ayra.Dimensions {
				return widget.TabsProps{Labels: []string{"One", "Two", "Three"}}.Layout(region(c, disabled), &state)
			})
		if state.Selected() != 0 {
			t.Errorf("a disabled row of tabs moved to %d", state.Selected())
		}
	})

	t.Run("button group", func(t *testing.T) {
		var state widget.Group
		refuses(t,
			func(c ayra.Context) bool { return state.Clicked(c) >= 0 },
			func(c ayra.Context, disabled bool) ayra.Dimensions {
				return widget.ButtonGroupProps{Labels: []string{"Day", "Week", "Month"}, Selected: -1}.Layout(region(c, disabled), &state)
			})
	})

	t.Run("item", func(t *testing.T) {
		var state widget.Button
		refuses(t,
			state.Clicked,
			func(c ayra.Context, disabled bool) ayra.Dimensions {
				return widget.ItemProps{Title: "Invoice 42", Pressable: true}.Layout(region(c, disabled), &state, nil)
			})
	})
}

// TestOtherDisabledControlsDoNotMove covers the rest of the controls with a
// flag, by the value each one holds.
func TestOtherDisabledControlsDoNotMove(t *testing.T) {
	t.Run("split", func(t *testing.T) {
		var state widget.Split
		refuses(t,
			func(c ayra.Context) bool { return state.Pressed(c) || state.Opened(c) },
			func(c ayra.Context, disabled bool) ayra.Dimensions {
				return widget.SplitProps{Label: "Save", Disabled: disabled}.Layout(c, &state)
			})
	})

	t.Run("select", func(t *testing.T) {
		var state widget.Select
		refuses(t,
			func(ayra.Context) bool { return state.Showing() || state.Selected() >= 0 },
			func(c ayra.Context, disabled bool) ayra.Dimensions {
				return widget.SelectProps{Options: []string{"viewer", "admin"}, Disabled: disabled}.Layout(c, &state)
			})
	})

	t.Run("number", func(t *testing.T) {
		var state widget.Stepper
		state.SetValue(5)
		refuses(t,
			func(ayra.Context) bool { return state.Value() != 5 },
			func(c ayra.Context, disabled bool) ayra.Dimensions {
				return widget.NumberProps{Disabled: disabled}.Layout(c, &state)
			})
	})

	t.Run("slider", func(t *testing.T) {
		var state widget.Slider
		state.SetValue(0.5)
		refuses(t,
			func(ayra.Context) bool { return state.Value() != 0.5 },
			func(c ayra.Context, disabled bool) ayra.Dimensions {
				return widget.SliderProps{Disabled: disabled}.Layout(c, &state)
			})
	})

	t.Run("rating", func(t *testing.T) {
		var state widget.Rating
		state.SetValue(2)
		refuses(t,
			func(ayra.Context) bool { return state.Value() != 2 },
			func(c ayra.Context, disabled bool) ayra.Dimensions {
				return widget.RatingProps{ReadOnly: disabled}.Layout(c, &state)
			})
	})

	t.Run("carousel", func(t *testing.T) {
		var state widget.Carousel
		refuses(t,
			func(ayra.Context) bool { return state.At() != 0 || state.Changed() },
			func(c ayra.Context, disabled bool) ayra.Dimensions {
				return widget.CarouselProps{Count: 5, Arrows: true, Dots: true, Disabled: disabled}.Layout(c, &state,
					func(c ayra.Context, index int) ayra.Dimensions {
						return ayra.Dimensions{Size: image.Pt(c.Constraints.Max.X, 80)}
					})
			})
	})
}

// TestAFocusedButtonThatBecomesDisabledIgnoresTheKeyboard is the order a form
// produces: the button has the focus, a press starts a request, and the button
// is drawn unavailable while it runs. A second Space must not send it again.
func TestAFocusedButtonThatBecomesDisabledIgnoresTheKeyboard(t *testing.T) {
	p := newPanel()
	var state widget.Button
	disabled := false
	answered := 0
	screen := func(c ayra.Context) ayra.Dimensions {
		if state.Clicked(c) {
			answered++
		}
		return widget.ButtonProps{Label: "Sign in", Disabled: disabled}.Layout(c, &state)
	}

	p.draw(screen)
	p.router.MoveFocus(key.FocusForward)
	p.draw(screen)
	p.draw(screen)
	p.router.Queue(key.Event{Name: key.NameSpace, State: key.Press}, key.Event{Name: key.NameSpace, State: key.Release})
	p.draw(screen)
	if answered != 1 {
		t.Fatalf("the focused button answered %d times to Space, want 1", answered)
	}

	disabled = true
	p.draw(screen)
	for range 3 {
		p.router.Queue(key.Event{Name: key.NameSpace, State: key.Press}, key.Event{Name: key.NameSpace, State: key.Release})
		p.draw(screen)
	}
	if answered != 1 {
		t.Errorf("the button answered %d more times while disabled", answered-1)
	}
}
