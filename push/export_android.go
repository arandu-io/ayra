//go:build android

package push

import "C"

import (
	"time"

	"github.com/arandu-io/ayra/internal/jni"
)

// The functions below are the native methods of io.arandu.ayra.push.AyraPush,
// named the way the virtual machine looks them up. Each runs on a thread of the
// machine, and each only hands what it was given to this package and returns:
// nothing here waits for the application.

//export Java_io_arandu_ayra_push_AyraPush_onToken
func Java_io_arandu_ayra_push_AyraPush_onToken(env, class, token uintptr) {
	hub.deliverToken(jni.EnvOf(env).GoString(token))
}

//export Java_io_arandu_ayra_push_AyraPush_onMessage
func Java_io_arandu_ayra_push_AyraPush_onMessage(env, class, id, title, body, data uintptr, sent int64, opened uint8) {
	e := jni.EnvOf(env)
	pairs := e.Strings(data)
	m := Message{ID: e.GoString(id), Title: e.GoString(title), Body: e.GoString(body)}
	if len(pairs) > 0 {
		m.Data = make(map[string]string, len(pairs)/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			m.Data[pairs[i]] = pairs[i+1]
		}
	}
	if sent > 0 {
		m.Sent = time.UnixMilli(sent)
	}
	if opened != 0 {
		hub.deliverOpened(m)
		return
	}
	hub.deliverMessage(m)
}

//export Java_io_arandu_ayra_push_AyraPush_onPermission
func Java_io_arandu_ayra_push_AyraPush_onPermission(env, class uintptr, status int32, code uintptr) {
	onPermission(int(status), jni.EnvOf(env).GoString(code))
}
