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
before you write the patch: this module requires `arandu-io/framework` and
nothing else, because it is imported by every project that draws a button, and
a second require is a download for all of them. A pull request that adds a
dependency needs to argue for it first, in an issue.

## Where a test goes

Under `tests/`, in one of its capitalized category directories, declaring a
lowercase package -- `tests/Unit` holds `package unit`. The exception is a test
that needs something the package does not export: that one goes beside the code
it tests, named `*_internal_test.go`, and the suffix is how it says so. This is
not a preference. `tests/test-layout-guard.sh` runs in CI and rejects a
`*_test.go` file that is neither.

Which of the two you are writing answers one question:

| where | when |
|---|---|
| `tests/`, importing the package | this is the **contract**. The test sees what a caller sees, which is the point |
| beside the code, `*_internal_test.go` in `package X` | this is the **implementation**, and the test genuinely needs something the package does not export |

The second one is beside the code because there is nowhere else it can be. A
file reaches what a package does not export only by compiling into that package,
and `go test` attributes coverage per directory -- so the package's own
directory is also the only place where what a test exercises is credited to the
code it exercises. A suite under `tests/` is credited to `tests/`, and short of
`-coverpkg` the package it imports reports what its own files reach. That is
what the first row costs, and for a contract test it is the right price: what it
measures is the exported surface, which is all a caller ever has.

Prefer the first. Take the second only when you use it -- `plans/testpackages.go`
in the arandu-io working tree checks exactly that, by intersecting the
identifiers a test names with what its package declares unexported, and the
checklist runs it across every Go repository in the project.

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
