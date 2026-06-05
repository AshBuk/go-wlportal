// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

// Command wlportal-type injects its arguments as keyboard input into the focused
// window through the RemoteDesktop portal.
//
//	go run ./examples/type "hello, world"
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AshBuk/go-wlportal/typing"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: wlportal-type <text>")
		os.Exit(2)
	}
	if !typing.Available() {
		fmt.Fprintln(os.Stderr, "RemoteDesktop portal not available on this session")
		os.Exit(1)
	}

	tokenPath := ""
	if dir, err := os.UserConfigDir(); err == nil {
		tokenPath = filepath.Join(dir, "wlportal-type", "token")
	}

	kbd, err := typing.NewKeyboard(typing.WithRestoreTokenPath(tokenPath))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer kbd.Close()

	if err := kbd.Type(strings.Join(os.Args[1:], " ")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
