//go:build darwin

package shell

/*
#cgo CFLAGS: -fobjc-arc -fmodules
#cgo ios LDFLAGS: -framework UIKit
#cgo !ios LDFLAGS: -framework AppKit

#include "reducemotion_darwin.h"
*/
import "C"

// systemReduceMotion is UIAccessibility.isReduceMotionEnabled on iOS and
// NSWorkspace.accessibilityDisplayShouldReduceMotion on macOS, which are the
// same switch under the two systems' names for it.
func systemReduceMotion() bool { return C.ayraSystemReduceMotion() != 0 }
