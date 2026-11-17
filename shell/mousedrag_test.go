package shell

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/app"
	"github.com/arandu-io/ayra/engine/io/event"
)

// toldMouseDrag plays frames through a window with this configuration and says
// whether each one was told that a mouse drag scrolls.
func toldMouseDrag(t *testing.T, cfg Config, n int) []bool {
	t.Helper()
	platform(t, false)
	events := make([]event.Event, 0, n+1)
	for i := 0; i < n; i++ {
		events = append(events, frame())
	}
	win := &scripted{events: append(events, app.DestroyEvent{})}

	var told []bool
	err := loop(win, cfg, nil, func(c ayra.Context) ayra.Dimensions {
		told = append(told, c.MouseDragScrolls)
		return ayra.Dimensions{}
	})
	if err != nil {
		t.Fatalf("the loop failed: %v", err)
	}
	return told
}

// TestAMouseDragDoesNotScrollUnlessTheApplicationSaysSo is the default: a
// desktop window keeps its wheel for scrolling and its drag for selecting.
func TestAMouseDragDoesNotScrollUnlessTheApplicationSaysSo(t *testing.T) {
	for i, got := range toldMouseDrag(t, Config{Title: "x"}, 3) {
		if got {
			t.Errorf("frame %d was told a mouse drag scrolls, and nothing asked for it", i)
		}
	}
}

// TestTheApplicationTurnsTheMouseDragOnForEveryFrame is the kiosk: the window
// reaches every list in it through the frame's context, so the application says
// it once.
func TestTheApplicationTurnsTheMouseDragOnForEveryFrame(t *testing.T) {
	for i, got := range toldMouseDrag(t, Config{Title: "x", MouseDragScrolls: true}, 3) {
		if !got {
			t.Errorf("frame %d was not told a mouse drag scrolls", i)
		}
	}
}
