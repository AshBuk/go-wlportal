// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

// Package shortcuts registers global hotkeys through the
// org.freedesktop.portal.GlobalShortcuts XDG desktop portal and reports their
// activations.
//
// Registration happens on the compositor side, so it works from inside a Flatpak
// sandbox without device permissions. The portal is implemented by GNOME and KDE;
// some wlroots compositors expose it but require the binding to be set in their
// own config. Call Available before use.
//
//	if shortcuts.Available() {
//		s, err := shortcuts.New([]shortcuts.Shortcut{
//			{ID: "record", Description: "Start recording", PreferredTrigger: "<Ctrl><Alt>space"},
//		})
//		if err == nil {
//			defer s.Close()
//			for e := range s.Events() {
//				if e.Pressed {
//					// handle e.ID
//				}
//			}
//		}
//	}
package shortcuts
