# Release Notes

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
