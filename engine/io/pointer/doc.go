/*
Package pointer is what a mouse, a finger and a pen have in common.

The three arrive as one kind of event with a [Source] saying which it was,
because almost nothing a control does differs between them: a button is pressed
the same way by all three, and the handful of places where it matters -- a hover
that a finger cannot perform, a scroll wheel a finger does not have -- read the
field. One vocabulary means a control written for a desk works on a phone
without being written twice.

A control does not receive events by existing. It declares itself with event.Op
and asks for what it wants with a [Filter]:

	area := clip.Rect(bounds).Push(ops)
	event.Op(ops, control)
	area.Pop()

	for {
		e, ok := source.Event(pointer.Filter{
			Target: control,
			Kinds:  pointer.Press | pointer.Release,
		})
		if !ok {
			break
		}
		...
	}

The tag passed to event.Op and named as the filter's Target is an identity, not
a value: two controls are two tags, and one tag used twice is one control that
answers for both places it was drawn.

# Hit areas

The clip pushed around the declaration is the hit area. This is deliberate
rather than incidental -- a control that carried its own rectangle would carry a
second one, and the two would disagree the first time a layout moved something.
What is drawn is what is touched, because it is the same shape.

A rectangle holds its low edge and not its high one. A press at Min lands
inside; a press at Max lands outside; the last position inside is anything
strictly below Max in both axes. The reason is that controls are laid edge to
edge, so the high edge of one is the low edge of the next: a closed boundary
would put that seam in two hit areas and a press there would reach two controls,
and an open one would put it in neither and leave a dead line between every pair
of buttons. Holding one side and not the other gives the seam to exactly one
control, whichever seam it is.

Areas nest, and nesting intersects. An event is inside an area only if it is
also inside every area enclosing it, which is the same rule painting follows: a
control clipped by a scrolling viewport is not pressable where it is not
visible. It is why a row scrolled half out of a list stops answering at the edge
of the list rather than at the edge of the row.

Only a rectangle and an ellipse are hit-tested as themselves. Every other clip
-- a rounded rectangle, an arbitrary path -- is approximated by its bounding
box, so a press in the corner outside a rounded rectangle's curve still counts
as inside it. For a control this is usually invisible and occasionally generous;
for one that relies on the shape to separate it from a neighbour, it is wrong,
and the way out is to give it a rectangle or an ellipse.

[Event.Position] arrives in the receiving control's own coordinates, undoing
whatever transforms were in effect where it was declared. A control that was
translated, scaled or rotated reads the same numbers it would have read at the
origin, so it can compare a press against its own size rather than against the
window's.

# Overlapping controls

Areas form a tree in the order they were pushed, and the search for a match runs
from the last one backwards. Later means nearer: a control drawn after another
is in front of it on the screen, and it is in front of it here.

What a match stops is the search among siblings, not the search. The first area
that contains the event and has a handler attached takes it, and everything
drawn before it at the same level is skipped: a dialog drawn over a form takes
the press and the form behind it receives nothing, which is the whole of what
makes a dialog modal to the pointer. An area that contains the event but has no
handler, or whose handlers are all pass-through, does not stop anything and the
search carries on to the area before it.

Enclosing areas are not skipped. The search resumes at the parent, so a handler
declared on a container receives the event as well as the handler on the control
inside it. In front is a relation between siblings; a control is never in front
of what it is inside.

# Pass-through

[PassOp] marks every handler declared inside it as one that does not stop the
search. Both it and whatever is behind it receive the event.

This is what lets an overlay be transparent to the pointer without being
transparent to the eye, and the case it exists for is a control whose hit area
is deliberately larger than its ink: the edge strip that opens a drawer covers a
band of the interface, and the interface underneath has to keep working while
the drawer is shut.

# A pressed pointer keeps its handlers

Before a [Press], every handler whose area contains the pointer receives
everything. The moment a pointer is pressed, the set of handlers that matched is
recorded, and it stops following the pointer: dragging out of a button still
delivers [Drag] to that button, and dragging into another does not deliver
anything to it. A press belongs to what was under it when it went down, which is
also what lets a person press a button, notice the mistake, slide off it and
release without acting.

Handlers leave that set two ways. One is by no longer being declared -- a
control that stops drawing stops receiving. The other is [GrabCmd]: a handler
that sends one claims the pointer for itself, every other handler in the set is
sent a [Cancel], and what it had begun -- a highlight, a half-drawn selection --
is what the Cancel tells it to undo.

# Priorities

[Event.Priority] tells a handler whether it is alone. [Shared] means the set
holds more than one handler and the gesture is still ambiguous; [Grabbed] means
the set is down to this one.

The ambiguity is real and common: a finger on a row of a scrolling list has not
yet said whether it is a tap on the row or a drag of the list. Both handlers
match, both are told Shared, and each does the part that is safe to undo -- the
row lights up, the list does not move. When the finger travels far enough to
settle it, the scroll handler grabs; the row receives a Cancel and drops its
highlight, and the scroll handler sees Grabbed from then on. Had the finger
lifted first, the row would have had its tap.

# Scrolling

[Scroll] is not part of that: it is not bound to a tracked pointer, carries no
[Event.PointerID], and is delivered by position alone.

A handler declares how much of it it can use with [Filter.ScrollX] and
[Filter.ScrollY], and receives no more than that. What it does not take is
offered to the next handler under the pointer, which is what makes a list inside
a list scroll the way a reader expects: the inner one moves until it reaches its
end, and the rest of the gesture moves the outer one.
*/
package pointer
