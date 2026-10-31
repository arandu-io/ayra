// Package wstest is the server half of RFC 6455, for tests of the client.
//
// It is a package of its own rather than a file of the client's tests because
// two packages need it -- the client and the player built on it -- and a test
// file cannot be imported. Nothing outside a test imports it.
package wstest

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Server accepts WebSocket upgrades and hands each accepted connection to a
// handler. It reports the request that opened every connection, so a test can
// check the path, the query and the headers the client sent.
type Server struct {
	// Listener is where it listens; Addr is its address.
	Listener net.Listener

	mu       sync.Mutex
	requests []*http.Request
	handler  func(*Session)
	refuse   int
	wg       sync.WaitGroup
}

// New starts a server on the loopback interface. The handler runs once per
// accepted connection, in a goroutine of its own, and the connection is closed
// when it returns.
func New(t testing.TB, handler func(*Session)) *Server {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Listener: l, handler: handler}
	go s.serve()
	t.Cleanup(func() {
		l.Close()
		s.wg.Wait()
	})
	return s
}

// URL answers the ws address of path on this server.
func (s *Server) URL(path string) string { return "ws://" + s.Listener.Addr().String() + path }

// Refuse makes every upgrade from now on answer with this status and no
// upgrade, as the node does for a pass it does not accept.
func (s *Server) Refuse(status int) {
	s.mu.Lock()
	s.refuse = status
	s.mu.Unlock()
}

// Requests answers the upgrade requests received so far.
func (s *Server) Requests() []*http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*http.Request(nil), s.requests...)
}

func (s *Server) serve() {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.accept(conn)
		}()
	}
}

func (s *Server) accept(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.requests = append(s.requests, req)
	refuse := s.refuse
	s.mu.Unlock()

	if refuse != 0 {
		fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Type: application/json\r\nContent-Length: 17\r\nConnection: close\r\n\r\n{\"detail\":\"pass\"}", refuse, http.StatusText(refuse))
		return
	}
	key := req.Header.Get("Sec-WebSocket-Key")
	sum := sha1.Sum([]byte(key + guid))
	fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		base64.StdEncoding.EncodeToString(sum[:]))

	session := &Session{Request: req, conn: conn, br: br, Query: req.URL.Query()}
	s.handler(session)
}

// Session is one accepted connection.
type Session struct {
	// Request is the upgrade request, and Query its query string.
	Request *http.Request
	Query   url.Values

	conn net.Conn
	br   *bufio.Reader
	wmu  sync.Mutex
}

// Text sends a text message.
func (s *Session) Text(message string) error { return s.frame(true, 0x1, []byte(message)) }

// Binary sends a binary message.
func (s *Session) Binary(message []byte) error { return s.frame(true, 0x2, message) }

// Fragmented sends a binary message in frames of at most n bytes, as a server
// that fragments does, with a ping between two of them as the protocol allows.
func (s *Session) Fragmented(message []byte, n int, pingBetween bool) error {
	first := true
	for len(message) > 0 {
		chunk := message
		if len(chunk) > n {
			chunk = chunk[:n]
		}
		message = message[len(chunk):]
		op := byte(0x0)
		if first {
			op = 0x2
		}
		if err := s.frame(len(message) == 0, op, chunk); err != nil {
			return err
		}
		if first && pingBetween && len(message) > 0 {
			if err := s.frame(true, 0x9, []byte("hi")); err != nil {
				return err
			}
		}
		first = false
	}
	return nil
}

// Ping sends a ping frame.
func (s *Session) Ping() error { return s.frame(true, 0x9, nil) }

// Raw sends bytes as they are, for a test of a client facing a broken server.
func (s *Session) Raw(b []byte) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_, err := s.conn.Write(b)
	return err
}

// Close sends a close frame with the code and the reason, then waits briefly for
// the client's reply.
func (s *Session) Close(code int, reason string) error {
	body := make([]byte, 2, 2+len(reason))
	binary.BigEndian.PutUint16(body, uint16(code))
	body = append(body, reason...)
	if err := s.frame(true, 0x8, body); err != nil {
		return err
	}
	s.conn.SetReadDeadline(time.Now().Add(time.Second))
	for {
		_, op, _, err := s.read()
		if err != nil || op == 0x8 {
			return nil
		}
	}
}

// Read waits for the next data frame from the client and answers its opcode and
// payload, answering pings on the way.
func (s *Session) Read(timeout time.Duration) (op byte, payload []byte, err error) {
	s.conn.SetReadDeadline(time.Now().Add(timeout))
	for {
		_, o, p, err := s.read()
		if err != nil {
			return 0, nil, err
		}
		switch o {
		case 0x9:
			_ = s.frame(true, 0xA, p)
		case 0xA:
		default:
			return o, p, nil
		}
	}
}

// Abort drops the connection without a close frame.
func (s *Session) Abort() { s.conn.Close() }

// read reads one frame, which a client always masks.
func (s *Session) read() (fin bool, op byte, payload []byte, err error) {
	var head [2]byte
	if _, err = io.ReadFull(s.br, head[:]); err != nil {
		return
	}
	fin, op = head[0]&0x80 != 0, head[0]&0x0F
	if head[1]&0x80 == 0 {
		return false, 0, nil, fmt.Errorf("wstest: the client sent an unmasked frame")
	}
	n := uint64(head[1] & 0x7F)
	switch n {
	case 126:
		var e [2]byte
		if _, err = io.ReadFull(s.br, e[:]); err != nil {
			return
		}
		n = uint64(binary.BigEndian.Uint16(e[:]))
	case 127:
		var e [8]byte
		if _, err = io.ReadFull(s.br, e[:]); err != nil {
			return
		}
		n = binary.BigEndian.Uint64(e[:])
	}
	var mask [4]byte
	if _, err = io.ReadFull(s.br, mask[:]); err != nil {
		return
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(s.br, payload); err != nil {
		return
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return
}

func (s *Session) frame(fin bool, op byte, payload []byte) error {
	b := []byte{op}
	if fin {
		b[0] |= 0x80
	}
	switch n := len(payload); {
	case n < 126:
		b = append(b, byte(n))
	case n <= 0xFFFF:
		b = append(b, 126, byte(n>>8), byte(n))
	default:
		b = append(b, 127, 0, 0, 0, 0, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	b = append(b, payload...)
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_, err := s.conn.Write(b)
	return err
}

// Host answers the host:port of a ws address.
func Host(address string) string {
	return strings.TrimPrefix(strings.TrimPrefix(address, "ws://"), "wss://")
}
