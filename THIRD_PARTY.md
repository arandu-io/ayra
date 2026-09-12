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

Every Go file in this repository declares its own provenance on the first line,
and the build of any derived corpus reads that mark rather than asking anyone:

| mark | meaning |
|---|---|
| `SPDX-License-Identifier: Unlicense OR MIT` | literal copy |
| `SPDX-License-Identifier: MIT AND (Unlicense OR MIT)` | rewritten here, still derived |
| `SPDX-License-Identifier: MIT` | written here |

The mark is mandatory and its absence fails the build, because "unmarked means
ours" would let the dangerous failure — rewriting badly and forgetting to mark —
pass in silence.

Count it yourself; the numbers here are a snapshot and the commands are not:

```sh
grep -rl 'SPDX-License-Identifier: Unlicense OR MIT'           --include='*.go' . | wc -l   # 178
grep -rl 'SPDX-License-Identifier: MIT AND (Unlicense OR MIT)' --include='*.go' . | wc -l   #   0
find engine -name '*.go' | wc -l                                                            # 201
```

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
