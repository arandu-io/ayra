package key

import "testing"

// The browser is the one platform where the shortcut key is decided while the
// program is running, so it is the one platform where that decision can be
// wrong on a machine and right on the build server.
//
// This file is named for the platform rather than for what it holds, because
// the suffix that restricts it to the browser has to be the last thing in the
// name: anything after it stops being a build constraint and starts being part
// of the word. It reaches the unexported half on purpose -- what is worth
// testing is the judgement about the platform string, and the two globals it
// sets are read by every other test in this package already.

// TestAppleHardwareIsRecognisedFromWhatTheBrowserReports is the whole of the
// browser's decision.
//
// Getting it wrong costs every shortcut in the application at once, in one
// direction or the other: ctrl-C on a Mac interrupts instead of copying, and
// command-C elsewhere is a key combination the keyboard does not have.
func TestAppleHardwareIsRecognisedFromWhatTheBrowserReports(t *testing.T) {
	for _, reported := range []string{
		"macintel",
		"macintosh",
		"macppc",
		"mac68k",
		"iphone",
		"ipad",
		"ipod touch",
	} {
		if !applePlatform(reported) {
			t.Errorf("%q was not taken for Apple hardware, so every shortcut there asks for a key the keyboard does not have", reported)
		}
	}

	for _, reported := range []string{
		"",
		"win32",
		"windows",
		"linux x86_64",
		"linux armv8l",
		"freebsd amd64",
		"android",
		"emacs", // Contains no prefix of ours, and must not be caught by one.
	} {
		if applePlatform(reported) {
			t.Errorf("%q was taken for Apple hardware, so ctrl there would stop meaning shortcut", reported)
		}
	}
}

// TestThePlatformIsReadInLowerCase fixes the folding, because what a browser
// reports is capitalised and the list compared against is not. Comparing the
// two as they come matches nothing at all, which is the failure that looks like
// working software everywhere except on a Mac.
func TestThePlatformIsReadInLowerCase(t *testing.T) {
	if applePlatform("MacIntel") {
		t.Fatal("a capitalised platform matched, so the comparison is not the one reportedPlatform prepares for")
	}
	if got := reportedPlatform(); got != "" && got != toLower(got) {
		t.Fatalf("the platform was read as %q, which is not folded to lower case", got)
	}
}

// toLower is here rather than imported so that the test asserts the folding
// instead of borrowing whatever the file under test used to do it.
func toLower(s string) string {
	folded := []rune(s)
	for i, r := range folded {
		if 'A' <= r && r <= 'Z' {
			folded[i] = r + ('a' - 'A')
		}
	}
	return string(folded)
}
