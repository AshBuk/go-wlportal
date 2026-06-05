// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package remotedesktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalDest    = "org.freedesktop.portal.Desktop"
	portalPath    = "/org/freedesktop/portal/desktop"
	portalRemote  = "org.freedesktop.portal.RemoteDesktop"
	portalRequest = "org.freedesktop.portal.Request"

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

var requestSeq uint64

// Available reports whether the RemoteDesktop portal exposes keyboard injection
// on the current session.
func Available() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var v dbus.Variant
	if err := conn.Object(portalDest, portalPath).
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

// Keyboard injects keyboard input through the RemoteDesktop portal. It is safe
// for concurrent use; calls are serialized. The portal session is opened lazily
// on first use, which may show a one-time permission dialog.
type Keyboard struct {
	tokenPath string
	timeout   time.Duration

	mu      sync.Mutex
	conn    *dbus.Conn
	signals chan *dbus.Signal
	session dbus.ObjectPath
}

// NewKeyboard creates a keyboard injector. It returns an error when the
// RemoteDesktop portal is unavailable.
func NewKeyboard(opts ...Option) (*Keyboard, error) {
	if !Available() {
		return nil, fmt.Errorf("remotedesktop: portal not available")
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
	call := k.conn.Object(portalDest, portalPath).Call(
		portalRemote+".NotifyKeyboardKeysym", 0,
		k.session, map[string]dbus.Variant{}, keysym, uint32(state))
	if call.Err != nil {
		return fmt.Errorf("remotedesktop: notify keysym: %w", call.Err)
	}
	return nil
}

// ensureSession lazily creates, configures and starts the keyboard session.
// A dedicated connection is kept open because the session lives with it.
func (k *Keyboard) ensureSession() error {
	if k.session != "" {
		return nil
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("remotedesktop: connect session bus: %w", err)
	}
	k.conn = conn
	k.signals = make(chan *dbus.Signal, 8)
	conn.Signal(k.signals)

	created, err := k.request("CreateSession", func(token string) []interface{} {
		return []interface{}{map[string]dbus.Variant{
			"handle_token":         dbus.MakeVariant(token),
			"session_handle_token": dbus.MakeVariant(token),
		}}
	})
	if err != nil {
		return err
	}
	handle, _ := created["session_handle"].Value().(string)
	if handle == "" {
		return fmt.Errorf("remotedesktop: empty session handle")
	}
	session := dbus.ObjectPath(handle)

	if _, err := k.request("SelectDevices", func(token string) []interface{} {
		opts := map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(token),
			"types":        dbus.MakeVariant(deviceKeyboard),
		}
		if k.tokenPath != "" {
			opts["persist_mode"] = dbus.MakeVariant(persistToken)
			if t := k.loadToken(); t != "" {
				opts["restore_token"] = dbus.MakeVariant(t)
			}
		}
		return []interface{}{session, opts}
	}); err != nil {
		return err
	}

	started, err := k.request("Start", func(token string) []interface{} {
		return []interface{}{session, "", map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(token),
		}}
	})
	if err != nil {
		return err
	}
	if t, ok := started["restore_token"].Value().(string); ok {
		k.saveToken(t)
	}
	k.session = session
	return nil
}

// request invokes a portal method and waits for its asynchronous Response signal.
func (k *Keyboard) request(method string, build func(token string) []interface{}) (map[string]dbus.Variant, error) {
	token := fmt.Sprintf("wlportal%d", atomic.AddUint64(&requestSeq, 1))
	sender := strings.ReplaceAll(strings.TrimPrefix(k.conn.Names()[0], ":"), ".", "_")
	reqPath := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)

	match := []dbus.MatchOption{
		dbus.WithMatchObjectPath(reqPath),
		dbus.WithMatchInterface(portalRequest),
		dbus.WithMatchMember("Response"),
	}
	if err := k.conn.AddMatchSignal(match...); err != nil {
		return nil, err
	}
	defer func() { _ = k.conn.RemoveMatchSignal(match...) }()

	var handle dbus.ObjectPath
	if err := k.conn.Object(portalDest, portalPath).
		Call(portalRemote+"."+method, 0, build(token)...).Store(&handle); err != nil {
		return nil, fmt.Errorf("remotedesktop: %s: %w", method, err)
	}

	timeout := time.After(k.timeout)
	for {
		select {
		case sig := <-k.signals:
			if sig == nil || sig.Path != reqPath || sig.Name != portalRequest+".Response" {
				continue
			}
			var code uint32
			var results map[string]dbus.Variant
			if err := dbus.Store(sig.Body, &code, &results); err != nil {
				return nil, fmt.Errorf("remotedesktop: %s response: %w", method, err)
			}
			if code != 0 {
				return nil, fmt.Errorf("remotedesktop: %s rejected (code %d)", method, code)
			}
			return results, nil
		case <-timeout:
			return nil, fmt.Errorf("remotedesktop: %s timed out", method)
		}
	}
}

func (k *Keyboard) loadToken() string {
	b, _ := os.ReadFile(k.tokenPath)
	return strings.TrimSpace(string(b))
}

func (k *Keyboard) saveToken(token string) {
	if token == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(k.tokenPath), 0o700)
	_ = os.WriteFile(k.tokenPath, []byte(token), 0o600)
}
