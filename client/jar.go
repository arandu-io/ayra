package client

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"
)

// jar is where a client keeps its session.
//
// A client without one signs in and is anonymous again on the next page,
// because the session the server established travels in a cookie and nothing
// would carry it. That failure is quiet -- every page answers, each one as a
// stranger -- so the jar is not optional and not a caller's decision.
//
// What is a caller's decision is whether it outlives the process, and that
// arrives as a [Store]. With none, this is the standard in-memory jar and
// nothing else.
type jar struct {
	inner *cookiejar.Jar
	store Store
	base  string

	// mu guards held. The http.Client calls SetCookies from whichever
	// goroutine a request finished on, and the application may sign out from
	// the one drawing the screen.
	mu sync.Mutex
	// held is what the store will be given: the cookies of this server, by
	// name, with the parts that decide their life still on them.
	//
	// The jar itself cannot answer this. Asking it for a URL's cookies gives
	// back names and values with the expiry, the path and the flags stripped,
	// because that is all a request needs -- so a jar is the wrong place to
	// read a session out of, and the right place to read it is where it
	// arrives.
	held map[string]*http.Cookie
}

// newJar returns the jar for a client of base, loading whatever a store holds.
func newJar(base *url.URL, store Store) (*jar, error) {
	inner, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("ayra/client: the session store could not be opened: %w", err)
	}

	j := &jar{inner: inner, store: store, base: sessionURL(base), held: map[string]*http.Cookie{}}
	if store == nil {
		return j, nil
	}

	cookies, err := store.Load(j.base)
	if err != nil {
		return nil, err
	}
	if len(cookies) == 0 {
		return j, nil
	}

	for _, c := range cookies {
		j.held[c.Name] = c
	}
	// Handed to the jar against the server's own address rather than the one
	// each cookie names: a cookie read back has whatever domain the server
	// wrote, and a jar rejects one whose domain does not cover the address it
	// is being set for. It covered it when it was received.
	inner.SetCookies(base, cookies)
	return j, nil
}

// SetCookies records what a server sent, and keeps it if there is a store.
func (j *jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.inner.SetCookies(u, cookies)
	if j.store == nil || sessionURL(u) != j.base {
		return
	}

	j.mu.Lock()
	now := time.Now()
	for _, c := range cookies {
		if deleted(c, now) {
			// This is how a server deletes a cookie, and it is how signing out
			// arrives. Keeping it would mean a sign-out that comes back on the
			// next start.
			delete(j.held, c.Name)
			continue
		}
		j.held[c.Name] = c
	}
	keeping := j.snapshot()
	j.mu.Unlock()

	// The error is dropped, and it is the one place in this package that drops
	// one. A session that could not be written is an application that will ask
	// for a password next time; a request that failed because of it is an
	// application that does not work now. The second is worse, and this is
	// called from inside the transport, where there is nobody to tell.
	_ = j.store.Save(j.base, keeping)
}

// Cookies answers what to send with a request.
func (j *jar) Cookies(u *url.URL) []*http.Cookie { return j.inner.Cookies(u) }

// forget drops the session here and wherever it was kept.
func (j *jar) forget() error {
	inner, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("ayra/client: the session store could not be opened: %w", err)
	}

	j.mu.Lock()
	j.inner = inner
	j.held = map[string]*http.Cookie{}
	store := j.store
	j.mu.Unlock()

	if store == nil {
		return nil
	}
	return store.Clear(j.base)
}

// snapshot answers what is held, in a slice the caller owns.
func (j *jar) snapshot() []*http.Cookie {
	keeping := make([]*http.Cookie, 0, len(j.held))
	for _, c := range j.held {
		keeping = append(keeping, c)
	}
	return keeping
}

// deleted reports whether a cookie is one the server is ending by its age.
//
// A server ends a cookie two ways, and each is answered by whichever side can
// see it. An age is relative to the moment it arrived -- it is not written down
// and cannot be, because by the next run "now" is a different now -- so only
// here can it mean anything. An expiry is an absolute time that survives being
// written down, and it is re-read on every load, because the thing that makes
// it pass is time going by while nothing is running.
//
// Splitting them this way is deliberate. Asking both questions in both places
// is two answers to one question, and the day they disagree is the day a
// signed-out session comes back.
func deleted(c *http.Cookie, now time.Time) bool {
	return c.MaxAge < 0 || (c.MaxAge == 0 && !c.Expires.IsZero() && !c.Expires.After(now))
}
