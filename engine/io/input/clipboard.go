package input

import (
	"io"
	"slices"

	"github.com/arandu-io/ayra/engine/io/clipboard"
	"github.com/arandu-io/ayra/engine/io/event"
)

// clipboardState is who is waiting for the clipboard, and it outlives frames.
//
// It has to. Reading the clipboard is a question put to the platform, and the
// answer comes back whenever the platform gets round to it -- after a
// permission prompt, on a system that asks -- which is never the frame that
// asked. The frame that asked has finished by then; what has to survive it is
// the list of handlers the answer is owed to.
type clipboardState struct {
	receivers []event.Tag
}

// clipboardQueue is the part that does not outlive the frame: what to hand the
// platform now, and whether it has to be asked anything.
type clipboardQueue struct {
	// requested is set when a handler joins the list of receivers, and cleared
	// as soon as the platform has been asked. Without it the platform would be
	// asked again on every frame for as long as the answer took to arrive --
	// once a frame, sixty times a second, for a question already outstanding.
	requested bool
	mime      string
	text      []byte
}

// WriteClipboard hands over the content waiting to be copied, if any, and gives
// it up in the same call.
//
// Taken rather than read: the caller is the one putting it on the platform's
// clipboard, and content left behind would be written again on the next frame
// that happened to ask.
func (q *clipboardQueue) WriteClipboard() (mime string, content []byte, ok bool) {
	if q.text == nil {
		return "", nil, false
	}
	content = q.text
	q.text = nil
	return q.mime, content, true
}

// ClipboardRequested reports whether the platform has to be asked for the
// clipboard, and answers yes only once per question.
//
// Both halves matter. There has to be somebody waiting for the answer, and the
// question has to be new: a handler still waiting through a dozen frames is one
// question outstanding, not a dozen.
func (q *clipboardQueue) ClipboardRequested(state clipboardState) bool {
	req := len(state.receivers) > 0 && q.requested
	q.requested = false
	return req
}

// Push delivers the platform's answer to everyone waiting for it, and empties
// the list.
//
// One answer serves every waiting handler: they all asked the same question of
// the same clipboard, and it was only asked once. Emptying the list is what
// makes the next request a new question rather than a second delivery of this
// one.
func (q *clipboardQueue) Push(state clipboardState, e event.Event) (clipboardState, []taggedEvent) {
	var evts []taggedEvent
	for _, r := range state.receivers {
		evts = append(evts, taggedEvent{tag: r, event: e})
	}
	state.receivers = nil
	return state, evts
}

// ProcessWriteClipboard drains the content and closes it, here and now.
//
// It cannot be kept and read later: the command is executed while a frame is
// being drawn, and whatever is behind the reader -- a file, a buffer a control
// is about to reuse -- belongs to the caller, who has been told it is finished
// with when Execute returns. A read that failed leaves the clipboard alone
// rather than putting half a document on it.
func (q *clipboardQueue) ProcessWriteClipboard(req clipboard.WriteCmd) {
	defer req.Data.Close()
	content, err := io.ReadAll(req.Data)
	if err != nil {
		return
	}
	q.mime = req.Type
	q.text = content
}

// ProcessReadClipboard enrols tag among those waiting for the clipboard.
//
// A tag already waiting is not enrolled twice and does not ask the platform
// again: one command buys one answer, and the handler that asks a second time
// while its first question stands would otherwise be handed the same answer
// twice over.
func (q *clipboardQueue) ProcessReadClipboard(state clipboardState, tag event.Tag) clipboardState {
	if slices.Contains(state.receivers, tag) {
		return state
	}
	// Appended to a slice capped at its own length, so that the new list is a
	// new array. The old one is held by the states of frames already recorded,
	// and writing into it would change what they say happened.
	n := len(state.receivers)
	state.receivers = append(state.receivers[:n:n], tag)
	q.requested = true
	return state
}
