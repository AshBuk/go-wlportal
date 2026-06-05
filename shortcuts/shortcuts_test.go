// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package shortcuts

import "testing"

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
