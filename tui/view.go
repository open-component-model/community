package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"ext.ocm.software/tui/internal/components"
)

// Back action results — used by OnBackPressed.
const (
	BackConsumed = 0
	BackExit     = 1
)

// View is the interface that each top-level TUI screen implements.
type View interface {
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	Render() string
	StatusInfo() string
	Hotkeys() []components.Hotkey
	OnBackPressed() int
	Resize(width, height int)
}

// MenuItem defines a menu entry in the root TUI.
type MenuItem struct {
	Label   string
	NewView func(width, height int) View
}
