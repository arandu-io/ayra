package client

import (
	"net/http"
	"sync"
)

// csrfHeader is where a request carries the token.
//
// The header and not the form, although the server reads either. The form
// belongs to the caller: adding a field to it would mean this editing values
// somebody else assembled, and a caller that had already put the token in would
// then send it twice with nothing saying which one counts. A header is the
// client's own and there is one of it.
const csrfHeader = "X-CSRF-Token"

// safeMethods are the ones a server does not ask a token for.
//
// The same four the protocol calls safe, and the same four the server's own
// check lets through. Sending a token on a GET would not be wrong, only
// pointless -- and the point of listing them is that a header sent on every
// request is a token in every access log.
var safeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
}

// tokens remembers the CSRF token a server last sent.
//
// Remembered rather than read from the page being acted on, because the page
// that carries a token is not always the page before the request. A screen
// signed in to hours ago is still the session the token belongs to, and a form
// posted from the third screen of a flow is posted under the token the first
// one arrived with.
type tokens struct {
	mu    sync.RWMutex
	value string
}

// remember keeps the token a page arrived with, if it arrived with one.
//
// A page with no token leaves what is held alone. The server sends one on the
// pages that were going to render a form and not on the others, and a page
// without one is not the session ending -- it is a page with no form on it.
func (t *tokens) remember(token string) {
	if token == "" {
		return
	}

	t.mu.Lock()
	t.value = token
	t.mu.Unlock()
}

// forget drops the token, which is what the end of a session means here.
func (t *tokens) forget() {
	t.mu.Lock()
	t.value = ""
	t.mu.Unlock()
}

// carry puts the token on a request that needs one.
func (t *tokens) carry(req *http.Request) {
	if safeMethods[req.Method] {
		return
	}

	t.mu.RLock()
	value := t.value
	t.mu.RUnlock()

	if value == "" {
		// Nothing to send. The refusal that follows is the server's, and it
		// says what is missing -- which is a better answer than one invented
		// here about a token this has never been given.
		return
	}
	req.Header.Set(csrfHeader, value)
}
