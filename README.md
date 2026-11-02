<p align="center">
  <img src=".github/logo.svg" alt="Arandu" width="180">
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

## Making it look like your product

The widgets are restylable the way the web components are (ADR-0131): each
publishes the parts it draws -- `root`, `title`, `label` -- under the names the
web component publishes in `data-part`, and the application, a variant it
registers, or one instance overrides any of them: a fill, a border, a radius, an
inset, a type.

```go
package main

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

var pay widget.Button

// payButton is a button of a variant the product registers, with its corners
// overridden on this one instance.
func payButton(c ayra.Context) ayra.Dimensions {
	return widget.ButtonProps{
		Label: "Pagar",
		ComponentProps: widget.ComponentProps{
			Custom: "brand", // a variant the theme registers, not a value of an enum
			Parts:  theme.Parts{"root": {Radius: theme.Pt(999)}},
		},
	}.Layout(c, &pay)
}

func main() { _ = payButton }
```

`theme/css` builds the theme from the product's own stylesheet, so the native
half is the web half's colours and not a copy of them. `mask`, `icons` and the
primitives a product's own widgets are written from (`widget.Surface`,
`widget.DrawText`, `widget.Clickable`) are exported. `parity_test.go` keeps the
set of components equal to the browser half's: a component added there fails
here until it has a pair or a written reason.

## What it is, and what it is not

**It draws for a server that is running.** Authorization is decided on the
server, always. There is no local policy, no local cache and no offline queue,
and there will not be: the credential the data path requires comes from a
session -- a cookie, or a bearer token the server issued and the application
hands the client with `client.WithBearer` -- and a process on a device does not
mint one.

**Screen readers reach one platform.** Of the nine platform backends beneath
this library, one walks a semantic tree; the other eight do not. On macOS,
Windows, iOS, Linux and in a browser, an application built here is one surface
with no accessible children. It is declared rather than discovered, and a test
guards the count so a regression cannot pass quietly.

Eight of those nine are unwritten rather than unreachable, and saying otherwise
would be claiming a limit the platforms do not impose: a browser can carry the
tree as elements beneath the canvas, Windows and the Linux desktops expose it
over interfaces this library can already speak, and the two Apple platforms
need code in a language this library already compiles. What is a real limit is
narrower and outlives all of that — a node here carries a label, a description,
a class from a set of five, and two flags. It cannot say that one control
describes another, that a region changed and should be read out now, or that a
box is half-checked. Those are missing from the model, not from the backends,
and a platform written before the node grows would be nine implementations of a
field that does not exist.

**The browser target is a preview.** The same native application, opened
without installing — a demonstration and a component catalogue. It is not a way
to build a web application, and the web half of Arandu is not going anywhere.

**Without a GPU it does not open.** There is no software rasteriser. On a
remote desktop session, a virtual machine without passthrough or a server host,
window creation can fail before any of this runs; the failure is detected at
start-up and reported with what was missing.

## Push notifications and live video

Two capabilities that need the device itself are packages of their own, so an
application that never imports them carries none of their code.

`push` is the device's half of a push notification and nothing else: the
permission, the token the server addresses the device by, the notification that
arrived while the application was open, and the one a person touched.

`live` plays the video a camera's node serves over a WebSocket, decoded by the
phone's hardware, and `widget.VideoProps` draws the frame it leaves with a hook
for what goes on top of the picture.

```go
package main

import (
	"context"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/live"
	"github.com/arandu-io/ayra/push"
	"github.com/arandu-io/ayra/shell"
	"github.com/arandu-io/ayra/widget"
)

func main() {
	var video widget.Video

	player := live.NewPlayer(live.PlayerConfig{
		Endpoint: "wss://api1.example.test/live/camera-7",
		// The pass is short-lived and the node ends a session every few
		// minutes, so the application is asked for a new one each time.
		Pass: func(ctx context.Context) (string, error) { return "", nil },
	})
	player.Start()

	go func() {
		if _, err := push.Request(context.Background()); err != nil {
			return
		}
		for opened := range push.Opened() {
			_ = opened.Link // where the notification that was touched goes
		}
	}()

	shell.Exit(shell.Run(shell.Config{Title: "Cameras"}, func(c ayra.Context) ayra.Dimensions {
		player.SetRedraw(c.Redraw)
		frame := player.Frame()
		return widget.VideoProps{
			Frame:   frame.Image,
			Seq:     frame.Seq,
			Label:   "Gate camera",
			Caption: caption(player.Status()),
		}.Layout(c, &video)
	}))
}

// caption is the sentence a state needs; the words are the application's.
func caption(status live.Status) string {
	switch status.State {
	case live.StateWaiting, live.StateStarting:
		return "Preparing the video"
	case live.StateLost:
		return "No signal from the camera"
	case live.StateFailed:
		return "The video is not available"
	}
	return ""
}
```

What each needs from the platform, and what it costs:

- **Push on Android** is Firebase Cloud Messaging, and the application is built
  with the libraries the project chooses: `aru native:build -android-lib DIR`
  takes the `.aar` and `.jar` files of `firebase-messaging` and what it depends
  on, and `-google-services` takes the project's `google-services.json`, which
  is never kept in a repository. Nothing is downloaded or carried by `aru` or by
  this module.
- **Push on iOS** is the APNs device token, without the Firebase SDK; `push.Kind`
  says which of the two a token is, because a server sending through Firebase
  maps an APNs token to a registration token. The provisioning profile needs the
  Push Notifications capability, and `aru` refuses to sign without it.
- **Live video** decodes with MediaCodec on Android and VideoToolbox on iOS and
  macOS. The windows of the other targets have no decoder: the stream connects
  and reports `live.ErrUnsupported`, and an application on them shows the status
  and the overlay over no picture, or hands `live.PlayerConfig.Decoder` its own.
  The decoded picture is converted to RGBA once on its way to the screen; on a
  phone at 1080p that conversion is the cost that matters, and drawing a codec
  surface as a texture would remove it.
- Neither package keeps anything: no queue that outlives the process, no cache
  of a message or a picture. The server decides what a device is sent and what it
  may watch.

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

A listing is one control that is two layouts. `widget.DataTable` asks a server
for a page the way Kyse's DataTable does -- the same state, the same parameter
names, so the route that draws the table in the browser answers the app -- and
draws a table on a wide screen and cards on a phone, with the filters in a sheet,
an endless list and pull to refresh. It owns no data: it asks a `DataSource`, and
`ServerSource` is the one that speaks an Arandu route.

The controls are the part worth reading first. Each one is a `Props` value you
fill in and a state value you hold, and the doc comment on each says what the
control guarantees and, where it matters, the fault the shape prevents.

`aru native:build -list` prints every platform this builds for and what each one
needs before it will. `aru native:dev` is the loop to develop in: it watches the
native sources and rebuilds.

The complete application guide lives at
[arandu.io/native](https://arandu.io/native). It follows the public workflow:
register the module, publish the six-file starter, develop with `native:run` or
`native:dev`, and build the selected platform artifact with `native:build`.

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
