package pointer

import (
	"strconv"

	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/op"
)

// Cursor is the shape the mouse is drawn as over an area.
//
// It is a shape a control asks for and never a picture it supplies: the set is
// closed, and every value in it is one the host window system already draws.
// A cursor of one's own would be an image this library had to scale for the
// display, colour for the theme and hide on a device with no mouse, and it
// would be visibly not the cursor the rest of the machine uses.
//
// The shape is a promise about what a press will do, which is why the set
// distinguishes so many arrows: one that says resize where nothing resizes is
// worse than the plain arrow, because the plain arrow promised nothing.
type Cursor byte

const (
	// CursorDefault is the ordinary arrow.
	CursorDefault Cursor = iota
	// CursorNone hides the cursor. Any other cursor shows it again.
	CursorNone
	// CursorText is for selecting and inserting text.
	CursorText
	// CursorVerticalText is for selecting and inserting vertical text.
	CursorVerticalText
	// CursorPointer is for something that can be followed, such as a link. It
	// is drawn as a pointing hand rather than the arrow its name suggests.
	CursorPointer
	// CursorCrosshair is for picking an exact position.
	CursorCrosshair
	// CursorAllScroll is for scrolling in every direction, drawn as arrows to
	// all four.
	CursorAllScroll
	// CursorColResize is for resizing a column: a vertical bar with arrows
	// east and west.
	CursorColResize
	// CursorRowResize is for resizing a row: a horizontal bar with arrows
	// north and south.
	CursorRowResize
	// CursorGrab is for content that can be picked up and moved, drawn as an
	// open hand.
	CursorGrab
	// CursorGrabbing is for content being moved, drawn as a closed hand. It is
	// separate from CursorGrab because the difference is the feedback that the
	// drag actually started.
	CursorGrabbing
	// CursorNotAllowed is for a target that will refuse what is being carried
	// to it, drawn as a circle with a line through.
	CursorNotAllowed
	// CursorWait is for work that blocks the interface, drawn as an hourglass
	// or whatever the host uses for one.
	CursorWait
	// CursorProgress is for work that does not block it: the ordinary arrow
	// with an hourglass beside it. The distinction is the whole point -- it
	// tells a person whether clicking now is worth trying.
	CursorProgress
	// CursorNorthWestResize is for resizing by the top-left corner.
	CursorNorthWestResize
	// CursorNorthEastResize is for resizing by the top-right corner.
	CursorNorthEastResize
	// CursorSouthWestResize is for resizing by the bottom-left corner.
	CursorSouthWestResize
	// CursorSouthEastResize is for resizing by the bottom-right corner.
	CursorSouthEastResize
	// CursorNorthSouthResize is for resizing top and bottom together, drawn as
	// one arrow with two ends.
	CursorNorthSouthResize
	// CursorEastWestResize is for resizing left and right together, drawn as
	// one arrow with two ends.
	CursorEastWestResize
	// CursorWestResize is for resizing by the left edge alone.
	CursorWestResize
	// CursorEastResize is for resizing by the right edge alone.
	CursorEastResize
	// CursorNorthResize is for resizing by the top edge alone.
	CursorNorthResize
	// CursorSouthResize is for resizing by the bottom edge alone.
	CursorSouthResize
	// CursorNorthEastSouthWestResize is for resizing along the top-right to
	// bottom-left diagonal, drawn as one arrow with two ends.
	CursorNorthEastSouthWestResize
	// CursorNorthWestSouthEastResize is for resizing along the top-left to
	// bottom-right diagonal, drawn as one arrow with two ends.
	CursorNorthWestSouthEastResize
)

// cursorNames is the text of each cursor, indexed by its value.
//
// The names drop the Cursor prefix that the constants carry, because the
// prefix is there to keep them apart from everything else in the package and
// says nothing in a line that is already about a cursor.
var cursorNames = [...]string{
	CursorDefault:                  "Default",
	CursorNone:                     "None",
	CursorText:                     "Text",
	CursorVerticalText:             "VerticalText",
	CursorPointer:                  "Pointer",
	CursorCrosshair:                "Crosshair",
	CursorAllScroll:                "AllScroll",
	CursorColResize:                "ColResize",
	CursorRowResize:                "RowResize",
	CursorGrab:                     "Grab",
	CursorGrabbing:                 "Grabbing",
	CursorNotAllowed:               "NotAllowed",
	CursorWait:                     "Wait",
	CursorProgress:                 "Progress",
	CursorNorthWestResize:          "NorthWestResize",
	CursorNorthEastResize:          "NorthEastResize",
	CursorSouthWestResize:          "SouthWestResize",
	CursorSouthEastResize:          "SouthEastResize",
	CursorNorthSouthResize:         "NorthSouthResize",
	CursorEastWestResize:           "EastWestResize",
	CursorWestResize:               "WestResize",
	CursorEastResize:               "EastResize",
	CursorNorthResize:              "NorthResize",
	CursorSouthResize:              "SouthResize",
	CursorNorthEastSouthWestResize: "NorthEastSouthWestResize",
	CursorNorthWestSouthEastResize: "NorthWestSouthEastResize",
}

// Add sets the cursor for the clip area currently in effect.
//
// It attaches to the area rather than to a handler, so a shape covers exactly
// what is drawn and needs no pointer events to appear: an area that does
// nothing but change the cursor declares no tag at all, which is what a
// resize edge is.
//
// The shape shown is the first one found searching from the frontmost area
// under the pointer outward through the areas containing it. An area that sets
// none defers to whatever encloses it, so a container can dress everything
// inside it and a control can still override that for itself.
func (c Cursor) Add(o *op.Ops) {
	data := ops.Write(&o.Internal, ops.TypeCursorLen)
	data[0] = byte(ops.TypeCursor)
	data[1] = byte(c)
}

// String names the cursor, or says the number of one this package does not
// name, for the reason given on [Kind.String].
func (c Cursor) String() string {
	if int(c) < len(cursorNames) {
		return cursorNames[c]
	}
	return "Cursor(" + strconv.FormatUint(uint64(c), 10) + ")"
}
