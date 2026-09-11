package ayra_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The first code a visitor copies has to compile.
//
// It did not. The README opened with a Go block calling a function this package
// does not have, on types it does not export, through a package layout it has
// never had -- a whole API invented in prose, in the one place a newcomer
// starts. Nothing failed: prose is not compiled, and the example had been read
// many times by people who already knew the real one.
//
// So it is compiled now. Every Go block in the README is extracted and built,
// against this module, the way somebody pasting it would.

// goBlock matches a fenced Go example.
var goBlock = regexp.MustCompile("(?s)```go\\n(.*?)```")

// TestEveryExampleInTheReadmeCompiles is the guard.
func TestEveryExampleInTheReadmeCompiles(t *testing.T) {
	body, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}

	blocks := goBlock.FindAllStringSubmatch(string(body), -1)
	if len(blocks) == 0 {
		t.Fatal("the README shows no Go, which is not what this project is for")
	}

	// The copy lands inside this module so the imports resolve against the real
	// go.mod: a directory elsewhere belongs to no module, and the compiler
	// refuses it before reading a line. The name begins with a dot so the go
	// command walks past it.
	root := ".readme-compile-check"
	t.Cleanup(func() { os.RemoveAll(root) })
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}

	for index, block := range blocks {
		source := block[1]
		if !strings.Contains(source, "package ") {
			// A fragment rather than a program. Compiling it would mean
			// inventing the file around it, and an invented wrapper is how a
			// test comes to prove something the reader never sees.
			t.Errorf("the Go example %d has no package clause, so nobody can run what it shows", index+1)
			continue
		}

		dir := filepath.Join(root, "example")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}

		build := exec.Command("go", "build", "-o", os.DevNull, ".")
		build.Dir = dir
		build.Env = append(os.Environ(), "GOWORK=off")
		if output, err := build.CombinedOutput(); err != nil {
			t.Errorf("the Go example %d in the README does not compile, and it is the first code anybody reads:\n%s", index+1, output)
		}
		os.RemoveAll(dir)
	}
}

// TestTheReadmeShowsTheWayInThatExists keeps the example from drifting to a
// second entry point.
//
// There is one way to open a window, and an example reaching for another would
// be describing an API this library does not offer even if some other spelling
// happened to compile.
func TestTheReadmeShowsTheWayInThatExists(t *testing.T) {
	body, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)

	for _, required := range []string{"shell.Run", "shell.Config"} {
		if !strings.Contains(text, required) {
			t.Errorf("the README does not show %s, which is how an application opens", required)
		}
	}
	for _, invented := range []string{"ayra.Run(", "ayra.Config{", "widget.Column(", "widget.Heading(", "widget.Button("} {
		if strings.Contains(text, invented) {
			t.Errorf("the README shows %s, which this library does not have", invented)
		}
	}
}
