// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package shortcuts

import (
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

func TestAllBound(t *testing.T) {
	want := []Shortcut{{ID: "a"}, {ID: "b"}}

	// shortcuts mirrors the a(sa{sv}) godbus decodes a ListShortcuts response into.
	shortcuts := func(ids ...string) map[string]dbus.Variant {
		entries := make([][]interface{}, 0, len(ids))
		for _, id := range ids {
			entries = append(entries, []interface{}{id, map[string]dbus.Variant{}})
		}
		return map[string]dbus.Variant{"shortcuts": dbus.MakeVariant(entries)}
	}

	if !allBound(shortcuts("a", "b"), want) {
		t.Error("allBound = false, want true when every ID is already bound")
	}
	if allBound(shortcuts("a"), want) {
		t.Error("allBound = true, want false when an ID is missing (app upgrade)")
	}
	if allBound(map[string]dbus.Variant{}, want) {
		t.Error("allBound = true, want false when nothing is bound (first run)")
	}
}
