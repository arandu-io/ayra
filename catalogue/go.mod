// The catalogue: one program that draws every control the library has.
//
// It is a module of its own, and this file is the reason. A program carries its
// whole graph behind it, so a command kept inside the library would put
// everything it needs to run into the requirements of every project that
// imported a button -- and in Go there is no optional dependency to make that a
// choice. Separate, installing the library installs the library.
//
// The replacement is what lets it be written against the tree it sits in rather
// than against the last release: a control renamed here breaks this build in
// the same commit, which is the only moment the rename is cheap.
module github.com/arandu-io/ayra/catalogue

go 1.26.0

require github.com/arandu-io/ayra v0.0.0

require (
	github.com/go-text/typesetting v0.3.4 // indirect
	golang.org/x/image v0.43.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.38.0 // indirect
)

replace github.com/arandu-io/ayra => ..
