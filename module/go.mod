// The half of Ayra that an Arandu application registers.
//
// It is a module of its own because it depends on the framework, and the
// drawing half must not: a program that only draws controls -- a catalogue, a
// tool, a screen in a test -- would otherwise carry routing, authorization and
// a database driver in its graph to put a button on screen.
//
// It is the same argument that puts a heavy driver in its own module
// elsewhere in this collection: Go has no optional dependency, so the only
// lever is the module boundary.
module github.com/arandu-io/ayra/module

go 1.26

require (
	github.com/arandu-io/framework v0.48.0
	github.com/arandu-io/hesape v0.42.2
)

require (
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
