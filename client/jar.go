package client

import (
	"fmt"
	"net/http/cookiejar"
)

// newJar returns the cookie jar a client keeps its session in.
//
// A client without one signs in and is anonymous again on the next page,
// because the session the server established travels in a cookie and nothing
// would carry it. That failure is quiet -- every page answers, each one as a
// stranger -- so the jar is not optional and not a caller's decision.
func newJar() (*cookiejar.Jar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("ayra/client: the session store could not be opened: %w", err)
	}
	return jar, nil
}
