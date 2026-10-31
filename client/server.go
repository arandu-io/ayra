package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ErrOffServer is what a request answers when it would have reached an address
// other than the server the client was made for.
//
// Everything a client carries belongs to that one server: the session, the
// CSRF token, and the body of a form that may hold a password. A path that is
// itself an address, or a redirect to another host, another port or another
// scheme, would hand all three to somebody else -- and an answer from there
// would be drawn, and its token sent back, as if the server had written it.
// So the request is refused before it leaves, and an answer that arrived from
// anywhere else is refused before it is read.
//
// A redirect that stays on the server is followed as before. Moving from https
// to http on the same host is a change of scheme and is refused like any other:
// it is the same server reached over a connection anybody on the path can read.
var ErrOffServer = errors.New("ayra/client: the request would leave the server it was made for")

// serverKey is the context key a request carries its client's base under, so
// the redirect check can tell which server the request belongs to.
type serverKey struct{}

// onServer returns ctx carrying base as the server its requests belong to.
func onServer(ctx context.Context, base *url.URL) context.Context {
	return context.WithValue(ctx, serverKey{}, base)
}

// stayOnServer wraps a transport's redirect policy with the refusal of any
// redirect that leaves the server a request belongs to.
//
// The server is read from the request's context rather than captured here, so
// a transport handed to two clients checks each request against its own
// client, and wrapping the same transport twice asks the same question twice
// rather than two different ones. A request that carries no server -- one the
// caller built and sent through the same transport -- is left to the policy
// that was already there.
func stayOnServer(next func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if base, ok := req.Context().Value(serverKey{}).(*url.URL); ok && !sameServer(base, req.URL) {
			return fmt.Errorf("%w: redirected to %s://%s", ErrOffServer, req.URL.Scheme, req.URL.Host)
		}
		if next != nil {
			return next(req, via)
		}
		// The limit the standard client applies when it has no policy, kept
		// because setting one replaces it.
		if len(via) >= maxRedirects {
			return fmt.Errorf("ayra/client: stopped after %d redirects", maxRedirects)
		}
		return nil
	}
}

// maxRedirects is how many redirects one request follows.
const maxRedirects = 10

// sameServer reports whether u addresses the server at base: the same scheme,
// the same host and the same port.
//
// The port is compared as it is meant rather than as it is written, so an
// address that spells out the scheme's own port is the same server as one
// that leaves it out.
func sameServer(base, u *url.URL) bool {
	if u == nil {
		return false
	}
	return strings.EqualFold(base.Scheme, u.Scheme) &&
		strings.EqualFold(base.Hostname(), u.Hostname()) &&
		port(base) == port(u)
}

// port answers the port u reaches, naming the scheme's own when u has none.
func port(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	}
	return ""
}
