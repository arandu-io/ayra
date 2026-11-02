package shell

import "github.com/arandu-io/ayra/engine/unit"

// keyboardLeast is the least the bottom inset has to grow beyond what the
// system's own bars leave for it to be taken as a keyboard, in points. The
// navigation bar is 48 and a gesture bar 24; the shortest keyboard is over 200.
const keyboardLeast unit.Dp = 120

// keyboardWatch tells the keyboard from the bars of the system.
//
// The platform reports one bottom inset, which is the navigation bar while the
// keyboard is down and the keyboard when it is up. What the bars leave is what
// that inset is at rest, and the lowest value it has been is the best account of
// it: a window opens with the keyboard down, and a field is what raises it.
type keyboardWatch struct {
	rest unit.Dp
	seen bool
}

// observe takes the bottom inset of a frame and answers the height of the
// keyboard in points, or zero when there is none.
func (k *keyboardWatch) observe(bottom unit.Dp) unit.Dp {
	if !k.seen || bottom < k.rest {
		k.rest, k.seen = bottom, true
	}
	if extra := bottom - k.rest; extra >= keyboardLeast {
		return extra
	}
	return 0
}
