package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/widget"
)

var longTabs = []string{"Visão geral das câmeras", "Leituras de placas do dia", "Configurações avançadas do motor", "Faturamento", "Integrações com outros sistemas"}

type tabsScreen struct {
	tabs widget.Tabs
}

func (s *tabsScreen) draw(c ayra.Context) ayra.Dimensions {
	s.tabs.Changed(c)
	return widget.TabsProps{Labels: longTabs}.Layout(c, &s.tabs)
}

// TestTabsTallerThanTheirTextAreEasyToHit: a tab was 33 points high.
func TestTabsAreAtLeast44PointsHigh(t *testing.T) {
	p, s := phone(100), new(tabsScreen)
	dims := p.draw(s.draw)
	if dims.Size.Y < 44 {
		t.Errorf("the tabs are %d points high, want at least 44", dims.Size.Y)
	}
}

// TestTabsWiderThanTheRoomScrollToTheLastOne: five tabs with their words are
// wider than a phone, and the ones past its edge could not be reached.
func TestTabsWiderThanTheRoomScrollToTheLastOne(t *testing.T) {
	p, s := phone(100), new(tabsScreen)
	p.draw(s.draw)

	p.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(100, 20), Scroll: f32.Pt(4000, 0)})
	p.draw(s.draw)
	p.draw(s.draw)

	// The last tab is at the right end of the strip now.
	p.tap(image.Pt(380, 20), s.draw)
	p.draw(s.draw)

	if got := s.tabs.Selected(); got != len(longTabs)-1 {
		t.Errorf("a press at the right end of the strip, scrolled to its end, selected tab %d, want %d", got, len(longTabs)-1)
	}
}

// TestSelectingATabBringsItIntoView: a tab chosen by the screen is not left
// beyond the edge.
func TestSelectingATabBringsItIntoView(t *testing.T) {
	p, s := phone(100), new(tabsScreen)
	s.tabs.Select(len(longTabs) - 1)
	p.draw(s.draw)
	p.draw(s.draw)

	// Pressing the left of the strip must not be the first tab: the strip moved.
	p.tap(image.Pt(20, 20), s.draw)
	p.draw(s.draw)
	if got := s.tabs.Selected(); got == 0 {
		t.Error("the strip still starts at the first tab after the last one was selected")
	}
}
