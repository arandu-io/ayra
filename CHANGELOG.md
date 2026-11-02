# Release Notes

## Unreleased

### Added

- `push`: push notifications on the device. `Request` asks for the permission
  and starts the registration, `Token` and `Tokens` answer the token,
  `Messages` delivers what arrives while the application is in front and
  `Opened` the notification a person touched, with its link. The channels hold
  what arrived before anybody read, so the notification that started the
  application is not lost. Android is Firebase Cloud Messaging; iOS is the APNs
  token, and `Kind` says which.
- `live`: the live video a camera's node serves over a WebSocket. `Stream`
  reads the node's format, joins a picture sent in pieces, renews the session
  when the node ends it with code 4001, and tells busy, full and lost apart.
  `Player` adds a hardware decoder and the frame to draw, and closes the socket
  when nothing has drawn the picture for five seconds. MediaCodec on Android,
  VideoToolbox on iOS and macOS; the other targets have no decoder.
- `widget.VideoProps` and `widget.Video`: a live video drawn uncropped, with a
  caption over a dimmed picture and an overlay hook that is given the picture's
  rectangle and the camera's own pixel size.
- `engine/app/permission/notifications`: the permission package for
  notifications.
- The Android activity tells native capabilities about the intents it receives
  and the permission results, which is how a notification that was touched
  reaches `push`.
- `client`: a refusal keeps the start of its body. `StatusError` gains `Body`
  (at most `ErrorBodyLimit`, 64 KiB), `ContentType` and `Into`, which decodes
  the server's error envelope, so a screen can act on the code the server gave
  with a 401 or a 403 and not only on the number.
- `client.WithBearer` sends `Authorization: Bearer` on every request, for a
  server that issues tokens. The provider is called under a lock, so concurrent
  requests wait for one renewal, and a request refused with 401 is repeated
  once after the provider is told which token was refused. The token is sent
  only to the server the client was made for.
- `client.Client.Download` fetches bytes -- an image, a document -- under the
  same credential and the same refusal to leave the server, with its own limit
  (`DefaultDownloadLimit`, `WithDownloadLimit`).

## [v0.8.0](https://github.com/arandu-io/ayra/compare/v0.7.0...v0.8.0) - 2026-11-01

### Fixed

- `engine/io/input`, `engine/app`, `engine/layout`: a text field low on a screen
  keeps its keyboard when the keyboard comes up. The keyboard takes the bottom of
  the window, the screen is laid out again in what is left, and a list in that
  room draws only the rows that fit -- and the field the person tapped could be
  one it left out. The router gives up the focus of a control that is not in the
  frame, and closes the keyboard raised for it, so the keyboard opened, closed in
  the same breath, and every key it had sent went nowhere. The window now holds the
  focus through the frames it takes to bring the field into view
  (`Router.HoldFocus`, `Router.ReleaseFocus`), scrolls toward the field where it
  was even when it is not drawn (`Router.BeginReveal`, `Router.ContinueReveal`,
  `Router.RevealFocus`, which now answers whether the field is in view), keeps a
  margin of 24 dp between the field and the keyboard, and does it again whenever a
  field asks for the keyboard, so a tap on another field with the keyboard up
  brings that one into view too. `layout.List` offers the scroll it is asked for
  in the first frame in a room of another size: the range it kept described the
  room before, and a form that fitted its window offered nothing to scroll by.
  The input connection stays: the editor is not told it lost the focus when it
  comes back, and the input method is not restarted. No application needs a window
  loop of its own for this; see UPGRADE.md.
- `widget`: the password's eye is inside the field, at its right end, and not
  beside it. Kyse draws the box and the control in one input group with one border;
  `PasswordProps` laid the control outside that border. The group draws the border
  -- focus ring, rejected colour -- and the input in it draws none. The group is
  published as the part `group`, and `Secret` keeps a `Button` for the eye.

**Full Changelog**: https://github.com/arandu-io/ayra/compare/v0.7.0...v0.8.0

## [v0.7.0](https://github.com/arandu-io/ayra/compare/v0.6.0...v0.7.0) - 2026-10-31

**Full Changelog**: https://github.com/arandu-io/ayra/compare/v0.6.0...v0.7.0

## v0.7.0 - 2026-10-31

### Added

- `engine/app`: the system's scheme. `Config.Appearance` is `AppearanceLight`,
  `AppearanceDark` or `AppearanceUnknown`, and a `ConfigEvent` carries each change
  while the window is open, delivered at once and without recreating anything.
  Android reads `uiMode` from the resources (`AyraView.getUiMode`) on
  `onConfigurationChanged`, iOS the trait collection of the view
  (`traitCollectionDidChange`), macOS the effective appearance of the view
  (`viewDidChangeEffectiveAppearance`), Windows the `AppsUseLightTheme` value and
  `WM_SETTINGCHANGE`, and the browser `prefers-color-scheme` and its change event.
  Linux (X11 and Wayland) reports `AppearanceUnknown`: there is no settings portal
  client in the engine, and a guess would draw the light palette on a dark desktop.
- `shell`: `Config.Mode` takes `theme.System` to follow the operating system, and
  `theme.Fixed`, the zero value, keeps the scheme asked for as before. A scheme
  chosen with `Context.ChangeScheme` wins over the system for as long as it stands;
  `Context.FollowSystem` hands the decision back, taking the scheme the system is
  in now. `Context.Mode` says which is in force. `theme.Mode` is the new type.
- `shell`: the status and navigation bars are painted with the background of the
  palette, and the icons on them are dark on the light palette and light on the
  dark one, when the window opens and each time the scheme changes. The whole
  window, the strips under the bars included, is painted with the background:
  from Android 15 the bars are transparent and the strip showed the surface's white
  clear colour.
- `engine/app`, `shell`, `ayra`: the application's stage. `app.StageEvent` says
  when the application goes to the background and comes back -- the Android
  activity stopped or started, the iOS app in the background, a window minimised
  or hidden on macOS and Windows, a browser tab hidden. `shell.Config.OnLifecycle`
  receives `ayra.Foreground`, `ayra.Background` and `ayra.Destroyed`, once per
  change, between two frames, so a screen can pause a socket or a video instead of
  guessing from a heartbeat. Linux reports no stage.
- `theme`: `Locale`, carried by `Theme.Locale` and set with `shell.Config.Locale`,
  is the language of the words the date widgets draw when the Props leave them
  empty: month and weekday names, the first day of the week, "Hoje", "Limpar",
  "Mês anterior" and "Próximo mês" (the names of the calendar's two steps, which
  are Kyse's carets, so a screen reader says them), "Ano anterior" and "Próximo
  ano", the placeholders and the end-of-period line.
  `theme.PortugueseBrazil()` is pt-BR with the same month and weekday names and
  the same "Hoje" and "Limpar" as the Kyse web half; the week starts on Sunday.
  `Calendar`, `DatePicker`, `DateRangePicker` and `DateTimePicker` read it, and a
  Props field that is set still wins. `CalendarProps.TodayLabel` and `ClearLabel`
  draw the two controls under the grid, which choose today and empty the choice.
- `widget`: `Menubar.Open`, `DatePicker.Open` and `Hover.Hold`, to restore a menu
  or a tooltip and to draw one open.
- `frame`: golden pictures, light and dark at 1x and 2x, for `Accordion`, `Dialog`,
  `Drawer` (right and bottom), `Empty`, `Figure`, `Menubar`, `Palette`, `Popover`,
  `Toast`, `Tooltip`, `DialogTrigger` and `ResponsiveImage`, and for the calendar,
  the date picker, the date range and the date time picker in pt-BR.

### Fixed

- `widget`: a `Tooltip` was clipped to its own control, so the bubble was cut
  away and never showed. The clip is closed before the bubble is drawn.

- `widget`: a press on a `Drawer` or a `Dialog` no longer counts as a press outside
  it. The barrier behind the panel was an enclosing area of the panel, and the
  walk that routes a press goes out through every enclosing handler, so a tap on
  the panel's own button reached the barrier too and closed the panel before the
  button answered. The scrim now closes its own clip before the panel is drawn;
  a tap on the scrim closes, a tap on the panel and its buttons does not. The
  regression is tested with a simulated touch pointer. The data table's sheet of
  filters is dismissible again, and the note about that limitation is gone from
  the upgrade guide.
- `widget`: a dismissible `Dialog` or `Drawer` took the keyboard on every frame, so
  a text field inside it lost the focus the moment it was touched and received
  nothing typed. It takes the keyboard once, when it opens, and escape is read
  whatever holds the focus.

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
