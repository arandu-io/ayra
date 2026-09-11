# Contributing

## Sign your commits

Every commit needs a `Signed-off-by` line:

```
git commit -s -m "what changed and why"
```

That line is the [Developer Certificate of Origin](https://developercertificate.org/):
you are stating that you wrote the change, or that you have the right to submit
it under this project's license. It is not a copyright assignment — you keep
your copyright, and this project can never be relicensed behind your back.

We use DCO rather than a CLA on purpose. A CLA would let the project relicense
later, and the price is that every contributor has to sign a legal document
before their first patch.

## Before you open a pull request

```
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go vet ./...
go test -race ./...
```

CI runs these three, and then a number of checks this file does not list --
`.github/workflows/ci.yml` is the one that decides, and a copy of it here would
only be a second list to keep in step. One of those checks is worth knowing
before you write the patch: this module's direct dependencies are fixed by a
test, and every one of them belongs to the engine it vendors. The test fails on
a ninth, and it fails as well on a name in the list that nothing requires any
more. A pull request that adds a dependency needs to argue for it first, in an
issue -- what is imported here is downloaded by every project that opens a
window.

## Where a test goes

Beside the code it tests, which is the whole rule. A package's tests live in its
own directory, in `package X` when they need what the package does not export
and in `package X_test` when they do not -- and the second is the one to reach
for, because what it sees is what a caller sees.

That differs from the rest of this project, where tests live under `tests/` in
category directories. The reason is the subject: a control is proved by drawing
it and reading what came out, and both halves of that -- the frame it is drawn
into and the picture it produced -- are the package's own. A suite one directory
away would import the package to draw, then reach back for the measurement, and
the reaching back is the thing `package X_test` exists to avoid.

There is one exception and it has a directory: `frame/testdata` holds the
approved pictures. They are read by `frame/golden_test.go` and by nothing else.

| where | when |
|---|---|
| `package X_test`, beside the code | this is the **contract**. The test sees what a caller sees, which is the point |
| `package X`, beside the code | this is the **implementation**, and the test genuinely needs something the package does not export |

Prefer the first, and take the second only when you use it. A test in the
package that never touches an unexported name is a contract test that gave up
its guarantee for nothing.

A `package main` has no external form: it cannot be imported, so nothing under
`tests/` can reach it. Its tests are internal, and they carry the suffix for the
reason every internal test carries it.

## What the commit message says

What changed and why. The why is the part that is not in the diff, and it is the
part someone will need in two years.

No AI attribution of any kind: no `Co-Authored-By` for an assistant, no
"generated with" footer. Commits are authored by the people who submit them.

## Architecture decisions

The decisions this project has already made live at arandu.io/docs, and every
one that closed a door has an ADR. If your change contradicts one, say so in the
pull request and argue for the change of decision — that is a normal thing to
do, and it is better than a patch that quietly works around it.
