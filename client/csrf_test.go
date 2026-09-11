package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/arandu-io/ayra/client"
)

// The server asks a request that changes something to prove it came from a page
// the server rendered for this session. A browser proves it with a hidden field
// the markup carries; a client that draws for itself is sent the same value in
// the page's own data and has to send it back.
//
// It did not. Every post to an application that protects its forms was refused,
// and the refusal carries a status the protocol does not name -- so what a
// developer saw was the request, a colon, and nothing after it.
//
// This was not found by a test. The proof of the native target ran against a
// stand-in server with no protection on it, so the whole path worked and would
// have gone on working until the first real application.

// protected is a server that asks for a token the way the real one does: a
// header or a form field, on anything that is not a safe method.
func protected(t *testing.T, token string) (*httptest.Server, func() []string) {
	t.Helper()

	var mu sync.Mutex
	var seen []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			// The server sends the token on the pages that were going to render
			// a form, and not on the others.
			if r.URL.Path == "/about" {
				_, _ = w.Write([]byte(`{"view":"about","data":{"Title":"About"}}`))
				return
			}
			if r.URL.Path == "/list" {
				// Values that are not an object at all, which an application is
				// entitled to answer with.
				_, _ = w.Write([]byte(`{"view":"list","data":[1,2,3]}`))
				return
			}
			_, _ = w.Write([]byte(`{"view":"auth.login","data":{"Title":"Sign in","Token":"` + token + `"}}`))
			return
		}

		sent := r.Header.Get("X-CSRF-Token")
		if sent == "" {
			_ = r.ParseForm()
			sent = r.PostFormValue("_token")
		}

		mu.Lock()
		seen = append(seen, sent)
		mu.Unlock()

		if sent != token {
			// The status a server of this project answers with, which the
			// protocol does not name.
			w.WriteHeader(419)
			return
		}
		_, _ = w.Write([]byte(`{"view":"home","data":{"Token":"` + token + `"}}`))
	}))
	t.Cleanup(server.Close)

	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// TestAPostCarriesTheTokenTheServerSent is the whole of it.
func TestAPostCarriesTheTokenTheServerSent(t *testing.T) {
	server, sent := protected(t, "the-token")

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	// The page that was going to render the form. This is where the token
	// arrives.
	if _, err := talk.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	page, err := talk.Post(context.Background(), "/login", url.Values{"email": {"paulo@hyz.is"}})
	if err != nil {
		t.Fatalf("the post was refused: %v", err)
	}
	if page.View != "home" {
		t.Errorf("the post answered %q", page.View)
	}
	if carried := sent(); len(carried) != 1 || carried[0] != "the-token" {
		t.Errorf("the server was sent %v", carried)
	}
}

// TestAPostWithNoTokenYetIsRefusedByTheServerAndSaysSo keeps the client from
// inventing a local refusal, and keeps the server's from arriving blank.
func TestAPostWithNoTokenYetIsRefusedByTheServerAndSaysSo(t *testing.T) {
	server, _ := protected(t, "the-token")

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Straight to the post, with no page fetched first, so nothing has sent a
	// token yet.
	_, err = talk.Post(context.Background(), "/login", url.Values{"email": {"paulo@hyz.is"}})
	if err == nil {
		t.Fatal("a post with no token was accepted")
	}
	if got := client.Status(err); got != 419 {
		t.Errorf("the refusal carried status %d", got)
	}

	message := err.Error()
	if !strings.Contains(message, "419") {
		t.Errorf("the error does not name the status: %q", message)
	}
	if strings.HasSuffix(strings.TrimSpace(message), ":") {
		t.Errorf("the error ends at the colon and says nothing: %q", message)
	}
	if !strings.Contains(message, "CSRF") {
		t.Errorf("the error does not say what was missing: %q", message)
	}
}

// TestTheTokenIsNotSentOnASafeRequest keeps a credential out of every access
// log the application has.
func TestTheTokenIsNotSentOnASafeRequest(t *testing.T) {
	var mu sync.Mutex
	var onGet []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			mu.Lock()
			onGet = append(onGet, r.Header.Get("X-CSRF-Token"))
			mu.Unlock()
		}
		w.Header().Set("Content-Type", client.ViewMediaType)
		_, _ = w.Write([]byte(`{"view":"home","data":{"Token":"the-token"}}`))
	}))
	defer server.Close()

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := talk.Get(context.Background(), "/"); err != nil {
			t.Fatal(err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for index, header := range onGet {
		if header != "" {
			t.Errorf("request %d carried a token on a safe method: %q", index+1, header)
		}
	}
}

// TestAPageWithNoTokenLeavesTheOneHeldAlone fixes what would otherwise happen
// between two forms.
//
// The server sends a token on the pages that were going to render a form and
// not on the others. A client that cleared what it held on every page without
// one would lose the token somewhere in the middle of a flow, and the post at
// the end would be refused for no reason anybody could see.
func TestAPageWithNoTokenLeavesTheOneHeldAlone(t *testing.T) {
	server, sent := protected(t, "the-token")

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := talk.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	// The same client, fetching a page with nothing on it. This is the frame
	// where a client that cleared what it held would lose the token.
	if _, err := talk.Get(context.Background(), "/about"); err != nil {
		t.Fatal(err)
	}

	if _, err := talk.Post(context.Background(), "/login", url.Values{"email": {"x"}}); err != nil {
		t.Fatalf("the post after a page with no token was refused: %v", err)
	}
	if carried := sent(); len(carried) != 1 || carried[0] != "the-token" {
		t.Errorf("the server was sent %v", carried)
	}
}

// TestSigningOutDropsTheToken keeps the credential of an ended session from
// being carried into the next request.
func TestSigningOutDropsTheToken(t *testing.T) {
	server, sent := protected(t, "the-token")

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := talk.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}
	if err := talk.Forget(); err != nil {
		t.Fatal(err)
	}

	if _, err := talk.Post(context.Background(), "/login", url.Values{"email": {"x"}}); err == nil {
		t.Fatal("a post after signing out was accepted")
	}
	carried := sent()
	if len(carried) != 1 || carried[0] != "" {
		t.Errorf("the token of an ended session was carried: %v", carried)
	}
}

// TestAPageThisCannotReadDoesNotLoseTheToken keeps a page whose values are not
// an object from clearing what is held.
//
// An application's page data is its own, and nothing says it is a JSON object.
// Failing to find a token in one is not the same as being told there is none.
func TestAPageThisCannotReadDoesNotLoseTheToken(t *testing.T) {
	server, sent := protected(t, "the-token")

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := talk.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	// The same client again, and values that are a list rather than an object.
	if _, err := talk.Get(context.Background(), "/list"); err != nil {
		t.Fatalf("a page whose values are a list was refused: %v", err)
	}

	if _, err := talk.Post(context.Background(), "/login", url.Values{"email": {"x"}}); err != nil {
		t.Fatalf("the post was refused after a page this could not read: %v", err)
	}
	if carried := sent(); len(carried) != 1 || carried[0] != "the-token" {
		t.Errorf("the server was sent %v", carried)
	}
}
