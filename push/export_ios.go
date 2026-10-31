//go:build ios

package push

/*
#include <stdlib.h>
*/
import "C"

import "unsafe"

// The functions below are called from push_ios.m, on whichever thread the
// system used. Each hands what it was given to this package and returns.

//export ayraPushToken
func ayraPushToken(token *C.char) { hub.deliverToken(C.GoString(token)) }

//export ayraPushPermission
func ayraPushPermission(status C.int) {
	p := Denied
	if status == 1 {
		p = Granted
	}
	if a, ok := current.(*apple); ok {
		a.settle(p)
	}
}

//export ayraPushMessage
func ayraPushMessage(id, title, body *C.char, kv **C.char, n C.int, opened C.int) {
	m := Message{ID: C.GoString(id), Title: C.GoString(title), Body: C.GoString(body)}
	if n > 0 {
		pairs := unsafe.Slice(kv, int(n))
		m.Data = make(map[string]string, int(n)/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			m.Data[C.GoString(pairs[i])] = C.GoString(pairs[i+1])
		}
	}
	if opened != 0 {
		hub.deliverOpened(m)
		return
	}
	hub.deliverMessage(m)
}
