// Package debug provides general debug feature management for Gio, including
// the ability to toggle debug features using the AYRADEBUG environment variable.
package debug

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	debugVariable = "AYRADEBUG"
	textSubsystem = "text"
	silentFeature = "silent"
)

// Text controls whether the text subsystem has debug logging enabled.
var Text atomic.Bool

var parseOnce sync.Once

// Parse processes the current value of AYRADEBUG. If it is unset, it does nothing.
// Otherwise it process its value, printing usage info the stderr if the value is
// not understood. Parse will be automatically invoked when the first application
// window is created, allowing applications to manipulate AYRADEBUG programmatically
// before it is parsed.
func Parse() {
	parseOnce.Do(func() {
		val, ok := os.LookupEnv(debugVariable)
		if !ok {
			return
		}
		print := false
		silent := false
		for part := range strings.SplitSeq(val, ",") {
			switch part {
			case textSubsystem:
				Text.Store(true)
			case silentFeature:
				silent = true
			default:
				print = true
			}
		}
		if print && !silent {
			fmt.Fprintf(os.Stderr,
				`Usage of %s:
	A comma-delimited list of debug subsystems to enable. Currently recognized systems:

	- %s: text debug info including system font resolution
	- %s: silence this usage message even if AYRADEBUG contains invalid content
`, debugVariable, textSubsystem, silentFeature)
		}
	})
}
