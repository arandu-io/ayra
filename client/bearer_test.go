package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/arandu-io/ayra/client"
)

func TestARefusalCarriesTheStartOfItsBody(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"code":"account_suspended","message":"Conta suspensa"}`))
	})

	_, err := c.Get(context.Background(), client.At("painel"))
	var refused *client.StatusError
	if !errors.As(err, &refused) || refused.Status != 403 {
		t.Fatalf("answered %v, want a 403", err)
	}
	if refused.ContentType != "application/problem+json" {
		t.Errorf("content type %q", refused.ContentType)
	}
	var envelope struct {
		Code string `json:"code"`
	}
	if err := refused.Into(&envelope); err != nil || envelope.Code != "account_suspended" {
		t.Errorf("the code the server gave was %q (%v)", envelope.Code, err)
	}
}

func TestARefusalBodyIsBounded(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(strings.Repeat("x", client.ErrorBodyLimit*3)))
	})
	_, err := c.Get(context.Background(), client.At())
	var refused *client.StatusError
	if !errors.As(err, &refused) || len(refused.Body) != client.ErrorBodyLimit {
		t.Fatalf("the body kept is %d bytes, want %d", len(refused.Body), client.ErrorBodyLimit)
	}
}

func TestARefusalWithNoJSONBodyStillCarriesItsStatus(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	_, err := c.Get(context.Background(), client.At())
	var refused *client.StatusError
	if !errors.As(err, &refused) || refused.Status != 404 {
		t.Fatal(err)
	}
	var v map[string]any
	if refused.Into(&v) == nil {
		t.Error("an empty body decoded")
	}
	if client.Status(err) != 404 {
		t.Error("the status helper lost the number")
	}
}

// bearerServer answers pages and records the credential each request carried.
func bearerServer(t *testing.T, accept func(token string) bool) (*httptest.Server, *[]string, *sync.Mutex) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		mu.Lock()
		seen = append(seen, token)
		mu.Unlock()
		if !accept(token) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/img") {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0})
			return
		}
		w.Header().Set("Content-Type", client.ViewMediaType)
		w.Write([]byte(`{"view":"home","data":null}`))
	}))
	t.Cleanup(s.Close)
	return s, &seen, &mu
}

func TestEveryRequestCarriesTheBearerToken(t *testing.T) {
	s, seen, mu := bearerServer(t, func(token string) bool { return token == "t1" })
	c, _ := client.New(s.URL, client.WithBearer(func(context.Context, string) (string, error) { return "t1", nil }))

	if _, err := c.Get(context.Background(), client.At()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Post(context.Background(), client.At("x"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Download(context.Background(), client.At("img", "1"), "image/*"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*seen) != 3 || (*seen)[0] != "t1" || (*seen)[1] != "t1" || (*seen)[2] != "t1" {
		t.Errorf("credentials seen: %v", *seen)
	}
}

// TestARejectedTokenIsRenewedOnceAndTheRequestRepeated is the token that expired
// between being issued and used.
func TestARejectedTokenIsRenewedOnceAndTheRequestRepeated(t *testing.T) {
	s, seen, mu := bearerServer(t, func(token string) bool { return token == "fresh" })
	var rejectedSeen []string
	c, _ := client.New(s.URL, client.WithBearer(func(_ context.Context, rejected string) (string, error) {
		rejectedSeen = append(rejectedSeen, rejected)
		if rejected != "" {
			return "fresh", nil
		}
		return "stale", nil
	}))

	if _, err := c.Get(context.Background(), client.At()); err != nil {
		t.Fatalf("the renewal did not save the request: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*seen) != 2 || (*seen)[0] != "stale" || (*seen)[1] != "fresh" {
		t.Errorf("tokens sent: %v", *seen)
	}
	if len(rejectedSeen) != 2 || rejectedSeen[0] != "" || rejectedSeen[1] != "stale" {
		t.Errorf("the provider was told %v, want the refused token on the second call", rejectedSeen)
	}
}

func TestATokenThatIsRefusedAfterRenewalIsAnError(t *testing.T) {
	s, seen, mu := bearerServer(t, func(string) bool { return false })
	c, _ := client.New(s.URL, client.WithBearer(func(context.Context, string) (string, error) { return "never", nil }))

	_, err := c.Get(context.Background(), client.At())
	if client.Status(err) != 401 {
		t.Fatalf("answered %v, want 401", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*seen) != 2 {
		t.Errorf("%d attempts; one retry and no more", len(*seen))
	}
}

func TestAProviderThatFailsFailsTheRequestBeforeItIsSent(t *testing.T) {
	s, seen, mu := bearerServer(t, func(string) bool { return true })
	boom := errors.New("signed out")
	c, _ := client.New(s.URL, client.WithBearer(func(context.Context, string) (string, error) { return "", boom }))

	if _, err := c.Get(context.Background(), client.At()); !errors.Is(err, boom) {
		t.Errorf("answered %v, want the provider's error", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*seen) != 0 {
		t.Error("a request was sent with no credential")
	}
}

// TestRenewalsAreSerialised: requests that all find the token spent wait for one
// renewal instead of each making their own.
func TestRenewalsAreSerialised(t *testing.T) {
	var current atomic.Value
	current.Store("old")
	s, _, _ := bearerServer(t, func(token string) bool { return token == "new" })
	var renewals, inside atomic.Int32
	c, _ := client.New(s.URL, client.WithBearer(func(_ context.Context, rejected string) (string, error) {
		if inside.Add(1) != 1 {
			t.Error("two calls to the provider ran at once")
		}
		defer inside.Add(-1)
		if rejected != "" && rejected == current.Load().(string) {
			renewals.Add(1)
			current.Store("new")
		}
		return current.Load().(string), nil
	}))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Get(context.Background(), client.At()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if renewals.Load() != 1 {
		t.Errorf("the token was renewed %d times, want once", renewals.Load())
	}
}

func TestADownloadAnswersTheBytesAndWhatTheyAre(t *testing.T) {
	s, _, _ := bearerServer(t, func(string) bool { return true })
	c, _ := client.New(s.URL)

	file, err := c.Download(context.Background(), client.At("img", "7"), "image/*")
	if err != nil || file.ContentType != "image/jpeg" || len(file.Data) != 4 {
		t.Errorf("%+v %v", file, err)
	}
}

func TestADownloadAsksForWhatItWasToldNotForValues(t *testing.T) {
	var got string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Accept")
		w.Write([]byte("x"))
	})
	if _, err := c.Download(context.Background(), client.At("f"), ""); err != nil {
		t.Fatal(err)
	}
	if got != "*/*" {
		t.Errorf("asked for %q", got)
	}
}

func TestADownloadOverTheLimitIsRefused(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 5000))
	}))
	t.Cleanup(s.Close)
	c, _ := client.New(s.URL, client.WithDownloadLimit(1000))

	_, err := c.Download(context.Background(), client.At("f"), "")
	var tooLarge *client.TooLargeError
	if !errors.As(err, &tooLarge) || tooLarge.Limit != 1000 {
		t.Errorf("answered %v", err)
	}
}

func TestARefusedDownloadCarriesItsBody(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"code":"not_yours"}`))
	})
	_, err := c.Download(context.Background(), client.At("img", "1"), "image/*")
	var refused *client.StatusError
	if !errors.As(err, &refused) || string(refused.Body) != `{"code":"not_yours"}` {
		t.Errorf("answered %v", err)
	}
}

// TestTheTokenNeverLeavesTheServer: a redirect to another host is refused before
// the credential is sent there.
func TestTheTokenNeverLeavesTheServer(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
	}))
	t.Cleanup(other.Close)
	home := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	}))
	t.Cleanup(home.Close)
	c, _ := client.New(home.URL, client.WithBearer(func(context.Context, string) (string, error) { return "secret", nil }))

	if _, err := c.Download(context.Background(), client.At("img"), ""); !errors.Is(err, client.ErrOffServer) {
		t.Errorf("answered %v, want ErrOffServer", err)
	}
	if leaked.Load() {
		t.Error("the bearer token reached another server")
	}
}
