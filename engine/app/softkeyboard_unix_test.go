//go:build (linux && !android) || freebsd || openbsd

package app

import (
	"sync"
	"testing"
	"time"
)

// recorder is a runner that writes down what it was asked to run.
type recorder struct {
	mu   sync.Mutex
	ran  []string
	seen chan string
}

func newRecorder() *recorder { return &recorder{seen: make(chan string, 16)} }

func (r *recorder) run(command string) {
	r.mu.Lock()
	r.ran = append(r.ran, command)
	r.mu.Unlock()
	r.seen <- command
}

func (r *recorder) commands() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.ran...)
}

func env(show, hide string) func(string) string {
	return func(k string) string {
		switch k {
		case envSoftKeyboardShow:
			return show
		case envSoftKeyboardHide:
			return hide
		}
		return ""
	}
}

const testDelay = 20 * time.Millisecond

func expect(t *testing.T, r *recorder, want string) {
	t.Helper()
	select {
	case got := <-r.seen:
		if got != want {
			t.Fatalf("ran %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("nothing ran, want %q", want)
	}
}

func TestEachStateRunsItsOwnCommand(t *testing.T) {
	r := newRecorder()
	k := newSoftKeyboard(env("kb show", "kb hide"), r.run, testDelay)
	k.set(true)
	expect(t, r, "kb show")
	k.set(false)
	expect(t, r, "kb hide")
}

func TestNothingRunsWithoutTheVariables(t *testing.T) {
	r := newRecorder()
	k := newSoftKeyboard(env("", ""), r.run, testDelay)
	k.set(true)
	k.set(false)
	time.Sleep(10 * testDelay)
	if got := r.commands(); len(got) != 0 {
		t.Fatalf("ran %v with the variables unset", got)
	}
}

func TestOneCommandAloneIsEnough(t *testing.T) {
	r := newRecorder()
	k := newSoftKeyboard(env("kb show", ""), r.run, testDelay)
	k.set(true)
	expect(t, r, "kb show")
	k.set(false) // no hide command: nothing to run, and nothing breaks
	time.Sleep(10 * testDelay)
	if got := r.commands(); len(got) != 1 {
		t.Fatalf("ran %v", got)
	}
}

func TestARepeatedStateRunsOnce(t *testing.T) {
	r := newRecorder()
	k := newSoftKeyboard(env("kb show", "kb hide"), r.run, testDelay)
	for i := 0; i < 5; i++ {
		k.set(true)
	}
	expect(t, r, "kb show")
	k.set(true)
	k.set(true)
	time.Sleep(10 * testDelay)
	if got := r.commands(); len(got) != 1 {
		t.Fatalf("ran %v, a keyboard that is already up is not shown again", got)
	}
}

func TestHidingWhileHiddenRunsNothing(t *testing.T) {
	r := newRecorder()
	k := newSoftKeyboard(env("kb show", "kb hide"), r.run, testDelay)
	k.set(false)
	time.Sleep(10 * testDelay)
	if got := r.commands(); len(got) != 0 {
		t.Fatalf("ran %v: the keyboard starts hidden", got)
	}
}

func TestAHideFollowedByAShowInTheDelayLeavesTheKeyboardAlone(t *testing.T) {
	r := newRecorder()
	k := newSoftKeyboard(env("kb show", "kb hide"), r.run, 80*time.Millisecond)
	k.set(true)
	expect(t, r, "kb show")
	// The focus moves from one field to the next: close, then open.
	k.set(false)
	k.set(true)
	time.Sleep(400 * time.Millisecond)
	if got := r.commands(); len(got) != 1 {
		t.Fatalf("ran %v: the keyboard flickered", got)
	}
}

func TestAShowFollowedByAHideInTheDelayRunsNothing(t *testing.T) {
	r := newRecorder()
	k := newSoftKeyboard(env("kb show", "kb hide"), r.run, 80*time.Millisecond)
	k.set(true)
	k.set(false)
	time.Sleep(400 * time.Millisecond)
	if got := r.commands(); len(got) != 0 {
		t.Fatalf("ran %v", got)
	}
}

func TestSetNeverWaitsForTheCommand(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	k := newSoftKeyboard(env("slow", "kb hide"), func(string) { started <- struct{}{}; <-release }, testDelay)
	done := make(chan struct{})
	go func() { k.set(true); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("set waited")
	}
	<-started
	finished := make(chan struct{})
	go func() { k.set(false); close(finished) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("set waited for a command that is still running")
	}
	close(release)
}

func TestTheShellRunsARealCommand(t *testing.T) {
	// runShell must not panic or hang on a failing command, and warns once.
	runShell("exit 3")
	runShell("true")
}
