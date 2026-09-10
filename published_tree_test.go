package ayra_test

import (
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// source is where the files a project receives are kept, and stagingRoot is
// the directory this test compiles them in.
//
// The published tree is kept under a name the go command walks past, so that
// building this repository does not try to compile a package that is meant to
// live in somebody else's. That is what makes this test necessary: without it
// a type error in a published file is found by whoever published it into their
// project, and nowhere earlier.
const (
	source      = "module/resources/_publish"
	stagingRoot = ".published-compile-check"
)

// TestThePublishedTreeCompiles builds the files a project receives.
//
// They are compiled here, against this module, because everything they import
// is in it. In a project they are compiled against that project, which imports
// this one -- so what this proves is the half that can be proven from here:
// that the tree is valid Go, that it uses this library's API as it is spelled
// today, and that a rename in a control breaks the gate rather than the person
// who published it a version later.
func TestThePublishedTreeCompiles(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(source, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("%s holds no Go file, so this gate compiled nothing", source)
	}

	// The copy lands inside this module so the imports resolve against the
	// real go.mod: a directory elsewhere belongs to no module, and the
	// compiler refuses it before reading a line. The name begins with a dot so
	// that the go command walks past this one too, and it is removed either
	// way.
	staging := filepath.Join(stagingRoot, "native")
	t.Cleanup(func() { os.RemoveAll(stagingRoot) })
	if err := os.RemoveAll(stagingRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(staging, filepath.Base(file)), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	build := exec.Command("go", "build", "./...")
	build.Dir = staging
	build.Env = append(os.Environ(), "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the published tree does not compile, and no other gate would have said so:\n%s", output)
	}
}

// TestThePublishedTreeIsOnePackage keeps a screen from being put in a package
// of its own.
//
// A second package would have to be imported by its full path, and that path
// begins with the module name of whichever project the files were published
// into -- which is not knowable here. The failure appears in that project, at
// the first build, as an import of a module that does not exist.
func TestThePublishedTreeIsOnePackage(t *testing.T) {
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			t.Errorf("%s/%s is a directory, and a screen in a package of its own cannot be imported from a project this does not know the name of", source, entry.Name())
		}
	}
}

// TestThePublishedTreeImportsNothingFromTheProject fixes the same rule from the
// other side.
//
// An import that is neither the standard library nor this library is an import
// of the project the files landed in, spelled by whoever wrote them -- and it
// compiles here only because it does not exist yet.
func TestThePublishedTreeImportsNothingFromTheProject(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(source, "*.go"))
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		for _, imported := range importsOf(string(body)) {
			if strings.HasPrefix(imported, "github.com/arandu-io/ayra") || standard(imported) {
				continue
			}
			t.Errorf("%s imports %q, which exists in no project this is published into", file, imported)
		}
	}
}

// standard reports whether a path is in the standard library.
//
// It asks the toolchain rather than guessing from the shape of the path. The
// guess that suggests itself -- "no dot in the first element" -- calls a
// project named "myapp" part of the standard library, which is exactly the
// import this test exists to catch.
func standard(path string) bool {
	found, err := build.Default.Import(path, ".", build.FindOnly)
	return err == nil && found.Goroot
}

// importsOf answers the paths a file imports, read from its import block.
func importsOf(body string) []string {
	start := strings.Index(body, "\nimport (")
	if start < 0 {
		return singleImport(body)
	}
	block := body[start+len("\nimport ("):]
	end := strings.Index(block, "\n)")
	if end < 0 {
		return nil
	}

	var paths []string
	for _, line := range strings.Split(block[:end], "\n") {
		if path := quoted(line); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

// singleImport answers the one path of a file that imports without a block.
func singleImport(body string) []string {
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "import ") {
			continue
		}
		if path := quoted(line); path != "" {
			return []string{path}
		}
	}
	return nil
}

// quoted answers what is between the first pair of quotes on a line.
func quoted(line string) string {
	open := strings.Index(line, `"`)
	if open < 0 {
		return ""
	}
	close := strings.Index(line[open+1:], `"`)
	if close < 0 {
		return ""
	}
	return line[open+1 : open+1+close]
}
