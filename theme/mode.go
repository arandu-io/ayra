package theme

// Mode is where the scheme of a window comes from.
//
// It is the native half of the web panel's Automático, Claro and Escuro: the
// first is System, and the other two are Fixed with a [Scheme]. The scheme is
// what is drawn; the mode is why.
type Mode uint8

const (
	// Fixed draws the scheme the application asked for and ignores the system
	// until it is told to follow it. It is the zero value, which is what a
	// window opened before the system could be followed has always done.
	Fixed Mode = iota
	// System draws the scheme the operating system is in, and changes it while
	// the application runs when a person changes the system. A platform that
	// cannot tell leaves the scheme the window opened with.
	System
)

// String names the mode.
func (m Mode) String() string {
	if m == System {
		return "system"
	}
	return "fixed"
}
