// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

// Command wlportal-shortcuts binds a demo global shortcut and prints its
// activations until interrupted.
//
//	go run ./examples/shortcuts
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AshBuk/go-wlportal/shortcuts"
)

func main() {
	if !shortcuts.Available() {
		fmt.Fprintln(os.Stderr, "GlobalShortcuts portal not available on this session")
		os.Exit(1)
	}

	s, err := shortcuts.New([]shortcuts.Shortcut{
		{ID: "demo", Description: "wlportal demo shortcut", PreferredTrigger: "<Ctrl><Alt>space"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer s.Close()

	go func() {
		for e := range s.Events() {
			if e.Pressed {
				fmt.Printf("activated: %s\n", e.ID)
			}
		}
	}()

	fmt.Println("Listening for shortcut activations. Press Ctrl+C to exit.")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
