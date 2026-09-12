package fling

import (
	"runtime"
	"testing"
	"time"

	"github.com/arandu-io/ayra/engine/unit"
)

// frame is a plausible gap between two redraws, and the step these tests hand
// out time in.
const frame = 16 * time.Millisecond

// The bounds are the ones the movement was tuned for.
//
// Stated here rather than read from the package: a test that takes the number
// from the code it is checking passes on whatever that code holds. Each of
// these is the difference between a fling that feels like the platform it is
// running on and one that does not, and none of them is derivable from
// anything else in the package.
func TestTheBoundsAreTheOnesTheMovementWasTunedFor(t *testing.T) {
	if minFlingVelocity != unit.Dp(50) {
		t.Errorf("the floor is %v dp per second and 50 was expected", float32(minFlingVelocity))
	}
	if maxFlingVelocity != unit.Dp(8000) {
		t.Errorf("the ceiling is %v dp per second and 8000 was expected", float32(maxFlingVelocity))
	}
	if thresholdVelocity != 1 {
		t.Errorf("the movement ends below %v pixels per second and 1 was expected", float32(thresholdVelocity))
	}
}

// A release slower than the floor starts nothing.
//
// Below it the hand was coming to rest, and carrying the movement on turns
// letting go into a nudge. The floor is inclusive: a release exactly at it is
// still a release that stops.
func TestAReleaseBelowTheFloorStartsNothing(t *testing.T) {
	var c unit.Metric
	floor := float32(c.Dp(minFlingVelocity))

	for _, velocity := range []float32{0, 1, floor / 2, floor, -floor, -floor / 2, -1} {
		var f Animation
		if f.Start(c, time.Now(), velocity) {
			t.Errorf("a release at %v per second started a fling", velocity)
		}
		if f.Active() {
			t.Errorf("a release at %v per second left the animation running", velocity)
		}
		if got := f.Tick(time.Now().Add(frame)); got != 0 {
			t.Errorf("a release at %v per second moved %d pixels", velocity, got)
		}
	}

	var f Animation
	if !f.Start(c, time.Now(), floor+1) {
		t.Errorf("a release at %v per second, just past the floor, started nothing", floor+1)
	}
}

// The floor is in device independent pixels, so it is the same distance on
// every display.
//
// A floor read as raw pixels would let go at half the speed on a dense screen,
// which is the kind of fault nobody reports and everybody feels.
func TestTheFloorFollowsTheDisplay(t *testing.T) {
	const velocity = 75

	sparse := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	dense := unit.Metric{PxPerDp: 2, PxPerSp: 2}

	var loose Animation
	if !loose.Start(sparse, time.Now(), velocity) {
		t.Errorf("%v pixels per second did not clear a floor of %v", float32(velocity), float32(minFlingVelocity))
	}

	var tight Animation
	if tight.Start(dense, time.Now(), velocity) {
		t.Errorf("%v pixels per second cleared a floor of %v dp on a display of two pixels to the dp",
			float32(velocity), float32(minFlingVelocity))
	}
}

// The zero value is a fling that is not running.
func TestAnUnstartedAnimationDoesNotMove(t *testing.T) {
	var f Animation
	if f.Active() {
		t.Error("an unstarted animation reports itself running")
	}
	if got := f.Tick(time.Now()); got != 0 {
		t.Errorf("an unstarted animation moved %d pixels", got)
	}
}

// The movement only ever slows, and it stops.
//
// Three things at once, because they are one property: the distance handed out
// each frame never grows, it never changes direction, and after a bounded
// number of frames there is nothing left to hand out. A curve that failed the
// last of these would ask the window for a new frame forever, at one pixel
// every few seconds.
func TestTheMovementSlowsAndStops(t *testing.T) {
	var c unit.Metric
	var f Animation
	at := time.Now()
	if !f.Start(c, at, 3000) {
		t.Fatal("a release at 3000 pixels per second started nothing")
	}

	previous, second := 0, false
	frames := 0
	for f.Active() {
		at = at.Add(frame)
		got := f.Tick(at)

		if got < 0 {
			t.Fatalf("frame %d of a fling thrown forwards moved %d pixels backwards", frames, -got)
		}
		// Within one pixel, because the distance is handed out in whole
		// pixels and the fraction left over is paid on a later frame. The
		// carry can put one extra pixel on a frame; it cannot put two.
		if second && got > previous+1 {
			t.Fatalf("frame %d moved %d pixels after a frame that moved %d, and the movement only slows",
				frames, got, previous)
		}
		previous, second = got, true

		frames++
		if frames > 10000 {
			t.Fatal("the fling was still running after ten thousand frames, which is not a stop")
		}
	}

	if got := f.Tick(at.Add(frame)); got != 0 {
		t.Errorf("a finished fling moved %d more pixels", got)
	}
}

// The whole distance is the one the curve promises, which is what pins the
// drag.
//
// A mass held back by a drag proportional to its own speed travels v0/|k| in
// total, and stopping it at the threshold leaves less than 1/|k| of that
// unspent. So the distance is the release divided by the drag, to within a
// pixel or two -- and it is the only place the drag is visible from outside.
// A coefficient changed by hand gives a fling that reads as plausible frame by
// frame and comes to rest in the wrong place, which is the fault this test
// exists for: at the two values in use, the same release travels 1500 pixels
// or 714.
func TestTheWholeDistanceIsWhatTheCurvePromises(t *testing.T) {
	const released = 3000

	var c unit.Metric
	var f Animation
	at := time.Now()
	if !f.Start(c, at, released) {
		t.Fatal("a release at 3000 pixels per second started nothing")
	}

	total := 0
	for f.Active() {
		at = at.Add(frame)
		total += f.Tick(at)
	}

	want := int(released / -tunedDecay())
	if total > want || total < want-2 {
		t.Errorf("a release at %v per second travelled %d pixels and the curve promises %d",
			float32(released), total, want)
	}
}

// A release past the ceiling travels exactly as far as one at it.
//
// The estimate is a fit over a handful of samples, and one stray pair of them
// reports a speed no finger produced. Unclamped it throws the content out of
// reach in a single frame, and the list arrives somewhere nobody asked for.
func TestAReleasePastTheCeilingTravelsNoFurther(t *testing.T) {
	var c unit.Metric
	ceiling := float32(c.Dp(maxFlingVelocity))

	if got, want := travel(t, c, 100*ceiling), travel(t, c, ceiling); got != want {
		t.Errorf("a release at a hundred times the ceiling travelled %d pixels and one at the ceiling travelled %d",
			got, want)
	}
}

// A fling thrown backwards is the mirror of one thrown forwards.
func TestAFlingThrownBackwardsIsTheMirrorOfOneThrownForwards(t *testing.T) {
	var c unit.Metric

	forwards := travel(t, c, 2500)
	backwards := travel(t, c, -2500)

	if forwards <= 0 {
		t.Fatalf("a fling thrown forwards travelled %d pixels", forwards)
	}
	if backwards != -forwards {
		t.Errorf("a fling thrown backwards travelled %d pixels and %d was expected", backwards, -forwards)
	}
}

// travel runs a fling to a standstill and answers how far it went.
func travel(t *testing.T, c unit.Metric, velocity float32) int {
	t.Helper()

	var f Animation
	at := time.Now()
	if !f.Start(c, at, velocity) {
		t.Fatalf("a release at %v per second started nothing", velocity)
	}

	total := 0
	for f.Active() {
		at = at.Add(frame)
		total += f.Tick(at)
	}
	return total
}

// tunedDecay is the drag coefficient the movement is tuned to, per second.
//
// The two values are written out here rather than read from the package,
// because this is the test that guards them. Which one applies is a property of
// where the binary is running, and that is read from the runtime for the same
// reason the package reads it.
func tunedDecay() float32 {
	if runtime.GOOS == "darwin" {
		return -2
	}
	return -4.2
}
