package widget

import (
	"context"
	"errors"
	"github.com/arandu-io/ayra"
	"image"
	"testing"
	"time"

	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/theme"
)

func TestTheOptimisticSwitchMovesAtOnceAndGoesBackOnARefusal(t *testing.T) {
	var state OptimisticToggle
	state.Set(true) // the position the press has already drawn

	state.ask(func(context.Context, bool) error { return errors.New("not allowed") }, true, func() {})
	for deadline := time.Now().Add(2 * time.Second); state.Pending(); {
		if time.Now().After(deadline) {
			t.Fatal("the server's answer never arrived")
		}
		time.Sleep(time.Millisecond)
	}

	if state.On() {
		t.Error("a refused change stayed where the person put it")
	}
	if state.Err() == nil {
		t.Error("the refusal was not kept")
	}
}

func TestTheOptimisticSwitchStaysWhenTheServerAgrees(t *testing.T) {
	var state OptimisticToggle
	state.Set(true)
	state.ask(func(context.Context, bool) error { return nil }, true, func() {})
	for deadline := time.Now().Add(2 * time.Second); state.Pending(); {
		if time.Now().After(deadline) {
			t.Fatal("no answer")
		}
		time.Sleep(time.Millisecond)
	}
	if !state.On() || state.Err() != nil {
		t.Errorf("an accepted change left %v, %v", state.On(), state.Err())
	}
}

func TestTheFormReportsRawValues(t *testing.T) {
	var state FormDialog
	props := FormDialogProps{Fields: []FormField{
		{Name: "document", Mask: brazil.CPF},
		{Name: "plate", Mask: brazil.Plate},
		{Name: "note"},
	}}
	c, _ := field(t, theme.Light, 400)
	state.Open()
	props.Layout(c, &state)

	state.fields[0].SetValue(brazil.CPF, "12345678909")
	state.fields[1].SetValue(brazil.Plate, "ABC1D23")
	state.fields[2].SetValue(nil, "hello")
	state.collect(props)

	values, ok := state.Submitted()
	if !ok || values["document"] != "12345678909" || values["plate"] != "ABC1D23" || values["note"] != "hello" {
		t.Errorf("the form reported %v, %v: a mask must be taken off", values, ok)
	}
	if _, again := state.Submitted(); again {
		t.Error("a submission was reported twice")
	}
	if !state.Showing() {
		t.Error("the dialog closed on a submission: it is the server that accepts a form")
	}
}

func TestTheFormOpensHoldingItsValues(t *testing.T) {
	var state FormDialog
	props := FormDialogProps{Fields: []FormField{{Name: "name", Value: "Gate 1"}}}
	c, _ := field(t, theme.Light, 400)
	state.Open()
	props.Layout(c, &state)
	if got := state.fields[0].Text(); got != "Gate 1" {
		t.Errorf("the field opened holding %q", got)
	}
}

func TestTheNarrowestRenditionThatIsWideEnoughIsChosen(t *testing.T) {
	p := ResponsiveImageProps{Variants: []ImageVariant{{Width: 1200}, {Width: 400}, {Width: 800}}}
	for _, c := range []struct{ pixels, want int }{{300, 1}, {400, 1}, {401, 2}, {900, 0}, {5000, 0}} {
		if got := p.pick(c.pixels); got != c.want {
			t.Errorf("a box of %d pixels picked %d, want %d", c.pixels, got, c.want)
		}
	}
	if (ResponsiveImageProps{}).pick(100) != -1 {
		t.Error("no renditions must pick none")
	}
}

func TestALazyRegionIsToldOnceWhenItIsFirstDrawn(t *testing.T) {
	var state LazyLoad
	c, _ := field(t, theme.Light, 400)
	empty := func(c ayra.Context) ayra.Dimensions { return ayra.Dimensions{} }

	if state.Visible() {
		t.Error("a region nobody drew was visible")
	}
	LazyLoadProps{}.Layout(c, &state, empty)
	if !state.Visible() || state.Visible() {
		t.Error("the first draw must say so once")
	}

	placeholder := LazyLoadProps{}.Layout(c, &state, func(c ayra.Context) ayra.Dimensions { return ayra.Dimensions{Size: image.Pt(5, 5)} })
	state.Ready()
	content := LazyLoadProps{}.Layout(c, &state, func(c ayra.Context) ayra.Dimensions { return ayra.Dimensions{Size: image.Pt(5, 5)} })
	if placeholder.Size == content.Size && placeholder.Size.X == 5 {
		t.Error("the placeholder stood in after the content was ready")
	}
	if content.Size != image.Pt(5, 5) {
		t.Errorf("the content took %v", content.Size)
	}
}

func TestADeleteRowAsksBeforeItAnswers(t *testing.T) {
	var state DeleteRow
	c, _ := field(t, theme.Light, 400)
	closed := DeleteRowProps{}.Layout(c, &state).Size
	state.armed = true
	armed := DeleteRowProps{}.Layout(c, &state).Size
	if armed.X <= closed.X {
		t.Errorf("the question (%v) is no wider than the trash (%v)", armed, closed)
	}
	if state.Confirmed() {
		t.Error("asking the question confirmed it")
	}
}

func TestTheOutputDrawsItsLabelAndItsValue(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	alone := OutputProps{Value: "R$ 10,00"}.Layout(c).Size
	labelled := OutputProps{Label: "Total", Value: "R$ 10,00"}.Layout(c).Size
	if labelled.Y <= alone.Y {
		t.Errorf("a label left the output at %v from %v", labelled, alone)
	}
}
