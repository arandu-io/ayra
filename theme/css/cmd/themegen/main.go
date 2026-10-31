// Command themegen writes a product's theme as Go from its stylesheet.
//
//	themegen -package brand -o theme_gen.go [-tokens tone-,box-] custom.css [more.css]
//
// Later stylesheets win over earlier ones, as later rules do. See package
// [github.com/arandu-io/ayra/theme/css] for what is read.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arandu-io/ayra/theme/css"
)

func main() {
	pkg := flag.String("package", "", "the package clause of the generated file")
	out := flag.String("o", "theme_gen.go", "the file to write")
	tokens := flag.String("tokens", "", "comma-separated prefixes of the colours to carry beyond the palette; empty carries all")
	flag.Parse()

	if *pkg == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: themegen -package name [-o file] [-tokens prefixes] stylesheet...")
		os.Exit(2)
	}

	var sources, names []string
	for _, path := range flag.Args() {
		body, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}
		sources = append(sources, string(body))
		names = append(names, filepath.Base(path))
	}

	sheet, err := css.Parse(sources...)
	if err != nil {
		fail(err)
	}

	var options css.Options
	if *tokens != "" {
		options.TokenPrefixes = strings.Split(*tokens, ",")
	}
	source, err := css.Generate(sheet, *pkg, strings.Join(names, ", "), options)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, source, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "themegen:", err)
	os.Exit(1)
}
