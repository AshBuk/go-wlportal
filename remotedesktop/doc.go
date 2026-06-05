// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

// Package remotedesktop injects keyboard input into the focused window through
// the org.freedesktop.portal.RemoteDesktop XDG desktop portal.
//
// Unlike wtype (zwp_virtual_keyboard) or ydotool (uinput), injection happens on
// the compositor side, so it works from inside a Flatpak sandbox without extra
// device permissions and is not affected by the Wayland security-context that
// blocks virtual-keyboard for sandboxed clients.
//
// Availability depends on the portal backend: GNOME and KDE implement
// RemoteDesktop; the wlroots/Hyprland portal currently does not. Call Available
// before use and fall back to another method when it returns false.
//
//	if remotedesktop.Available() {
//		kbd, err := remotedesktop.NewKeyboard()
//		if err == nil {
//			defer kbd.Close()
//			_ = kbd.Type("hello")
//		}
//	}
package remotedesktop
