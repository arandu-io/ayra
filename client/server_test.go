package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/arandu-io/ayra/client"
)

// A client is pointed at one server, and what it carries -- the session, the
// CSRF token, the body of a form with a password in it -- is that server's.
// Each test here is a way it used to reach another one.

// elsewhere is a server that is not the one the client was made for, and
// records what arrived at it.
type elsewhere struct {
	server *httptest.Server

	mu     sync.Mutex
	hits   int
	token  string
	method string
	body   string
}

func newElsewhere(t *testing.T) *elsewhere {
	t.Helper()

	e := &elsewhere{}
	e.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()

		e.mu.Lock()
		e.hits++
		e.token = r.Header.Get("X-CSRF-Token")
		e.method = r.Method
		e.body = r.PostForm.Encode()
		e.mu.Unlock()

		// A well-formed page with a token of its own, which is what a server
		// that wanted to be believed would answer.
		w.Header().Set("Content-Type", client.ViewMediaType)
		_, _ = w.Write([]byte(`{"view":"auth.login","data":{},"token":"foreign-token"}`))
	}))
	t.Cleanup(e.server.Close)
	return e
}

// seen answers how many requests arrived, and what the last one carried.
func (e *elsewhere) seen() (hits int, token, method, body string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.hits, e.token, e.method, e.body
}

// home is the server the client is made for. It hands out a token on GET /,
// redirects /login wherever to says, and records the token the next post to
// /after carried.
func home(t *testing.T, to func() string, status int) (*httptest.Server, func() string) {
	t.Helper()

	var mu sync.Mutex
	var after string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.Redirect(w, r, to(), status)
			return
		case "/after":
			mu.Lock()
			after = r.Header.Get("X-CSRF-Token")
			mu.Unlock()
		}
		w.Header().Set("Content-Type", client.ViewMediaType)
		_, _ = w.Write([]byte(`{"view":"home","data":{},"token":"home-token"}`))
	}))
	t.Cleanup(server.Close)

	return server, func() string {
		mu.Lock()
		defer mu.Unlock()
		return after
	}
}

// TestARedirectToAnotherServerIsNotFollowed is a sign-in whose answer points
// somewhere else -- an open "intended" parameter on the server is enough.
//
// The redirect used to be followed, the token went with it in its header, and
// a 307 or a 308 sent the form again, password and all. The foreign answer was
// then taken as a page: its view drawn and its token remembered, so the next
// form posted to the real server carried a token somebody else chose.
func TestARedirectToAnotherServerIsNotFollowed(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			foreign := newElsewhere(t)
			server, after := home(t, func() string { return foreign.server.URL + "/phish" }, status)

			talk, err := client.New(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if _, err := talk.Get(ctx, "/"); err != nil {
				t.Fatal(err)
			}

			page, err := talk.Post(ctx, "/login", url.Values{"email": {"a@b.c"}, "password": {"hunter2"}})
			if err == nil {
				t.Errorf("a redirect to another server answered a page, %q", page.View)
			}
			if !errors.Is(err, client.ErrOffServer) {
				t.Errorf("the refusal does not say why: %v", err)
			}
			if hits, token, method, body := foreign.seen(); hits != 0 {
				t.Errorf("the other server was reached: %s with token %q and body %q", method, token, body)
			}

			if _, err := talk.Post(ctx, "/after", nil); err != nil {
				t.Fatal(err)
			}
			if got := after(); got != "home-token" {
				t.Errorf("the next post to the real server carried %q, want the token it sent", got)
			}
		})
	}
}

// TestARedirectOnTheSameServerIsStillFollowed keeps the refusal above to what
// it is for. Posting a form and being sent to the page that follows is how
// every sign-in ends.
func TestARedirectOnTheSameServerIsStillFollowed(t *testing.T) {
	var server *httptest.Server
	server, _ = home(t, func() string { return server.URL + "/dashboard" }, http.StatusSeeOther)

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := talk.Post(context.Background(), "/login", url.Values{"email": {"a@b.c"}})
	if err != nil {
		t.Fatalf("a redirect on the same server was refused: %v", err)
	}
	if page.View != "home" {
		t.Errorf("the redirect answered %q", page.View)
	}
}

// TestAPathCannotNameAnotherServer is the same leak without a redirect: a path
// built from something the server sent -- a "next" value, an href -- that is
// itself an address.
func TestAPathCannotNameAnotherServer(t *testing.T) {
	foreign := newElsewhere(t)
	server, _ := home(t, func() string { return "/" }, http.StatusFound)

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := talk.Get(ctx, "/"); err != nil {
		t.Fatal(err)
	}

	host := foreign.server.Listener.Addr().String()
	for _, path := range []string{
		"//" + host + "/steal",
		foreign.server.URL + "/steal",
		"http:" + host,
	} {
		_, _ = talk.Post(ctx, path, url.Values{"comment": {"hi"}})
	}

	if hits, token, method, body := foreign.seen(); hits != 0 {
		t.Errorf("a path reached another server: %s with token %q and body %q", method, token, body)
	}
}
