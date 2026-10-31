//go:build android

package push

import (
	"context"
	"sync"

	"github.com/arandu-io/ayra/internal/jni"

	// The permission to show a notification, which the packager reads from the
	// graph and writes into the manifest.
	_ "github.com/arandu-io/ayra/engine/app/permission/notifications"
)

// The Java half is io.arandu.ayra.push.AyraPush, which the packager compiles
// from this package's android directory when the package is in the graph.
const javaClass = "io.arandu.ayra.push.AyraPush"

func init() { current = &android{} }

// android is the backend of a phone running Android: Firebase Cloud Messaging.
type android struct {
	mu       sync.Mutex
	attached bool
	asking   []chan answer
}

type answer struct {
	permission Permission
	err        error
}

func (*android) kind() string { return "fcm" }

// ensure attaches the Java half. It is retried until the virtual machine is
// there, which it is not while the library is only being loaded.
func (a *android) ensure() {
	a.mu.Lock()
	attached := a.attached
	a.mu.Unlock()
	if attached {
		return
	}
	err := jni.Do(func(env jni.Env) error {
		class, err := env.Class(javaClass)
		if err != nil {
			return err
		}
		return env.CallStaticVoid(class, "attach", "(Landroid/content/Context;)V", jni.Obj(jni.Context()))
	})
	if err != nil {
		return
	}
	a.mu.Lock()
	a.attached = true
	a.mu.Unlock()
}

func (a *android) request(ctx context.Context) (Permission, error) {
	wait := make(chan answer, 1)
	a.mu.Lock()
	first := len(a.asking) == 0
	a.asking = append(a.asking, wait)
	a.mu.Unlock()

	if first {
		err := jni.Do(func(env jni.Env) error {
			class, err := env.Class(javaClass)
			if err != nil {
				return err
			}
			return env.CallStaticVoid(class, "request", "(Landroid/content/Context;)V", jni.Obj(jni.Context()))
		})
		if err != nil {
			a.settle(answer{Undetermined, err})
		}
	}

	select {
	case got := <-wait:
		return got.permission, got.err
	case <-ctx.Done():
		return Undetermined, ctx.Err()
	}
}

// settle hands one answer to everyone waiting for it.
func (a *android) settle(got answer) {
	a.mu.Lock()
	waiting := a.asking
	a.asking = nil
	a.mu.Unlock()
	for _, w := range waiting {
		w <- got
	}
}

// onPermission is the Java half reporting what the person chose. A problem it
// met on the way -- the application has no Firebase configuration -- comes as a
// code beside the permission, because the permission is still an answer.
func onPermission(status int, code string) {
	got := answer{permission: Undetermined}
	switch status {
	case 1:
		got.permission = Granted
	case 2:
		got.permission = Denied
	}
	if code == "not_configured" {
		got.err = ErrNotConfigured
	} else if code != "" {
		got.err = errorString(code)
	}
	current.(*android).settle(got)
}

type errorString string

func (e errorString) Error() string { return "push: " + string(e) }
