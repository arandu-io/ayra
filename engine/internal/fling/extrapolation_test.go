package fling

import (
	"testing"
	"time"
)

// The window these tests are written against.
//
// The numbers are stated here rather than read from the package for the reason
// every guarded constant in this repository is: a test that takes the value
// from the code it is checking agrees with whatever that code holds, including
// a value somebody changed by accident. A tuned constant that no test states is
// a constant that can be retuned in silence, and the retuning shows up as a
// list that stops too early rather than as a failure.
func TestTheWindowIsTheOneTheFitWasTunedFor(t *testing.T) {
	if degree != 2 {
		t.Errorf("the curve is of degree %d and the fit is tuned for 2", degree)
	}
	if historySize != 20 {
		t.Errorf("the ring keeps %d samples and 20 were expected", historySize)
	}
	if maxAge != 100*time.Millisecond {
		t.Errorf("the window reaches %v back and 100ms was expected", maxAge)
	}
	if maxSampleGap != 40*time.Millisecond {
		t.Errorf("a pause of %v ends the window and 40ms was expected", maxSampleGap)
	}
}

// A finger that did not move was not moving.
//
// The whole estimate hangs off differences between samples, so a run of
// identical positions has to answer zero and not a residue of the fit. A
// residue here is a list that drifts after a press that went nowhere.
func TestSamplesThatDoNotMoveAreNoMovement(t *testing.T) {
	var e Extrapolation
	for i := range 8 {
		e.Sample(time.Duration(i)*10*time.Millisecond, 100)
	}

	got := e.Estimate()
	if got.Velocity != 0 {
		t.Errorf("velocity %v from samples that never moved", got.Velocity)
	}
	if got.Distance != 0 {
		t.Errorf("distance %v from samples that never moved", got.Distance)
	}
}

// A drag at a steady speed is reported at that speed, against it.
//
// The velocity carries the opposite sign to the samples, and that is the
// contract rather than an accident of this test: the samples are where the
// finger is, and what the caller moves is what is under the finger, which
// travels the other way. A scroll adds this straight onto the distance it
// measures as the previous position minus the current one, and the two agree.
//
// The distance does not follow that flip. It is the span of the window in the
// samples' own direction, and it is asked for its size -- a caller compares it
// against the slop of its own gesture, in both directions at once.
func TestASteadyDragIsReportedAtItsSpeedAgainstIt(t *testing.T) {
	const speed = 2000

	var e Extrapolation
	steady(&e, 0, 13, 8*time.Millisecond, speed)

	got := e.Estimate()
	if !closeTo(got.Velocity, -speed, speed/100) {
		t.Errorf("velocity %v from a drag at %v per second", got.Velocity, float32(speed))
	}
	if want := float32(speed) * 0.096; !closeTo(got.Distance, want, want/100) {
		t.Errorf("distance %v across the window and %v was expected", got.Distance, want)
	}
}

// Fewer points than the curve has room for is no answer at all.
//
// The fit needs more points than the degree of the polynomial, and below that
// there is no unique curve. The zero estimate is the honest answer, and it is
// also the safe one: the caller compares the distance against its own slop
// before starting anything, and zero starts nothing.
func TestFewerPointsThanTheCurveNeedsIsNoAnswer(t *testing.T) {
	for count := range degree + 1 {
		var e Extrapolation
		steady(&e, 0, count, 8*time.Millisecond, 2000)

		if got := e.Estimate(); got != (Estimate{}) {
			t.Errorf("%d samples answered %+v and nothing was expected", count, got)
		}
	}

	var e Extrapolation
	steady(&e, 0, degree+1, 8*time.Millisecond, 2000)
	if got := e.Estimate(); got.Velocity == 0 {
		t.Errorf("%d samples answered nothing and that is one more than the curve needs", degree+1)
	}
}

// What the finger did a tenth of a second ago is not what it was doing when it
// left.
//
// The estimate has to be the same whether or not the older samples were ever
// offered, which is what says they were dropped rather than merely diluted.
func TestSamplesOlderThanTheWindowAreNotPartOfTheFling(t *testing.T) {
	const interval = 10 * time.Millisecond
	const speed = 1500

	var whole Extrapolation
	steady(&whole, 0, 30, interval, speed)

	// The last ten are the ones inside the window: the eleventh back is a full
	// maxAge old, and the window is open at that end.
	var recent Extrapolation
	steady(&recent, 20, 30, interval, speed)

	if got, want := whole.Estimate(), recent.Estimate(); got != want {
		t.Errorf("thirty samples answered %+v and the ten inside the window answered %+v", got, want)
	}
}

// A pause ends the window, and everything before the pause belongs to whatever
// the finger was doing then.
func TestAPauseEndsTheWindow(t *testing.T) {
	const interval = 8 * time.Millisecond
	const pause = maxSampleGap

	// Six samples, a pause, then six more. Only the six after the pause count.
	var interrupted Extrapolation
	var after Extrapolation
	at := time.Duration(0)
	for i := range 6 {
		interrupted.Sample(at, float32(i)*10)
		at += interval
	}
	at += pause
	for i := range 6 {
		value := 1000 + float32(i)*20
		interrupted.Sample(at, value)
		after.Sample(at, value)
		at += interval
	}

	if got, want := interrupted.Estimate(), after.Estimate(); got != want {
		t.Errorf("a run interrupted by a pause answered %+v and the run after the pause answered %+v", got, want)
	}
}

// Turning the drag around turns the answer around, and changes nothing else.
//
// The fit is linear in the sampled values, so this is exact rather than close:
// an axis that ran one way slightly faster than the other would be a list that
// flings further up than down.
func TestMirroringTheSamplesMirrorsTheAnswer(t *testing.T) {
	const interval = 8 * time.Millisecond

	var forward, backward Extrapolation
	at := time.Duration(0)
	for i := range 10 {
		value := float32(i*i) * 3
		forward.Sample(at, value)
		backward.Sample(at, -value)
		at += interval
	}

	got, mirrored := forward.Estimate(), backward.Estimate()
	if got.Velocity == 0 {
		t.Fatal("the drag being mirrored was not moving, so the test proves nothing")
	}
	if mirrored.Velocity != -got.Velocity {
		t.Errorf("velocity %v mirrored to %v", got.Velocity, mirrored.Velocity)
	}
	if mirrored.Distance != -got.Distance {
		t.Errorf("distance %v mirrored to %v", got.Distance, mirrored.Distance)
	}
}

// The ring keeps the newest samples, and keeps them in order.
//
// Fed more than it holds, it has to answer exactly what it would have answered
// had it been given only the ones it kept. An index that wrapped wrongly
// answers something plausible -- the samples are all real ones, just in the
// wrong order -- which is why this compares against the whole estimate rather
// than checking that it looks sensible.
func TestTheRingKeepsTheNewestSamples(t *testing.T) {
	const interval = 4 * time.Millisecond
	const speed = 900

	var overfilled Extrapolation
	steady(&overfilled, 0, 3*historySize, interval, speed)

	var kept Extrapolation
	steady(&kept, 2*historySize, 3*historySize, interval, speed)

	if got, want := overfilled.Estimate(), kept.Estimate(); got != want {
		t.Errorf("%d samples answered %+v and the %d it keeps answered %+v",
			3*historySize, got, historySize, want)
	}
}

// A relative sample is an absolute one that did the addition itself.
func TestRelativeSamplesAccumulate(t *testing.T) {
	const interval = 8 * time.Millisecond

	var absolute, relative Extrapolation
	at := time.Duration(0)
	running := float32(0)
	for i := range 10 {
		step := float32(i) * 1.5
		running += step
		absolute.Sample(at, running)
		relative.SampleDelta(at, step)
		at += interval
	}

	if got, want := relative.Estimate(), absolute.Estimate(); got != want {
		t.Errorf("relative samples answered %+v and the same movement given absolutely answered %+v", got, want)
	}
}

// No samples at all is no answer, and not a division by however many there
// were.
func TestNoSamplesIsNoAnswer(t *testing.T) {
	var e Extrapolation
	if got := e.Estimate(); got != (Estimate{}) {
		t.Errorf("an unsampled estimation answered %+v", got)
	}
}

// steady samples a movement at a constant speed over the half-open range of
// frames given, one sample every interval.
//
// A sample's position is a function of its own frame number and nothing else,
// so a long run and the tail of it agree to the bit -- which is what lets a
// test assert that two estimates are equal rather than merely close, and that
// is the only strength at which "these samples were dropped" can be said.
func steady(e *Extrapolation, from, to int, interval time.Duration, speed float32) {
	for i := from; i < to; i++ {
		at := time.Duration(i) * interval
		e.Sample(at, speed*float32(at.Seconds()))
	}
}

// closeTo reports whether got is within tolerance of want.
func closeTo(got, want, tolerance float32) bool {
	d := got - want
	return -tolerance <= d && d <= tolerance
}
