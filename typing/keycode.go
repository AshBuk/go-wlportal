// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package typing

// Keycode is a Linux input-event/evdev keyboard code for
// NotifyKeyboardKeycode. It is useful for shortcuts whose physical keys matter,
// such as paste, where a keysym may be layout-dependent.
type Keycode int32

// Linux input-event/evdev keycodes (KEY_* in linux/input-event-codes.h),
// covering the keys commonly used in shortcuts. Any other key can be passed
// as a raw Keycode value.
const (
	KeycodeEsc        Keycode = 1
	Keycode1          Keycode = 2
	Keycode2          Keycode = 3
	Keycode3          Keycode = 4
	Keycode4          Keycode = 5
	Keycode5          Keycode = 6
	Keycode6          Keycode = 7
	Keycode7          Keycode = 8
	Keycode8          Keycode = 9
	Keycode9          Keycode = 10
	Keycode0          Keycode = 11
	KeycodeMinus      Keycode = 12
	KeycodeEqual      Keycode = 13
	KeycodeBackspace  Keycode = 14
	KeycodeTab        Keycode = 15
	KeycodeQ          Keycode = 16
	KeycodeW          Keycode = 17
	KeycodeE          Keycode = 18
	KeycodeR          Keycode = 19
	KeycodeT          Keycode = 20
	KeycodeY          Keycode = 21
	KeycodeU          Keycode = 22
	KeycodeI          Keycode = 23
	KeycodeO          Keycode = 24
	KeycodeP          Keycode = 25
	KeycodeLeftBrace  Keycode = 26
	KeycodeRightBrace Keycode = 27
	KeycodeEnter      Keycode = 28
	KeycodeLeftCtrl   Keycode = 29
	KeycodeA          Keycode = 30
	KeycodeS          Keycode = 31
	KeycodeD          Keycode = 32
	KeycodeF          Keycode = 33
	KeycodeG          Keycode = 34
	KeycodeH          Keycode = 35
	KeycodeJ          Keycode = 36
	KeycodeK          Keycode = 37
	KeycodeL          Keycode = 38
	KeycodeSemicolon  Keycode = 39
	KeycodeApostrophe Keycode = 40
	KeycodeGrave      Keycode = 41
	KeycodeLeftShift  Keycode = 42
	KeycodeBackslash  Keycode = 43
	KeycodeZ          Keycode = 44
	KeycodeX          Keycode = 45
	KeycodeC          Keycode = 46
	KeycodeV          Keycode = 47
	KeycodeB          Keycode = 48
	KeycodeN          Keycode = 49
	KeycodeM          Keycode = 50
	KeycodeComma      Keycode = 51
	KeycodeDot        Keycode = 52
	KeycodeSlash      Keycode = 53
	KeycodeRightShift Keycode = 54
	KeycodeLeftAlt    Keycode = 56
	KeycodeSpace      Keycode = 57
	KeycodeF1         Keycode = 59
	KeycodeF2         Keycode = 60
	KeycodeF3         Keycode = 61
	KeycodeF4         Keycode = 62
	KeycodeF5         Keycode = 63
	KeycodeF6         Keycode = 64
	KeycodeF7         Keycode = 65
	KeycodeF8         Keycode = 66
	KeycodeF9         Keycode = 67
	KeycodeF10        Keycode = 68
	KeycodeF11        Keycode = 87
	KeycodeF12        Keycode = 88
	KeycodeRightCtrl  Keycode = 97
	KeycodeRightAlt   Keycode = 100
	KeycodeHome       Keycode = 102
	KeycodeUp         Keycode = 103
	KeycodePageUp     Keycode = 104
	KeycodeLeft       Keycode = 105
	KeycodeRight      Keycode = 106
	KeycodeEnd        Keycode = 107
	KeycodeDown       Keycode = 108
	KeycodePageDown   Keycode = 109
	KeycodeInsert     Keycode = 110
	KeycodeDelete     Keycode = 111
	KeycodeLeftMeta   Keycode = 125
	KeycodeRightMeta  Keycode = 126
)

// pressCombo presses all keycodes in order, then releases them in reverse
// order. On error it releases whatever is still pressed (best-effort) and
// returns the first failure.
func pressCombo(send func(Keycode, KeyState) error, keycodes []Keycode) error {
	release := func(pressed []Keycode) {
		for i := len(pressed) - 1; i >= 0; i-- {
			_ = send(pressed[i], Released)
		}
	}
	pressed := make([]Keycode, 0, len(keycodes))
	for _, keycode := range keycodes {
		if err := send(keycode, Pressed); err != nil {
			release(pressed)
			return err
		}
		pressed = append(pressed, keycode)
	}
	for i := len(pressed) - 1; i >= 0; i-- {
		if err := send(pressed[i], Released); err != nil {
			release(pressed[:i])
			return err
		}
	}
	return nil
}
