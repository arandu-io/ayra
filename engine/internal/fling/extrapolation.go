// Package fling carries a released drag on, and brings it to rest.
//
// A finger that leaves the glass is still moving, and what was under it has to
// keep moving or the list stops dead under the hand. Two halves answer that.
// [Extrapolation] reads the positions reported while the finger was down and
// says how fast it was going at the instant it left; [Animation] takes that
// speed and hands out the distance travelled since the last frame, slowing
// until there is nothing left to hand out.
//
// Both are one-dimensional, so a gesture that moves in two directions keeps one
// of each per axis. Both are held by the caller and both are ready to use as
// their zero value, which is also how a caller throws one away: a new gesture
// assigns a fresh value rather than calling anything to reset.
package fling

import (
	"time"
)

// Extrapolation estimates how fast a drag was moving when it was let go.
//
// Positions are added with [Extrapolation.Sample] or [Extrapolation.SampleDelta]
// while the finger is down, and [Extrapolation.Estimate] is asked once, at the
// release. Samples arrive at whatever rate the device reports them and are kept
// in a ring, so a drag that goes on for a minute costs no more than one that
// lasts a frame.
//
// Reset by assigning the zero value rather than by copying a used one: the
// sample slice is a window onto the array beside it, so a copy would go on
// writing into the value it was copied from.
type Extrapolation struct {
	// next is where the following sample goes, so the newest is the one before
	// it and the ring wraps at the end of the buffer.
	next int
	// samples is the ring itself, grown into ring until it is full.
	samples []sample
	// last is the absolute position the most recent relative sample was added
	// to.
	last float32
	// ring is the storage, held here so that an estimation allocates nothing.
	ring [historySize]sample

	// values and times are the fitted window, kept as fields for the same
	// reason: they are rebuilt on every estimate and a fresh pair each time
	// would allocate during a gesture.
	values [historySize]float32
	times  [historySize]float32
}

// sample is one reported position and when it was reported.
type sample struct {
	at    time.Duration
	value float32
}

// Estimate is what a release is worth.
type Estimate struct {
	// Velocity is the speed at the instant of the newest sample, in the units
	// the samples were given in per second.
	//
	// It carries the opposite sign to the samples themselves, because what a
	// caller moves is what was under the finger and that travels the other way.
	// A scroll measures its own distance as the previous position minus the
	// current one, and this is in the same terms: the two are added together.
	Velocity float32

	// Distance is how far the gesture travelled across the window that was
	// used, in the samples' own direction.
	//
	// It is what tells a drag that went somewhere from a press that wandered a
	// pixel, and it is asked for its size: a caller compares it against the
	// slop of its own gesture, in both directions at once.
	Distance float32
}

const (
	// degree is the order of the curve fitted through the window.
	//
	// A parabola, so that the fit has a second derivative to spend: a finger is
	// almost never at a constant speed as it leaves -- it is still gathering
	// pace, or already braking -- and a straight line through the same points
	// answers the average speed across the window instead of the speed at the
	// end of it. The velocity is then the first order coefficient, which is the
	// slope at zero, and zero is the newest sample because the window is
	// measured backwards from it.
	degree = 2

	// historySize is how many samples the ring holds.
	//
	// It is not the window -- maxAge is -- and it is deliberately larger than
	// the window can use at the rates a touch screen reports at, so that the
	// answer is decided by how old a sample is and never by how many arrived.
	// A device that reports faster than expected loses the oldest samples,
	// which are the ones the window would have dropped anyway.
	historySize = 20

	// maxAge is how far back the window reaches.
	//
	// What the finger was doing a tenth of a second before it left is not what
	// it was doing when it left. Reaching further back averages a flick
	// together with the slow drag that set it up and answers something between
	// the two, which is a flick that goes nowhere.
	maxAge = 100 * time.Millisecond

	// maxSampleGap ends the window at a pause.
	//
	// Two samples further apart than this did not come from one continuous
	// movement: the finger rested, or the device stopped reporting. Whatever
	// happened before the pause belongs to an earlier movement, and fitting a
	// curve across the gap answers a speed that was never reached -- the
	// distance is real and the time it was covered in is not.
	maxSampleGap = 40 * time.Millisecond
)

// SampleDelta adds a sample given as a step from the one before it.
func (e *Extrapolation) SampleDelta(at time.Duration, delta float32) {
	e.Sample(at, e.last+delta)
}

// Sample adds a reported position and the time it was reported at.
func (e *Extrapolation) Sample(at time.Duration, value float32) {
	e.last = value
	if e.samples == nil {
		e.samples = e.ring[:0]
	}

	s := sample{at: at, value: value}
	if e.next == len(e.samples) {
		// Still filling: the ring has not yet been round once, and appending
		// stays inside the array because next is wrapped before it reaches the
		// end of it.
		e.samples = append(e.samples, s)
	} else {
		e.samples[e.next] = s
	}

	e.next++
	if e.next == cap(e.samples) {
		e.next = 0
	}
}

// Estimate answers the velocity and the distance implied by the samples, or the
// zero estimate when they imply nothing.
//
// The answer is a least squares fit over the whole recent window rather than
// the difference between the last two samples, and the difference is not
// accuracy for its own sake. Two samples are two readings, each carrying the
// error of the digitiser and the jitter of whenever the frame happened to read
// it, and dividing a small distance by a small and uncertain interval turns
// both into a large error in the answer. Worse, it believes whatever the last
// pair happened to be: a finger that hesitates for a single frame before
// lifting reports almost no movement across that pair, and the difference calls
// a fast drag a stop. A curve through every sample in the window lets them all
// speak, and the second order term absorbs the acceleration rather than
// smearing it across the answer.
//
// The zero estimate is returned when the window has fewer points than the curve
// needs, or when they do not determine one. It is the honest answer and also
// the safe one: a caller weighs the distance against its own slop first, and no
// distance starts nothing.
func (e *Extrapolation) Estimate() Estimate {
	if len(e.samples) == 0 {
		return Estimate{}
	}

	values := e.values[:0]
	times := e.times[:0]

	newest := e.back(0)
	previous := newest.at
	for n := range e.samples {
		s := e.back(n)
		age := newest.at - s.at
		if age >= maxAge || previous-s.at >= maxSampleGap {
			break
		}
		previous = s.at
		// Measured backwards from the newest sample, which sits at the origin:
		// times run into the past and are negative, and the fitted slope at
		// zero is therefore the speed at the release.
		times = append(times, float32(-age.Seconds()))
		values = append(values, newest.value-s.value)
	}

	curve, ok := polyFit(times, values)
	if !ok {
		return Estimate{}
	}
	return Estimate{
		Velocity: curve[1],
		// The span of the window. The subtraction says what is meant even
		// though the first value is zero by construction.
		Distance: values[len(values)-1] - values[0],
	}
}

// back answers the sample n places before the newest one.
func (e *Extrapolation) back(n int) sample {
	return e.samples[(e.next-1-n+len(e.samples))%len(e.samples)]
}
