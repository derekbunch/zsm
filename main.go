package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	layout := flag.String("layout", "", "zellij layout to use (overrides config)")
	float := flag.Bool("f", false, "fullscreen mode (use full terminal height; intended for floating panes)")
	flag.BoolVar(float, "float", false, "fullscreen mode (use full terminal height; intended for floating panes)")
	flag.Parse()

	// Set ZSM_DEBUG=/tmp/zsm.log (or any path) to write debug output via log.Println.
	// Without this, log.Print writes to stderr and corrupts the TUI.
	if path := os.Getenv("ZSM_DEBUG"); path != "" {
		f, err := tea.LogToFile(path, "zsm")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening debug log: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		debugEnabled = true
	}

	cfg := loadConfig()

	if *layout != "" {
		cfg.Layout = *layout
	}
	// -f forces fullscreen — set Height to 0 so the list fills the terminal.
	if *float {
		cfg.Height = 0
	}

	sessions, err := listSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing sessions: %v\n", err)
		os.Exit(1)
	}

	dirs, err := listDirectories()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing directories: %v\n", err)
		os.Exit(1)
	}

	items := buildItems(sessions, dirs, maxNameLen(), cfg.PathAliases)
	p := tea.NewProgram(newModel(items, cfg))

	final, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Execute post-TUI action (attach/create needs the terminal back)
	if fm, ok := final.(model); ok && fm.action != nil {
		item := fm.chosen
		verb := "creating"
		if item.Active {
			verb = "attaching to"
		}
		fmt.Fprintf(os.Stderr, "zsm: %s session %q", verb, item.SessionName())
		if item.Dir != "" {
			fmt.Fprintf(os.Stderr, " in %s", item.Dir)
		}
		fmt.Fprintln(os.Stderr)

		if err := fm.action(); err != nil {
			fmt.Fprintf(os.Stderr, "zsm: error: %v\n", err)
			os.Exit(1)
		}
	}
}
