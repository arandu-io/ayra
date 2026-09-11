package ayra_test

import (
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// browserTarget is the pair the toolchain is asked for, and browserStaging is
// the directory the starter is compiled in.
//
// The staging directory begins with a dot so the go command walks past it, for
// the same reason the compile gate's does: a tree meant for somebody else's
// project must not be compiled as part of this one.
const (
	browserTarget  = "js/wasm"
	browserStaging = ".published-browser-size-check"
)

// The sizes the browser target must stay under, in bytes. The argument for
// both numbers is on the test below.
const (
	browserRawCeiling        = 22 << 20
	browserCompressedCeiling = 6 << 20
)

// TestTheBrowserTargetStaysUnderItsCeiling weighs what the browser target
// compiles to.
//
// The target was announced and never measured, which is how a preview becomes
// a download nobody has looked at. What is weighed here is the program the
// starter publishes rather than this library, because a library does not link:
// the only honest figure is the one a project would actually serve.
//
// Both the raw bytes and the compressed bytes are checked, and neither stands
// in for the other. The compressed size is what crosses the network, and it is
// what somebody on a slow connection waits through before anything is drawn.
// The raw size is what the device decompresses into memory and hands to the
// WebAssembly compiler, and on a phone that is the number that decides whether
// the tab is killed. Checking only the compressed figure would let a change
// that adds bytes which are already compressed -- a packed texture, a typeface
// carried as a blob -- pass while the memory cost climbed, because bytes like
// those compress to nothing and move the raw number alone.
//
// Today the program compiles to 20,480,342 bytes raw and 5,453,982
// compressed. The ceilings are 22 MiB and 6 MiB, which leaves about two and a
// half megabytes of room on the raw figure and eight hundred kilobytes on the
// compressed one. Adding a screen or a control moves this by kilobytes and
// will not reach either ceiling; carrying a second collection of typefaces, or
// linking a subsystem that was not linked before, moves it by megabytes and
// will. That is the change this is here to stop, and it is the only one worth
// stopping: a gate set tight enough to trip on ordinary work is a gate that
// gets raised without being read.
//
// The compressed figure is produced by the compressor in the standard library,
// which is slightly worse than the one a web server usually runs -- the same
// bytes through the command-line tool measured about one per cent smaller.
// There is enough room under the ceiling that the difference decides nothing.
//
// The packager does not strip this target: it adds an identifier to the link
// and nothing else. So the figure measured here is the figure that ships, and
// not a floor the shipped artifact sits above. Stripping was measured at about
// two per cent off the raw bytes, which is not an answer to a number this
// size.
//
// It costs one build and one compression pass: about three seconds with
// nothing cached and a little over one once warm, of which the compression is
// most. There is no guard for a short run, because that cost does not warrant
// one and a ceiling skipped by default is a ceiling nothing is ever measured
// against.
func TestTheBrowserTargetStaysUnderItsCeiling(t *testing.T) {
	if !toolchainBuildsFor(t, browserTarget) {
		t.Skipf("this toolchain does not build for %s, so there is nothing to weigh", browserTarget)
	}

	files, err := filepath.Glob(filepath.Join(source, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("%s holds no Go file, so this gate weighed nothing", source)
	}

	staging := filepath.Join(browserStaging, "native")
	t.Cleanup(func() { os.RemoveAll(browserStaging) })
	if err := os.RemoveAll(browserStaging); err != nil {
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

	binary, err := filepath.Abs(filepath.Join(staging, "main.wasm"))
	if err != nil {
		t.Fatal(err)
	}

	goos, goarch, ok := strings.Cut(browserTarget, "/")
	if !ok {
		t.Fatalf("%q does not name an operating system and an architecture", browserTarget)
	}

	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = staging
	// Built without cgo, and said rather than left to the environment. This
	// target has no cgo at all, but a machine with it switched on passes that
	// setting down: a font package in the dependency graph then selects a
	// standard-library implementation that does not exist for this pair, and
	// the build fails inside the standard library with a list of undefined
	// symbols that names nothing in this repository. The suite is run with cgo
	// on, so without this line the gate fails exactly where it is run.
	build.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the browser target does not compile, and it is announced as one that does:\n%s", output)
	}

	raw, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	// An empty file is under every ceiling, so a build that quietly produced
	// nothing would pass this test while proving the opposite of what it is
	// for.
	if len(raw) == 0 {
		t.Fatal("the build left an empty file, and nothing was weighed")
	}

	compressed := compressedSize(t, raw)
	t.Logf("%s: %d bytes raw, %d bytes compressed", browserTarget, len(raw), compressed)

	if len(raw) > browserRawCeiling {
		t.Errorf("the browser target is %d bytes raw, over the ceiling of %d: that is what a device decompresses and compiles before a frame is drawn", len(raw), browserRawCeiling)
	}
	if compressed > browserCompressedCeiling {
		t.Errorf("the browser target is %d bytes compressed, over the ceiling of %d: that is what somebody downloads before anything appears", compressed, browserCompressedCeiling)
	}
}

// toolchainBuildsFor answers whether the toolchain in use has a target.
//
// It is asked before the build rather than read out of one that failed. A
// build that fails because there is no such target and a build that fails
// because this repository stopped compiling for it exit the same way and are
// opposite problems: reporting the first as a failure breaks this gate on a
// machine that was never able to run it, and reporting the second as a skip
// lets the target rot in the silence this exists to end.
func toolchainBuildsFor(t *testing.T, target string) bool {
	t.Helper()

	list, err := exec.Command("go", "tool", "dist", "list").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(list), "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

// compressedSize answers what the bytes weigh once compressed.
//
// At the highest setting, because a server serving this would use it and a
// ceiling measured against a weaker setting is a ceiling measured against
// bytes nobody sends.
func compressedSize(t *testing.T, raw []byte) int {
	t.Helper()

	var held bytes.Buffer
	writer, err := gzip.NewWriterLevel(&held, gzip.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return held.Len()
}
