package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/arandu-io/ayra/client"
)

// A server limits a session cookie with Max-Age, which is how the session
// cookie of this project is written. An age is relative to the moment it
// arrived, and the store dropped it: the file held the cookie with no expiry,
// so it was loaded back as alive on every start, forever after the server had
// meant it to end. A store in memory kept the age itself, and a restart
// counted it again from the start.

// TestAnAgeIsWrittenAsAnExpiry reads the file a session is kept in.
func TestAnAgeIsWrittenAsAnExpiry(t *testing.T) {
	server := signing(t, &http.Cookie{Name: "arandu_session", Value: "id.sig", Path: "/", MaxAge: 60, HttpOnly: true})
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	store, err := client.FileSession("Proof")
	if err != nil {
		t.Skipf("this account has no configuration directory: %v", err)
	}

	talk, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if _, err := talk.Get(context.Background(), client.At("login")); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(findSession(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	var held map[string][]struct {
		Name    string    `json:"name"`
		Expires time.Time `json:"expires"`
	}
	if err := json.Unmarshal(body, &held); err != nil {
		t.Fatalf("the session file is not what was written: %v", err)
	}

	found := false
	for _, cookies := range held {
		for _, c := range cookies {
			if c.Name != "arandu_session" {
				continue
			}
			found = true
			// The zero time is what an expiry that was never written reads
			// back as, so the field being present says nothing.
			if c.Expires.Before(before.Add(55*time.Second)) || c.Expires.After(time.Now().Add(65*time.Second)) {
				t.Errorf("a cookie with an age of 60 seconds was written to expire at %v", c.Expires)
			}
		}
	}
	if !found {
		t.Fatalf("the session was not written: %s", body)
	}
}

// TestASessionWithAnAgeEndsWhenTheAgeIsOver is the same fault seen from the
// next start, in both stores.
func TestASessionWithAnAgeEndsWhenTheAgeIsOver(t *testing.T) {
	for name, open := range map[string]func(t *testing.T) client.Store{
		"file":   fileStore,
		"memory": func(*testing.T) client.Store { return client.MemorySession() },
	} {
		t.Run(name, func(t *testing.T) {
			server := signing(t, &http.Cookie{Name: "session", Value: "held", Path: "/", MaxAge: 1})
			store := open(t)

			first, err := client.New(server.URL, client.WithSession(store))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := first.Get(context.Background(), client.At("login")); err != nil {
				t.Fatal(err)
			}

			time.Sleep(1500 * time.Millisecond)

			second, err := client.New(server.URL, client.WithSession(store))
			if err != nil {
				t.Fatal(err)
			}
			if page, _ := ask(t, second); page.View != "auth.login" {
				t.Errorf("a session past its age was carried into a new run: %q", page.View)
			}
		})
	}
}

// TestAStoreGivenAnAgeKeepsItsEnd is the file store asked directly, the way a
// caller with a session of its own to keep would ask it.
func TestAStoreGivenAnAgeKeepsItsEnd(t *testing.T) {
	store := fileStore(t)
	const base = "http://example.test"

	if err := store.Save(base, []*http.Cookie{
		{Name: "session", Value: "held", Path: "/", MaxAge: 60},
		{Name: "ended", Value: "gone", Path: "/", MaxAge: -1},
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Name != "session" {
		t.Fatalf("the store loaded %v, want the one cookie still alive", loaded)
	}
	if until := time.Until(loaded[0].Expires); until < 55*time.Second || until > 65*time.Second {
		t.Errorf("a cookie with an age of 60 seconds was kept until %v", loaded[0].Expires)
	}
}
