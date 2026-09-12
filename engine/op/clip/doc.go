/*
Package clip confines what a drawing is allowed to touch.

Nothing here paints. A clip answers where, and it is the half of every drawing
that decides whether a control stays inside the room it was given: a label that
overruns its button, a row that draws past the end of the list holding it, a
panel whose background covers its neighbour. What colour, and with which brush,
belongs elsewhere.

The area in force starts as everything. Pushing one narrows it to what it
already was intersected with what was pushed, and releasing restores what was
in force before. It only ever narrows, and that is what makes controls safe to
nest: a child cannot reach outside its parent by claiming a larger area,
however large the area it claims.

A rectangle is the common case and travels as one -- four numbers, pixel
aligned, with nothing to rasterise. Everything else is built from [Path]:
a rounded corner, an ellipse, an arbitrary outline, and the widened band of a
[Stroke]. What is inside a path is decided by counting crossings, so a contour
left open does not fill with a notch in it; the count escapes through the gap
and the fill runs away across everything beyond.
*/
package clip
