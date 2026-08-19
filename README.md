# go-wlportal

[![Go Reference](https://pkg.go.dev/badge/github.com/AshBuk/go-wlportal.svg)](https://pkg.go.dev/github.com/AshBuk/go-wlportal)
[![Go Report Card](https://goreportcard.com/badge/github.com/AshBuk/go-wlportal)](https://goreportcard.com/report/github.com/AshBuk/go-wlportal)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Small, dependency-light Go bindings for XDG desktop portals that have no good
pure-Go equivalent yet.

- **`typing`** — inject keyboard input into the focused window via the
  `org.freedesktop.portal.RemoteDesktop` portal.
- **`shortcuts`** — register global hotkeys and receive their activations via the
  `org.freedesktop.portal.GlobalShortcuts` portal.

Both work on the **compositor side**, so they run from inside a **Flatpak sandbox
without extra device permissions** and are not affected by the Wayland
security-context that blocks `zwp_virtual_keyboard` for sandboxed clients (which
is what breaks `wtype` in a sandbox). The packages are independent — import only
what you need.

> Only dependency: [`github.com/godbus/dbus/v5`](https://github.com/godbus/dbus).
>
> Extracted from and used by [dabri](https://github.com/AshBuk/dabri).

## Install

```bash
go get github.com/AshBuk/go-wlportal
```

## Usage

```go
package main

import (
	"log"

	"github.com/AshBuk/go-wlportal/typing"
)

func main() {
	if !typing.Available() {
		log.Fatal("RemoteDesktop portal not available")
	}

	kbd, err := typing.NewKeyboard(
		// Persist the permission so the consent dialog shows only once.
		typing.WithRestoreTokenPath("/home/me/.config/myapp/portal.token"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer kbd.Close()

	if err := kbd.Type("héllo, мир 👍"); err != nil {
		log.Fatal(err)
	}
}
```

A ready-to-run CLI lives in [`examples/typing`](examples/typing):

```bash
go run ./examples/typing "hello, world"
```

### Global shortcuts

```go
import "github.com/AshBuk/go-wlportal/shortcuts"

s, err := shortcuts.New([]shortcuts.Shortcut{
	{ID: "record", Description: "Start recording", PreferredTrigger: "<Ctrl><Alt>space"},
},
	// GNOME rejects an unidentified app, so non-sandboxed apps must set an
	// app id matching an installed .desktop file.
	shortcuts.WithAppID("com.example.myapp"),
)
if err != nil {
	log.Fatal(err)
}
defer s.Close()

for e := range s.Events() {
	if e.Pressed {
		log.Printf("activated: %s", e.ID)
	}
}
```

A ready-to-run CLI lives in [`examples/shortcuts`](examples/shortcuts):

```bash
go run ./examples/shortcuts
```

## API (`typing`)

```go
func Available() bool

func NewKeyboard(opts ...Option) (*Keyboard, error)
func WithRestoreTokenPath(path string) Option
func WithCallTimeout(d time.Duration) Option
func WithAppID(id string) Option

func (k *Keyboard) Type(text string) error
func (k *Keyboard) Key(keysym int32, state KeyState) error // Pressed / Released
func (k *Keyboard) Keycode(keycode Keycode, state KeyState) error
func (k *Keyboard) KeyCombo(keycodes ...Keycode) error
func (k *Keyboard) Close() error

func RuneToKeysym(r rune) int32
```

- The portal session opens lazily on the first injection call (`Type`, `Key`,
  `Keycode`, `KeyCombo`) and may show a one-time permission dialog. With
  `WithRestoreTokenPath` the dialog is shown only once across restarts.
- `WithAppID` declares the app id to the portal so the consent dialog shows the
  app's name and icon; it should match an installed `.desktop` file.
- `Type` maps Latin-1 runes 1:1 and other code points to the Unicode keysym
  range, so non-ASCII text works where the compositor supports it.
- `Keycode`/`KeyCombo` call `NotifyKeyboardKeycode` with Linux input-event
  keycodes. They are meant for physical-key shortcuts such as
  `KeyCombo(KeycodeLeftCtrl, KeycodeV)` or
  `KeyCombo(KeycodeLeftShift, KeycodeInsert)`.

## API (`shortcuts`)

```go
func Available() bool

func New(list []Shortcut, opts ...Option) (*Session, error)
func WithCallTimeout(d time.Duration) Option
func WithAppID(id string) Option

func (s *Session) Events() <-chan Event // closed on Close
func (s *Session) Configure(parentWindow string) error
func (s *Session) Close() error

type Shortcut struct{ ID, Description, PreferredTrigger string }
type Event struct{ ID string; Pressed bool }
```

- `New` opens the session and binds all shortcuts in one request, then delivers
  `Activated`/`Deactivated` as `Event`s on `Events()`. Binding happens on every
  session; backends show their consent dialog only for shortcuts the app has not
  bound before.
- `PreferredTrigger` is a portal accelerator string (e.g. `<Ctrl><Alt>space`);
  empty lets the user choose the binding. Converting an app-specific hotkey
  format into this syntax is the caller's responsibility.
- `WithAppID` declares the app id to the portal. GNOME's backend rejects an
  unidentified app, so non-sandboxed apps must set it (sandboxed apps are
  identified by the sandbox); it should match an installed `.desktop` file.
- `Configure` opens the compositor's shortcut configuration UI, for an explicit
  "reconfigure shortcuts" action in the app. It needs version 2 of the portal
  interface; older backends answer with an unknown-method error.

## Keyboard layout limitation

`NotifyKeyboardKeysym` does not type a character directly — it hands a **keysym**
to the compositor, which then looks up a **keycode in the active keyboard
layout**. Characters absent from that layout (e.g. Cyrillic on a US layout) have
no keycode, so the compositor silently drops them (observed on GNOME/mutter).

Unlike the `zwp_virtual_keyboard` protocol used by `wtype`, the RemoteDesktop
portal does **not** let the client upload its own keymap, so reliable injection
of arbitrary Unicode is not possible through it. For text outside the active
layout, fall back to the clipboard (this is what `dabri` does).

## Roadmap

**libei (EIS) — evaluated, deliberately not pursued.** We looked into routing
keyboard input through `libei` (via the portal's `ConnectToEIS`) for fuller
multilingual coverage and decided against it. The reasoning:

- It would **not** solve arbitrary-Unicode injection anyway. libei is
  keycode-only (no keysym event) and does **not** let the client upload its own
  keymap — the keymap comes from the compositor, as for any Wayland client. The
  only protocol that uploads a client keymap is `zwp_virtual_keyboard` (what
  `wtype` uses), which is precisely the sandbox-blocked, GNOME-unsupported path
  this library exists to avoid.
- Its real benefit is narrow: `ConnectToEIS` can fetch the active keymap
  (`ei_device_keyboard_get_keymap`), enabling layout-aware reverse mapping —
  reaching characters in *already-installed* layouts and making the clipboard
  fallback boundary precise. It still can't type emoji, unmapped scripts, or
  arbitrary Unicode.
- The cost is high: CGO for both `libei` and `libxkbcommon`. That negates the
  whole point of this library — one dependency, pure-Go, Flatpak-friendly.
- For the typical workload (whole sentences in arbitrary languages), the
  **clipboard is the correct primary path regardless** of libei, and that
  belongs in the application layer, not in a portal binding.

So multilingual text outside the active layout is handled by clipboard fallback
in the consuming app (this is what `dabri` does). If a layout-aware EIS typer is
ever wanted, it should live in a separate optional module — never in this core
package — to keep the zero-CGO story intact.

## Compositor support

| Compositor | `typing` (RemoteDesktop) | `shortcuts` (GlobalShortcuts) |
|------------|--------------------------|-------------------------------|
| GNOME      | ✅ | ✅ |
| KDE Plasma | ✅ | ✅ |
| wlroots / Hyprland / Sway | ❌ (portal does not implement RemoteDesktop) | ⚠️ registers, but the binding must be set in the compositor config |

Always check `Available()` for the package you use and fall back to another
method when it returns `false`.

## License

MIT © Asher Buk
