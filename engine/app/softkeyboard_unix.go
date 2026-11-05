//go:build (linux && !android) || freebsd || openbsd

package app

import (
	goctx "context"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// A Linux or BSD desktop has no protocol that makes a window ask for the
// on-screen keyboard: X11 has none at all, and Wayland's text-input-v3 reaches
// only a compositor that runs an input method. A kiosk or a car kit that runs
// an on-screen keyboard of its own (Onboard, squeekboard, wvkbd) says how to
// show and hide it with two commands in the environment, and the window runs
// them when a field asks for the keyboard and when the focus leaves it:
//
//	AYRA_SOFT_KEYBOARD_SHOW   run by /bin/sh -c when a text field takes the focus
//	AYRA_SOFT_KEYBOARD_HIDE   run by /bin/sh -c when no text field has it
//
// With neither set nothing happens, as before.
const (
	envSoftKeyboardShow = "AYRA_SOFT_KEYBOARD_SHOW"
	envSoftKeyboardHide = "AYRA_SOFT_KEYBOARD_HIDE"

	// softKeyboardDelay is how long a request waits for the next one. Moving the
	// focus from one field to another closes the keyboard and opens it again in
	// the same breath, and the person must not see it leave and come back.
	softKeyboardDelay = 120 * time.Millisecond

	// softKeyboardTimeout ends a command that hangs: it runs beside the window,
	// and a keyboard that never answers must not pile up processes.
	softKeyboardTimeout = 5 * time.Second
)

// softKeyboard runs the two commands. It never blocks the caller: set only
// records the wish and arms a timer, and the command runs on another goroutine.
type softKeyboard struct {
	show, hide string
	delay      time.Duration
	run        func(command string)

	mu      sync.Mutex
	wanted  bool
	shown   bool // the state the last command left the keyboard in
	timer   *time.Timer
	nothing bool
}

// systemSoftKeyboard is the window's: the commands of the environment, run by
// the shell.
var systemSoftKeyboard = newSoftKeyboard(os.Getenv, runShell, softKeyboardDelay)

func newSoftKeyboard(getenv func(string) string, run func(string), delay time.Duration) *softKeyboard {
	k := &softKeyboard{
		show:  getenv(envSoftKeyboardShow),
		hide:  getenv(envSoftKeyboardHide),
		delay: delay,
		run:   run,
	}
	k.nothing = k.show == "" && k.hide == ""
	return k
}

// set says whether a field wants the keyboard. Every request waits for the
// delay, and the last one made within it is the one that counts, so a hide
// followed by a show runs only the show. A show is always run, also when the
// keyboard is already up as far as the window knows: a tap on a field asks again,
// and the keyboard may have been closed by its own key since; showing a keyboard
// that is up changes nothing. A hide is run only when a show ran before it.
func (k *softKeyboard) set(show bool) {
	if k.nothing {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.wanted = show
	if k.timer != nil {
		k.timer.Stop()
		k.timer = nil
	}
	if !show && !k.shown {
		return
	}
	k.timer = time.AfterFunc(k.delay, k.fire)
}

// fire runs the command for what is wanted now.
func (k *softKeyboard) fire() {
	k.mu.Lock()
	k.timer = nil
	show := k.wanted
	if !show && !k.shown {
		k.mu.Unlock()
		return
	}
	k.shown = show
	command := k.hide
	if show {
		command = k.show
	}
	k.mu.Unlock()
	if command != "" {
		k.run(command)
	}
}

// runShell runs a command with /bin/sh -c and waits for it, so that two
// requests never run at once. The caller is a timer's goroutine, never the
// window's.
func runShell(command string) {
	ctx, cancel := goctx.WithTimeout(goctx.Background(), softKeyboardTimeout)
	defer cancel()
	if err := exec.CommandContext(ctx, "/bin/sh", "-c", command).Run(); err != nil {
		warnSoftKeyboardOnce(err)
	}
}

var softKeyboardWarned sync.Once

func warnSoftKeyboardOnce(err error) {
	softKeyboardWarned.Do(func() { log.Printf("ayra: the soft keyboard command failed: %v", err) })
}
