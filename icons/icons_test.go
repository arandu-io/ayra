package icons_test

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/icons/lookup"
)

func TestThePathsOfARealIconParse(t *testing.T) {
	for name, icon := range map[string]icons.Icon{"Acorn": icons.Acorn(), "AddressBook": icons.AddressBook(), "Trash": icons.Trash(), "Gear": icons.Gear()} {
		for _, d := range []string{icon.Shade(), icon.Body()} {
			if d == "" {
				continue
			}
			segments, err := icons.ParsePath(d)
			if err != nil {
				t.Errorf("%s: %v", name, err)
			}
			if len(segments) < 3 {
				t.Errorf("%s: %d segments", name, len(segments))
			}
		}
	}
}

// TestEveryIconOfTheSetParses is the guard on the parser: it reads all 1512
// icons, both halves, and refuses nothing. A command it did not know would draw
// half of an icon, which nobody can see until the icon is looked at.
func TestEveryIconOfTheSetParses(t *testing.T) {
	names := lookup.Names()
	if len(names) < 1500 {
		t.Fatalf("the set has %d names", len(names))
	}
	for _, name := range names {
		icon, _ := lookup.Icon(name)
		for _, d := range []string{icon.Shade(), icon.Body()} {
			if d == "" {
				continue
			}
			segments, err := icons.ParsePath(d)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			for _, s := range segments {
				for _, p := range s.P {
					if math.IsNaN(float64(p.X)) || math.IsNaN(float64(p.Y)) || math.Abs(float64(p.X)) > 600 || math.Abs(float64(p.Y)) > 600 {
						t.Fatalf("%s: a point left the square: %+v", name, p)
					}
				}
			}
		}
	}
}

func TestTheParserKnowsTheCommands(t *testing.T) {
	for _, c := range []struct {
		name, d string
		last    icons.Segment
		count   int
	}{
		{"relative line", "M10,10 l5,5", icons.Segment{Op: icons.LineTo, P: [3]icons.Point{{15, 15}}}, 2},
		{"horizontal and vertical", "M1,1 H5 V7", icons.Segment{Op: icons.LineTo, P: [3]icons.Point{{5, 7}}}, 3},
		{"implicit lineto after a move", "M0,0 4,4", icons.Segment{Op: icons.LineTo, P: [3]icons.Point{{4, 4}}}, 2},
		{"close", "M0,0 L4,0 Z", icons.Segment{Op: icons.Close}, 3},
		{"cubic", "M0,0 C1,1 2,2 3,3", icons.Segment{Op: icons.CubicTo, P: [3]icons.Point{{1, 1}, {2, 2}, {3, 3}}}, 2},
	} {
		got, err := icons.ParsePath(c.d)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if len(got) != c.count || got[len(got)-1] != c.last {
			t.Errorf("%s: %d segments ending %+v, want %d ending %+v", c.name, len(got), got[len(got)-1], c.count, c.last)
		}
	}

	if _, err := icons.ParsePath("L1,1"); err != nil {
		// A path with no move is the one that needs the lenient reading; what
		// matters is that it does not panic.
		t.Log(err)
	}
	if _, err := icons.ParsePath("M1,1 Lx"); err == nil {
		t.Error("a path that is not a number was accepted")
	}
}

// TestAnArcEndsWhereItWasToldToEnd is the conversion that has to be exact at
// the ends and close in the middle: a half circle of radius 10 from (0,0) to
// (20,0) passes through (10,-10) or (10,10), depending on the sweep.
func TestAnArcEndsWhereItWasToldToEnd(t *testing.T) {
	segments, err := icons.ParsePath("M0,0 A10,10 0 0 1 20,0")
	if err != nil {
		t.Fatal(err)
	}
	last := segments[len(segments)-1]
	if last.P[2] != (icons.Point{X: 20, Y: 0}) {
		t.Errorf("the arc ends at %+v", last.P[2])
	}
	topmost := float32(0)
	for _, s := range segments[1:] {
		for _, p := range s.P {
			topmost = min(topmost, p.Y)
		}
	}
	if topmost > -9 {
		t.Errorf("a sweep of 1 from left to right arcs over the top, and its control points reach y=%v", topmost)
	}
}

func TestLookupFindsAnIconByEitherName(t *testing.T) {
	for _, name := range []string{"Trash", "trash", "AddressBook", "address-book"} {
		icon, found := lookup.Icon(name)
		if !found || icon.IsZero() {
			t.Errorf("%q is not found", name)
		}
	}
	if icon, found := lookup.Icon("Trahs"); found || !icon.IsZero() {
		t.Error("a misspelt name answered with an icon")
	}
}

// TestTheSetIsKyses compares the names with Kyse's, in both directions, when Kyse
// is beside this repository. Without it there is nothing to compare, and the
// test says so.
func TestTheSetIsKyses(t *testing.T) {
	directory := filepath.Join("..", "..", "kyse", "icons")
	files, _ := filepath.Glob(filepath.Join(directory, "icons_*.go"))
	if len(files) == 0 {
		t.Skip("kyse is not beside this repository: the set was not compared")
	}

	declaration := regexp.MustCompile(`(?m)^func ([A-Z][A-Za-z0-9]*)\(p Props\) template\.HTML`)
	kyse := map[string]bool{}
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range declaration.FindAllStringSubmatch(string(body), -1) {
			kyse[m[1]] = true
		}
	}

	ours := map[string]bool{}
	for _, name := range lookup.Names() {
		if name != strings.ToLower(name) {
			ours[name] = true
		}
	}

	var missing, extra []string
	for name := range kyse {
		if !ours[name] {
			missing = append(missing, name)
		}
	}
	for name := range ours {
		if !kyse[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("Kyse has %d icons this does not (run go generate ./icons): %v", len(missing), missing[:min(len(missing), 5)])
	}
	if len(extra) > 0 {
		t.Errorf("this has %d icons Kyse does not: %v", len(extra), extra[:min(len(extra), 5)])
	}
}
