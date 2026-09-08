// Package menu provides a cursor-navigable menu list component.
package menu

import (
	tea "github.com/charmbracelet/bubbletea"

	"ext.ocm.software/tui/internal/theme"
)

// Menu is a vertical selectable list with cursor navigation.
type Menu struct {
	Items  []string
	Cursor int
}

// New creates a menu with the given items.
func New(items ...string) Menu {
	return Menu{Items: items}
}

// Update handles j/k/up/down navigation and enter selection.
// Returns true if an item was selected (enter pressed).
func (m *Menu) Update(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		if m.Cursor > 0 {
			m.Cursor--
		}
	case tea.KeyDown:
		if m.Cursor < len(m.Items)-1 {
			m.Cursor++
		}
	case tea.KeyEnter:
		return true
	}

	switch msg.String() {
	case "j":
		if m.Cursor < len(m.Items)-1 {
			m.Cursor++
		}
	case "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	}

	return false
}

// Selected returns the currently highlighted index.
func (m Menu) Selected() int {
	return m.Cursor
}

// View renders the menu list.
func (m Menu) View() string {
	t := theme.Current()

	var lines string
	for i, item := range m.Items {
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}
		line := cursor + item
		if i == m.Cursor {
			line = t.Selected.Render(line)
		}
		if i > 0 {
			lines += "\n"
		}
		lines += line
	}
	return lines
}
