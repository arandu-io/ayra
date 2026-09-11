package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store is where a session is kept between runs of an application.
//
// Without one a native application signs in, works, and is a stranger again the
// next time it opens -- which is not what closing a window means to anybody. A
// browser tab closing ends a session; an application quitting does not, and the
// person who quit it expects to come back to where they were.
//
// It is an interface because where a session may be written is a property of
// the platform and not of this library: a file is right on a desktop, a
// platform's own secret store is right where there is one, and a test wants
// neither. [FileSession] is the one this ships with.
//
// What reaches a Store is what the server sent. Implementations must treat it
// as a credential -- it is the thing that proves who somebody is -- and must
// not log it, copy it anywhere else, or leave it readable by other accounts on
// the machine.
type Store interface {
	// Load answers the cookies held for base, and nothing when none are.
	//
	// A store with nothing in it is not an error: it is every first run.
	Load(base string) ([]*http.Cookie, error)

	// Save replaces what is held for base.
	Save(base string, cookies []*http.Cookie) error

	// Clear removes what is held for base, which is what signing out means on
	// this side.
	Clear(base string) error
}

// WithSession keeps the session across runs of the application.
//
// Without it the client holds its session in memory, which is the safe default
// and the wrong one for an application somebody installs: they sign in, quit,
// and are asked to sign in again.
//
// It is an option rather than the default because it writes a credential to
// wherever the store puts it, and that is a decision the application makes
// knowingly -- a library that started writing tokens to disk on everybody's
// behalf would be a library doing it for the applications that should not.
func WithSession(store Store) Option {
	return func(c *Client) { c.session = store }
}

// Forget ends the session on this side: the client is a stranger again and
// nothing is left on disk.
//
// It is called when the server says the session is over, which is the half a
// sign-out request does not do. Without it a signed-out application still
// carries the cookie, and the store still holds it after the process ends.
func (c *Client) Forget() error {
	if j, ok := c.http.Jar.(*jar); ok {
		return j.forget()
	}
	return nil
}

// FileSession keeps a session in a file under the account's own configuration
// directory.
//
// The name is the application's, and it is what separates one application's
// session from another's on the same machine. The file is readable by this
// account and no other, and the directory holding it likewise.
//
// It is a file and not the platform's secret store, and that is a trade written
// here rather than hidden: a secret store is stronger and each platform has a
// different one, reached through code that is not Go. A file is the same
// everywhere, needs nothing installed, and is as safe as the account it belongs
// to -- which is the same account that can read the application's own data.
func FileSession(name string) (Store, error) {
	if name == "" {
		return nil, errors.New("ayra/client: a session file needs the application's name, and nothing separates two applications' sessions without it")
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("ayra/client: this account has no configuration directory to keep a session in: %w", err)
	}
	return &fileStore{path: filepath.Join(dir, safeName(name), "session.json")}, nil
}

// safeName answers a directory name that cannot escape the directory it is
// joined to.
//
// The name comes from the application, and an application named with a
// separator in it would otherwise write its session wherever that separator
// pointed.
func safeName(name string) string {
	safe := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			safe = append(safe, r)
		case r == '-', r == '_', r == '.':
			safe = append(safe, r)
		default:
			safe = append(safe, '-')
		}
	}
	if len(safe) == 0 || string(safe) == "." || string(safe) == ".." {
		return "application"
	}
	return string(safe)
}

// fileStore is the [Store] a desktop application gets.
type fileStore struct {
	mu   sync.Mutex
	path string
}

// held is what the file holds: the cookies of each server, by address.
//
// By address, because an application run against a development server and then
// against a real one has two sessions, and one file holding both keeps them
// apart. A single list would sign somebody in to production with the session
// they were handed in development, or -- more often -- lose one every time they
// switch.
type held map[string][]storedCookie

// storedCookie is a cookie with the parts that decide whether it is still good.
//
// Only these. The rest of an http.Cookie describes how a browser should have
// treated it on the way in, and it arrived already treated.
type storedCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Path     string    `json:"path,omitempty"`
	Domain   string    `json:"domain,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HTTPOnly bool      `json:"httpOnly,omitempty"`
}

// alive reports whether a stored cookie is still within the life the server
// gave it.
//
// A cookie with no expiry is one the server meant to last as long as the
// session did, and here the session lasts as long as the application is
// installed. That is the whole point of this store and it is also a departure
// from what the server asked for, so it is written where somebody reading the
// code will find it: if the server wants a shorter life it says so with an
// expiry, and that is honoured.
func (s storedCookie) alive(now time.Time) bool {
	return s.Expires.IsZero() || s.Expires.After(now)
}

func (s *fileStore) Load(base string) ([]*http.Cookie, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.read()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cookies := make([]*http.Cookie, 0, len(all[base]))
	for _, c := range all[base] {
		if !c.alive(now) {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  c.Expires,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}
	return cookies, nil
}

func (s *fileStore) Save(base string, cookies []*http.Cookie) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.read()
	if err != nil {
		return err
	}

	now := time.Now()
	kept := make([]storedCookie, 0, len(cookies))
	for _, c := range cookies {
		stored := storedCookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  c.Expires,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
		}
		if !stored.alive(now) {
			// An expiry already past is how a server deletes a cookie. Writing
			// it down would mean a sign-out that comes back after a restart.
			continue
		}
		kept = append(kept, stored)
	}

	// Sorted, so that the same session writes the same bytes: a file whose
	// contents move about on every save is one nobody can diff, and one a
	// backup copies again for nothing.
	sort.Slice(kept, func(i, j int) bool { return kept[i].Name < kept[j].Name })

	if len(kept) == 0 {
		delete(all, base)
	} else {
		all[base] = kept
	}
	return s.write(all)
}

func (s *fileStore) Clear(base string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.read()
	if err != nil {
		return err
	}
	if _, held := all[base]; !held {
		return nil
	}
	delete(all, base)
	return s.write(all)
}

// read answers what the file holds, and nothing when there is no file.
//
// A file that cannot be parsed is treated as empty and left alone. The
// alternative is an application that refuses to start because a byte of its
// session file is wrong, and the worst that this costs is one sign-in.
func (s *fileStore) read() (held, error) {
	body, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return held{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ayra/client: the session could not be read: %w", err)
	}

	var all held
	if err := json.Unmarshal(body, &all); err != nil || all == nil {
		return held{}, nil
	}
	return all, nil
}

// write replaces the file, and replaces it whole.
//
// Written beside and renamed over, because a write interrupted halfway leaves a
// file that is neither the old session nor the new one -- and the next start
// reads it, finds nothing it can use, and asks for a password with no
// explanation. A rename on the same filesystem is the one step that cannot be
// caught halfway.
func (s *fileStore) write(all held) error {
	body, err := json.Marshal(all)
	if err != nil {
		return fmt.Errorf("ayra/client: the session could not be written: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("ayra/client: the session directory could not be made: %w", err)
	}

	temp, err := os.CreateTemp(dir, "session-*.tmp")
	if err != nil {
		return fmt.Errorf("ayra/client: the session could not be written: %w", err)
	}
	defer os.Remove(temp.Name())

	// The mode is set before anything is written, so the values are never on
	// disk under a mode another account could read them through.
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("ayra/client: the session file could not be made private: %w", err)
	}
	if _, err := temp.Write(body); err != nil {
		temp.Close()
		return fmt.Errorf("ayra/client: the session could not be written: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("ayra/client: the session could not be written: %w", err)
	}
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("ayra/client: the session could not be replaced: %w", err)
	}
	return nil
}

// MemorySession keeps a session for as long as the process lives, and is what
// a test wants.
//
// It exists so that a test of something that needs a session does not write one
// to the machine running the test -- and so that a store can be handed to a
// client without a file being the only way to do it.
func MemorySession() Store { return &memoryStore{held: map[string][]*http.Cookie{}} }

type memoryStore struct {
	mu   sync.Mutex
	held map[string][]*http.Cookie
}

func (m *memoryStore) Load(base string) ([]*http.Cookie, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return copyCookies(m.held[base]), nil
}

func (m *memoryStore) Save(base string, cookies []*http.Cookie) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.held[base] = copyCookies(cookies)
	return nil
}

// copyCookies answers cookies the caller owns.
//
// Copying the slice alone is not enough and looks like it is: the entries are
// pointers, so two slices hold one cookie each and writing through either
// changes what the store holds. The file store gets this for nothing -- it
// builds a cookie from what it read -- which is exactly why the one that keeps
// them in memory has to do it on purpose.
func copyCookies(cookies []*http.Cookie) []*http.Cookie {
	copied := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		if c == nil {
			continue
		}
		one := *c
		copied = append(copied, &one)
	}
	return copied
}

func (m *memoryStore) Clear(base string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.held, base)
	return nil
}

// sessionURL is the address a session is filed under.
//
// The scheme and the host, and nothing else: a session belongs to a server, not
// to a page of it, and filing it by the path the application happened to open
// with would give two sessions to one server.
func sessionURL(base *url.URL) string {
	return (&url.URL{Scheme: base.Scheme, Host: base.Host}).String()
}
