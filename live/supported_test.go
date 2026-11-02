package live_test

import (
	"runtime"
	"testing"

	"github.com/arandu-io/ayra/live"
)

// A screen chooses between the moving picture and a still one by asking, before
// it opens a socket, and the answer has to be the one NewDecoder gives.
func TestSupportedIsWhatNewDecoderCanDo(t *testing.T) {
	want := runtime.GOOS == "darwin" || runtime.GOOS == "ios" || runtime.GOOS == "android"
	if live.Supported() != want {
		t.Fatalf("Supported() = %v on %s, want %v", live.Supported(), runtime.GOOS, want)
	}
	if !live.Supported() {
		if _, err := live.NewDecoder(nil); err != live.ErrUnsupported {
			t.Fatalf("a platform without a decoder answers ErrUnsupported, got %v", err)
		}
	}
}
