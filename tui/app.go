package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/components"
	"ext.ocm.software/tui/internal/components/menu"
	"ext.ocm.software/tui/internal/components/palette"
	"ext.ocm.software/tui/internal/explorer"
	"ext.ocm.software/tui/internal/theme"
)

// Config holds the menu items for the TUI application.
type Config struct {
	MenuItems []MenuItem
}

// App is the root bubbletea model.
type App struct {
	config Config
	keys   KeyMap

	// Menu
	menu menu.Menu

	// Active view (nil when showing menu)
	activeView      View
	activeMenuIndex int

	// Command palette
	palette palette.Model

	// Layout
	width  int
	height int
	ready  bool
}

// NewApp creates the root TUI application model.
func NewApp(cfg Config) App {
	// Build menu items.
	var labels []string
	for _, item := range cfg.MenuItems {
		labels = append(labels, item.Label)
	}

	// Build palette entries.
	var entries []palette.Entry
	for i, item := range cfg.MenuItems {
		entries = append(entries, palette.Entry{Label: item.Label, ID: string(rune('0' + i))})
	}
	entries = append(entries, palette.Entry{Label: "Menu (go back)", ID: "menu"})
	entries = append(entries, palette.Entry{Label: "Quit", ID: "quit"})

	return App{
		config:          cfg,
		keys:            DefaultKeyMap(),
		menu:            menu.New(labels...),
		palette:         palette.New(entries),
		activeMenuIndex: -1,
	}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Window resize.
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		a.width = wsm.Width
		a.height = wsm.Height
		a.ready = true
		if a.activeView != nil {
			a.activeView.Resize(wsm.Width, wsm.Height-2)
		}
		return a, nil
	}

	// ctrl+c always quits.
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "ctrl+c" {
		return a, tea.Quit
	}

	// Palette messages (selection results).
	switch msg := msg.(type) {
	case palette.SelectedMsg:
		return a.handlePaletteSelection(msg.Entry)
	case palette.ClosedMsg:
		return a, nil
	}

	// Palette input handling.
	if a.palette.IsOpen() {
		cmd := a.palette.Update(msg)
		return a, cmd
	}

	// ":" opens command palette.
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, a.keys.Command) {
		cursorAt := 0
		if a.activeMenuIndex >= 0 {
			cursorAt = a.activeMenuIndex
		}
		cmd := a.palette.Open(cursorAt)
		return a, cmd
	}

	// Active view: esc -> OnBackPressed.
	if a.activeView != nil {
		if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyEsc {
			if a.activeView.OnBackPressed() == BackExit {
				a.activeView = nil
				a.activeMenuIndex = -1
				return a, nil
			}
			return a, nil
		}

		// Cross-view: explorer -> transfer.
		if trMsg, ok := msg.(explorer.TransferRequestMsg); ok {
			return a.handleTransferRequest(trMsg)
		}

		cmd := a.activeView.Update(msg)
		return a, cmd
	}

	// Menu mode.
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "q" {
		return a, tea.Quit
	}

	return a.updateMenu(msg)
}

func (a App) handlePaletteSelection(entry palette.Entry) (tea.Model, tea.Cmd) {
	switch entry.ID {
	case "quit":
		return a, tea.Quit
	case "menu":
		a.activeView = nil
		a.activeMenuIndex = -1
		return a, nil
	default:
		// Menu item by index.
		for i, item := range a.config.MenuItems {
			if entry.Label == item.Label {
				a.activeView = item.NewView(a.width, a.height-2)
				a.activeMenuIndex = i
				cmd := a.activeView.Init()
				return a, cmd
			}
		}
	}
	return a, nil
}

func (a App) handleTransferRequest(msg explorer.TransferRequestMsg) (tea.Model, tea.Cmd) {
	// Build the full source reference: "repo//component:version"
	source := fmt.Sprintf("%s:%s", msg.Component, msg.Version)
	if msg.Reference != "" {
		if idx := strings.Index(msg.Reference, "//"); idx >= 0 {
			repoPrefix := msg.Reference[:idx]
			source = fmt.Sprintf("%s//%s:%s", repoPrefix, msg.Component, msg.Version)
		}
	}

	for i, item := range a.config.MenuItems {
		if strings.Contains(item.Label, "Transfer") {
			view := item.NewView(a.width, a.height-2)
			// Pre-fill source and skip to target step.
			if tv, ok := view.(interface{ SetSource(string) }); ok {
				tv.SetSource(source)
			}
			a.activeView = view
			a.activeMenuIndex = i
			cmd := a.activeView.Init()
			return a, cmd
		}
	}
	return a, nil
}

func (a App) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}

	if a.menu.Update(keyMsg) {
		idx := a.menu.Selected()
		if idx < len(a.config.MenuItems) {
			item := a.config.MenuItems[idx]
			a.activeView = item.NewView(a.width, a.height-2)
			a.activeMenuIndex = idx
			cmd := a.activeView.Init()
			return a, cmd
		}
	}

	return a, nil
}

func (a App) View() string {
	if !a.ready {
		return "Initializing..."
	}

	if a.palette.IsOpen() {
		return a.palette.View(a.width, a.height)
	}

	if a.activeView != nil {
		layout := components.Layout{
			Title:      "ocm tui",
			StatusInfo: a.activeView.StatusInfo(),
			Content:    a.activeView.Render(),
			Hotkeys:    a.activeView.Hotkeys(),
			Width:      a.width,
			Height:     a.height,
		}
		return layout.Render()
	}

	return a.viewMenu()
}

func (a App) viewMenu() string {
	t := theme.Current()

	var sections []string
	sections = append(sections, t.Title.MarginBottom(2).Render("OCM TUI"))
	sections = append(sections, a.menu.View())
	sections = append(sections, t.Help.MarginTop(2).Render("j/k: navigate  enter: select  :: command palette  q: quit"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, content)
}
