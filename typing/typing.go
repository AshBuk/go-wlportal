// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package typing

import (
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/AshBuk/go-wlportal/internal/portal"
)

const (
	portalRemote = "org.freedesktop.portal.RemoteDesktop"

	deviceKeyboard = uint32(1) // RemoteDesktop DeviceType bitmask: KEYBOARD
	persistToken   = uint32(2) // persist_mode: keep permission across restarts
)

// KeyState is the state of a key in a NotifyKeyboardKeysym call.
type KeyState uint32

const (
	// Released is sent when a key is released.
	Released KeyState = 0
	// Pressed is sent when a key is pressed.
	Pressed KeyState = 1
)

// Available reports whether the RemoteDesktop portal exposes keyboard injection
// on the current session.
func Available() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var v dbus.Variant
	if err := conn.Object(portal.Dest, portal.Path).
		Call("org.freedesktop.DBus.Properties.Get", 0, portalRemote, "AvailableDeviceTypes").
		Store(&v); err != nil {
		return false
	}
	types, ok := v.Value().(uint32)
	return ok && types&deviceKeyboard != 0
}

// Option configures a Keyboard.
type Option func(*Keyboard)

// WithRestoreTokenPath persists the portal permission token at the given path so
// the consent dialog is shown only once across restarts. When empty (default),
// the permission is not persisted.
func WithRestoreTokenPath(path string) Option {
	return func(k *Keyboard) { k.tokenPath = path }
}

// WithCallTimeout sets how long to wait for each portal call to be answered.
// The default is 60s, which covers the user interacting with the consent dialog.
func WithCallTimeout(d time.Duration) Option {
	return func(k *Keyboard) { k.timeout = d }
}

// WithAppID sets the app id declared to the portal before the session is created,
// so the consent dialog shows the app's name and icon. It should match an
// installed .desktop file.
func WithAppID(id string) Option {
	return func(k *Keyboard) { k.appID = id }
}

// Keyboard injects keyboard input through the RemoteDesktop portal. It is safe
// for concurrent use; calls are serialized. The portal session is opened lazily
// on first use, which may show a one-time permission dialog.
type Keyboard struct {
	tokenPath string
	timeout   time.Duration
	appID     string

	mu      sync.Mutex
	conn    *portal.Conn
	session dbus.ObjectPath
}

// NewKeyboard creates a keyboard injector. It returns an error when the
// RemoteDesktop portal is unavailable.
func NewKeyboard(opts ...Option) (*Keyboard, error) {
	if !Available() {
		return nil, fmt.Errorf("typing: portal not available")
	}
	k := &Keyboard{timeout: 60 * time.Second}
	for _, opt := range opts {
		opt(k)
	}
	return k, nil
}

// Type injects the given UTF-8 text into the focused window.
func (k *Keyboard) Type(text string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := k.ensureSession(); err != nil {
		return err
	}
	for _, r := range text {
		keysym := RuneToKeysym(r)
		if err := k.notify(keysym, Pressed); err != nil {
			return err
		}
		if err := k.notify(keysym, Released); err != nil {
			return err
		}
	}
	return nil
}

// Key presses or releases a single X11 keysym. Use it for modifiers or control
// keys that Type does not cover.
func (k *Keyboard) Key(keysym int32, state KeyState) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := k.ensureSession(); err != nil {
		return err
	}
	return k.notify(keysym, state)
}

// Close ends the portal session and releases its connection.
func (k *Keyboard) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.conn == nil {
		return nil
	}
	err := k.conn.Close()
	k.conn = nil
	k.session = ""
	return err
}

func (k *Keyboard) notify(keysym int32, state KeyState) error {
	call := k.conn.Call(portalRemote, "NotifyKeyboardKeysym",
		k.session, map[string]dbus.Variant{}, keysym, uint32(state))
	if call.Err != nil {
		return fmt.Errorf("typing: notify keysym: %w", call.Err)
	}
	return nil
}

// ensureSession lazily creates, configures and starts the keyboard session.
func (k *Keyboard) ensureSession() error {
	if k.session != "" {
		return nil
	}
	conn, err := portal.Connect(k.timeout)
	if err != nil {
		return err
	}
	k.conn = conn

	// Declare the app id before CreateSession so the dialog can resolve the app.
	if err := k.conn.Register(k.appID); err != nil {
		return err
	}

	created, err := k.conn.Request(portalRemote, "CreateSession", func(token string) []any {
		return []any{map[string]dbus.Variant{
			"handle_token":         dbus.MakeVariant(token),
			"session_handle_token": dbus.MakeVariant(token),
		}}
	})
	if err != nil {
		return err
	}
	handle, _ := created["session_handle"].Value().(string)
	if handle == "" {
		return fmt.Errorf("typing: empty session handle")
	}
	session := dbus.ObjectPath(handle)

	if _, err := k.conn.Request(portalRemote, "SelectDevices", func(token string) []any {
		opts := map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(token),
			"types":        dbus.MakeVariant(deviceKeyboard),
		}
		if k.tokenPath != "" {
			opts["persist_mode"] = dbus.MakeVariant(persistToken)
			if t := portal.LoadToken(k.tokenPath); t != "" {
				opts["restore_token"] = dbus.MakeVariant(t)
			}
		}
		return []any{session, opts}
	}); err != nil {
		return err
	}

	started, err := k.conn.Request(portalRemote, "Start", func(token string) []any {
		return []any{session, "", map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(token),
		}}
	})
	if err != nil {
		return err
	}
	if t, ok := started["restore_token"].Value().(string); ok {
		portal.SaveToken(k.tokenPath, t)
	}
	k.session = session
	return nil
}
