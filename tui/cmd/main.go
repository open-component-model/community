package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	tui "ext.ocm.software/tui"
	"ext.ocm.software/tui/internal/explorer"
	ocmboot "ext.ocm.software/tui/internal/ocm"
	"ext.ocm.software/tui/internal/transfer"
)

func main() {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(os.Stderr, "ocm-tui requires an interactive terminal")
		os.Exit(1)
	}

	f, err := tea.LogToFile("tui-debug.log", "ocm-tui")
	if err != nil {
		log.Fatalf("could not open log file: %v", err)
	}
	defer f.Close()

	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			log.Printf("PANIC: %v\n%s", r, stack)
			fmt.Fprintf(os.Stderr, "\nocm-tui crashed. Details logged to tui-debug.log\n\nPanic: %v\n\n%s\n", r, stack)
			os.Exit(1)
		}
	}()

	ctx := context.Background()
	rt, err := ocmboot.Bootstrap(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize OCM runtime: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := rt.Shutdown(shutdownCtx); err != nil {
			log.Printf("plugin shutdown error: %v", err)
		}
	}()

	fetcherFactory := ocmboot.NewFetcherFactory(rt)
	downloader := ocmboot.NewResourceDownloader(rt)
	transferExec := ocmboot.NewTransferExecutor(rt)

	cfg := tui.Config{
		MenuItems: []tui.MenuItem{
			{
				Label: "Explore components",
				NewView: func(w, h int) tui.View {
					return explorer.NewView(explorer.Config{
						FetcherFactory: fetcherFactory,
						Downloader:     downloader,
					}, w, h)
				},
			},
			{
				Label: "Transfer component versions",
				NewView: func(w, h int) tui.View {
					return transfer.NewView(transfer.Config{
						Executor: transferExec,
					}, w, h)
				},
			},
		},
	}

	p := tea.NewProgram(tui.NewApp(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
