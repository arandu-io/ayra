package ws_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/arandu-io/ayra/live/internal/ws"
	"github.com/arandu-io/ayra/live/internal/wstest"
)

// dial opens a connection to a server running handler.
func dial(t *testing.T, handler func(*wstest.Session), o ws.Options) (*ws.Conn, *wstest.Server) {
	t.Helper()
	server := wstest.New(t, handler)
	conn, err := ws.Dial(context.Background(), server.URL("/live/cam-1?pass=abc"), o)
	if err != nil {
		t.Fatalf("the connection did not open: %v", err)
	}
	t.Cleanup(func() { conn.Close(ws.CloseNormal, "") })
	return conn, server
}

// TestTheUpgradeCarriesWhatTheNodeChecks fixes what reaches the server: the
// path and the pass in the query, and the origin the node compares against its
// list.
func TestTheUpgradeCarriesWhatTheNodeChecks(t *testing.T) {
	_, server := dial(t, func(s *wstest.Session) { s.Close(ws.CloseNormal, "") }, ws.Options{
		Header: http.Header{"Origin": {"https://alpr.sh"}},
	})

	deadline := time.Now().Add(2 * time.Second)
	for len(server.Requests()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	requests := server.Requests()
	if len(requests) != 1 {
		t.Fatalf("%d upgrade requests, want 1", len(requests))
	}
	req := requests[0]
	if req.URL.Path != "/live/cam-1" || req.URL.Query().Get("pass") != "abc" {
		t.Errorf("the request was for %s, want /live/cam-1?pass=abc", req.URL)
	}
	if req.Header.Get("Origin") != "https://alpr.sh" {
		t.Errorf("the origin was %q", req.Header.Get("Origin"))
	}
}

func TestTextAndBinaryMessagesAreTold(t *testing.T) {
	conn, _ := dial(t, func(s *wstest.Session) {
		s.Text(`{"type":"ping"}`)
		s.Binary([]byte{1, 2, 3})
		s.Close(ws.CloseNormal, "")
	}, ws.Options{})

	kind, message, err := conn.Read(2 * time.Second)
	if err != nil || kind != ws.Text || string(message) != `{"type":"ping"}` {
		t.Fatalf("first message: %v %q %v", kind, message, err)
	}
	kind, message, err = conn.Read(2 * time.Second)
	if err != nil || kind != ws.Binary || !bytes.Equal(message, []byte{1, 2, 3}) {
		t.Fatalf("second message: %v %v %v", kind, message, err)
	}
}

// TestAFragmentedMessageIsJoinedAcrossAPing is the shape a proxy produces, and
// the one the protocol allows: a control frame may sit between the pieces of a
// message.
func TestAFragmentedMessageIsJoinedAcrossAPing(t *testing.T) {
	picture := bytes.Repeat([]byte("abcdefgh"), 1000)
	conn, _ := dial(t, func(s *wstest.Session) {
		s.Fragmented(picture, 1500, true)
		s.Close(ws.CloseNormal, "")
	}, ws.Options{})

	kind, message, err := conn.Read(2 * time.Second)
	if err != nil || kind != ws.Binary || !bytes.Equal(message, picture) {
		t.Fatalf("the joined message is %d bytes (%v), want %d: %v", len(message), kind, len(picture), err)
	}
}

// TestTheCloseCodeIsReadAndAnswered is the reason this package exists: a
// session that ended is told apart from a socket that failed by the code the
// node closed it with.
func TestTheCloseCodeIsReadAndAnswered(t *testing.T) {
	answered := make(chan error, 1)
	conn, _ := dial(t, func(s *wstest.Session) {
		answered <- s.Close(4001, "session ended")
	}, ws.Options{})

	_, _, err := conn.Read(2 * time.Second)
	var closed *ws.CloseError
	if !errors.As(err, &closed) || closed.Code != 4001 || closed.Reason != "session ended" {
		t.Fatalf("the read answered %v, want a close with code 4001", err)
	}
	if err := <-answered; err != nil {
		t.Fatal(err)
	}
}

func TestARefusedPassIsAHandshakeError(t *testing.T) {
	server := wstest.New(t, func(*wstest.Session) {})
	server.Refuse(http.StatusUnauthorized)

	_, err := ws.Dial(context.Background(), server.URL("/live/x?pass=bad"), ws.Options{})
	var refused *ws.HandshakeError
	if !errors.As(err, &refused) || refused.Status != http.StatusUnauthorized {
		t.Fatalf("a refused upgrade answered %v, want a handshake error with status 401", err)
	}
}

func TestAReadThatWaitsTooLongTimesOut(t *testing.T) {
	release := make(chan struct{})
	conn, _ := dial(t, func(s *wstest.Session) { <-release }, ws.Options{})
	defer close(release)

	started := time.Now()
	_, _, err := conn.Read(80 * time.Millisecond)
	if !ws.IsTimeout(err) {
		t.Fatalf("a silent socket answered %v, want a timeout", err)
	}
	if time.Since(started) > time.Second {
		t.Errorf("the timeout took %v", time.Since(started))
	}
}

// TestCloseFromAnotherGoroutineEndsTheRead is how a viewer who left stops the
// reader: Close is called while a Read is waiting.
func TestCloseFromAnotherGoroutineEndsTheRead(t *testing.T) {
	release := make(chan struct{})
	conn, _ := dial(t, func(s *wstest.Session) { <-release }, ws.Options{})
	defer close(release)

	result := make(chan error, 1)
	go func() {
		_, _, err := conn.Read(0)
		result <- err
	}()
	time.Sleep(50 * time.Millisecond)
	conn.Close(ws.CloseNormal, "")

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("the read returned without an error after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not end the read")
	}
}

func TestAMessageOverTheLimitIsRefused(t *testing.T) {
	conn, _ := dial(t, func(s *wstest.Session) {
		s.Binary(make([]byte, 5000))
		time.Sleep(200 * time.Millisecond)
	}, ws.Options{MaxMessage: 1000})

	if _, _, err := conn.Read(2 * time.Second); err == nil {
		t.Fatal("a message of 5000 bytes was read under a limit of 1000")
	}
}

func TestAMaskedServerFrameIsRefused(t *testing.T) {
	conn, _ := dial(t, func(s *wstest.Session) {
		s.Raw([]byte{0x81, 0x81, 1, 2, 3, 4, 'x' ^ 1})
		time.Sleep(200 * time.Millisecond)
	}, ws.Options{})

	if _, _, err := conn.Read(2 * time.Second); err == nil {
		t.Fatal("a masked frame from the server was accepted")
	}
}

func TestAnAddressThatIsNotWebSocketIsRefused(t *testing.T) {
	if _, err := ws.Dial(context.Background(), "ftp://example.test/live/a", ws.Options{}); err == nil {
		t.Fatal("an ftp address was dialled")
	}
}

func TestADialThatIsCancelledReturns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ws.Dial(ctx, "ws://127.0.0.1:1/live/a", ws.Options{}); err == nil {
		t.Fatal("a cancelled dial succeeded")
	}
}
