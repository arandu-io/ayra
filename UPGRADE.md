# Upgrade guide

What changed in a way that stops your code compiling, and what to write instead.

Additions are not listed. A new symbol breaks nothing.

## Unreleased -- a request path is built from segments

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
