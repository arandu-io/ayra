package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
)

// TestTheServerIsItsSchemeHostAndPort fixes what "the same server" means to
// the redirect policy, the token and the answer.
//
// The case that matters most is the first one refused: the same host reached
// over plain http after the session was established over https. Nothing about
// the host has changed, and everything about who can read the request has.
func TestTheServerIsItsSchemeHostAndPort(t *testing.T) {
	base, err := url.Parse("https://app.example.test")
	if err != nil {
		t.Fatal(err)
	}

	for address, want := range map[string]bool{
		"https://app.example.test/dashboard":     true,
		"https://APP.example.test/dashboard":     true,
		"https://app.example.test:443/dashboard": true,
		"http://app.example.test/dashboard":      false,
		"http://app.example.test:443/dashboard":  false,
		"https://app.example.test:8443/":         false,
		"https://evil.example.test/":             false,
		"https://app.example.test.evil.test/":    false,
	} {
		u, err := url.Parse(address)
		if err != nil {
			t.Fatal(err)
		}
		if got := sameServer(base, u); got != want {
			t.Errorf("%s: same server = %v, want %v", address, got, want)
		}
	}
}

// TestARedirectThatDropsTlsIsRefused is the downgrade, asked of the policy
// itself: two test servers cannot share a host and a port while differing in
// scheme, so the request is built here.
func TestARedirectThatDropsTlsIsRefused(t *testing.T) {
	base, err := url.Parse("https://app.example.test")
	if err != nil {
		t.Fatal(err)
	}
	policy := stayOnServer(nil)

	req, err := http.NewRequestWithContext(onServer(context.Background(), base), http.MethodGet, "http://app.example.test/dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := policy(req, []*http.Request{req}); !errors.Is(err, ErrOffServer) {
		t.Errorf("a redirect from https to http was allowed: %v", err)
	}

	same, err := http.NewRequestWithContext(onServer(context.Background(), base), http.MethodGet, "https://app.example.test/dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := policy(same, []*http.Request{same}); err != nil {
		t.Errorf("a redirect on the same server was refused: %v", err)
	}
}

// TestTheTokenIsCarriedOnlyToItsServer keeps the token off a request built for
// anywhere else, whatever built it.
func TestTheTokenIsCarriedOnlyToItsServer(t *testing.T) {
	base, err := url.Parse("https://app.example.test")
	if err != nil {
		t.Fatal(err)
	}
	var held tokens
	held.remember("the-token")

	for address, want := range map[string]string{
		"https://app.example.test/login": "the-token",
		"https://evil.example.test/":     "",
		"http://app.example.test/login":  "",
	} {
		req, err := http.NewRequest(http.MethodPost, address, nil)
		if err != nil {
			t.Fatal(err)
		}
		held.carry(req, base)
		if got := req.Header.Get(csrfHeader); got != want {
			t.Errorf("%s carried %q, want %q", address, got, want)
		}
	}
}
