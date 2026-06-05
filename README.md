# go-wlportal

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

A ready-to-run CLI lives in [`examples/type`](examples/type):

```bash
go run ./examples/type "hello, world"
```

### Global shortcuts

```go
import "github.com/AshBuk/go-wlportal/shortcuts"

s, err := shortcuts.New([]shortcuts.Shortcut{
	{ID: "record", Description: "Start recording", PreferredTrigger: "<Ctrl><Alt>space"},
})
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

func (k *Keyboard) Type(text string) error
func (k *Keyboard) Key(keysym int32, state KeyState) error // Pressed / Released
func (k *Keyboard) Close() error

func RuneToKeysym(r rune) int32
```

- The portal session opens lazily on first `Type`/`Key` and may show a one-time
  permission dialog. With `WithRestoreTokenPath` the dialog is shown only once
  across restarts.
- `Type` maps Latin-1 runes 1:1 and other code points to the Unicode keysym
  range, so non-ASCII text works where the compositor supports it.

## API (`shortcuts`)

```go
func Available() bool

func New(list []Shortcut, opts ...Option) (*Session, error)
func WithCallTimeout(d time.Duration) Option

func (s *Session) Events() <-chan Event // closed on Close
func (s *Session) Close() error

type Shortcut struct{ ID, Description, PreferredTrigger string }
type Event struct{ ID string; Pressed bool }
```

- `New` opens the session and binds all shortcuts in one request (one consent
  dialog), then delivers `Activated`/`Deactivated` as `Event`s on `Events()`.
- `PreferredTrigger` is a portal accelerator string (e.g. `<Ctrl><Alt>space`);
  empty lets the user choose the binding. Converting an app-specific hotkey
  format into this syntax is the caller's responsibility.

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

- **libei (EIS)** path for fuller multilingual coverage. Tracked but uncertain:
  it requires the `libei` C library (CGO) and the client still typically uses the
  compositor-provided keymap, so it does not automatically solve arbitrary-Unicode
  injection. Until then, clipboard remains the recommended fallback for non-layout
  characters.

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
