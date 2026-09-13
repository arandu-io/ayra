package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
)

var legacyMetalSelection = regexp.MustCompile(`(?m)\tif runtime\.GOOS == "darwin" \{\n\t\t([A-Za-z0-9_.\[\]]+)\.MetalLib = ([A-Za-z0-9_]+)\n\t\}\n\tif runtime\.GOOS == "ios" \{\n\t\tif runtime\.GOARCH == "amd64" \{\n\t\t\t([A-Za-z0-9_.\[\]]+)\.MetalLib = ([A-Za-z0-9_]+)\n\t\t\} else \{\n\t\t\t([A-Za-z0-9_.\[\]]+)\.MetalLib = ([A-Za-z0-9_]+)\n\t\t\}\n\t\}`)

func main() {
	file := flag.String("file", "shaders.go", "generated shader source to post-process")
	flag.Parse()

	source, err := os.ReadFile(*file)
	if err != nil {
		fatal(err)
	}
	processed, err := postprocess(source)
	if err != nil {
		fatal(err)
	}
	if bytes.Equal(source, processed) {
		return
	}
	info, err := os.Stat(*file)
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*file, processed, info.Mode().Perm()); err != nil {
		fatal(err)
	}
}

func postprocess(source []byte) ([]byte, error) {
	var transformErr error
	processed := legacyMetalSelection.ReplaceAllFunc(source, func(match []byte) []byte {
		parts := legacyMetalSelection.FindSubmatch(match)
		if len(parts) != 7 {
			transformErr = errors.New("unexpected legacy Metal selection shape")
			return match
		}
		if !bytes.Equal(parts[1], parts[3]) || !bytes.Equal(parts[1], parts[5]) {
			transformErr = fmt.Errorf("Metal assignments target different shaders: %q, %q, and %q", parts[1], parts[3], parts[5])
			return match
		}
		return fmt.Appendf(nil, "\t%s.MetalLib = metalLibraryFor(runtime.GOOS, targetIsIOSSimulator(), %s, %s, %s)", parts[1], parts[2], parts[6], parts[4])
	})
	if transformErr != nil {
		return nil, transformErr
	}
	if bytes.Contains(processed, []byte(`runtime.GOARCH == "amd64"`)) {
		return nil, errors.New("generated shaders still select Metal bytecode by architecture")
	}
	if !bytes.Contains(processed, []byte("metalLibraryFor(runtime.GOOS, targetIsIOSSimulator(),")) {
		return nil, errors.New("generated shaders contain no target-aware Metal selection")
	}
	return processed, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
