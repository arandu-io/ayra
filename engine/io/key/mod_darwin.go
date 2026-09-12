package key

// On Apple hardware the shortcut key is command and not ctrl, and the two are
// not interchangeable: ctrl-C there interrupts, and a person who presses it
// expecting copy gets nothing copied. Alt -- option on these keyboards -- is
// what widens a movement to a whole word.
//
// The file name is the whole of the build constraint, and it covers iOS as well
// as macOS, which is right: an iPad with a keyboard attached is one of these.
const (
	// ModShortcut is the modifier held for a shortcut. It is the command key
	// here and ctrl elsewhere.
	ModShortcut = ModCommand
	// ModShortcutAlt is the modifier held for the wider form of a shortcut --
	// by word rather than by character. It is the alt key here and ctrl
	// elsewhere.
	ModShortcutAlt = ModAlt
)
