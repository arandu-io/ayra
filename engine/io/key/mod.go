//go:build !darwin && !js

package key

// Everywhere but Apple hardware, the key that means "this is a shortcut" is
// ctrl, and there is no second one: alt is a modifier people type characters
// with, so a shortcut that claimed it would eat somebody's keyboard layout.
// Both constants are therefore ctrl, and the alternative exists only so that a
// control can be written once for every platform.
const (
	// ModShortcut is the modifier held for a shortcut. It is ctrl here and the
	// command key on Apple platforms.
	ModShortcut = ModCtrl
	// ModShortcutAlt is the modifier held for the wider form of a shortcut --
	// by word rather than by character. It is ctrl here and the alt key on
	// Apple platforms.
	ModShortcutAlt = ModCtrl
)
