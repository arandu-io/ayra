# Release Notes

## [v0.6.0](https://github.com/arandu-io/ayra/compare/v0.5.0...v0.6.0) - 2026-10-31

**Full Changelog**: https://github.com/arandu-io/ayra/compare/v0.5.0...v0.6.0

## v0.6.0 - 2026-10-31

### Added

- `widget`: each component draws, inside itself, the icons the Kyse component
  draws. The caret of the accordion, the collapsible, the combobox, the select
  and the split button is Kyse's `CaretDown`, and the accordion's turns half a
  turn when it opens; the tree's is `CaretRight`, turned a quarter; the
  breadcrumb's separator is `CaretRight`; the calendar's two steps and the date
  range picker's are `CaretLeft` and `CaretRight`; the date fields carry
  `CalendarBlank`; the number input's steps are `Minus` and `Plus`; the password
  field's reveal is `Eye` and `EyeSlash`; the rating's marks are `Star`, lit in
  the primary and unlit in the muted foreground at 40%, at the 24 points Kyse's
  stylesheet gives them; the checkbox's tick is `Check`; the command palette's
  field starts with `MagnifyingGlass`; the toast and the form dialog close with
  `X`; and the data table's sortable headings end in `ArrowsDownUp`, `ArrowUp`
  or `ArrowDown`, its filter button begins with `PlusCircle`, its filter menu
  marks the chosen options with `Check`, its clear button ends in `X` and its
  page-size button in `CaretUpDown`. The colour is the theme's in both schemes.
- `widget.InnerIcons` answers, by the name of the Kyse component each widget
  pairs with, the Kyse function names of the icons it draws inside itself. The
  widgets get these from one table (`widget/inner.go`), and a test fails on an
  `icons.Something()` anywhere else in the package.
- `icons_parity_test.go` reads Kyse's source, through `AYRA_KYSE` as
  `parity_test.go` does, and fails when a component's icons are not the ones its
  Kyse component draws. What cannot match is written down with its reason: the
  icons the browser draws from its stylesheet (the checkbox's tick, the select's
  chevron) are Lucide strokes behind a CSS variable that `theme/basecoat/base.css`
  defines, and the test pairs each with the nearest of Kyse's own set; the
  icons of a feature the native half does not have (the table's column menu,
  the password requirements, the sidebar's groups, the theme toggle's system
  mode) are listed under `withoutIcons`.
- `ButtonProps.IconOnly` draws the icon alone and keeps `Label` as the name a
  screen reader says, and `ButtonProps.Trailing` draws the icon after the label.
  `icons.Icon.WithoutShade` leaves out the lighter half, which is how Kyse draws
  `Check`.
- The names of the buttons that now draw an icon are properties, as in Kyse:
  `NumberProps.DecrementLabel` and `IncrementLabel`, `PasswordProps.ShowLabel`
  and `HideLabel`, `SplitProps.MenuLabel`, `FormDialogProps.CloseLabel` and
  `EditInPlaceProps.EditLabel`. `DeleteRowProps` takes an `Icon` and `IconOnly`
  as Kyse's does. `Secret.Reveal` shows or hides the value from the screen.
- `frame`: the pictures `inner-structure`, `inner-inputs`, `inner-overlays` and
  `inner-palette`, in both schemes at two densities, show every component whose
  inside changed.

### Changed

- The accordion, the collapsible and the tree no longer draw a filled triangle
  of their own: the caret is Kyse's, and the accordion's and the collapsible's
  sit at the end of the summary, where Kyse puts them, so the content under them
  is no longer indented to clear a marker at the start. The table's sort mark is
  no longer a triangle, and the calendar's steps, the number input's, the
  password reveal's, the split button's and the pagination's ellipsis no longer
  say `<`, `>`, `-`, `+`, `Show`, `Hide`, `...` in text where Kyse draws an
  icon; the ellipsis is the `…` Kyse writes.
- `DeleteRow` and `EditInPlace` no longer draw a trash and a pencil of their
  own, because Kyse's draw none: the first is a button that says "Delete" and
  takes an icon from the product, the second shows the value and "Edit".
- The toast shows the `X` that Kyse's has, and the form dialog the one in its
  corner unless it is an alert.

## [v0.5.0](https://github.com/arandu-io/ayra/compare/v0.4.0...v0.5.0) - 2026-10-31

**Full Changelog**: https://github.com/arandu-io/ayra/compare/v0.4.0...v0.5.0

## v0.5.0 - 2026-10-31

### Added

- `widget`: `DataTable`, the listing of Kyse's DataTable drawn natively, with the
  same state. `DataTableState`, `DataTableValues`, `DataTableChange` and
  `DataTableParams` are Kyse's, field for field, so one route serves the browser
  and the app: the address the app asks is the one the web table writes into its
  links (a search, an order, facets, a page and a size, a default left out, a
  repeated value once and in order). `compat/` runs the same inputs through Kyse
  and hesape's listing and compares what each writes and reads.
- `widget`: `ServerSource`, the `DataSource` that speaks an Arandu route. It asks
  the route's address with the table's state and the view media type, and reads
  the rows, the total and the facet counts from the `{view, data, token}` the
  handler answers with, at paths the application names. Any other `DataSource`
  -- `Source` is the same interface -- is the application's own, over the client
  and whatever credential it holds.
- `widget`: under 600 points (`Breakpoint`) a `DataTable` draws cards instead of
  the table: the primary column is the title, the others lines of label and
  value, the row actions are icons under them, and a column draws its cell with a
  widget of the application's own (`DataColumn.Cell`, `Row.Data`). The search is
  on top and the filters -- choice, text and period -- are in a sheet from the
  bottom that asks once. `Infinite` makes the list endless, and the list is
  pulled from its top to ask again.
- `widget`: `Live` tells a `DataTable` the rows changed on the server. The table
  asks again for every page on screen, swaps the answer in whole, and neither
  goes inert nor loses its place; `LiveEvent` forwards the `<table>.changed`
  frames of the web tables, and `DataTable.Notify` is the hook for any other way
  of knowing.

## [v0.4.0](https://github.com/arandu-io/ayra/compare/module/v0.3.0...v0.4.0) - 2026-10-31

**Full Changelog**: https://github.com/arandu-io/ayra/compare/module/v0.3.0...v0.4.0

## v0.4.0 - 2026-10-31

### Added

- `theme`: a widget is restylable the way a Kyse component is (ADR-0131). `Style`
  says what a class would -- background, foreground, border and its width,
  radius, padding, font, and the hover and pressed fills -- and every field left
  unset leaves the widget's own choice alone. A `Look` holds styles by widget and
  part, `Theme.Styles` carries the application's look, its named `Variants` and
  the `Tokens` of its stylesheet, and `Theme.Resolve` lays four layers over what
  a widget would draw on its own: the theme's look, the registered variant, the
  instance's `Style` and the instance's `Parts`. A variant is a name the product
  registers (`ComponentProps.Custom`), not a value of the closed enum, and a
  built-in one is restyled by its own name.
- `widget`: every `Props` type takes `ComponentProps`, publishes its parts in
  `PartNames()` and resolves them, except `Table`, `Toolbar` and `Player`, which
  are declared in `widget/root_test.go` with the branch that takes them. The
  three tests of ADR-0131 exist natively: every component takes an override, and
  it shows; every published part is resolved and every resolved part published;
  and the directory is read, so a `Props` type with no row fails. With nothing
  registered the pictures are byte-for-byte the ones that were approved.
- `theme.Typography` takes the scale's typefaces (`Sans`, `Monospace`) and the
  weights a widget's four resolve to, and `Typography.Missing` says which
  typefaces a font collection lacks; `shell.Run` refuses a configuration that
  names a face its fonts do not carry (`shell.MissingFaceError`).
- `theme/css` makes a `Theme` from a product's stylesheet, at run time
  (`css.Parse`, `Sheet.Theme`) or as committed Go (`css.Generate`, and the
  command `theme/css/cmd/themegen`). It reads `:root`, `@theme` and `.dark`,
  resolves `var()` per scheme, converts hex, `rgb()`, `hsl()` and `oklch()`, and
  steps over what is not a token. `shell.Config.Theme` and `frame.Options.Theme`
  take the result.
- `widget`: the primitives the widgets are written from are exported for the
  product's own: `Surface`, `DrawText`, `DrawIcon`, `DrawImage`, `Hairline`,
  `ControlRadius`, `Clickable`, `Announce`, `Fade` and `Press`.
- `mask`: formatting that keeps what a program stores. The package is a copy of
  `hesape/mask`, kept identical by a test, with `Mask`, `Money` (an amount typed
  from the cents) and `mask/brazil` naming CPF, CNPJ, phone, CEP, licence plate
  (old and Mercosul alike) and the real as shapes, not rules. `widget.Masked` is
  the field.
- `widget`: `OneTimeCode`, `DateRangePicker` with presets (`LastDays`,
  `ThisMonth`, `PreviousMonth`), `DateTimePicker` in its five kinds (date, time,
  datetime, month, week), `Combobox` and `ActiveSearch` over a `Lookup`,
  `FileUpload` over a `Chooser` and an `Uploader`, `FormDialog`, `HoverCard`,
  `DialogTrigger`, `DeleteRow`, `LazyLoad`, `OptimisticToggle`, `Output`,
  `RelativeTime`, `LoadMore`, `EditInPlace`, `InputGroup`, `ThemeToggle`,
  `ImageProps`, `ResponsiveImage`, and `IconProps`. `AvatarProps` draws a `Photo`;
  `ButtonProps` takes an `Icon`; the calendar takes the application's month and
  weekday names and marks a period.
- `icons`: the Phosphor set under Kyse's function names, generated from Kyse's
  package by `internal/icongen`; `icons/lookup` is the whole set by name.
- `ayra.Context.ChangeScheme` asks the window to draw in another palette from the
  next frame.
- `parity_test.go`: every component of Kyse has a pair here or a reason it does
  not (`HiddenInput` and `SkipLink` alone), and CI checks Kyse out so that the
  test fails instead of skipping.


### Changed

- `theme.Theme` carries a pointer, `Styles`, and stays comparable with `==`.
  `Typography` gains fields and is still written by key.
- The dialog's root part is its panel, not the overlay behind it.

## [v0.3.0](https://github.com/arandu-io/ayra/compare/module/v0.2.0...v0.3.0) - 2026-10-31

**Full Changelog**: https://github.com/arandu-io/ayra/compare/module/v0.2.0...v0.3.0

## [v0.3.0](https://github.com/arandu-io/ayra/compare/v0.2.0...v0.3.0) - 2026-10-31

### Fixed

- `widget`: a text field is focused by a tap anywhere on it. The field was
  painted across the room it was given, but the editor inside it measured only
  its text -- one point wide while empty -- whenever its column had no minimum
  width, which is every column a screen centres. A tap landed on the border, no
  editor was focused, and on Android the keyboard came up and every key it sent
  went nowhere: the sign-in of a phone app took no input. `Input` now asks its
  editor for the whole width, and the painted field, border and padding
  included, takes the tap and gives the editor the focus and the keyboard.
  Applies to `Password` and every `Kind`.

### Security

- `client`: a request never leaves the server the client was made for. A
  redirect to another host, port or scheme -- including https to http -- is
  refused before it is sent, so the CSRF token and a re-sent form body no longer
  reach it, and an answer that arrived from anywhere else is not read and its
  token is not remembered. The refusal wraps the new `client.ErrOffServer`.
- `client`: a request path is built from segments, each escaped on its own, so a
  value placed in a path can no longer move the request to another route or add
  query fields. `Get` and `Post` take a `client.Path` built with `client.At`;
  see UPGRADE.md.
- `widget`: a control drawn disabled -- by its own flag or inside a disabled
  context -- no longer reports a press or a key, and a checkbox, radio or switch
  drawn disabled no longer changes its value. The clickable beneath every
  control registers no target while it is disabled.
- `engine/widget`: a masked editor -- the password kind of `widget.Input` --
  ignores copy and cut, and the snippet it hands the platform input method (the
  hidden text area in the browser, the keyboard application on a phone) carries
  the mask rather than the text.
- `client`: an answer is refused with the new `*client.TooLargeError` once it
  holds more than `client.DefaultPageLimit` (8 MiB) after decompression, so a
  small compressed body can no longer inflate into hundreds of megabytes.
  `client.WithPageLimit` sets a different limit.
- `widget`: a pager and a carousel hold a button only for each page or position
  they show, and compute the row from the window, so a total sent by the server
  no longer costs an allocation of that size on every frame.
- `client`: a session cookie limited by `Max-Age` is kept with the expiry that
  age means, in the file store and in memory, so it ends when the server said it
  would instead of living on disk with no end; an age below zero deletes it.
- `widget`: a select whose choice is past the end of its options answers no
  choice, as it draws; a file picker does not draw, and so never reports, an
  entry whose name is empty, `.`, `..` or holds a separator or a NUL.
- `engine/widget`: a read-only editor refuses text that arrives as data -- a
  paste answered after it became read-only, or text dropped onto it.

## [v0.2.0](https://github.com/arandu-io/ayra/compare/module/v0.1.2...v0.2.0) - 2026-10-25

**Full Changelog**: https://github.com/arandu-io/ayra/compare/module/v0.1.2...v0.2.0

## v0.1.0 - 2026-09-14

First release.
