package system

import (
	"strings"

	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/op"
)

// Action is a set of window decoration actions.
//
// It is a set rather than a single value because the answers come together: a
// titlebar reports everything that was pressed in one frame, and a window is
// told what it is allowed to do as everything at once.
type Action uint

const (
	// ActionMinimize minimizes a window.
	ActionMinimize Action = 1 << iota
	// ActionMaximize maximizes a window.
	ActionMaximize
	// ActionUnmaximize restores a maximized window.
	ActionUnmaximize
	// ActionFullscreen makes a window fullscreen.
	ActionFullscreen
	// ActionRaise asks the platform to bring this window above all others.
	// Platforms allow it only under their own conditions, such as when a
	// window of the same application already has focus, and one that does not
	// allow it does nothing.
	ActionRaise
	// ActionCenter centers the window on the screen. It is ignored in
	// fullscreen mode and on platforms that do not let a window place itself.
	ActionCenter
	// ActionClose closes a window.
	ActionClose
	// ActionMove moves a window, directed by the user.
	ActionMove
)

// ActionInputOp marks the current clip area as the part of the window that
// stands for an action.
//
// It is what turns a drawn titlebar into one. The application paints the bar
// and then says, area by area, which part of it the platform should treat as
// the window itself -- and without that the bar is a picture of a titlebar on a
// window nobody can drag.
//
// Only ActionMove is acted on here. A platform that finds it under a press
// hands the drag to the window manager, which is the one action an application
// cannot carry out for itself. The others reach the application instead, and it
// is the application that acts on them.
type ActionInputOp Action

// Add records the op.
//
// The action travels as a single byte, which is what bounds the set at the
// eight declared here. A ninth would be written as nought, and the area would
// answer to nothing: a titlebar mark that draws, presses, and does not act,
// which reads as a fault in the window rather than in the set.
func (a ActionInputOp) Add(o *op.Ops) {
	data := ops.Write(&o.Internal, ops.TypeActionInputLen)
	data[0] = byte(ops.TypeActionInput)
	data[1] = byte(a)
}

// String names every action in the set, separated by a bar.
func (a Action) String() string {
	var named strings.Builder
	for bit := Action(1); a != 0 && bit != 0; bit <<= 1 {
		if a&bit == 0 {
			continue
		}
		a &^= bit

		name := bit.name()
		if name == "" {
			// A bit this set does not declare, passed over rather than
			// written as its empty name: written, it leaves a separator with
			// nothing after it, and the line then reads as a set whose last
			// member has no name rather than one carrying a member nobody
			// declared.
			continue
		}
		if named.Len() > 0 {
			named.WriteByte('|')
		}
		named.WriteString(name)
	}
	return named.String()
}

// name is what a single action is called, and is empty for anything else.
func (a Action) name() string {
	switch a {
	case ActionMinimize:
		return "ActionMinimize"
	case ActionMaximize:
		return "ActionMaximize"
	case ActionUnmaximize:
		return "ActionUnmaximize"
	case ActionFullscreen:
		return "ActionFullscreen"
	case ActionRaise:
		return "ActionRaise"
	case ActionCenter:
		return "ActionCenter"
	case ActionClose:
		return "ActionClose"
	case ActionMove:
		return "ActionMove"
	}
	return ""
}
