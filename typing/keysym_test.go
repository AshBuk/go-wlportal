// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package typing

import "testing"

func TestRuneToKeysym(t *testing.T) {
	cases := map[rune]int32{
		'a':  0x61, // ASCII 1:1
		'A':  0x41, // ASCII 1:1
		' ':  0x20, // space 1:1
		'\n': KeysymReturn,
		'\t': KeysymTab,
		'\b': KeysymBackSpace,
		'é':  0xe9,             // Latin-1 1:1
		'ÿ':  0xff,             // Latin-1 upper bound
		'Ā':  0x01000100,       // beyond Latin-1 -> Unicode keysym
		'м':  0x01000000 + 'м', // Cyrillic via Unicode keysym range
		'👍':  0x01000000 + '👍', // emoji
	}
	for r, want := range cases {
		if got := RuneToKeysym(r); got != want {
			t.Errorf("RuneToKeysym(%q) = %#x, want %#x", r, got, want)
		}
	}
}
