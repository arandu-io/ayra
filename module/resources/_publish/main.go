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
	"os"
	"path/filepath"
	"strings"

	"github.com/arandu-io/ayra/client"
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
//
// Empty, and derived below from the name this was packaged under. A constant
// here would be a second place to write the application's name, and the one
// that gets forgotten is this one: the installed artifact says Proof and the
// window above it says whatever the starter shipped with.
const title = ""

func main() {
	address := flag.String("server", server, "the address of the Arandu server this draws for")
	dark := flag.Bool("dark", false, "open with the dark palette")
	forget := flag.Bool("forget", false, "do not keep the session between runs")
	flag.Parse()

	shell.Exit(run(Config{
		Server:  *address,
		Title:   windowTitle(),
		Scheme:  scheme(*dark),
		Fonts:   faces(),
		Session: session(*forget),
	}))
}

// session is where this application keeps somebody signed in between runs.
//
// A file under the account's own configuration directory, readable by that
// account and no other. Return nil instead to keep the session only while the
// process lives, which is the right answer for an application handling
// something somebody would not want left on a shared machine.
//
// A store that cannot be opened is not a reason to refuse to start: the worst
// it costs is a sign-in, and an application that will not open because it could
// not find a directory is worse than one that asks for a password.
func session(forget bool) client.Store {
	if forget {
		return nil
	}

	store, err := client.FileSession(windowTitle())
	if err != nil {
		return nil
	}
	return store
}

// windowTitle answers what to call the window.
//
// The name the application was packaged under is the name of the file that is
// running, which every platform here takes from what was passed to the
// packager. Reading it means the window, the icon's label and the installed
// artifact cannot disagree.
func windowTitle() string {
	if title != "" {
		return title
	}
	if len(os.Args) == 0 {
		return "Application"
	}

	name := filepath.Base(os.Args[0])
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "Application"
	}
	return name
}
