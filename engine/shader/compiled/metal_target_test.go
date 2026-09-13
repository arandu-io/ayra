package compiled

import "testing"

// TestMetalBytecodeFollowsTheApplePlatform fixes arm64 selecting device
// bytecode merely because only x86_64 used to mean simulator.
func TestMetalBytecodeFollowsTheApplePlatform(t *testing.T) {
	const (
		macOS        = "macos-library"
		iOS          = "ios-library"
		iOSSimulator = "ios-simulator-library"
	)

	for name, test := range map[string]struct {
		goos      string
		simulator bool
		want      string
	}{
		"macOS":               {goos: "darwin", want: macOS},
		"iOS device":          {goos: "ios", want: iOS},
		"iOS simulator arm64": {goos: "ios", simulator: true, want: iOSSimulator},
		"non-Metal target":    {goos: "linux", want: ""},
	} {
		t.Run(name, func(t *testing.T) {
			got := metalLibraryFor(test.goos, test.simulator, macOS, iOS, iOSSimulator)
			if got != test.want {
				t.Errorf("selected %q, want %q", got, test.want)
			}
		})
	}
}
