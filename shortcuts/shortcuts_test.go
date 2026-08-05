// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package shortcuts

import (
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestToPortal(t *testing.T) {
	out := toPortal([]Shortcut{
		{ID: "full", Description: "desc", PreferredTrigger: "<Ctrl>a"},
		{ID: "bare"},
	})
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}

	if out[0].ID != "full" {
		t.Errorf("ID = %q, want full", out[0].ID)
	}
	if got := out[0].Data["description"].Value(); got != "desc" {
		t.Errorf("description = %v, want desc", got)
	}
	if got := out[0].Data["preferred_trigger"].Value(); got != "<Ctrl>a" {
		t.Errorf("preferred_trigger = %v, want <Ctrl>a", got)
	}

	// Empty optional fields must be omitted, not sent as empty strings.
	if len(out[1].Data) != 0 {
		t.Errorf("bare shortcut data = %v, want empty", out[1].Data)
	}
}

type recordingRequester struct {
	iface  string
	method string
	args   []any
}

func (r *recordingRequester) Request(iface, method string, build func(string) []any) (map[string]dbus.Variant, error) {
	r.iface = iface
	r.method = method
	r.args = build("test-token")
	return nil, nil
}

func TestBindShortcuts(t *testing.T) {
	recorder := &recordingRequester{}
	handle := dbus.ObjectPath("/org/freedesktop/portal/desktop/session/test")
	list := []Shortcut{{ID: "record", Description: "Record", PreferredTrigger: "<Alt>r"}}

	if err := bindShortcuts(recorder, handle, list); err != nil {
		t.Fatalf("bindShortcuts: %v", err)
	}
	if recorder.iface != portalShortcuts || recorder.method != "BindShortcuts" {
		t.Fatalf("request = %s.%s, want %s.BindShortcuts", recorder.iface, recorder.method, portalShortcuts)
	}

	wantArgs := []any{
		handle,
		toPortal(list),
		"",
		map[string]dbus.Variant{"handle_token": dbus.MakeVariant("test-token")},
	}
	if !reflect.DeepEqual(recorder.args, wantArgs) {
		t.Errorf("args = %#v, want %#v", recorder.args, wantArgs)
	}
}
