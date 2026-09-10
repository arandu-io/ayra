// Package module is what an Arandu application registers to gain a native
// target.
//
// Registering it publishes a tree the project owns from then on: the window,
// the navigation and one file per screen. Nothing here draws -- the drawing
// half is a separate module, and this one exists so that an application can
// say "I have a native target" in the same place it says everything else.
package module

import (
	"embed"
	"io/fs"
	"path"
	"sort"

	"github.com/arandu-io/framework/foundation"
	"github.com/arandu-io/framework/http"
)

// The tree this publishes into a project.
//
//go:embed all:resources/_publish
var sources embed.FS

// sourceRoot is the directory inside the archive that is published, and
// publishRoot is where it lands in a project.
//
// They are spelled once each, and the paths under sourceRoot are the paths the
// files take below publishRoot -- so a screen sits in the archive at the
// address it will sit at in the project, and there is no third spelling for
// the other two to disagree with.
const (
	sourceRoot  = "resources/_publish"
	publishRoot = "cmd/native"
)

// Module is the native target, as an application registers it.
type Module struct{}

// Compile-time proof that this honors the contracts it claims.
var (
	_ foundation.Module      = (*Module)(nil)
	_ foundation.Publishable = (*Module)(nil)
)

// New returns the module.
//
// It takes nothing and cannot fail, and both are the point: this module owns
// no state, no table and no connection. What it hands over is a tree of files,
// and a tree of files is either compiled in or it is not.
func New() *Module { return &Module{} }

// Name is what this module is called wherever a module is named.
func (m *Module) Name() string { return "ayra" }

// Routes registers nothing.
//
// A native application draws for a server that is already running, and the
// addresses it asks for are the application's own -- the same handlers a
// browser reaches, answering the same values. A route registered here would be
// a second address for a page that already has one, which is the second path
// this whole design exists to avoid.
func (m *Module) Routes(r *http.Router) {}

// Publishes declares the tree this offers a project.
//
// One tree, under one tag, and the tag is the one for a page the project is
// meant to edit -- because that is what these are. A screen is markup's
// equivalent on this side: the file that says what the page looks like, and
// the file nobody but the application can write, because a package cannot know
// what a screen should say in a product it has never seen.
//
// They are plain Go and not a template language. A second templating language
// for the native target would be a second way to draw the same page, and the
// answer this collection already gave to that question was two packages rather
// than a mode on one.
//
// What lands is a program and not a library, so `go run ./cmd/native` is the
// whole of running it. A library would need a main written somewhere else that
// imported it by a path beginning with this project's module name -- which is
// not knowable from here.
func (m *Module) Publishes() []foundation.Publication {
	return []foundation.Publication{{
		Tag:   foundation.PublishView,
		Files: sources,
		From:  sourceRoot,
		To:    publishRoot,
	}}
}

// PublishedPaths are the files this writes into a project, each relative to
// the project root and in a stable order.
//
// It reads the archive rather than repeating it as a list, because a list
// written by hand is a second answer to "what does this publish" and the two
// diverge the first time a screen is added.
func PublishedPaths() []string {
	var paths []string

	fs.WalkDir(sources, sourceRoot, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepathRel(sourceRoot, file)
		if err != nil {
			return err
		}
		paths = append(paths, path.Join(publishRoot, relative))
		return nil
	})

	sort.Strings(paths)
	return paths
}

// filepathRel answers file relative to root, for the slash-separated paths an
// embedded archive uses.
func filepathRel(root, file string) (string, error) {
	if root == "" || root == "." {
		return file, nil
	}
	prefix := root + "/"
	if len(file) <= len(prefix) || file[:len(prefix)] != prefix {
		return "", &pathError{root: root, file: file}
	}
	return file[len(prefix):], nil
}

// pathError is what a file outside the published root answers with. It cannot
// happen while the walk starts at that root, and saying so beats returning a
// path that quietly lost its prefix.
type pathError struct {
	root string
	file string
}

func (e *pathError) Error() string {
	return "ayra/module: " + e.file + " is not under " + e.root
}
