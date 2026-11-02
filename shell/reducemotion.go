package shell

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// reduceMotionEvery is the longest a reading of the system's setting is kept. A
// setting a person changes in the system's own screens does not need a call into
// the platform on every frame -- on Android that call crosses into the Java
// virtual machine -- and one a second is a change noticed while the person is
// still looking at the screen they came back to.
const reduceMotionEvery = time.Second

// reading is the one reading of the system's setting, and when it was made.
var reading struct {
	mu    sync.Mutex
	at    time.Time
	value bool
}

// probe asks the platform. It is a variable for one reason: a test replaces it
// with a platform that answers what the test says, and counts the questions.
var probe = systemReduceMotion

// now is the clock the cache is kept by, replaced by a test for the same reason.
var now = time.Now

// SystemReduceMotion reports whether the person has asked the operating system
// for less movement, the way the platform says it:
//
//	Android   the animation scale set to zero, which is what the accessibility
//	          option "Remove animations" does (Settings.Global.ANIMATOR_DURATION_SCALE)
//	iOS       UIAccessibility.isReduceMotionEnabled
//	macOS     NSWorkspace.accessibilityDisplayShouldReduceMotion
//	browser   the prefers-reduced-motion media query
//
// Any other platform, and a platform that cannot be asked yet, answers false: the
// animations run.
//
// It is read at most once a second and kept in between, so a screen may call it
// every frame. It is what [Config.ReduceMotion] answers when the application did
// not set one, and an application that wants to decide for itself -- a switch of
// its own in its settings -- sets that and may call this from it.
func SystemReduceMotion() bool {
	reading.mu.Lock()
	defer reading.mu.Unlock()

	if t := now(); reading.at.IsZero() || t.Sub(reading.at) >= reduceMotionEvery || t.Before(reading.at) {
		reading.value, reading.at = probe(), t
	}
	return reading.value
}

// reduceMotion is the answer a frame is given: the application's own when it set
// one, the system's when it did not.
func (cfg Config) reduceMotion() bool {
	if cfg.ReduceMotion != nil {
		return cfg.ReduceMotion()
	}
	return SystemReduceMotion()
}

// animationsOff reads the text of Android's animation scale. It lives here, with
// no build tag, so that the host can test it; only Android calls it.
//
// It reads the text of the animation scale: "0", "0.0" and "0.00" are
// off, "1.0" and "0.5" are not, and nothing set or unreadable is not.
func animationsOff(setting string) bool {
	setting = strings.TrimSpace(setting)
	if setting == "" {
		return false
	}
	scale, err := strconv.ParseFloat(setting, 64)
	return err == nil && scale == 0
}
