package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// TestEverySegmentIsAsWideAsTheWidest is the bar of Kyse's segmented control: a
// track with the segments in it, none narrower than the others. Joined buttons
// of the widths of their words put "Seg" beside "Segunda" at two sizes.
func TestEverySegmentIsAsWideAsTheWidest(t *testing.T) {
	c, _ := frame(t, theme.Light, 360)
	var even, uneven widget.Segments

	a := widget.SegmentedProps{Options: []string{"Automático", "Claro", "Escuro"}}.Layout(c, &even).Size
	b := widget.SegmentedProps{Options: []string{"Automático", "Automático", "Automático"}}.Layout(c, &uneven).Size

	if a.X != b.X {
		t.Errorf("a bar of three words of different lengths is %d wide, and of three as long as the longest %d", a.X, b.X)
	}
	if a.Y != 33 {
		t.Errorf("the bar is %d tall, and a text field is 33", a.Y)
	}
}

// TestTheChosenSegmentCanBeToldFromTheTrack is the bar of the Mais screen, where
// the chosen item was #e6e6e6 on #f4f4f4. The chosen segment is the page's own
// colour on the muted track, with an edge, in both schemes.
func TestTheChosenSegmentCanBeToldFromTheTrack(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		var state widget.Segments
		state.Choose(0)
		picture := drawn(t, 360, 60, scheme, func(c ayra.Context) ayra.Dimensions {
			return widget.SegmentedProps{Options: []string{"Automático", "Claro", "Escuro"}}.Layout(c, &state)
		})

		// Inside the chosen segment, beside its label; and in the track's own
		// margin, outside every segment.
		pill, track := luminance(picture.RGBAAt(8, 16)), luminance(picture.RGBAAt(1, 16))
		if gap := pill - track; gap < 8 && gap > -8 {
			t.Errorf("%v: the chosen segment is %d and the track %d: the choice cannot be told", scheme, pill, track)
		}

		// Its edge: the pixel just inside the segment's border is not the fill.
		edge := luminance(picture.RGBAAt(3, 16))
		if edge == pill && edge == track {
			t.Errorf("%v: the chosen segment has no edge", scheme)
		}
	}
}
