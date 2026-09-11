package widget

import (
	"image"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// TestNoControlReadsTheDiskItself is the guard for the decision the picker is
// built on.
//
// The listing arrives from the caller, and nothing here goes looking for it. A
// control that opened a directory could not be drawn in a test, because a test
// has no directory to point it at; could not be driven by a listing that came
// over the wire; and would do the reading inside the frame, where a volume that
// has gone to sleep stops the application until it wakes.
//
// It is a check rather than a note because the shortcut is one line -- reading
// the directory in Layout is less code than threading the entries through the
// props -- and it compiles.
func TestNoControlReadsTheDiskItself(t *testing.T) {
	refused := map[string]string{
		`"os"`:            "opens files and directories",
		`"io/fs"`:         "walks a filesystem",
		`"path/filepath"`: "resolves a path on whichever machine this is running on",
	}

	for _, path := range sources(t) {
		for _, line := range strings.Split(read(t, path), "\n") {
			imported := strings.TrimSpace(line)
			if fields := strings.Fields(imported); len(fields) == 2 {
				// An aliased import: the path is the second word.
				imported = fields[1]
			}

			if why, forbidden := refused[imported]; forbidden {
				t.Errorf("%s imports %s, which %s: a control that reaches the disk cannot be drawn without one, and blocks the frame on a slow volume",
					filepath.Base(path), imported, why)
			}
		}
	}
}

// TestASizeIsWrittenAtEveryScale fixes what the figure beside a file says.
//
// The boundaries are the whole test: a formatter that is right in the middle of
// a range and wrong at its ends is wrong on exactly the files somebody stops to
// look at.
func TestASizeIsWrittenAtEveryScale(t *testing.T) {
	for _, test := range []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{999, "999 B"},
		{1000, "1.0 kB"},
		// A thousand and twenty-four is a kilobyte in the other reading, and
		// this one calls it the same thing a thousand is.
		{1024, "1.0 kB"},
		{1500, "1.5 kB"},
		{999_949, "999.9 kB"},
		{1_000_000, "1.0 MB"},
		{1_500_000_000, "1.5 GB"},
		{9_223_372_036_854_775_807, "9.2 EB"},
	} {
		if got := formatSize(test.bytes); got != test.want {
			t.Errorf("%d bytes reads as %q, want %q", test.bytes, got, test.want)
		}
	}
}

// TestASizeCarriesInsteadOfReachingAThousand keeps the figure out of the gap
// between two units.
//
// One decimal place rounds 999.999 up to 1000.0, and "1000.0 kB" is precisely
// the number the next unit exists to prevent -- it is longer than the figure it
// replaces and says nothing the reader could not have worked out.
func TestASizeCarriesInsteadOfReachingAThousand(t *testing.T) {
	for _, bytes := range []int64{999_999, 999_999_999, 999_999_999_999} {
		got := formatSize(bytes)

		if strings.HasPrefix(got, "1000") {
			t.Errorf("%d bytes reads as %q, which is a thousand of a unit that has a name", bytes, got)
		}
		if !strings.HasPrefix(got, "1.0 ") {
			t.Errorf("%d bytes reads as %q, want it carried to the next unit", bytes, got)
		}
	}
}

// TestASizeNobodyKnowsIsNotWrittenAsZero keeps the picker from claiming a file
// is empty when what it was handed is the absence of a measurement.
func TestASizeNobodyKnowsIsNotWrittenAsZero(t *testing.T) {
	if got := formatSize(-1); got != "" {
		t.Errorf("an unmeasured file reads as %q, want nothing drawn", got)
	}
	if got := formatSize(0); got != "0 B" {
		t.Errorf("an empty file reads as %q, and empty is a measurement", got)
	}
}

// TestDirectoriesAreDrawnBeforeFiles fixes the order of the listing.
//
// Interleaved, the way down out of a directory is scattered through the things
// that are not a way down, and somebody looking for a file that is not in this
// one has to read every line to find that out.
func TestDirectoriesAreDrawnBeforeFiles(t *testing.T) {
	props := FilePickerProps{Entries: []FileEntry{
		{Name: "notes.md"},
		{Name: "reports", Dir: true},
		{Name: "budget.csv"},
		{Name: "archive", Dir: true},
	}}

	var order []string
	for _, entry := range props.visible("") {
		order = append(order, entry.Name)
	}

	want := []string{"reports", "archive", "notes.md", "budget.csv"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("the listing is drawn as %v, want %v", order, want)
	}
}

// TestTheOrderInsideEachKindIsTheCallersOwn keeps the picker from overruling a
// caller that already sorted -- by date, by relevance, by anything this cannot
// see.
func TestTheOrderInsideEachKindIsTheCallersOwn(t *testing.T) {
	props := FilePickerProps{Entries: []FileEntry{
		{Name: "zebra.txt"},
		{Name: "apple.txt"},
		{Name: "mango.txt"},
	}}

	entries := props.visible("")

	if entries[0].Name != "zebra.txt" || entries[2].Name != "mango.txt" {
		t.Errorf("the files were reordered: %v", entries)
	}
}

// TestTheFilterNarrowsBothKinds fixes what a filter is for.
//
// Sparing the directories would leave a screen that is supposed to have got
// shorter still carrying every one of them, which is the half of a long listing
// a filter is usually being used to get past.
func TestTheFilterNarrowsBothKinds(t *testing.T) {
	props := FilePickerProps{Entries: []FileEntry{
		{Name: "reports", Dir: true},
		{Name: "archive", Dir: true},
		{Name: "report.pdf"},
		{Name: "budget.csv"},
	}}

	entries := props.visible("REPORT")

	if len(entries) != 2 {
		t.Fatalf("%d entries matched, want the directory and the file", len(entries))
	}
	if entries[0].Name != "reports" || entries[1].Name != "report.pdf" {
		t.Errorf("the match is %v, want the directory first and then the file", entries)
	}
}

// TestTheFilterDoesNotDependOnCapitalisation keeps a listing from going blank
// because somebody typed a name the way they would say it.
func TestTheFilterDoesNotDependOnCapitalisation(t *testing.T) {
	props := FilePickerProps{Entries: []FileEntry{{Name: "Quarterly.PDF"}}}

	for _, filter := range []string{"quarterly", "QUARTERLY", "Quarterly", ".pdf"} {
		if len(props.visible(filter)) != 1 {
			t.Errorf("%q matched nothing in a listing that holds Quarterly.PDF", filter)
		}
	}
}

// TestTheListingTheCallerGaveIsNotRearranged keeps the ordering and the
// narrowing out of the caller's own slice.
//
// A control that worked in place would change a value it was only shown, and
// the caller would find its listing reordered, or shorter, or with an entry
// written twice -- next frame, next screen, and anywhere else it had handed the
// same slice. Reusing the array the caller owns is the way that happens, and it
// is invisible until a filter moves something.
func TestTheListingTheCallerGaveIsNotRearranged(t *testing.T) {
	want := []string{"notes.md", "reports", "budget.csv"}

	for _, filter := range []string{"", "reports", "s"} {
		entries := []FileEntry{
			{Name: "notes.md"},
			{Name: "reports", Dir: true},
			{Name: "budget.csv"},
		}
		props := FilePickerProps{Entries: entries}

		props.visible(filter)

		for index, name := range want {
			if entries[index].Name != name {
				t.Fatalf("filtering by %q left the caller's slice as %v", filter, entries)
			}
		}
	}
}

// TestTheRowsAreRebuiltWhenTheListingChanges is the guard for the defect that
// makes a picker choose a file nobody touched.
//
// A press target is found by its address. Kept across a move into another
// directory, the target that was pressed on the third row of the one left
// behind is the target of the third row of the one arrived at -- and the picker
// reports that file as chosen, with nothing on the screen having been pressed.
func TestTheRowsAreRebuiltWhenTheListingChanges(t *testing.T) {
	state := &FilePicker{}

	state.relist([]FileEntry{{Name: "a.txt"}, {Name: "b.txt"}, {Name: "c.txt"}})
	before := &state.rows[0]

	// The same count, so a check that compared lengths would see no change.
	state.relist([]FileEntry{{Name: "x.txt"}, {Name: "y.txt"}, {Name: "z.txt"}})

	if &state.rows[0] == before {
		t.Error("the press targets survived a change of listing, so a press on the old third row fires for the new one")
	}
	if strings.Join(state.listed, ",") != "x.txt,y.txt,z.txt" {
		t.Errorf("the rows belong to %v, which is not what is being drawn", state.listed)
	}
}

// TestTheRowsSurviveARedrawOfTheSameListing is the other half: rebuilding every
// frame would throw away the press between the finger going down and coming up,
// so the control could be held and would never fire.
func TestTheRowsSurviveARedrawOfTheSameListing(t *testing.T) {
	state := &FilePicker{}
	entries := []FileEntry{{Name: "a.txt"}, {Name: "b.txt"}}

	state.relist(entries)
	before := &state.rows[0]
	state.relist(entries)

	if &state.rows[0] != before {
		t.Error("an unchanged listing rebuilt its press targets, so a press begun on one frame is lost on the next")
	}
}

// TestAMarkGoesWhenItsRowDoes keeps the picker from reporting a selection the
// person cannot see.
func TestAMarkGoesWhenItsRowDoes(t *testing.T) {
	state := &FilePicker{}
	state.relist([]FileEntry{{Name: "a.txt"}, {Name: "b.txt"}})
	state.Choose("b.txt")

	state.relist([]FileEntry{{Name: "a.txt"}, {Name: "b.txt"}, {Name: "c.txt"}})
	if state.Selected() != "b.txt" {
		t.Error("a listing that still holds the marked row gave up the mark")
	}

	state.relist([]FileEntry{{Name: "q.txt"}})
	if state.Selected() != "" {
		t.Errorf("the mark is still on %q, which is not in the listing being drawn", state.Selected())
	}
}

// TestAMarkMadeBeforeTheFirstFrameSurvivesIt fixes the state a screen arrives
// with: a caller that knows which file is chosen says so before anything is
// drawn, and the first draw must not throw that away.
func TestAMarkMadeBeforeTheFirstFrameSurvivesIt(t *testing.T) {
	state := &FilePicker{}
	state.Choose("b.txt")

	state.relist([]FileEntry{{Name: "a.txt"}, {Name: "b.txt"}})

	if state.Selected() != "b.txt" {
		t.Errorf("the first draw dropped the mark the caller arrived with: %q", state.Selected())
	}
}

// TestEachIntentIsReportedOnceAndThenGone is the guard for a control that
// answers the same press on every frame.
//
// What a caller does with each of these is work -- read a directory, open a
// file, navigate -- and a press reported sixty times a second has it done sixty
// times for one press.
func TestEachIntentIsReportedOnceAndThenGone(t *testing.T) {
	state := &FilePicker{}
	state.act(FileEntry{Name: "reports", Dir: true})
	state.act(FileEntry{Name: "budget.csv"})
	state.depth, state.climbed = 2, true

	if name, asked := state.Opened(); !asked || name != "reports" {
		t.Errorf("the directory was reported as %q, %v", name, asked)
	}
	if _, asked := state.Opened(); asked {
		t.Error("the directory was reported a second time")
	}

	if name, picked := state.Chosen(); !picked || name != "budget.csv" {
		t.Errorf("the file was reported as %q, %v", name, picked)
	}
	if _, picked := state.Chosen(); picked {
		t.Error("the file was reported a second time")
	}

	if depth, asked := state.Up(); !asked || depth != 2 {
		t.Errorf("the step was reported as %d, %v", depth, asked)
	}
	if _, asked := state.Up(); asked {
		t.Error("the step was reported a second time")
	}
}

// TestADirectoryIsOpenedAndNeverChosen fixes the difference between the two
// answers.
//
// A directory is a request for another listing, and a file is the answer. A
// picker that reported a directory as chosen would hand the caller a name it
// cannot open, and the person would be left in a picker that closed on them.
func TestADirectoryIsOpenedAndNeverChosen(t *testing.T) {
	state := &FilePicker{}
	state.act(FileEntry{Name: "reports", Dir: true})

	if _, picked := state.Chosen(); picked {
		t.Error("pressing a directory reported a file as chosen")
	}
	if state.Selected() != "" {
		t.Errorf("pressing a directory marked %q", state.Selected())
	}
	if name, asked := state.Opened(); !asked || name != "reports" {
		t.Errorf("pressing a directory reported %q, %v", name, asked)
	}
}

// TestAChosenFileIsAlsoTheMarkedOne keeps the answer and the drawing saying the
// same thing: a picker that reported a file and marked nothing shows a listing
// with no sign of what was picked.
func TestAChosenFileIsAlsoTheMarkedOne(t *testing.T) {
	state := &FilePicker{}
	state.act(FileEntry{Name: "budget.csv"})

	if state.Selected() != "budget.csv" {
		t.Errorf("the marked row is %q", state.Selected())
	}
	if _, asked := state.Opened(); asked {
		t.Error("pressing a file asked for a directory listing")
	}
}

// TestTheTrailNeverReportsTheStepSomebodyIsOn keeps a press from asking the
// caller to move to where it already is, and keeps a target left over from a
// deeper path from naming a step that is no longer on the screen.
func TestTheTrailNeverReportsTheStepSomebodyIsOn(t *testing.T) {
	props := FilePickerProps{Path: []string{"reports", "2026"}}

	for depth, want := range map[int]bool{-1: false, 0: true, 1: true, 2: false, 3: false} {
		if got := props.ascends(depth); got != want {
			t.Errorf("step %d of a three-step trail reads as %v, want %v", depth, got, want)
		}
	}

	// At the top there is nowhere above to go, whatever the trail kept.
	top := FilePickerProps{}
	if top.ascends(0) {
		t.Error("the top of the trail reported a step above itself")
	}
}

// TestTheTopOfTheTrailIsAlwaysNamed keeps the first step from being an empty
// word nobody can press.
func TestTheTopOfTheTrailIsAlwaysNamed(t *testing.T) {
	unnamed := FilePickerProps{}
	if got := unnamed.steps(); len(got) != 1 || got[0] == "" {
		t.Errorf("a picker with no path draws the trail as %v", got)
	}

	named := FilePickerProps{Root: "Bucket", Path: []string{"invoices"}}
	if got := named.steps(); strings.Join(got, "/") != "Bucket/invoices" {
		t.Errorf("the trail reads as %v", got)
	}
}

// TestTheTrailDoesNotWriteIntoTheCallersPath keeps the head of the trail out of
// the slice the caller owns.
func TestTheTrailDoesNotWriteIntoTheCallersPath(t *testing.T) {
	path := make([]string, 1, 4)
	path[0] = "reports"
	props := FilePickerProps{Root: "Files", Path: path}

	props.steps()

	if path[0] != "reports" {
		t.Errorf("the caller's path came back as %v", path)
	}
}

// TestARowIsAsTallAsTheLineBesideIt is the guard for a defect this package has
// found three times.
//
// The mark's box is as tall as the name it sits beside, and that height comes
// from measuring the name. Read from the constraints instead it is nought,
// because Constraints.Min.Y of a flex child is nought -- and every row collapses
// to the height of its mark, with the names drawn down over each other.
func TestARowIsAsTallAsTheLineBesideIt(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	c.Constraints.Min = image.Point{} // what a flex child is handed.

	props := FilePickerProps{}
	entry := FileEntry{Name: "quarterly-report.pdf", Size: 4096}

	side, gap := c.Dp(unit.Dp(14)), c.Dp(unit.Dp(10))
	beside := c
	beside.Constraints.Max.X = c.Constraints.Max.X - side - gap
	beside.Constraints.Min = image.Pt(beside.Constraints.Max.X, 0)
	name := props.names(beside, entry)

	row := props.line(c, entry)

	if row.Size.Y != name.Size.Y {
		t.Errorf("the row is %dpt tall and the line in it is %dpt: the row never measured its own name", row.Size.Y, name.Size.Y)
	}
	if row.Size.Y <= side {
		t.Errorf("the row is %dpt tall and the mark alone is %dpt, so nothing was measured", row.Size.Y, side)
	}
}

// TestEveryRowTakesTheSameRoom keeps the names in a column.
//
// A directory row a point taller or a point wider than a file row would move
// every name that follows it, and a listing whose names do not line up is the
// kind of wrong that is felt long before it is seen.
func TestEveryRowTakesTheSameRoom(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	c.Constraints.Min = image.Point{}
	props := FilePickerProps{}

	directory := props.line(c, FileEntry{Name: "reports", Dir: true})
	file := props.line(c, FileEntry{Name: "reports", Size: 2048})

	if directory.Size != file.Size {
		t.Errorf("a directory row took %v and a file row took %v", directory.Size, file.Size)
	}
}

// TestBothMarksTakeTheSameSquare fixes the same property one level down, where
// it is decided.
func TestBothMarksTakeTheSameSquare(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	ink := c.Theme.Colours.Foreground

	folder := entryMark(c, true, 14, 20, ink)
	page := entryMark(c, false, 14, 20, ink)

	if folder.Size != page.Size {
		t.Errorf("the directory mark took %v and the file mark took %v", folder.Size, page.Size)
	}
	if folder.Size != image.Pt(14, 20) {
		t.Errorf("a mark asked for a 14 by 20 box took %v", folder.Size)
	}
}

// TestAMarkIsNeverShorterThanItself keeps a row too short to hold the mark from
// clipping it away, which reads as a listing of entries with no kind at all.
func TestAMarkIsNeverShorterThanItself(t *testing.T) {
	c, _ := field(t, theme.Light, 400)

	mark := entryMark(c, true, 14, 0, c.Theme.Colours.Foreground)

	if mark.Size.Y != 14 {
		t.Errorf("a mark in a row of no height took %v, want its own square", mark.Size)
	}
}

// TestThePanelHoldsItsWidthWithNothingToDraw fixes the width of the bordered
// box itself, which is not the width of the picker around it.
//
// The field above the panel fills the room whatever the panel does, so the
// picker measures the same either way and only the box moves. A message is as
// wide as its own sentence, and a panel that shrank to it would change size
// every time somebody typed another letter into a filter that matches less and
// less.
func TestThePanelHoldsItsWidthWithNothingToDraw(t *testing.T) {
	props := FilePickerProps{}

	for _, entries := range [][]FileEntry{nil, {{Name: "a.txt"}}} {
		c, _ := field(t, theme.Light, 400)
		c.Constraints.Min = image.Point{} // what a flex child is handed.

		state := &FilePicker{}
		state.relist(entries)

		dims := props.listing(c, state, entries)

		if dims.Size.X != 400 {
			t.Errorf("a panel holding %d entries took %dpt of 400", len(entries), dims.Size.X)
		}
	}
}

// TestADirectoryReportsNoSize keeps the picker from putting a number where it
// has none.
//
// What could go there is the size of everything inside the directory, which is
// not known without opening every one of them -- and a nought would be a claim
// about a directory that may hold a great deal.
func TestADirectoryReportsNoSize(t *testing.T) {
	if got := entryFigure(FileEntry{Name: "reports", Dir: true, Size: 4_096}); got != "" {
		t.Errorf("a directory reports %q as its size", got)
	}
	if got := entryFigure(FileEntry{Name: "budget.csv", Size: 4_096}); got != "4.1 kB" {
		t.Errorf("a file reports %q as its size", got)
	}
	if got := entryFigure(FileEntry{Name: "unknown.bin", Size: -1}); got != "" {
		t.Errorf("an unmeasured file reports %q as its size", got)
	}
}
