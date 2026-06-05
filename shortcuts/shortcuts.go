// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package shortcuts

import (
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/AshBuk/go-wlportal/internal/portal"
)

const portalShortcuts = "org.freedesktop.portal.GlobalShortcuts"

// Shortcut describes one global hotkey to bind.
type Shortcut struct {
	// ID is the app-defined identifier echoed back on activation.
	ID string
	// Description is shown in the compositor's shortcut UI.
	Description string
	// PreferredTrigger is a portal accelerator string, e.g. "<Ctrl><Alt>space".
	// Empty lets the user pick the binding.
	PreferredTrigger string
}

// Event is delivered when a bound shortcut changes state.
type Event struct {
	// ID is the shortcut that fired.
	ID string
	// Pressed is true for Activated, false for Deactivated.
	Pressed bool
}

// Available reports whether the GlobalShortcuts portal is present on the session.
func Available() bool {
	return portal.HasInterface("GlobalShortcuts")
}

// Option configures a Session.
type Option func(*config)

type config struct {
	timeout time.Duration
}

// WithCallTimeout sets how long to wait for each portal call to be answered.
// The default is 60s, which covers the user interacting with the consent dialog.
func WithCallTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// Session is an open GlobalShortcuts portal session. Activations are delivered
// on Events until the session is closed.
type Session struct {
	conn   *portal.Conn
	handle dbus.ObjectPath
	events chan Event
	done   chan struct{}

	wg        sync.WaitGroup
	closeOnce sync.Once
}

// portalShortcut mirrors the a(sa{sv}) entry expected by BindShortcuts.
type portalShortcut struct {
	ID   string
	Data map[string]dbus.Variant
}

// New opens a session and binds the given shortcuts in a single request, which
// may show a one-time consent dialog. It returns once binding is confirmed.
func New(list []Shortcut, opts ...Option) (*Session, error) {
	if len(list) == 0 {
		return nil, fmt.Errorf("shortcuts: no shortcuts to bind")
	}
	if !Available() {
		return nil, fmt.Errorf("shortcuts: GlobalShortcuts portal not available")
	}

	cfg := config{timeout: 60 * time.Second}
	for _, o := range opts {
		o(&cfg)
	}

	conn, err := portal.Connect(cfg.timeout)
	if err != nil {
		return nil, err
	}
	s := &Session{conn: conn, events: make(chan Event, 8), done: make(chan struct{})}

	created, err := conn.Request(portalShortcuts, "CreateSession", func(token string) []interface{} {
		return []interface{}{map[string]dbus.Variant{
			"handle_token":         dbus.MakeVariant(token),
			"session_handle_token": dbus.MakeVariant(token),
		}}
	})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	handle, _ := created["session_handle"].Value().(string)
	if handle == "" {
		_ = conn.Close()
		return nil, fmt.Errorf("shortcuts: empty session handle")
	}
	s.handle = dbus.ObjectPath(handle)

	// Match activation signals before binding so none are missed afterwards.
	for _, member := range []string{"Activated", "Deactivated"} {
		if err := conn.AddMatch(
			dbus.WithMatchObjectPath(portal.Path),
			dbus.WithMatchInterface(portalShortcuts),
			dbus.WithMatchMember(member),
		); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("shortcuts: add match %s: %w", member, err)
		}
	}

	// GlobalShortcuts has no restore_token; the portal persists bindings per
	// application. After CreateSession, ListShortcuts silently (no dialog)
	// returns the shortcuts bound in a previous run. Bind only when a requested
	// shortcut is missing, so a returning app is not prompted again — while a
	// new build that added a shortcut still triggers a (necessary) bind.
	listed, err := conn.Request(portalShortcuts, "ListShortcuts", func(token string) []interface{} {
		return []interface{}{s.handle, map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(token),
		}}
	})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !allBound(listed, list) {
		if _, err := conn.Request(portalShortcuts, "BindShortcuts", func(token string) []interface{} {
			return []interface{}{s.handle, toPortal(list), "", map[string]dbus.Variant{
				"handle_token": dbus.MakeVariant(token),
			}}
		}); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}

	s.wg.Add(1)
	go s.listen()
	return s, nil
}

// Events returns the channel of activation/deactivation events. It is closed
// when the session is closed or the connection is lost.
func (s *Session) Events() <-chan Event { return s.events }

// Close ends the session and releases its connection.
func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.done)
		err = s.conn.Close()
		s.wg.Wait()
	})
	return err
}

func (s *Session) listen() {
	defer s.wg.Done()
	defer close(s.events)
	for sig := range s.conn.Signals() {
		if sig == nil || len(sig.Body) < 2 {
			continue
		}
		var pressed bool
		switch sig.Name {
		case portalShortcuts + ".Activated":
			pressed = true
		case portalShortcuts + ".Deactivated":
			pressed = false
		default:
			continue
		}
		session, ok := sig.Body[0].(dbus.ObjectPath)
		if !ok || session != s.handle {
			continue
		}
		id, ok := sig.Body[1].(string)
		if !ok {
			continue
		}
		select {
		case s.events <- Event{ID: id, Pressed: pressed}:
		case <-s.done:
			return
		}
	}
}

// boundIDs extracts the shortcut IDs from a ListShortcuts/BindShortcuts
// response, whose "shortcuts" field is an a(sa{sv}) array of (id, metadata).
func boundIDs(results map[string]dbus.Variant) map[string]bool {
	ids := map[string]bool{}
	v, ok := results["shortcuts"]
	if !ok {
		return ids
	}
	entries, ok := v.Value().([][]interface{})
	if !ok {
		return ids
	}
	for _, e := range entries {
		if len(e) > 0 {
			if id, ok := e[0].(string); ok && id != "" {
				ids[id] = true
			}
		}
	}
	return ids
}

// allBound reports whether every requested shortcut is already bound according
// to the portal's response, meaning no BindShortcuts call (and consent dialog)
// is needed.
func allBound(results map[string]dbus.Variant, want []Shortcut) bool {
	have := boundIDs(results)
	for _, sc := range want {
		if !have[sc.ID] {
			return false
		}
	}
	return true
}

func toPortal(list []Shortcut) []portalShortcut {
	out := make([]portalShortcut, 0, len(list))
	for _, sc := range list {
		data := map[string]dbus.Variant{}
		if sc.Description != "" {
			data["description"] = dbus.MakeVariant(sc.Description)
		}
		if sc.PreferredTrigger != "" {
			data["preferred_trigger"] = dbus.MakeVariant(sc.PreferredTrigger)
		}
		out = append(out, portalShortcut{ID: sc.ID, Data: data})
	}
	return out
}
