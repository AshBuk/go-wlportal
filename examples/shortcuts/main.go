// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

// Command wlportal-shortcuts binds a demo global shortcut and prints its
// activations until interrupted.
//
//	go run ./examples/shortcuts
//	go run ./examples/shortcuts -configure
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AshBuk/go-wlportal/shortcuts"
)

func main() {
	configure := flag.Bool("configure", false, "open the compositor's shortcut configuration UI")
	flag.Parse()

	if !shortcuts.Available() {
		fmt.Fprintln(os.Stderr, "GlobalShortcuts portal not available on this session")
		os.Exit(1)
	}
	fmt.Printf("ConfigureShortcuts supported: %t\n", shortcuts.Configurable())

	s, err := shortcuts.New([]shortcuts.Shortcut{
		{ID: "demo", Description: "wlportal demo shortcut", PreferredTrigger: "<Ctrl><Alt>space"},
	},
		// GNOME rejects an unidentified app; in a real app this must match an
		// installed .desktop file.
		shortcuts.WithAppID("io.github.ashbuk.wlportal-shortcuts"),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer s.Close()

	if *configure {
		// No window of our own, so the portal parents the UI to the compositor.
		if err := s.Configure(""); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}

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
