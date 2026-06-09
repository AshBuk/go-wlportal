// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

// Package portal holds the plumbing shared by the public portal packages: a
// dedicated session-bus connection and the asynchronous Request/Response dance
// every org.freedesktop.portal.* method uses, plus restore-token persistence.
package portal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	// Dest is the well-known name of the XDG desktop portal.
	Dest = "org.freedesktop.portal.Desktop"
	// Path is the object path of the XDG desktop portal.
	Path = "/org/freedesktop/portal/desktop"

	requestIface  = "org.freedesktop.portal.Request"
	registryIface = "org.freedesktop.host.portal.Registry"
)

var requestSeq uint64

// Conn is a dedicated session-bus connection backing a single portal session.
// A portal session lives with its connection, so each session keeps its own.
type Conn struct {
	conn    *dbus.Conn
	signals chan *dbus.Signal
	timeout time.Duration
}

// Connect opens a session-bus connection and subscribes to its signals.
func Connect(timeout time.Duration) (*Conn, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("portal: connect session bus: %w", err)
	}
	c := &Conn{conn: conn, signals: make(chan *dbus.Signal, 8), timeout: timeout}
	conn.Signal(c.signals)
	return c, nil
}

// Close releases the underlying connection (and thereby the portal session).
func (c *Conn) Close() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

// Call invokes a method on the portal desktop object without waiting for a
// Request/Response round-trip (e.g. RemoteDesktop.NotifyKeyboardKeysym).
func (c *Conn) Call(iface, method string, args ...any) *dbus.Call {
	return c.conn.Object(Dest, Path).Call(iface+"."+method, 0, args...)
}

// Register declares the app id to the portal so non-sandboxed apps are
// identified before opening a session. GNOME's GlobalShortcuts backend rejects
// an unidentified app, and consent dialogs use the id to resolve the app's
// .desktop name and icon. No-op when appID is empty or the host Registry
// interface is missing (older xdg-desktop-portal).
func (c *Conn) Register(appID string) error {
	if appID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	err := c.conn.Object(Dest, Path).
		CallWithContext(ctx, registryIface+".Register", 0, appID, map[string]dbus.Variant{}).
		Err
	if isUnsupported(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("portal: register app id: %w", err)
	}
	return nil
}

// isUnsupported reports whether err means the portal lacks the Registry interface.
func isUnsupported(err error) bool {
	var derr dbus.Error
	if !errors.As(err, &derr) {
		return false
	}
	switch derr.Name {
	case "org.freedesktop.DBus.Error.UnknownMethod",
		"org.freedesktop.DBus.Error.UnknownInterface",
		"org.freedesktop.DBus.Error.UnknownObject",
		"org.freedesktop.DBus.Error.NotSupported":
		return true
	}
	return false
}

// Signals returns the channel receiving every signal on this connection. After
// the initial Request round-trips complete, a single consumer may range over it
// to receive portal signals (e.g. GlobalShortcuts.Activated).
func (c *Conn) Signals() <-chan *dbus.Signal { return c.signals }

// AddMatch installs a signal match rule on the connection.
func (c *Conn) AddMatch(opts ...dbus.MatchOption) error {
	return c.conn.AddMatchSignal(opts...)
}

// HasInterface reports whether the portal desktop object exposes the named
// interface. It uses a short timeout because the portal may be activatable but
// not yet running, which would otherwise block on the default D-Bus timeout.
func HasInterface(name string) bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var data string
	if err := conn.Object(Dest, Path).
		CallWithContext(ctx, "org.freedesktop.DBus.Introspectable.Introspect", 0).
		Store(&data); err != nil {
		return false
	}
	return strings.Contains(data, name)
}

// Request invokes iface.method on the portal and blocks until the matching
// org.freedesktop.portal.Request.Response signal arrives. build receives a
// unique handle_token and returns the call arguments.
func (c *Conn) Request(iface, method string, build func(token string) []any) (map[string]dbus.Variant, error) {
	token := fmt.Sprintf("wlportal%d", atomic.AddUint64(&requestSeq, 1))
	sender := strings.ReplaceAll(strings.TrimPrefix(c.conn.Names()[0], ":"), ".", "_")
	reqPath := dbus.ObjectPath(Path + "/request/" + sender + "/" + token)

	match := []dbus.MatchOption{
		dbus.WithMatchObjectPath(reqPath),
		dbus.WithMatchInterface(requestIface),
		dbus.WithMatchMember("Response"),
	}
	if err := c.conn.AddMatchSignal(match...); err != nil {
		return nil, err
	}
	defer func() { _ = c.conn.RemoveMatchSignal(match...) }()

	var handle dbus.ObjectPath
	if err := c.conn.Object(Dest, Path).
		Call(iface+"."+method, 0, build(token)...).Store(&handle); err != nil {
		return nil, fmt.Errorf("portal: %s: %w", method, err)
	}

	timeout := time.After(c.timeout)
	for {
		select {
		case sig := <-c.signals:
			if sig == nil || sig.Path != reqPath || sig.Name != requestIface+".Response" {
				continue
			}
			var code uint32
			var results map[string]dbus.Variant
			if err := dbus.Store(sig.Body, &code, &results); err != nil {
				return nil, fmt.Errorf("portal: %s response: %w", method, err)
			}
			if code != 0 {
				return nil, fmt.Errorf("portal: %s rejected (code %d)", method, code)
			}
			return results, nil
		case <-timeout:
			return nil, fmt.Errorf("portal: %s timed out", method)
		}
	}
}

// LoadToken reads a persisted restore token; missing/unreadable files yield "".
func LoadToken(path string) string {
	b, _ := os.ReadFile(path)
	return strings.TrimSpace(string(b))
}

// SaveToken persists a restore token; empty token or path is a no-op.
func SaveToken(path, token string) {
	if token == "" || path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte(token), 0o600)
}
