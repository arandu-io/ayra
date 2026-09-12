# Third-party notices

## The engine

`engine/` began as a copy of an existing Go GUI toolkit and is maintained here.

    upstream   gioui.org
    version    v0.10.2
    shaders    gioui.org/shader v1.0.9, under engine/shader
    terms      Unlicense OR MIT, reproduced in engine/LICENSE and
               engine/shader/LICENSE

### Which of the two this repository chose

The upstream terms are a choice offered to the recipient, and **this repository
chooses MIT**. The project is MIT throughout, so MIT over MIT needs no further
analysis, and the Unlicense is a public-domain dedication whose effect is not
recognised everywhere — including where this project is domiciled, and where
part of an author's rights cannot be waived.

That choice binds nobody else. Anyone receiving this code may still exercise
either option against the original source on their own terms.

Under MIT the notice travels with the distribution, which is why the two
`LICENSE` files above stay where they are. **They do not leave this repository,
and there is no date on which they do** — four conditions keep them, and one is
enough:

1. any file is still a literal copy, or still derived
2. the compiled shader programs are still here
3. the C, Objective-C and Java sources are still here
4. **the published history contains the original blobs** — `git clone` hands
   them to whoever clones, permanently

The fourth holds on its own, and honouring it costs two text files.

### What is being rewritten, and how far it has got

**No source file carries a licence header.** The terms live in the two `LICENSE`
files and in this one, which is where this project keeps them and what Go itself
does. A tag repeated at the top of two hundred files is a claim made two hundred
times and checked nowhere.

Provenance is **measured rather than declared**. A file is still the original
when its content still is, and that is a comparison anybody can run — not a line
anybody can forget to update, or update wrongly:

```sh
GOWORK=off go test -run TestNothingClaimedAsOursIsStillTheOriginal ./...
```

It reads a pinned copy of the original, normalises both sides — no whitespace,
no comments, because a rewrite that only touches comments is exactly what it
exists to catch — and reports how much of each file is still what it was. The
tree-wide figure is the honest answer to "how much of this is ours", and it is a
measurement carrying a date rather than a status somebody typed.

Renaming an identifier and rewriting a comment does not end derivation. What is
protected is the expression — how the work divides into files and types, the
signatures, the order of operations, the shape of the API. A file stops being
derived when it was written against a published specification or against
observed behaviour rather than against the original; where such a specification
exists, the reimplementation is genuinely independent, and where it does not,
the API is the expression.

The term "clean room" is not used about this work anywhere in this project. It
means something precise that is not what happens here.

### What is not being rewritten

**Regenerated instead of rewritten**, because machine output from a published
description has no author: 3.701 lines of C carrying the protocol generator's
stamp, and the 126 compiled shader programs, whose 14 sources are in the tree.

**Staying as they are**: 1.997 lines of hand-written Objective-C and Java, and
the 34 symbols that bind Java to Go by name. They reach the installable
artifact.

### Keeping up with fixes published upstream

The vulnerability scanner runs over the whole tree in CI. It covers the
dependency graph and it **does not** cover the copied code: a memory defect in a
graphics backend carries no advisory against this module.

So the rest is read by hand, weekly, against the version recorded above, with
the date of the last check kept alongside the verdict. A security verdict
without a date cannot be told apart from a fact.

    last checked   2026-09-11, against v0.10.2

## Design tokens

`theme/basecoat/base.css` is vendored under MIT, with the notice beside it in
`theme/basecoat/LICENSE.md`. Only the stylesheet travelled: it is the source the
palette is generated from, so that a colour named the same thing on both halves
of the product is the same colour.

## Everything else

Written here.
