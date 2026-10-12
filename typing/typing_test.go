// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package typing

import (
	"errors"
	"testing"
)

// sessionRecorder counts retryOnce's session opens, drops and fn runs. fn fails
// on the runs listed in fail, reporting sent for each failed run.
type sessionRecorder struct {
	opens, drops, runs int
	openErr            error
	fail               map[int]bool
	sent               bool
}

var errBoom = errors.New("boom")

func (r *sessionRecorder) open() error { r.opens++; return r.openErr }
func (r *sessionRecorder) drop()       { r.drops++ }
func (r *sessionRecorder) fn() (bool, error) {
	r.runs++
	if r.fail[r.runs] {
		return r.sent, errBoom
	}
	return true, nil
}

func (r *sessionRecorder) run(t *testing.T, wantErr error, opens, drops, runs int) {
	t.Helper()
	if err := retryOnce(r.open, r.drop, r.fn); !errors.Is(err, wantErr) {
		t.Fatalf("retryOnce = %v, want %v", err, wantErr)
	}
	if r.opens != opens || r.drops != drops || r.runs != runs {
		t.Fatalf("opens/drops/runs = %d/%d/%d, want %d/%d/%d",
			r.opens, r.drops, r.runs, opens, drops, runs)
	}
}

func TestRetryOnce(t *testing.T) {
	t.Run("success keeps the session", func(t *testing.T) {
		(&sessionRecorder{}).run(t, nil, 1, 0, 1)
	})

	t.Run("open failure is returned without running fn", func(t *testing.T) {
		(&sessionRecorder{openErr: errBoom}).run(t, errBoom, 1, 0, 0)
	})

	t.Run("failure before any key retries on a fresh session", func(t *testing.T) {
		(&sessionRecorder{fail: map[int]bool{1: true}}).run(t, nil, 2, 1, 2)
	})

	t.Run("failure after a sent key drops without retry", func(t *testing.T) {
		(&sessionRecorder{fail: map[int]bool{1: true}, sent: true}).run(t, errBoom, 1, 1, 1)
	})

	t.Run("second failure is returned", func(t *testing.T) {
		(&sessionRecorder{fail: map[int]bool{1: true, 2: true}}).run(t, errBoom, 2, 2, 2)
	})
}
