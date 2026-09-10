# Upstream

The engine under `engine/` began as a copy of an existing Go GUI toolkit, at
the version recorded below, and is maintained here from now on.

    upstream   gioui.org
    version    v0.10.2
    licence    Unlicense OR MIT, reproduced in engine/LICENSE
    shaders    gioui.org/shader v1.0.9, under engine/shader

## Why this file exists

The cost of a fork is not writing it, it is following it. Every fix published
upstream is a port here, for as long as this exists. That port is cheap while
the files still look like theirs, and expensive the moment they do not — so the
rule for this directory is the opposite of the rule everywhere else in this
repository:

> **Files under `engine/` stay as close to upstream as possible.**
> Comments are not rewritten, names are not improved, and style is not brought
> into line with the rest of the tree. A diff against the original has to stay
> readable, because that diff is how a security fix gets in.

Everything that is ours lives outside this directory and is written to our own
standard. The boundary is physical, so nobody has to remember it.

## How to port a fix

1. Fetch the upstream version you are porting from.
2. Diff the file there against the same path here.
3. Apply. If the file is in the divergence list below, read that entry first —
   it says what was changed and why, so a conflict is expected rather than
   surprising.
4. Run the upstream tests that came with the fork. They are the proof that the
   port did not change behaviour; sixteen packages carry them.

## Deliberate divergences

Everything in this list was changed on purpose. Anything not in this list that
differs from upstream is a mistake.

### 1. The window does not draw its decorations with the upstream theme

**Changed:** `app/window.go` no longer imports the upstream widget theme.
`app/decorations.go` is new and draws the fallback title bar with this
project's palette.

**Why:** the title bar is part of the window and appears before any application
code runs. Keeping the upstream theme for it would decide what the first thing
a person sees looks like, and decide it for every application built here —
including the ones that never chose that look. The upstream package is also not
copied at all, so the coupling cannot come back by accident.

**Expect a conflict in:** `app/window.go` around the decoration fields and the
two call sites, whenever upstream touches its own decorations.

### 2. Only the packages an application reaches were copied

**Changed:** the fork carries the closure an application needs across the
targets this project supports, and not the whole upstream tree. Examples,
command-line tools and the packages nothing here imports were left behind.

**Why:** what is not carried is not maintained. A package copied "in case"
still has to be read on every port.

**Expect:** a port that touches a package absent here needs that package
brought in first, with the same rewrite of import paths.

### 3. Three struct literals in `internal/f32` name their fields

**Changed:** `internal/f32/f32.go` writes `Point{X: x0, Y: y0}` where upstream
writes `Point{x0, y0}`, at the three sites `go vet` reports. Two lines, and the
values are the same.

**Why:** `Point` in that file is an alias for the exported `f32.Point` of
another package, so the composites check reads the literal as belonging to an
imported type and flags it. Upstream is not reported because the check is
scoped to the module being vetted, and there the alias target and the literal
are the same module. Here they are too — the difference is which module the
tool was pointed at. The alternative was to stop vetting `engine/` altogether,
which trades a two-line diff for no vet across two hundred files.

The change is equivalent by construction — `Point` declares `X, Y float32` in
that order — but the file had no tests and neither does upstream, and nothing
that imports it exercises these methods, so a wrong sign would have reached a
drawn frame before it reached a failure. `internal/f32/rectangle_ayra_test.go`
is new and covers `Rect`, `Add`, `Sub` and `Size`. A new file never conflicts;
the name keeps it clear of a test file upstream may add later.

**Expect a conflict in:** those five lines, whenever upstream touches `Rect`,
`Rectangle.Add` or `Rectangle.Sub`. Resolve by taking upstream's arithmetic and
keeping the field names.

## Known, and deliberately not resolved yet

The Android backend carries Java classes whose names come from the original
project, and the JNI symbols in the C sources are spelled to match them. Unlike
a comment, those names reach the packaged application. Renaming them touches
the Java, the JNI symbol names and the manifest at once, so it belongs to the
work that first produces a package for that platform and not before — changing
them now would mean carrying a large divergence through every port until then,
for a target that does not build yet.

## What was not changed, and is not a mistake

The import paths, which had to move into this module's namespace. Every other
byte of a copied file is upstream's, including the comments that name the
original project — rewriting them would make every future diff noisier for no
functional gain, which is the opposite of what this file is for.
