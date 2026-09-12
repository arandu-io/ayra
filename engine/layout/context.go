package layout

import (
	"time"

	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
)

// Context is the per-frame state a widget uses to measure, draw and receive
// input.
//
// Its zero value has no events or operation list, uses a one-to-one display
// metric and carries the zero time.
type Context struct {
	// Constraints are the minimum and maximum size of the active widget.
	Constraints Constraints

	// Metric converts logical lengths and text sizes to device pixels.
	Metric unit.Metric
	// Now is the time represented by the current frame.
	Now time.Time

	// Locale describes the system's language preferences.
	// BUG(whereswaldon): this field is not currently populated automatically.
	// Interested users must look up and populate these values manually.
	Locale system.Locale

	// Values carries application-wide data. Widgets should not use it for their
	// own state.
	Values map[string]any

	input.Source
	*op.Ops
}

// Dp converts a device-independent length to whole device pixels.
func (c Context) Dp(v unit.Dp) int {
	return c.Metric.Dp(v)
}

// Sp converts a scaled text size to whole device pixels.
func (c Context) Sp(v unit.Sp) int {
	return c.Metric.Sp(v)
}

// Disabled returns a copy that does not deliver events. Drawing state,
// constraints and application values are unchanged.
func (c Context) Disabled() Context {
	c.Source = c.Source.Disabled()
	return c
}
