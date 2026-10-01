# Release Notes

## Unreleased

### Security

- `client`: a request never leaves the server the client was made for. A
  redirect to another host, port or scheme -- including https to http -- is
  refused before it is sent, so the CSRF token and a re-sent form body no longer
  reach it; a path that is itself an address is refused; an answer that arrived
  from anywhere else is not read and its token is not remembered. The refusal
  wraps the new `client.ErrOffServer`.

## v0.1.0 - 2026-09-14

First release.
