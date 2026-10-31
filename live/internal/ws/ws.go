// Package ws is the client half of RFC 6455, as much of it as reading a live
// video needs.
//
// It exists because the close code is part of the protocol the node speaks:
// code 4001 means "the session ended, ask for a new pass", which is not a
// failure, and a client that cannot read the code cannot tell the two apart.
// The standard library has no WebSocket and the dependency budget of this
// module is fixed, so this is written here, small enough to read in one sitting
// and tested against a server written beside it.
//
// It does not compress, does not negotiate an extension or a sub-protocol and
// does not reconnect. A frame it sends is always masked, which is what a client
// must do, and a frame it receives that is masked is refused, which is what a
// server must not send.
package ws

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Type is the kind of a data message.
type Type int

const (
	// Text is a UTF-8 message.
	Text Type = 1
	// Binary is a message of bytes.
	Binary Type = 2
)

// The frame opcodes.
const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
)

// guid is the constant RFC 6455 section 1.3 mixes into the handshake.
const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Common close codes.
const (
	CloseNormal         = 1000
	CloseGoingAway      = 1001
	CloseProtocolError  = 1002
	CloseTooBig         = 1009
	CloseTryAgainLater  = 1013
	CloseNoStatus       = 1005
	CloseAbnormal       = 1006
	DefaultMaxMessage   = 4 << 20
	defaultDialTimeout  = 15 * time.Second
	handshakeBodyBudget = 512
)

// CloseError is the peer closing the connection, with the code it gave.
type CloseError struct {
	// Code is the status the peer sent, or [CloseNoStatus] when it sent none.
	Code int
	// Reason is the text that came with it.
	Reason string
}

func (e *CloseError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("ws: closed by the peer with code %d", e.Code)
	}
	return fmt.Sprintf("ws: closed by the peer with code %d: %s", e.Code, e.Reason)
}

// HandshakeError is a server that answered the upgrade with something other
// than 101.
type HandshakeError struct {
	// Status is the HTTP status. A pass the node refuses is 401, and it is
	// answered before the upgrade, so it arrives here.
	Status int
	// Body is the start of the response body, for a diagnostic.
	Body string
}

func (e *HandshakeError) Error() string {
	return fmt.Sprintf("ws: the server answered the upgrade with %d", e.Status)
}

// Options configure a connection.
type Options struct {
	// Header is added to the upgrade request. Origin is the usual one.
	Header http.Header
	// TLS configures a wss connection. Nil takes the defaults.
	TLS *tls.Config
	// MaxMessage bounds one assembled message. Zero takes [DefaultMaxMessage].
	MaxMessage int
	// DialTimeout bounds the connection and the upgrade. Zero is 15 seconds.
	DialTimeout time.Duration
	// Dial replaces how the TCP connection is opened, for a test or a proxy.
	Dial func(ctx context.Context, network, address string) (net.Conn, error)
}

// Conn is one connection. Reads belong to one goroutine; Close and the writes
// it makes for control frames are safe to call from another.
type Conn struct {
	c    net.Conn
	br   *bufio.Reader
	max  int
	wmu  sync.Mutex
	done chan struct{}
	once sync.Once
}

// Dial opens a connection to a ws or wss address (an http or https one is read
// as the same).
func Dial(ctx context.Context, rawURL string, o Options) (*Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("ws: %w", err)
	}
	secure := false
	switch strings.ToLower(u.Scheme) {
	case "wss", "https":
		secure = true
	case "ws", "http":
	default:
		return nil, fmt.Errorf("ws: %q is not a ws or wss address", rawURL)
	}
	host := u.Host
	if u.Port() == "" {
		if secure {
			host = net.JoinHostPort(u.Hostname(), "443")
		} else {
			host = net.JoinHostPort(u.Hostname(), "80")
		}
	}

	timeout := o.DialTimeout
	if timeout <= 0 {
		timeout = defaultDialTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dial := o.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	raw, err := dial(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("ws: %w", err)
	}
	conn := raw
	if secure {
		cfg := o.TLS
		if cfg == nil {
			cfg = &tls.Config{}
		} else {
			cfg = cfg.Clone()
		}
		if cfg.ServerName == "" {
			cfg.ServerName = u.Hostname()
		}
		tlsConn := tls.Client(raw, cfg)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			raw.Close()
			return nil, fmt.Errorf("ws: %w", err)
		}
		conn = tlsConn
	}

	// The context ends the handshake; once the upgrade is done the deadline is
	// cleared and the reader owns the timing.
	stop := context.AfterFunc(ctx, func() { conn.SetDeadline(time.Unix(1, 0)) })
	c, err := handshake(conn, u, host, o)
	stopped := !stop()
	if err != nil {
		conn.Close()
		if stopped || ctx.Err() != nil {
			if cause := ctx.Err(); cause != nil {
				return nil, fmt.Errorf("ws: %w", cause)
			}
		}
		return nil, err
	}
	if stopped {
		conn.Close()
		return nil, fmt.Errorf("ws: %w", ctx.Err())
	}
	conn.SetDeadline(time.Time{})
	return c, nil
}

func handshake(conn net.Conn, u *url.URL, host string, o Options) (*Conn, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(nonce[:])

	target := u.RequestURI()
	var req strings.Builder
	fmt.Fprintf(&req, "GET %s HTTP/1.1\r\n", target)
	fmt.Fprintf(&req, "Host: %s\r\n", u.Host)
	req.WriteString("Upgrade: websocket\r\nConnection: Upgrade\r\n")
	fmt.Fprintf(&req, "Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n", key)
	for name, values := range o.Header {
		switch http.CanonicalHeaderKey(name) {
		case "Host", "Upgrade", "Connection", "Sec-Websocket-Key", "Sec-Websocket-Version":
			continue
		}
		for _, v := range values {
			if strings.ContainsAny(name+v, "\r\n") {
				return nil, errors.New("ws: a header carries a line break")
			}
			fmt.Fprintf(&req, "%s: %s\r\n", name, v)
		}
	}
	req.WriteString("\r\n")
	if _, err := io.WriteString(conn, req.String()); err != nil {
		return nil, fmt.Errorf("ws: %w", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodGet})
	if err != nil {
		return nil, fmt.Errorf("ws: reading the upgrade answer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, handshakeBodyBudget))
		return nil, &HandshakeError{Status: resp.StatusCode, Body: string(body)}
	}
	sum := sha1.Sum([]byte(key + guid))
	want := base64.StdEncoding.EncodeToString(sum[:])
	if !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") || resp.Header.Get("Sec-WebSocket-Accept") != want {
		return nil, errors.New("ws: the server's upgrade answer is not a WebSocket handshake")
	}

	max := o.MaxMessage
	if max <= 0 {
		max = DefaultMaxMessage
	}
	return &Conn{c: conn, br: br, max: max, done: make(chan struct{})}, nil
}

// Read waits for the next data message. A deadline of zero waits without end.
//
// Pings are answered and pongs are dropped here, without being shown to the
// caller. A close frame is answered and returned as a [*CloseError].
func (c *Conn) Read(timeout time.Duration) (Type, []byte, error) {
	var (
		kind    Type
		message []byte
		started bool
	)
	for {
		if timeout > 0 {
			c.c.SetReadDeadline(time.Now().Add(timeout))
		} else {
			c.c.SetReadDeadline(time.Time{})
		}
		fin, op, payload, err := c.frame()
		if err != nil {
			return 0, nil, err
		}
		switch op {
		case opPing:
			if err := c.write(opPong, payload); err != nil {
				return 0, nil, err
			}
		case opPong:
		case opClose:
			code, reason := CloseNoStatus, ""
			if len(payload) >= 2 {
				code = int(binary.BigEndian.Uint16(payload))
				reason = string(payload[2:])
			}
			// The reply echoes the code, as the protocol says, and a failure to
			// send it changes nothing: the peer is closing either way.
			if len(payload) >= 2 {
				_ = c.write(opClose, payload[:2])
			} else {
				_ = c.write(opClose, nil)
			}
			c.shut()
			return 0, nil, &CloseError{Code: code, Reason: reason}
		case opText, opBinary:
			if started {
				return 0, nil, c.fail(CloseProtocolError, "a data frame began inside a message")
			}
			started = true
			kind = Text
			if op == opBinary {
				kind = Binary
			}
			message = payload
			if fin {
				return kind, message, nil
			}
		case opContinuation:
			if !started {
				return 0, nil, c.fail(CloseProtocolError, "a continuation with no message open")
			}
			if len(message)+len(payload) > c.max {
				return 0, nil, c.fail(CloseTooBig, "message too large")
			}
			message = append(message, payload...)
			if fin {
				return kind, message, nil
			}
		default:
			return 0, nil, c.fail(CloseProtocolError, "unknown opcode")
		}
	}
}

// frame reads one frame.
func (c *Conn) frame() (fin bool, op byte, payload []byte, err error) {
	var head [2]byte
	if _, err = io.ReadFull(c.br, head[:]); err != nil {
		return false, 0, nil, err
	}
	fin = head[0]&0x80 != 0
	if head[0]&0x70 != 0 {
		return false, 0, nil, c.fail(CloseProtocolError, "reserved bits set")
	}
	op = head[0] & 0x0F
	if head[1]&0x80 != 0 {
		return false, 0, nil, c.fail(CloseProtocolError, "a server frame is masked")
	}
	n := uint64(head[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, err
		}
		n = binary.BigEndian.Uint64(ext[:])
	}
	if op >= opClose && (n > 125 || !fin) {
		return false, 0, nil, c.fail(CloseProtocolError, "a control frame is too long or fragmented")
	}
	if n > uint64(c.max) {
		return false, 0, nil, c.fail(CloseTooBig, "frame too large")
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(c.br, payload); err != nil {
		return false, 0, nil, err
	}
	return fin, op, payload, nil
}

// fail closes the connection for a protocol violation and answers the error.
func (c *Conn) fail(code int, reason string) error {
	var body [2]byte
	binary.BigEndian.PutUint16(body[:], uint16(code))
	_ = c.write(opClose, append(body[:], reason...))
	c.shut()
	return fmt.Errorf("ws: protocol error: %s", reason)
}

// Close sends a close frame with the code and ends the connection. It is safe
// to call from a goroutine other than the reader's, and it unblocks the reader.
func (c *Conn) Close(code int, reason string) error {
	var body [2]byte
	binary.BigEndian.PutUint16(body[:], uint16(code))
	// A short deadline: the point of Close is that it ends, and a peer that
	// does not read must not hold it.
	c.c.SetWriteDeadline(time.Now().Add(time.Second))
	_ = c.write(opClose, append(body[:], reason...))
	c.shut()
	return nil
}

func (c *Conn) shut() {
	c.once.Do(func() {
		close(c.done)
		c.c.Close()
	})
}

// write sends one masked, unfragmented control or data frame.
func (c *Conn) write(op byte, payload []byte) error {
	if len(payload) > 125 && op >= opClose {
		return errors.New("ws: a control frame is too long")
	}
	frame := make([]byte, 0, 14+len(payload))
	frame = append(frame, 0x80|op)
	switch n := len(payload); {
	case n < 126:
		frame = append(frame, 0x80|byte(n))
	case n <= 0xFFFF:
		frame = append(frame, 0x80|126, byte(n>>8), byte(n))
	default:
		frame = append(frame, 0x80|127, 0, 0, 0, 0, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	frame = append(frame, mask[:]...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	select {
	case <-c.done:
		return net.ErrClosed
	default:
	}
	_, err := c.c.Write(frame)
	return err
}

// WriteText sends a text message.
func (c *Conn) WriteText(message []byte) error { return c.write(opText, message) }

// Done is closed when the connection has ended.
func (c *Conn) Done() <-chan struct{} { return c.done }

// IsTimeout reports whether err is a read that waited out its timeout.
func IsTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
