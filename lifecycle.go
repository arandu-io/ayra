package ayra

// Lifecycle is where the application is in front of the person.
//
// A screen is a function of a frame and is never drawn while the application is
// out of sight, so it cannot notice that it went there. What runs for it -- a
// socket, a video, a poll -- can, and the shell tells the application through
// this: it is the signal to stop that work when the application goes to the
// background and to start it again when it comes back, in place of a heartbeat
// that guesses from the frames having stopped.
type Lifecycle uint8

const (
	// Foreground is the application on screen. It is what a window opens in, so
	// the first notice is only ever a return from [Background].
	Foreground Lifecycle = iota
	// Background is the application out of sight: an Android activity that
	// stopped, an iOS app that went to the background, a window minimised or
	// hidden, a browser tab put behind another. No frame is drawn until it
	// returns.
	Background
	// Destroyed is the window gone. It is the last notice, and the process is
	// about to end.
	Destroyed
)

// String names the stage.
func (l Lifecycle) String() string {
	switch l {
	case Background:
		return "background"
	case Destroyed:
		return "destroyed"
	}
	return "foreground"
}
