//go:build ios

package push

/*
#cgo CFLAGS: -fobjc-arc -fmodules
#cgo LDFLAGS: -framework UIKit -framework UserNotifications

#include "push_ios.h"
*/
import "C"

import (
	"context"
	"sync"

	// The remote-notification background mode and the aps-environment
	// entitlement, which the packager reads from the graph.
	_ "github.com/arandu-io/ayra/engine/app/permission/notifications"
)

func init() { current = &apple{} }

// apple is the backend of a phone running iOS: the Apple Push Notification
// service.
//
// The token is the APNs device token and not a Firebase registration token.
// Asking Firebase for one would put its iOS SDK, and every library it carries,
// in the graph of every application that imports this package; a server that
// sends through Firebase maps the APNs token to a registration token with
// Firebase's own import call, and that is where the cost belongs.
type apple struct {
	once   sync.Once
	mu     sync.Mutex
	asking []chan answer
}

type answer struct {
	permission Permission
	err        error
}

func (*apple) kind() string { return "apns" }

func (a *apple) ensure() { a.once.Do(func() { C.ayra_push_ensure() }) }

func (a *apple) request(ctx context.Context) (Permission, error) {
	wait := make(chan answer, 1)
	a.mu.Lock()
	first := len(a.asking) == 0
	a.asking = append(a.asking, wait)
	a.mu.Unlock()

	if first {
		C.ayra_push_request()
	}
	select {
	case got := <-wait:
		return got.permission, got.err
	case <-ctx.Done():
		return Undetermined, ctx.Err()
	}
}

func (a *apple) settle(p Permission) {
	a.mu.Lock()
	waiting := a.asking
	a.asking = nil
	a.mu.Unlock()
	for _, w := range waiting {
		w <- answer{permission: p}
	}
}
