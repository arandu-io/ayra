package main

import (
	"bytes"
	"testing"
)

func TestPostprocessSelectsMetalLibraryByTarget(t *testing.T) {
	legacy := []byte(`func init() {
	if runtime.GOOS == "darwin" {
		Shader_example.MetalLib = example_metallibmacos
	}
	if runtime.GOOS == "ios" {
		if runtime.GOARCH == "amd64" {
			Shader_example.MetalLib = example_metallibiossimulator
		} else {
			Shader_example.MetalLib = example_metallibios
		}
	}
}
`)

	got, err := postprocess(legacy)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`func init() {
	Shader_example.MetalLib = metalLibraryFor(runtime.GOOS, targetIsIOSSimulator(), example_metallibmacos, example_metallibios, example_metallibiossimulator)
}
`)
	if !bytes.Equal(got, want) {
		t.Fatalf("postprocess result:\n%s\nwant:\n%s", got, want)
	}

	again, err := postprocess(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, got) {
		t.Fatal("postprocess is not idempotent")
	}
}

func TestPostprocessRejectsArchitectureSelection(t *testing.T) {
	source := []byte(`var stale = runtime.GOARCH == "amd64"
var current = metalLibraryFor(runtime.GOOS, targetIsIOSSimulator(), macOS, iOS, iOSSimulator)
`)
	if _, err := postprocess(source); err == nil {
		t.Fatal("postprocess accepted architecture-based Metal selection")
	}
}
