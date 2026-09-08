package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the global key bindings for the TUI.
type KeyMap struct {
	Quit      key.Binding
	Command   key.Binding
	Help      key.Binding
	Tab       key.Binding
}

// DefaultKeyMap returns the default global key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "command"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch pane"),
		),
	}
}
