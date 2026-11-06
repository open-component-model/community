// Command ocm-tui is an interactive terminal UI for exploring and transferring OCM component versions.
//
// Usage: ocm-tui [explore [reference] | transfer [source]]
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"ext.ocm.software/tui/internal/app"
	"ext.ocm.software/tui/internal/ocm"
	"ext.ocm.software/tui/internal/ui"
	"ext.ocm.software/tui/internal/view"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ocm-tui:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("an interactive terminal is required")
	}
	logFile, err := openLog()
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	ctx := context.Background()
	rt, err := ocm.Bootstrap(ctx)
	if err != nil {
		return fmt.Errorf("initializing OCM runtime: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := rt.Shutdown(ctx); err != nil {
			log.Printf("plugin shutdown: %v", err)
		}
	}()

	views := view.All(rt)
	start, err := startView(views, args)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(app.New(views, start), tea.WithAltScreen()).Run()
	return err
}

// startView maps "<view> [arg]" command line arguments to the view to open first.
func startView(views []app.View, args []string) (*ui.OpenMsg, error) {
	if len(args) == 0 {
		return nil, nil
	}
	for _, v := range views {
		if v.Name == args[0] && len(args) <= 2 {
			start := &ui.OpenMsg{View: v.Name}
			if len(args) == 2 {
				start.Arg = args[1]
			}
			return start, nil
		}
	}
	return nil, fmt.Errorf("usage: ocm-tui [explore [reference] | transfer [source]]")
}

// openLog sends logs to ocm-tui/debug.log in the user cache directory,
// keeping the directory the TUI runs in clean.
func openLog() (*os.File, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "ocm-tui")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return tea.LogToFile(filepath.Join(dir, "debug.log"), "ocm-tui")
}
