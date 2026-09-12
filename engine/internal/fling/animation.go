package fling

import (
	"math"
	"runtime"
	"time"

	"github.com/arandu-io/ayra/engine/unit"
)

// Animation carries a released drag on and slows it to a stop.
//
// It is started with the speed the release was worth, asked once per frame for
// the distance travelled since the last one, and reports itself finished when
// there is nothing left to hand out. The zero value is a fling that is not
// running, which is also how a caller stops one.
type Animation struct {
	// travelled is how much has already been handed out, in whole pixels.
	//
	// The fraction the frames have earned but not been paid stays here rather
	// than being rounded away, and is paid on a later frame. Dropped instead,
	// a slow fling rounds to nothing every frame and stops while it is still
	// moving.
	travelled float32
	// released is when the fling began. The curve is read from it rather than
	// stepped forward per frame, so frames of uneven length and frames that
	// were missed altogether come out in the right place.
	released time.Time
	// speed is what it began with, in pixels per second. Zero means it is over.
	speed float32
}

const (
	// minFlingVelocity is the speed a release has to beat to start anything, in
	// dp per second.
	//
	// Below it the hand was already coming to rest, and carrying the movement
	// on turns letting go into a nudge -- a list that creeps after every drag
	// and never sits where it was put.
	minFlingVelocity = unit.Dp(50)

	// maxFlingVelocity is the ceiling, in dp per second.
	//
	// The speed is fitted from a handful of samples, and samples are what a
	// device reports rather than what a finger did: one bad pair of them
	// implies a speed no hand produces. Unclamped, that throws the content out
	// of reach in a single frame and the list arrives somewhere nobody asked
	// for.
	maxFlingVelocity = unit.Dp(8000)

	// thresholdVelocity is the speed, in pixels per second, below which the
	// fling is finished.
	//
	// The curve approaches a stop without ever reaching one, so something has
	// to declare it over. Below a pixel a second there is no further whole
	// pixel to hand out, and the frames after it would only ask the window to
	// redraw what it already shows.
	thresholdVelocity = 1
)

// Start begins a fling at the given speed, and reports whether one began.
//
// The bounds are in device independent pixels and are converted here, so the
// same release behaves the same way on a dense display as on a sparse one. Read
// as raw pixels they would be half the speed on one of them, which is the kind
// of fault nobody reports and everybody feels.
func (f *Animation) Start(c unit.Metric, now time.Time, velocity float32) bool {
	floor := float32(c.Dp(minFlingVelocity))
	if -floor <= velocity && velocity <= floor {
		return false
	}

	ceiling := float32(c.Dp(maxFlingVelocity))
	if velocity > ceiling {
		velocity = ceiling
	} else if velocity < -ceiling {
		velocity = -ceiling
	}

	*f = Animation{released: now, speed: velocity}
	return true
}

// Active reports whether there is still movement to hand out.
func (f *Animation) Active() bool {
	return f.speed != 0
}

// Tick answers the distance travelled since the last call, in whole pixels.
func (f *Animation) Tick(now time.Time) int {
	if !f.Active() {
		return 0
	}

	k := decay()
	t := now.Sub(f.released)

	// A mass thrown and then held back by a drag proportional to its own speed
	// has acceleration
	//
	//	x''(t) = k*x'(t)
	//
	// and starting from x(0) = 0 at a speed of x'(0) = v0 that integrates to
	//
	//	x(t) = v0*e^(k*t)/k - v0/k
	//
	// with k negative, so the exponential falls away. Which is why the movement
	// never quite stops on its own, and why the whole journey is bounded: as
	// the exponential goes to nothing, x(t) approaches -v0/k and no further.
	ekt := float32(math.Exp(float64(k) * t.Seconds()))
	x := f.speed*ekt/k - f.speed/k

	// Whole pixels only, and the remainder is left on the clock.
	distance := x - f.travelled
	whole := int(distance)
	f.travelled += float32(whole)

	// Differentiating the same curve gives x'(t) = v0*e^(k*t), so the speed
	// now costs nothing beyond the exponential already in hand.
	if v := f.speed * ekt; -thresholdVelocity < v && v < thresholdVelocity {
		f.speed = 0
	}
	return whole
}

// decay is the drag coefficient, per second. The sign is what makes this a
// slowdown rather than a launch.
//
// Two values, because a fling is judged against everything else on the device:
// the same flick has to carry about as far here as it does in whatever the
// person was using a minute ago, and the platforms did not settle on one
// answer. Where the runtime reports darwin the movement coasts -- half the
// drag, so roughly twice the distance from the same release -- and everywhere
// else it settles sooner.
func decay() float32 {
	if runtime.GOOS == "darwin" {
		return -2
	}
	return -4.2
}
