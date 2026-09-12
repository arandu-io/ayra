package key

import (
	"strings"
	"syscall/js"
)

// In a browser one build runs on every platform, so which key means "shortcut"
// is the one thing about it that cannot be known when the program is compiled.
// These are therefore variables where every other platform has constants, set
// once before anything is drawn and never written again -- a control reads them
// the same way everywhere, and only this file knows the difference.
var (
	// ModShortcut is the modifier held for a shortcut. It is ctrl, or the
	// command key when the browser says it is running on Apple hardware.
	ModShortcut = ModCtrl
	// ModShortcutAlt is the modifier held for the wider form of a shortcut --
	// by word rather than by character. It is ctrl, or the alt key on Apple
	// hardware.
	ModShortcutAlt = ModCtrl
)

func init() {
	if applePlatform(reportedPlatform()) {
		ModShortcut = ModCommand
		ModShortcutAlt = ModAlt
	}
}

// reportedPlatform answers what the browser says it is running on, folded to
// lower case, and the empty string when it will not say.
//
// Everything is checked before it is read. This runs before the application
// does, in a host that is not always a browser -- a test runner, an embedded
// engine, a page that has replaced half the globals -- and a missing navigator
// there would take the program down at start-up, before anything exists to
// report it. An unknown platform is not a failure: it means the default, which
// is right on the large majority of machines.
func reportedPlatform() string {
	navigator := js.Global().Get("navigator")
	if !navigator.Truthy() {
		return ""
	}
	platform := navigator.Get("platform")
	if !platform.Truthy() {
		return ""
	}
	return strings.ToLower(platform.String())
}

// applePlatform reports whether a platform string names Apple hardware.
//
// The strings are a short fixed list because that is what the value is: browsers
// froze it years ago rather than remove it, so it is a handful of words that no
// longer change, and one of them is reported by hardware that is not the machine
// it names -- a tablet says it is a desktop. That does not matter here. The
// question is which key the person's keyboard has, and every value in this list
// answers command.
//
// Matched on a prefix rather than anywhere in the string, because the words are
// short enough to appear inside something else: a platform that merely contains
// "mac" is not necessarily one.
func applePlatform(platform string) bool {
	for _, apple := range []string{"mac", "iphone", "ipad", "ipod"} {
		if strings.HasPrefix(platform, apple) {
			return true
		}
	}
	return false
}
