<p align="center">
  <img src=".github/logo.png" alt="Arandu" width="140" height="140">
</p>

<h1 align="center">arandu-io/ayra</h1>

<p align="center">Native Applications for Arandu</p>

<p align="center">
<a href="https://github.com/arandu-io/ayra/actions/workflows/ci.yml"><img src="https://github.com/arandu-io/ayra/actions/workflows/ci.yml/badge.svg" alt="Build Status"></a>
<a href="https://pkg.go.dev/github.com/arandu-io/ayra"><img src="https://pkg.go.dev/badge/github.com/arandu-io/ayra.svg" alt="Go Reference"></a>
<a href="https://github.com/arandu-io/ayra/tags"><img src="https://img.shields.io/github/v/tag/arandu-io/ayra?label=version" alt="Latest Version"></a>
<a href="LICENSE.md"><img src="https://img.shields.io/github/license/arandu-io/ayra" alt="License"></a>
</p>

---

## About

`ayra` draws an Arandu application as a native one: its own window, its own
rendering, its own input, and an artefact that installs — on macOS, Windows,
Linux, Android, iOS and in a browser through WebAssembly.

It is not the web application in a frame. Nothing here renders markup, loads a
stylesheet or runs a script.

```go
package main

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/shell"
	"github.com/arandu-io/ayra/widget"
)

func main() {
	var pay widget.Button

	shell.Exit(shell.Run(shell.Config{Title: "Faturas"}, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.TextProps{
					Content: "Faturas em aberto",
					Role:    widget.Heading,
				}.Layout(c.With(gtx))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.ButtonProps{
					Label:   "Pagar",
					Variant: widget.Destructive,
				}.Layout(c.With(gtx), &pay)
			}),
		)
	}))
}
```

The vocabulary is the one the web components already use — `Variant`, `Size`,
the token names — so a developer who can write a screen for the browser can
write this one. What is **not** shared is code: markup, stylesheet classes and
browser behaviours have no meaning in a drawing list, and a component that
tried to serve both would carry one medium's compromises into the other.

## What it is, and what it is not

**It draws for a server that is running.** Authorization is decided on the
server, always. There is no local policy, no local cache and no offline queue,
and there will not be: the credential the data path requires comes from a
session, and a process on a device does not mint one.

**Screen readers reach one platform.** Of the nine platform backends beneath
this library, one walks a semantic tree; the other eight do not. On macOS,
Windows, iOS, Linux and in a browser, an application built here is one surface
with no accessible children. That is a boundary of the drawing model, it is
declared rather than discovered, and a test guards the count so a regression
cannot pass quietly.

**The browser target is a preview.** The same native application, opened
without installing — a demonstration and a component catalogue. It is not a way
to build a web application, and the web half of Arandu is not going anywhere.

**Without a GPU it does not open.** There is no software rasteriser. On a
remote desktop session, a virtual machine without passthrough or a server host,
window creation can fail before any of this runs; the failure is detected at
start-up and reported with what was missing.

## Install

```sh
go get github.com/arandu-io/ayra
```

macOS, Windows and the browser target build with the Go toolchain and nothing
else. **Linux needs its development packages first** -- X11, Wayland, EGL,
Vulkan and xkbcommon -- because the window and the input come from those
libraries through cgo. Without them the build fails at the first C header and
reads like a broken checkout; the list this project's own CI installs is in
`.github/workflows/ci.yml`.

Packaging for a phone needs that platform's own toolchain: an Android SDK with
its build tools, and Xcode with a provisioning profile for iOS. Both are stated
here rather than found out after the Go half has finished compiling.

## The name

**Ayra** comes from Old Tupi *a'yra*: son, and, in the sense recorded by
Navarro, someone originary or native of a land — *"filho de uma dada terra"*.
An application built here is one born for the platform it runs on.

`a'yra` is the rigorous spelling, where the apostrophe is a glottal stop and
the `y` is a central vowel with no equivalent in Portuguese; the historical
form is /aˈʔɨ.ɾa/. `ayra` is the spelling this project uses, with precedent in
Barbosa's *Curso de Tupi Antigo* (1956, §240).

Sources: Navarro, *Dicionário de Tupi Antigo* (2013); Carvalho and Birchall,
LIAMES v. 22 (2022); Dooley, *Léxico Guarani, dialeto Mbyá* (1998).

## Learning Arandu

The API reference is generated from the doc comments and lives on
[pkg.go.dev](https://pkg.go.dev/github.com/arandu-io/ayra). Every exported
symbol carries one, and that is deliberate: it is the documentation that cannot
drift from the code, because it sits in the same file.

The controls are the part worth reading first. Each one is a `Props` value you
fill in and a state value you hold, and the doc comment on each says what the
control guarantees and, where it matters, the fault the shape prevents.

`aru native:build -list` prints every platform this builds for and what each one
needs before it will. `aru native:dev` is the loop to develop in: it watches the
native sources and rebuilds.

A guide and a website do not exist yet, and that is a decision rather than a
gap: a guide written against an API that still moves is work done twice, and the
second time is worse — there is wrong documentation published.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Before opening a pull request, the
commands at the top of that file have to pass, and CI runs them plus the ones a
laptop cannot: a known vulnerability in either module, a dependency that entered
the graph without a decision, and the pictures — every screen is drawn headless
in both palettes at two densities and compared with the approved one, because
everything below a layout call compiles whether or not it works.

## Security Vulnerabilities

Please review [our security policy](SECURITY.md) on how to report a
vulnerability. Never open a public issue for one.

## License

Open-sourced software licensed under the [MIT license](LICENSE.md). See
[THIRD_PARTY.md](THIRD_PARTY.md) for what travels with it.
