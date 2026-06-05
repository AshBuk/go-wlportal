# go-wlportal

Small, dependency-light Go bindings for XDG desktop portals that have no good
pure-Go equivalent yet.

- **`remotedesktop`** — inject keyboard input into the focused window via the
  `org.freedesktop.portal.RemoteDesktop` portal.

The injection happens on the **compositor side**, so it works from inside a
**Flatpak sandbox without extra device permissions** and is not affected by the
Wayland security-context that blocks `zwp_virtual_keyboard` for sandboxed
clients (which is what breaks `wtype` in a sandbox).

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

	"github.com/AshBuk/go-wlportal/remotedesktop"
)

func main() {
	if !remotedesktop.Available() {
		log.Fatal("RemoteDesktop portal not available")
	}

	kbd, err := remotedesktop.NewKeyboard(
		// Persist the permission so the consent dialog shows only once.
		remotedesktop.WithRestoreTokenPath("/home/me/.config/myapp/portal.token"),
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

## API

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

| Compositor | RemoteDesktop keyboard |
|------------|------------------------|
| GNOME      | ✅ |
| KDE Plasma | ✅ |
| wlroots / Hyprland / Sway | ❌ (portal does not implement RemoteDesktop) |

Always check `Available()` and fall back to another method (e.g. `ydotool`,
`wtype`, or clipboard) when it returns `false`.

## License

MIT © Asher Buk
