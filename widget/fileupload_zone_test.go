package widget_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// TestAnyPointOfTheZoneOpensTheChooser: the zone is a frame with its prompt in
// it, and the whole of it is the press, as Kyse's label around the input is.
func TestAnyPointOfTheZoneOpensTheChooser(t *testing.T) {
	var asked atomic.Int32
	props := widget.FileUploadProps{Chooser: widget.ChooserFunc(func(context.Context) ([]widget.File, error) {
		asked.Add(1)
		return nil, nil
	})}
	var state widget.FileUpload
	screen := func(c ayra.Context) ayra.Dimensions { return props.Layout(c, &state) }

	p := newPanel()
	zone := p.draw(screen)
	p.draw(screen)
	// The left edge of the zone, where the old button was not.
	tap(p, screen, 6, zone.Size.Y/2)
	waitFor(t, "the chooser to be asked", func() bool { return asked.Load() > 0 })
}

// TestRemovingAFileIsReachedByAFinger: the button is as small as Kyse draws it
// and the area that answers is not.
func TestRemovingAFileIsReachedByAFinger(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	props := widget.FileUploadProps{}

	reaches := func(y int) bool {
		var state widget.FileUpload
		state.Add(ctx, props, file("a.pdf", 10))
		screen := func(c ayra.Context) ayra.Dimensions { return props.Layout(c, &state) }
		p := newPanel()
		p.draw(screen)
		p.draw(screen)
		state.Changed()
		tap(p, screen, 380, y)
		return state.Changed()
	}

	first, last, count := -1, -1, 0
	for y := 0; y < 200; y++ {
		if reaches(y) {
			if first < 0 {
				first = y
			}
			last = y
			count++
		}
	}
	if count < 40 {
		t.Errorf("the remove button answers over %d points of height (%d to %d), want at least 44", count, first, last)
	}
}
