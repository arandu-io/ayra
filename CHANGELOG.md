# Release Notes

## Unreleased

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

## v0.1.0 - 2026-09-14

First release.
