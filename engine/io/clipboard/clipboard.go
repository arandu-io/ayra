// Package clipboard puts something on the system clipboard, and asks for what
// is already on it.
//
// The two directions are not symmetric, and that asymmetry is the whole of what
// this package has to teach. Writing is handed over and finished with. Reading
// is a question put to the operating system, and the answer returns the way
// every other input does: in a later frame, addressed to a tag. Nothing here
// returns the clipboard's contents, because there is no point during a frame at
// which they are known.
//
// Both types are commands. They are given to Execute, which records them for
// the frame and answers nothing.
package clipboard

import (
	"io"

	"github.com/arandu-io/ayra/engine/io/event"
)

// WriteCmd puts Data on the clipboard.
//
// A clipboard holds one thing, so the last writer of a frame is the only one
// that reaches the platform. Two of these executed while drawing the same frame
// leave the second on the clipboard and the first nowhere, with no error on
// either -- which is worth knowing before writing a control that copies on
// every press and one that copies on selection into the same screen.
type WriteCmd struct {
	// Type is the MIME type of Data.
	Type string

	// Data is the content to copy.
	//
	// It is drained to the end and closed by the runtime as the command is
	// executed. The caller neither closes it nor reads from it afterwards: it
	// has already been read, and a second reader finds it empty rather than
	// finding an error.
	Data io.ReadCloser
}

// ReadCmd asks for the clipboard's contents. The answer is an event, not a
// return value.
//
// Executing it starts a question and does nothing else. No field of the command
// is filled in afterwards, no call anywhere hands back the text, and the frame
// that asked carries on and finishes without it. What arrives is a
// [github.com/arandu-io/ayra/engine/io/transfer.DataEvent] addressed to Tag, in
// some later frame -- after the platform has answered, which on a system that
// asks the person for permission first is however long that takes.
//
// So the handler has to be standing there to catch it: Tag declared with
// [event.Op], and a transfer.TargetFilter naming that tag asked for on every
// frame, not only on the frame that executed the command. A handler that
// filters once, in the frame it asked, is listening at the one moment the
// answer cannot arrive.
//
// Code written as though the value came back compiles, because there is no
// value to fail to assign and nothing in the signature that says so. It pastes
// nothing, and it does that at run time on somebody else's machine.
//
// One command buys one answer. The request stands until it is answered, and
// asking again with the same tag while it stands adds nothing -- the platform is
// asked once. A handler that wants the clipboard a second time executes a second
// command. Every handler waiting when the answer arrives receives that same
// answer.
type ReadCmd struct {
	// Tag is the handler the answer is addressed to, and is the tag that
	// handler declared with [event.Op].
	Tag event.Tag
}

func (WriteCmd) ImplementsCommand() {}
func (ReadCmd) ImplementsCommand()  {}
