/*
Package paint fills areas with a colour, a gradient or a picture.

There is one operation that marks anything, [PaintOp], and it fills the current
clip with the current brush under the current transformation. Everything drawn
is that operation with a different area pushed in front of it: a button's
shape, a row's background, the ground under a screen. Nothing here decides
where a mark ends -- the clip does, and what falls outside it is discarded
rather than clamped.

The brush is state and not an argument. It is set once and used by every fill
that follows until something sets it again, which is what makes a list of a
hundred rows in one colour cost one brush rather than a hundred. Three
operations set it: [ColorOp] for a single colour, [LinearGradientOp] for a
colour that changes along a line, and [ImageOp] for a picture.

Colours are [color.NRGBA] in the sRGB space, with alpha held apart from the
three channels rather than multiplied into them. That is what lets a colour
survive being stored and read back: premultiplication loses precision as alpha
falls, and at alpha nought it loses the colour entirely, so a value that has
been through it cannot be edited afterwards.
*/
package paint
