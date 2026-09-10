package module_test

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/arandu-io/framework/foundation"
	"github.com/arandu-io/framework/http"

	ayramodule "github.com/arandu-io/ayra/module"
)

// TestItPublishesTheTreeItCarries fixes that the archive is not empty.
//
// An embed pattern that matches nothing is a compile error, but one that
// matches the wrong directory is not: it produces a module that registers, that
// publishes, and that writes no file at all. Whoever ran the command sees it
// succeed.
func TestItPublishesTheTreeItCarries(t *testing.T) {
	publications, err := foundation.Publications(ayramodule.New())
	if err != nil {
		t.Fatalf("the publications were refused: %v", err)
	}
	if len(publications) != 1 {
		t.Fatalf("this publishes %d trees, want 1", len(publications))
	}

	p := publications[0]
	if p.Tag != foundation.PublishView {
		t.Errorf("the tree is published as %q, want a page the project edits", p.Tag)
	}

	var count int
	err = fs.WalkDir(p.Files, p.From, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("the archive could not be walked: %v", err)
	}
	if count == 0 {
		t.Error("the archive carries no file, and publishing it would write nothing while reporting success")
	}
}

// TestEveryPublishedPathIsUnderTheProjectDirectory keeps a published file from
// landing somewhere a project did not agree to.
func TestEveryPublishedPathIsUnderTheProjectDirectory(t *testing.T) {
	paths := ayramodule.PublishedPaths()
	if len(paths) == 0 {
		t.Fatal("nothing is published, so this gate checked nothing")
	}

	for _, p := range paths {
		if !strings.HasPrefix(p, "cmd/native/") {
			t.Errorf("%q lands outside the directory this module owns", p)
		}
		if path.IsAbs(p) || strings.Contains(p, "..") {
			t.Errorf("%q escapes the project root", p)
		}
	}
}

// TestThePublishedPathsMatchTheArchive keeps the two answers to "what does this
// publish" from drifting.
func TestThePublishedPathsMatchTheArchive(t *testing.T) {
	publications, err := foundation.Publications(ayramodule.New())
	if err != nil {
		t.Fatal(err)
	}
	p := publications[0]

	var fromArchive []string
	fs.WalkDir(p.Files, p.From, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		fromArchive = append(fromArchive, path.Join(p.To, strings.TrimPrefix(file, p.From+"/")))
		return nil
	})
	slices.Sort(fromArchive)

	if got := ayramodule.PublishedPaths(); !slices.Equal(got, fromArchive) {
		t.Errorf("PublishedPaths answers\n  %v\nand the archive carries\n  %v", got, fromArchive)
	}
}

// TestPublishedPathsIsACopy keeps a caller from editing what the next caller
// receives.
func TestPublishedPathsIsACopy(t *testing.T) {
	first := ayramodule.PublishedPaths()
	if len(first) == 0 {
		t.Fatal("nothing is published, so this gate checked nothing")
	}
	first[0] = "tampered"

	if second := ayramodule.PublishedPaths(); second[0] == "tampered" {
		t.Error("one caller's edit reached the next")
	}
}

// TestNothingLandsUnderAReservedDirectory fixes a defect this collection has
// already shipped once.
//
// The go command leaves a file under a directory named "vendor" out of the
// module zip, and refuses to import a package whose path carries that element.
// A published file placed there is in the repository, absent for everyone who
// downloads the module, and unusable for anyone who has it.
func TestNothingLandsUnderAReservedDirectory(t *testing.T) {
	for _, p := range ayramodule.PublishedPaths() {
		for _, element := range strings.Split(p, "/") {
			if element == "vendor" {
				t.Errorf("%q carries a reserved path element, and the go command would leave it out of the module zip", p)
			}
		}
	}
}

// TestItRegistersNoRoute keeps the native target from becoming a second address
// for a page that already has one.
//
// The pages a native application draws are the application's own, answered by
// the handlers a browser reaches. A route added here would be a second address
// for one of them, with its own authorization to keep in step -- and the two
// only stay in step until somebody changes one.
func TestItRegistersNoRoute(t *testing.T) {
	router := http.NewRouter()
	before := len(router.Routes())

	ayramodule.New().Routes(router)

	if added := len(router.Routes()) - before; added != 0 {
		t.Errorf("the native module registered %d routes", added)
	}
}
