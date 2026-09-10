package client

import "testing"

// TestEveryClientLeavesHereWithATimeout is the check the external tests cannot
// make.
//
// They can prove that a transport somebody supplied gets one, because they
// supply it and can read it back. The client built for a caller who supplied
// nothing is inside, and it is the one every application gets: a hang there is
// a screen stuck on "Signing in..." with its controls disabled and no way out
// but to kill the application.
func TestEveryClientLeavesHereWithATimeout(t *testing.T) {
	c, err := New("https://example.test")
	if err != nil {
		t.Fatal(err)
	}

	if c.http.Timeout == 0 {
		t.Fatal("the default client waits forever")
	}
	if c.http.Timeout != requestTimeout {
		t.Errorf("the default client waits %s, want %s", c.http.Timeout, requestTimeout)
	}
}

// TestTheTimeoutIsLongEnoughToBeAnAnswerAndShortEnoughToBeOne keeps the number
// from drifting to either end.
//
// Too short turns a slow connection into a failure on a working network. Too
// long is the hang it exists to end, arriving later.
func TestTheTimeoutIsLongEnoughToBeAnAnswerAndShortEnoughToBeOne(t *testing.T) {
	if requestTimeout.Seconds() < 5 {
		t.Errorf("the timeout is %s, which fails on a slow connection that would have answered", requestTimeout)
	}
	if requestTimeout.Minutes() > 2 {
		t.Errorf("the timeout is %s, which is a hang with a longer fuse", requestTimeout)
	}
}
