# Upgrade guide

What changed in a way that stops your code compiling, and what to write instead.

Additions are not listed. A new symbol breaks nothing.

## Unreleased -- Kyse's icons

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

`widget.DataTable` is new and nothing that compiled changes. Three things are
worth knowing before writing a screen with it:

- `Result.Total` is negative for a source that does not count (a route that pages
  by position). The table then draws no total and offers the next page for as
  long as a page comes back full. A source that always counts is unaffected.
- The words default to Kyse's: `"{total} row(s)."` and `"Page {n} of {total}"`.
  A sentence copied from the web half's catalogue works unchanged; `{n}` and
  `{page}` and `{pages}` are still read.
- The sheet of filters is not dismissed by a press outside it: on a touch screen
  `Drawer` closes on the press that lands on its own buttons. It closes with its
  own button, and `DrawerProps.Dismissible` should not be set on a touch screen
  until that is fixed in `Drawer`.

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
