// Package push is the device's half of a push notification: the permission to
// show one, the token the server addresses the device by, the message that
// arrives while the application is open, and the notification a person touched.
//
// It is the half that runs on a phone and nothing else. The server that decides
// whom to notify, and sends it, is a different program; what this package hands
// it is the token, and what it hands the application is what arrived. There is
// no queue here that outlives the process, no store of messages and no rule
// about who may be notified of what: the server decides, as it does for every
// other thing a native application shows.
//
// The same calls work on Android and on iOS. On Android the token is a
// Firebase Cloud Messaging registration token. On iOS it is the APNs device
// token, as a hexadecimal string, and [Kind] says so, because a server sending
// through Firebase has to map an APNs token to a registration token before it
// can address it. Where the platform has no push -- the desktop targets and the
// browser -- every call answers [ErrUnsupported] or nothing, and an application
// asks [Supported] before it draws a switch for notifications.
//
// A message the application did not open the app for arrives on [Messages]; one
// that opened it -- the person touched the notification -- arrives on [Opened],
// carrying the link to go to. Both channels hold what arrived before anybody was
// reading, which is how the notification that started the application is not
// lost to the time it takes a window to open. They hold a few, and when they
// are full the oldest is dropped: a reader that is not keeping up is not owed
// every message, and the bridge that delivers them must never wait on a screen.
package push

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Errors the package answers with.
var (
	// ErrUnsupported is a platform with no push: the desktop targets and the
	// browser.
	ErrUnsupported = errors.New("push: this platform has no push notifications")
	// ErrNotConfigured is an application packaged without the push service's
	// configuration -- for Android, no google-services.json -- so there is
	// nothing to register with.
	ErrNotConfigured = errors.New("push: the application was packaged without a push configuration")
)

// Permission is whether the application may show notifications.
type Permission int

const (
	// Undetermined is a person who has not been asked.
	Undetermined Permission = iota
	// Granted is a person who allowed notifications.
	Granted
	// Denied is a person who refused, or turned them off in the system's
	// settings. The system will not ask again, so an application that needs
	// them tells the person where to switch them on.
	Denied
)

// String names the permission for a diagnostic.
func (p Permission) String() string {
	switch p {
	case Granted:
		return "granted"
	case Denied:
		return "denied"
	}
	return "undetermined"
}

// Message is a push notification, as the device received it.
type Message struct {
	// ID is the service's identifier for the message, empty when it sent none.
	ID string
	// Title and Body are what the notification says. A notification touched in
	// the system tray while the application was closed reaches the application
	// without them on Android, because the system drew it from the payload and
	// handed only the data over; the server puts what the screen needs in Data.
	Title, Body string
	// Link is where touching the notification goes: the value of the "link" key
	// of Data, or the address of the intent that opened the application. It is
	// the application's own deep link, and the application decides what to do
	// with it -- the server decides what the person may see once they get there.
	Link string
	// Data is the key-value payload the server attached.
	Data map[string]string
	// Sent is when the service says the message was sent, zero when unknown.
	Sent time.Time
}

// queueSize is how many messages or tokens a channel holds for a reader that is
// not there yet.
const queueSize = 32

// Kind says what [Token] is: "fcm" for a Firebase registration token and "apns"
// for an Apple device token. It is empty where the platform has no push.
func Kind() string { return current.kind() }

// Supported reports whether the platform can receive push at all.
func Supported() bool { return current.kind() != "" }

// Request asks for the permission to show notifications, and starts
// registering the device.
//
// It shows the system's own dialog the first time, on the platforms that have
// one, and answers what the person chose; afterwards it answers the standing
// decision without asking. Calling it is also what makes a token appear, so it
// belongs where the application explains why it wants notifications -- after the
// person signs in, rather than at the first frame, where a refusal is a refusal
// of a question nobody understood.
//
// The context bounds the wait for the person. A cancelled context answers its
// error and leaves the dialog where the system put it.
func Request(ctx context.Context) (Permission, error) {
	current.ensure()
	return current.request(ctx)
}

// Token answers the current token, or an empty string when there is none yet:
// the permission was not granted, the service has not answered, or the platform
// has no push.
func Token() string {
	current.ensure()
	return hub.currentToken()
}

// Tokens delivers the token when it is first known and every time it changes.
// The service rotates tokens, and a server that kept the old one sends to
// nobody. A token that arrived before the call is delivered on the first read.
func Tokens() <-chan string {
	current.ensure()
	return hub.tokens.ch
}

// Messages delivers the notifications that arrived while the application was in
// front of the person. The system shows nothing for these; drawing something is
// the application's choice.
func Messages() <-chan Message {
	current.ensure()
	return hub.messages.ch
}

// Opened delivers the notification a person touched, which is what opened the
// application or brought it forward. It carries the link to go to.
func Opened() <-chan Message {
	current.ensure()
	return hub.opened.ch
}

// backend is the platform half: what the system is asked, and nothing that is
// the same on every platform.
type backend interface {
	// kind is "fcm", "apns" or "".
	kind() string
	// request asks for permission and starts registration.
	request(ctx context.Context) (Permission, error)
	// ensure connects the platform's half to this one, once. It is called
	// before anything is read, because a notification can be waiting for the
	// application before any of its own code has run, and nothing delivers it
	// until the platform is asked to.
	ensure()
}

// current is the platform's backend, set once by the file for the platform.
var current backend = unsupported{}

// hub is where the platform delivers what it received. It is the single place
// that decides how a slow reader is treated.
var hub = newHub()

type hubState struct {
	mu       sync.Mutex
	token    string
	tokens   *queue[string]
	messages *queue[Message]
	opened   *queue[Message]
}

func newHub() *hubState {
	return &hubState{
		tokens:   newQueue[string](),
		messages: newQueue[Message](),
		opened:   newQueue[Message](),
	}
}

func (h *hubState) currentToken() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.token
}

// deliverToken is called by the platform when the token is known or changes.
func (h *hubState) deliverToken(token string) {
	if token == "" {
		return
	}
	h.mu.Lock()
	same := h.token == token
	h.token = token
	h.mu.Unlock()
	if !same {
		h.tokens.put(token)
	}
}

// deliverMessage is called by the platform with a notification that arrived
// while the application was in front.
func (h *hubState) deliverMessage(m Message) { h.messages.put(normalise(m)) }

// deliverOpened is called by the platform with a notification that was touched.
func (h *hubState) deliverOpened(m Message) { h.opened.put(normalise(m)) }

// normalise settles where the link of a message is: its own field, or the
// "link" the server put in the data.
func normalise(m Message) Message {
	if m.Link == "" {
		m.Link = m.Data["link"]
	}
	return m
}

// queue is a channel that never blocks the one writing to it: when it is full
// the oldest entry is dropped to make room.
type queue[T any] struct {
	mu sync.Mutex
	ch chan T
}

func newQueue[T any]() *queue[T] { return &queue[T]{ch: make(chan T, queueSize)} }

func (q *queue[T]) put(v T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for {
		select {
		case q.ch <- v:
			return
		default:
		}
		select {
		case <-q.ch:
		default:
		}
	}
}

// unsupported is the backend of a platform with no push.
type unsupported struct{}

func (unsupported) kind() string { return "" }

func (unsupported) ensure() {}

func (unsupported) request(context.Context) (Permission, error) {
	return Undetermined, ErrUnsupported
}
