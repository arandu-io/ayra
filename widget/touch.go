package widget

import "github.com/arandu-io/ayra/engine/unit"

// touchTarget is the least height, in points, of a row or a strip a finger has
// to hit: the tabs, the summary of a section, a step of a breadcrumb and a
// destination of the navigation. The text in them is 17 points tall, and a row
// that is only as tall as its text is one a thumb misses.
const touchTarget unit.Dp = 44
