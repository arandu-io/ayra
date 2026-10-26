package widget

import "testing"

// The number of pages and the number of slides come from the server, and a
// pager that allocated one button per page and one int per page on every frame
// turned a total of ten million into a screen that stopped drawing. What a
// pager holds and what it computes are bounded by what it shows.

// most is the longest row a window of w produces: both ends, two gaps, and the
// current page with w either side.
func most(w int) int { return 2*w + 5 }

// TestAPagerHoldsOnlyWhatItShows is the server sending an enormous total.
func TestAPagerHoldsOnlyWhatItShows(t *testing.T) {
	var state Pages
	props := PaginationProps{Total: 10_000_000}

	for range 3 {
		props.Layout(carouselFrame(t, 2000), &state)
	}
	state.Show(5_000_000)
	props.Layout(carouselFrame(t, 2000), &state)

	if len(state.numbers) > most(props.window()) {
		t.Errorf("a pager over %d pages holds %d buttons", props.Total, len(state.numbers))
	}
	if pages := props.visible(5_000_000, props.window()); cap(pages) > most(props.window()) {
		t.Errorf("the row of %d pages was computed in room for %d", len(pages), cap(pages))
	}
}

// TestTheVisibleRowIsTheSameRow fixes that computing the row from its window
// changed nothing about which pages are in it.
func TestTheVisibleRowIsTheSameRow(t *testing.T) {
	// every walks all the pages, which is the definition and the cost.
	every := func(total, current, window int) []int {
		var pages []int
		for page := 1; page <= total; page++ {
			if page == 1 || page == total || (page >= current-window && page <= current+window) {
				pages = append(pages, page)
				continue
			}
			if len(pages) > 0 && pages[len(pages)-1] != 0 {
				pages = append(pages, 0)
			}
		}
		return pages
	}

	for total := 1; total <= 30; total++ {
		for current := 1; current <= total; current++ {
			for window := 0; window <= 4; window++ {
				got := PaginationProps{Total: total}.visible(current, window)
				want := every(total, current, window)
				if len(got) != len(want) {
					t.Fatalf("%d pages at %d, window %d: %v, want %v", total, current, window, got, want)
				}
				for index := range got {
					if got[index] != want[index] {
						t.Fatalf("%d pages at %d, window %d: %v, want %v", total, current, window, got, want)
					}
				}
			}
		}
	}
}

// TestAPressOnAPageNumberStillMovesThePager keeps the buttons answering now
// that they are held per page shown rather than per page.
func TestAPressOnAPageNumberStillMovesThePager(t *testing.T) {
	var state Pages
	props := PaginationProps{Total: 1000}
	props.Layout(carouselFrame(t, 2000), &state)

	button := state.numbers[1000]
	if button == nil {
		t.Fatal("the last page has no button")
	}
	button.click.Click()
	props.Layout(carouselFrame(t, 2000), &state)

	if state.Current() != 1000 {
		t.Errorf("a press on the last page left the pager on %d", state.Current())
	}
}

// TestACarouselHoldsOnlyTheDotsItShows is the same total arriving as slides.
func TestACarouselHoldsOnlyTheDotsItShows(t *testing.T) {
	state := &Carousel{}
	props := CarouselProps{Count: 10_000_000, Dots: true, Arrows: true}

	for range 3 {
		props.Layout(carouselFrame(t, 400), state, blank)
	}
	if len(state.dots) > most(2) {
		t.Errorf("a carousel over %d positions holds %d dots", props.Count, len(state.dots))
	}
}
