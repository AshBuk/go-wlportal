// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package remotedesktop

// Common control keysyms (X11 keysymdef).
const (
	KeysymBackSpace = 0xff08
	KeysymTab       = 0xff09
	KeysymReturn    = 0xff0d
	KeysymEscape    = 0xff1b
)

// RuneToKeysym maps a Unicode rune to an X11 keysym. Latin-1 code points map
// 1:1; everything else uses the Unicode keysym range (0x01000000 + code point),
// which lets compositors type characters that are not on the active layout.
func RuneToKeysym(r rune) int32 {
	switch r {
	case '\n':
		return KeysymReturn
	case '\t':
		return KeysymTab
	case '\b':
		return KeysymBackSpace
	}
	if r <= 0x7e || (r >= 0xa0 && r <= 0xff) {
		return int32(r)
	}
	return int32(0x01000000 + r)
}
