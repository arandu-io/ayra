<h1 align="center">arandu-io/ayra</h1>

<p align="center">Native Applications for Arandu</p>

---

## About

`ayra` draws an Arandu application as a native one: its own window, its own
rendering, its own input, and an artefact that installs — on macOS, Windows,
Linux, Android, iOS and in a browser through WebAssembly.

It is not the web application in a frame. Nothing here renders markup, loads a
stylesheet or runs a script.

```go
ayra.Run(ayra.Config{
    Title: "Faturas",
    Screen: func(ctx ayra.Context) ayra.Dimensions {
        return widget.Column(ctx,
            widget.Heading(ctx, "Faturas em aberto"),
            widget.Button(ctx, &pay, widget.ButtonProps{
                Label:   "Pagar",
                Variant: widget.Destructive,
            }),
        )
    },
})
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

Desktop and the browser target build with the Go toolchain and nothing else.
Packaging for a phone needs that platform's own toolchain, and that is stated
here rather than found out later.

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

## License

MIT. See [LICENSE.md](LICENSE.md), and [THIRD_PARTY.md](THIRD_PARTY.md) for
what travels with it.
