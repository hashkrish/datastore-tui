package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/krishnan/datastore-tui/app"
	"github.com/krishnan/datastore-tui/config"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version" || os.Args[1] == "-v") {
		fmt.Println(versionString())
		return
	}

	cfg, err := config.Load(context.Background(), os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return // usage was already printed by the flag package
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "datastore-tui:", err)
		os.Exit(1)
	}

	m := app.New(cfg.Client(), cfg.ReadOnly)
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "datastore-tui:", err)
		os.Exit(1)
	}
}
