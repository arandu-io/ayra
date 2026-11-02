# Upgrade guide

What changed in a way that stops your code compiling, and what to write instead.

Additions are not listed. A new symbol breaks nothing.

## Unreleased -- translucent colours blend as the browser blends them

Nothing stops compiling, and every picture of the dark scheme changes. The engine
blended translucent ink in straightened light and a browser blends in sRGB, so
white at 15% over black came out at 108 of 255 in the app and 38 on the site. It
now comes out at 38, over any ground, for every alpha: a border, an input, a
hover wash, a scrim, a disabled label, a badge and the antialiased edge of a
shape. A widget changes nothing to get it.

What moves in the engine: the frame, the layers and the images are 8-bit
formats the hardware does not bend (`TextureFormatRGBA8`; `TextureFormatSRGBA` is
gone), a colour reaches the rasteriser as `f32color.PremulFromSRGB`, and the
surface of a window is not asked to be sRGB (`BGRA8Unorm` on Metal, an `*_UNORM`
swapchain on Vulkan and Direct3D 11, no `EGL_GL_COLORSPACE_SRGB`, no
`FRAMEBUFFER_SRGB`, and the intermediate framebuffer that emulated one on OpenGL
ES is deleted).

An application that drew its own Metal layer or its own swapchain for the engine
gives it the non-sRGB format: `layer.pixelFormat = MTLPixelFormatBGRA8Unorm`.
A golden picture of yours that carries translucent ink or antialiased edges is
regenerated; a workaround that pre-mixed a translucent colour over its ground to
get the browser's look (a field's fill, a faded label) is no longer needed.

## v0.8.0 -- the keyboard is the window's, not the application's

An application that opened its own window loop to keep a field above the keyboard
-- its own `app.Window`, an `app.NewContext` followed by a `layout.Inset` of the
frame's insets, a check of `e.Insets.Bottom` against a height to guess the
keyboard -- deletes it and calls `shell.Run`. That loop is what made the form
vanish and the keyboard close: `app.NewContext` has already taken the insets off the
context, so the `layout.Inset` took them off a second time, and with the keyboard
up the room left was too small to draw the focused field in. A router drops the
focus of a control that is not in the frame, together with the keyboard raised for
it.

```go
// before: cmd/native/window.go, runWindow, frames
// now
shell.Run(shell.Config{Title: "Nivela", Scheme: scheme, Fonts: fonts, Theme: theme}, app.Layout)
```

`shell.Run` already paints the strips under the status and navigation bars with the
palette and colours the bars; it applies the insets of the bars and of the keyboard
once; it scrolls the focused field above the keyboard with a margin; and it keeps
the input connection. A screen that scrolls does so with a `layout.List` (or any
control that takes scroll), as before: nothing in the screen mentions the keyboard,
and a screen that kept its own "scroll the touched block into view" code can delete
that too.

`Router.RevealFocus` now returns a `bool`. A call that ignored it compiles as it was.

`PasswordProps` draws the eye inside the field. A screen that laid the field out
by its width and the eye's separately has nothing to change; one that set the part
`root` to restyle the field restyles `group` as well.

## v0.7.0 -- the system theme

Nothing that compiled stops compiling. `app.Config`, `ayra.Context`, `theme.Theme`
and `shell.Config` have more fields, so a literal written with positional values
needs the names. Three things are worth knowing.

A window follows the system when it is told to; one that is not keeps the scheme
it opened with, as before:

```go
shell.Run(shell.Config{
	Title:  "Alpr.sh",
	Mode:   theme.System, // Scheme is used until the system says, and where it cannot
	Locale: theme.PortugueseBrazil(),
	OnLifecycle: func(l ayra.Lifecycle) {
		// stop the socket on ayra.Background, start it on ayra.Foreground
	},
}, screen)
```

A theme toggle keeps working: `ChangeScheme` is now a choice that wins over the
system, and `FollowSystem` is the "Automático" that undoes it.

A dismissible `Dialog` or `Drawer` takes the keyboard once when it opens and no
longer on every frame, and reads escape whatever holds the focus. A screen that
relied on the panel keeping the focus away from a field inside it has the field
working now.

On Android the app has to be built with `aru` from this change or later: the
activity declares `uiMode` among the configuration changes it handles, and
without it the system recreates the activity when the scheme changes.

## v0.6.0 -- Kyse's icons

Nothing that compiled stops compiling, and the pictures of an application that
keeps its own goldens will change: the widgets draw inside themselves the icons
the Kyse component draws, where they drew a path of their own or a word.

- `DeleteRow` drew a trash and `EditInPlace` a pencil. Kyse's draw neither, so
  `DeleteRow` is now a button that says "Delete" (`Label` changes the word) and
  `EditInPlace` shows the value and "Edit" (`EditLabel`). To keep an icon on the
  first, pass it: `widget.DeleteRowProps{Icon: icons.Trash(), IconOnly: true, Label: "Delete invoice 114"}`.
- The two steps of a calendar and the buttons of a number input were named by
  their text, `<`, `>`, `-` and `+`. They draw an icon now and are named for a
  screen reader by `PreviousLabel` and `NextLabel` ("Previous month", "Next
  month" when empty) and by `DecrementLabel` and `IncrementLabel`.
- A rating's stars are 24 points and lit in the primary colour, as Kyse's are.
- A `ButtonProps` whose `Icon` is set and whose `Label` is empty was a button
  with no name. Set `IconOnly: true` and `Label` to the name.

## v0.5.0 -- the data table

`widget.DataTable` is new and nothing that compiled changes. Two things are
worth knowing before writing a screen with it:

- `Result.Total` is negative for a source that does not count (a route that pages
  by position). The table then draws no total and offers the next page for as
  long as a page comes back full. A source that always counts is unaffected.
- The words default to Kyse's: `"{total} row(s)."` and `"Page {n} of {total}"`.
  A sentence copied from the web half's catalogue works unchanged; `{n}` and
  `{page}` and `{pages}` are still read.

## v0.4.0 -- restylable widgets

Nothing that compiled stops compiling for an application that uses the widgets as
they were: every addition is a field a keyed literal leaves at zero, and the
approved pictures are unchanged. Three things are worth knowing.

A `Props` literal written with positional fields, `widget.BadgeProps{"x", widget.Default}`,
stops compiling, because `ComponentProps` is now its first field. Write the
fields by name.

`theme.Typography` has more fields; one written positionally needs the names.

The way to make an application look like its web panel is a theme and not a copy
of the values:

```go
sheet, _ := css.Parse(customCSS) // the product's own stylesheet, embedded
shell.Run(shell.Config{Title: "Alpr.sh", Theme: sheet.Func(), Fonts: fonts}, screen)
```

and a variant of the product's own, which the closed `Variant` enum never allowed:

```go
th.Styles = &theme.Styles{Variants: map[string]theme.Look{
	"read": {"badge": {"root": {Background: theme.Colour(th.Token("tone-read"))}}},
}}
// ...
widget.BadgeProps{Label: "Lida", ComponentProps: widget.ComponentProps{Custom: "read"}}
```

A widget the product writes itself is written from the exported primitives and is
restylable the same way:

```go
root := c.Theme.Resolve("plate", "root", "", theme.Style{
	Background: theme.Colour(c.Theme.Colours.Card),
	Padding:    theme.InsetXY(10, 4),
}, theme.Component{})
widget.Surface(c, root, func(c ayra.Context) ayra.Dimensions {
	return widget.DrawText(c, "ABC1D23", root, widget.TextOptions{Monospace: true})
})
```

## v0.3.0 -- a request path is built from segments

`client.Client.Get` and `client.Client.Post` take a `client.Path` instead of a
string. A path is built with `client.At`, one argument per segment, and a query
with `Path.Query`:

```go
c.Get(ctx, "/")                                   // before
c.Get(ctx, client.At())                           // now

c.Post(ctx, "/login", form)                       // before
c.Post(ctx, client.At("login"), form)             // now

c.Get(ctx, "/teams/"+slug+"/members?page=2")      // before
c.Get(ctx, client.At("teams", slug, "members").
	Query(url.Values{"page": {"2"}}))             // now
```

A string spliced into a path was read as whatever its characters spelled: a
slug of `../admin/users/7?role=admin#` moved the request to another route and
added a field the server merges into its input. Each segment is now escaped on
its own, so a `/`, `?` or `#` inside one stays inside it, and the query arrives
only through `Path.Query`. A segment that is empty, `.` or `..` is refused with
`client.ErrSegment`, because escaping does not change those.

A path is always from the server's root, as an absolute string path was
before. The six files `aru` publishes for a native application use the new
form; a project published earlier has three calls to change, in `app.go`,
`home.go` and `sign_in.go`.
