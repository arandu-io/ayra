package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// listing is the directory these tests browse: two ways down and three files,
// deliberately given in the order a volume would report them rather than in the
// order they are drawn.
func listing() []widget.FileEntry {
	return []widget.FileEntry{
		{Name: "notes.md", Size: 2_400},
		{Name: "reports", Dir: true},
		{Name: "budget.csv", Size: 18_000},
		{Name: "archive", Dir: true},
		{Name: "photo.jpg", Size: 3_400_000},
	}
}

// TestAPickerFillsTheRoomItIsGiven fixes the one layout decision a listing
// somebody walks through depends on.
//
// A listing that ended where its longest name did would move its right edge
// every time somebody went into a directory, and the sizes down the right would
// move with it -- which reads as a panel that resizes itself for no reason.
func TestAPickerFillsTheRoomItIsGiven(t *testing.T) {
	for _, width := range []int{240, 400, 800} {
		c, _ := frame(t, theme.Light, width)
		var state widget.FilePicker

		dims := widget.FilePickerProps{Entries: listing()}.Layout(c, &state)

		if dims.Size.X != width {
			t.Errorf("in %dpt of room the picker took %dpt, want the whole width", width, dims.Size.X)
		}
	}
}

// TestAPickerGrowsWithItsListing is the first thing the control has to do and
// the thing a broken one fails silently: a panel that took the same room for
// five entries as for one drew four of them on top of each other, or not at all.
func TestAPickerGrowsWithItsListing(t *testing.T) {
	measure := func(entries []widget.FileEntry) int {
		c, _ := frame(t, theme.Light, 400)
		var state widget.FilePicker
		return widget.FilePickerProps{Entries: entries}.Layout(c, &state).Size.Y
	}

	one := measure(listing()[:1])
	three := measure(listing()[:3])
	five := measure(listing())

	if !(one < three && three < five) {
		t.Errorf("the panel does not grow with the listing: 1=%d 3=%d 5=%d", one, three, five)
	}
}

// TestAnEmptyDirectoryStillDraws keeps a directory with nothing in it from
// looking like a panel that failed to load.
//
// It holds its width as well as its height. A message is as wide as its own
// sentence, and a panel that shrank to it would change size every time somebody
// typed another letter into a filter that matches less and less.
func TestAnEmptyDirectoryStillDraws(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.FilePicker

	dims := widget.FilePickerProps{}.Layout(c, &state)

	if dims.Size.Y <= 0 {
		t.Fatalf("an empty directory took %v, and a region left blank says nothing about why", dims.Size)
	}
	if dims.Size.X != 400 {
		t.Errorf("an empty directory took %dpt of 400, so the panel shrank to its own message", dims.Size.X)
	}
}

// TestAFilterThatMatchesNothingHoldsTheWidth is the same property on the path
// that reaches it through the filter rather than through an empty listing.
func TestAFilterThatMatchesNothingHoldsTheWidth(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.FilePicker
	state.SetFilter("nothing here is called this")

	dims := widget.FilePickerProps{Entries: listing()}.Layout(c, &state)

	if dims.Size.X != 400 {
		t.Errorf("a filter that matched nothing left the panel %dpt of 400 wide", dims.Size.X)
	}
}

// TestAFilterNarrowsWhatIsDrawn fixes that the filter reaches the drawing, not
// only the state.
func TestAFilterNarrowsWhatIsDrawn(t *testing.T) {
	measure := func(filter string) int {
		c, _ := frame(t, theme.Light, 400)
		var state widget.FilePicker
		state.SetFilter(filter)
		return widget.FilePickerProps{Entries: listing()}.Layout(c, &state).Size.Y
	}

	whole, narrowed := measure(""), measure("budget")

	if narrowed >= whole {
		t.Errorf("the filtered listing took %dpt and the whole one %dpt", narrowed, whole)
	}
}

// TestAPickerReportsNothingUntilSomethingIsPressed keeps a screen from acting
// on the frame it was opened in.
//
// A picker that answered its intents from the first draw would send the caller
// off to read a directory nobody asked for, before the person has seen the one
// they are in.
func TestAPickerReportsNothingUntilSomethingIsPressed(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.FilePicker

	widget.FilePickerProps{Entries: listing(), Path: []string{"reports"}}.Layout(c, &state)

	if name, asked := state.Opened(); asked {
		t.Errorf("a picker nobody touched asked for the listing of %q", name)
	}
	if name, picked := state.Chosen(); picked {
		t.Errorf("a picker nobody touched reported %q as chosen", name)
	}
	if depth, asked := state.Up(); asked {
		t.Errorf("a picker nobody touched asked to go back to depth %d", depth)
	}
}

// TestADisabledPickerDoesNotAnswer is the half a test usually forgets: a control
// drawn as unavailable that still reports a press is worse than one that is not
// drawn as unavailable at all, because the screen says one thing and does
// another.
func TestADisabledPickerDoesNotAnswer(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	var state widget.FilePicker

	dims := widget.FilePickerProps{Entries: listing(), Disabled: true}.Layout(c, &state)

	if dims.Size.Y <= 0 {
		t.Fatal("a disabled picker drew nothing at all")
	}
	if _, asked := state.Opened(); asked {
		t.Error("a disabled picker asked for a listing")
	}
	if _, picked := state.Chosen(); picked {
		t.Error("a disabled picker reported a file as chosen")
	}
}

// TestTheCallersListingComesBackUntouched keeps the ordering the control does
// out of the slice it was only shown.
//
// Sorted in place, the caller finds its own listing rearranged -- on the next
// frame, on the next screen, and anywhere else it had handed the same slice.
func TestTheCallersListingComesBackUntouched(t *testing.T) {
	for _, filter := range []string{"", "reports", "o"} {
		c, _ := frame(t, theme.Light, 400)
		var state widget.FilePicker
		state.SetFilter(filter)

		entries := listing()
		widget.FilePickerProps{Entries: entries}.Layout(c, &state)

		for index, want := range listing() {
			if entries[index].Name != want.Name {
				t.Fatalf("filtering by %q left the caller's listing as %q at %d, want %q", filter, entries[index].Name, index, want.Name)
			}
		}
	}
}

// TestAPickerDrawsInBothSchemes keeps a colour from being read only in the one
// somebody develops in.
func TestAPickerDrawsInBothSchemes(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		t.Run(scheme.String(), func(t *testing.T) {
			c, _ := frame(t, scheme, 400)
			var state widget.FilePicker
			state.Choose("budget.csv")

			dims := widget.FilePickerProps{Entries: listing(), Root: "Bucket", Path: []string{"2026"}}.Layout(c, &state)

			if dims.Size.X <= 0 || dims.Size.Y <= 0 {
				t.Errorf("took %v", dims.Size)
			}
			if state.Selected() != "budget.csv" {
				t.Errorf("the mark on a row that is in the listing was given up: %q", state.Selected())
			}
		})
	}
}

// TestTheSameStateDrawnTwiceIsSteady fixes that a redraw of an unchanged
// listing changes nothing, which is what a frame loop does sixty times a second.
func TestTheSameStateDrawnTwiceIsSteady(t *testing.T) {
	var state widget.FilePicker
	props := widget.FilePickerProps{Entries: listing()}

	first, _ := frame(t, theme.Light, 400)
	before := props.Layout(first, &state)

	second, _ := frame(t, theme.Light, 400)
	after := props.Layout(second, &state)

	if before.Size != after.Size {
		t.Errorf("the picker took %v and then %v with nothing changed", before.Size, after.Size)
	}
}
