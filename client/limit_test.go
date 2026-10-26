package client_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arandu-io/ayra/client"
)

// bomb answers a page whose values inflate to size bytes from a body a few
// kilobytes long on the wire.
func bomb(t *testing.T, size int) http.HandlerFunc {
	t.Helper()

	var wire bytes.Buffer
	zw, err := gzip.NewWriterLevel(&wire, gzip.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = zw.Write([]byte(`{"view":"home","data":"`))
	chunk := bytes.Repeat([]byte("A"), 1<<20)
	for written := 0; written < size; written += len(chunk) {
		_, _ = zw.Write(chunk)
	}
	_, _ = zw.Write([]byte(`"}`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	compressed := wire.Bytes()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(compressed)
	}
}

// TestAnAnswerLargerThanTheLimitIsRefused is a body that is small on the wire
// and enormous once the transport has inflated it.
//
// Decoded with no limit, a quarter of a megabyte became a quarter of a
// gigabyte in memory, which on a phone is the application killed by the
// system with nothing on the screen to say why.
func TestAnAnswerLargerThanTheLimitIsRefused(t *testing.T) {
	c := serve(t, bomb(t, 2*client.DefaultPageLimit))

	page, err := c.Get(context.Background(), client.At())
	var tooLarge *client.TooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("an answer of twice the limit was taken: %d bytes of values, error %v", len(page.Data), err)
	}
	if tooLarge.Limit != client.DefaultPageLimit {
		t.Errorf("the refusal names a limit of %d, want %d", tooLarge.Limit, client.DefaultPageLimit)
	}
	if !strings.Contains(err.Error(), "GET /") {
		t.Errorf("the refusal does not name the request: %q", err)
	}
}

// TestThePageLimitIsTheCallers keeps the limit a decision an application can
// make for its own pages, in both directions.
func TestThePageLimitIsTheCallers(t *testing.T) {
	page := `{"view":"home","data":"` + strings.Repeat("A", 4096) + `"}`

	server := httptest.NewServer(viewData(page))
	t.Cleanup(server.Close)

	tight, err := client.New(server.URL, client.WithPageLimit(1024))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tight.Get(context.Background(), client.At()); !errors.As(err, new(*client.TooLargeError)) {
		t.Errorf("a page past a limit of 1024 was taken: %v", err)
	}

	roomy, err := client.New(server.URL, client.WithPageLimit(1<<20))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := roomy.Get(context.Background(), client.At()); err != nil {
		t.Errorf("a page within the limit was refused: %v", err)
	}
}
