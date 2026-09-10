// Command native is this application's native target: the window it opens, the
// server it draws for, and which screen is showing.
//
// It is yours from the moment it was published. Nothing upstream reads it and
// nothing regenerates it, so a screen added here stays added and a decision
// changed here stays changed.
//
// Every file here is one package, and that package is the program. A screen in
// a package of its own would have to be imported by its full path, and that
// path begins with the module name of this project -- which whoever published
// these files could not know. One package needs no such import, and being the
// program means there is nothing else to write before it runs.
package main

import (
	"flag"

	"github.com/arandu-io/ayra/shell"
)

// server is where this application looks for its server when the flag is not
// passed. Change it to yours before shipping.
//
// There is no discovery and no fallback: a native binary that guessed would go
// out pointing at whichever machine it was built on, and would look like it
// worked for the person who built it.
const server = "http://localhost:8080"

// title is what the window is called, in the dock and in the task switcher.
const title = "My Application"

func main() {
	address := flag.String("server", server, "the address of the Arandu server this draws for")
	dark := flag.Bool("dark", false, "open with the dark palette")
	flag.Parse()

	shell.Exit(run(Config{
		Server: *address,
		Title:  title,
		Scheme: scheme(*dark),
		Fonts:  faces(),
	}))
}
