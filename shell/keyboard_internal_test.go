package shell

import "testing"

// TestTheKeyboardIsTheBottomInsetBeyondTheBars: the navigation bar is not a
// keyboard, and the keyboard is the inset that grows past it.
func TestTheKeyboardIsTheBottomInsetBeyondTheBars(t *testing.T) {
	var k keyboardWatch
	if got := k.observe(48); got != 0 {
		t.Errorf("a navigation bar of 48 points is a keyboard of %v", got)
	}
	if got := k.observe(48 + 280); got != 280 {
		t.Errorf("a keyboard of 280 over a bar of 48 is %v points", got)
	}
	if got := k.observe(48 + 60); got != 0 {
		t.Errorf("a suggestion strip of 60 points is a keyboard of %v", got)
	}
	if got := k.observe(48); got != 0 {
		t.Errorf("the keyboard is still up after it went: %v", got)
	}
}

// TestAWindowThatOpensWithTheKeyboardUpLearnsTheBarsWhenItGoes: until the bottom
// inset has been lower there is nothing to measure it against.
func TestAWindowThatOpensWithTheKeyboardUpLearnsTheBarsWhenItGoes(t *testing.T) {
	var k keyboardWatch
	if got := k.observe(328); got != 0 {
		t.Errorf("the first inset seen is a keyboard of %v", got)
	}
	k.observe(48)
	if got := k.observe(328); got != 280 {
		t.Errorf("the keyboard, once the bars are known, is %v, want 280", got)
	}
}
