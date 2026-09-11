package ayra_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A name from somewhere else must not reach a shipped artifact.
//
// The engine of this runtime began as a fork, and the record of where it came
// from is kept in one file beside it. That is a document; a package path is
// not. A directory name becomes an import path, an import path becomes a symbol
// in the compiled binary, and the binary is what a customer installs and what
// anybody can run strings over.
//
// This was found by doing exactly that to a packaged application: the module
// carried no occurrence of the upstream module path anywhere, and one directory
// of shaders had kept its original leaf name, so every artifact built from the
// runtime exported a symbol naming the project it was forked from.

// foreign are the names that must not appear in a path or a package clause.
var foreign = []string{"gio", "gioui"}

// TestNoPackagePathCarriesAForeignName walks the module and reads the name of
// every directory.
func TestNoPackagePathCarriesAForeignName(t *testing.T) {
	root := moduleRoot(t)

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if skipped(entry.Name()) {
			return filepath.SkipDir
		}

		for _, name := range foreign {
			if strings.EqualFold(entry.Name(), name) {
				relative, _ := filepath.Rel(root, path)
				t.Errorf("the directory %s is a package path, and it names another project; every binary built from this exports it", relative)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestNoPackageClauseCarriesAForeignName is the same rule read from the source,
// because a package may be named differently from the directory holding it.
func TestNoPackageClauseCarriesAForeignName(t *testing.T) {
	clause := regexp.MustCompile(`(?m)^package ([A-Za-z_][A-Za-z0-9_]*)`)
	root := moduleRoot(t)

	read := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skipped(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		read++

		found := clause.FindSubmatch(source)
		if found == nil {
			return nil
		}
		declared := string(found[1])
		for _, name := range foreign {
			if strings.EqualFold(strings.TrimSuffix(declared, "_test"), name) {
				relative, _ := filepath.Rel(root, path)
				t.Errorf("%s declares package %s, which names another project", relative, declared)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// A walk that read nothing would pass in silence.
	if read < 100 {
		t.Errorf("only %d source files were read, and this module has many more", read)
	}
}

// skipped answers whether a directory is outside what ships.
func skipped(name string) bool {
	switch {
	case name == "testdata", name == "vendor", name == "node_modules":
		return true
	case strings.HasPrefix(name, "."):
		// The version control directory, the editor's, and the scratch
		// worktrees an agent works in. None of them is compiled.
		return name != "."
	}
	return false
}

// moduleRoot answers the directory holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()

	at, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(at, "go.mod")); err == nil {
			return at
		}
		parent := filepath.Dir(at)
		if parent == at {
			t.Fatal("no go.mod above the working directory")
		}
		at = parent
	}
}
