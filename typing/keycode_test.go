// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package typing

import (
	"errors"
	"fmt"
	"testing"
)

// comboRecorder records notifyKeycode calls and fails the n-th one (1-based;
// 0 never fails).
type comboRecorder struct {
	calls  []string
	failAt int
	err    error
}

func (r *comboRecorder) send(kc Keycode, state KeyState) error {
	r.calls = append(r.calls, fmt.Sprintf("%d:%d", kc, state))
	if r.failAt == len(r.calls) {
		return r.err
	}
	return nil
}

func assertCalls(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("calls[%d] = %s, want %s (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestPressCombo(t *testing.T) {
	combo := []Keycode{KeycodeLeftCtrl, KeycodeLeftShift, KeycodeV}
	errBoom := errors.New("boom")

	t.Run("presses in order, releases in reverse", func(t *testing.T) {
		r := &comboRecorder{}
		if err := pressCombo(r.send, combo); err != nil {
			t.Fatalf("pressCombo = %v, want nil", err)
		}
		assertCalls(t, r.calls, []string{
			"29:1", "42:1", "47:1", // Ctrl, Shift, V pressed
			"47:0", "42:0", "29:0", // released in reverse
		})
	})

	t.Run("empty combo sends nothing", func(t *testing.T) {
		r := &comboRecorder{}
		if err := pressCombo(r.send, nil); err != nil {
			t.Fatalf("pressCombo = %v, want nil", err)
		}
		assertCalls(t, r.calls, nil)
	})

	t.Run("press failure releases already-pressed keys", func(t *testing.T) {
		r := &comboRecorder{failAt: 3, err: errBoom} // V press fails
		if err := pressCombo(r.send, combo); !errors.Is(err, errBoom) {
			t.Fatalf("pressCombo = %v, want %v", err, errBoom)
		}
		assertCalls(t, r.calls, []string{
			"29:1", "42:1", "47:1", // V press attempt fails
			"42:0", "29:0", // Shift, Ctrl rolled back in reverse
		})
	})

	t.Run("release failure still releases the rest", func(t *testing.T) {
		r := &comboRecorder{failAt: 5, err: errBoom} // Shift release fails
		if err := pressCombo(r.send, combo); !errors.Is(err, errBoom) {
			t.Fatalf("pressCombo = %v, want %v", err, errBoom)
		}
		assertCalls(t, r.calls, []string{
			"29:1", "42:1", "47:1",
			"47:0", "42:0", // Shift release fails
			"29:0", // Ctrl still released best-effort
		})
	})
}
