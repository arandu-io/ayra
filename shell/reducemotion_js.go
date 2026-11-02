//go:build js

package shell

import "syscall/js"

// systemReduceMotion is the browser's prefers-reduced-motion media query, which
// is where the operating system's setting reaches a page.
func systemReduceMotion() bool {
	match := js.Global().Get("matchMedia")
	if match.IsUndefined() {
		return false
	}
	query := match.Invoke("(prefers-reduced-motion: reduce)")
	return !query.IsUndefined() && query.Get("matches").Bool()
}
