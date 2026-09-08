// Package palette provides a filterable command palette overlay.
package palette

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/theme"
)

// Entry is a single item in the command palette.
type Entry struct {
	Label string
	ID    string // opaque identifier for the caller
}

// SelectedMsg is sent when the user selects an entry.
type SelectedMsg struct {
	Entry Entry
}

// ClosedMsg is sent when the user closes the palette without selecting.
type ClosedMsg struct{}

// Model is the command palette state.
type Model struct {
	entries []Entry
	input   textinput.Model
	cursor  int
	open    bool
}

// New creates a new command palette.
func New(entries []Entry) Model {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 64
	ti.Width = 40
	ti.Prompt = ": "

	return Model{
		entries: entries,
		input:   ti,
	}
}

// Open activates the palette with the cursor at the given index.
func (m *Model) Open(cursorAt int) tea.Cmd {
	m.open = true
	m.cursor = cursorAt
	m.input.SetValue("")
	m.input.Focus()
	return textinput.Blink
}

// IsOpen returns whether the palette is visible.
func (m Model) IsOpen() bool {
	return m.open
}

// Filtered returns entries matching the current filter text.
func (m Model) Filtered() []Entry {
	filter := strings.ToLower(strings.TrimSpace(m.input.Value()))
	if filter == "" {
		return m.entries
	}
	var result []Entry
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.Label), filter) {
			result = append(result, e)
		}
	}
	return result
}

// Update handles palette input. Returns a tea.Cmd (may contain SelectedMsg or ClosedMsg).
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	filtered := m.Filtered()

	switch km.Type {
	case tea.KeyEsc:
		m.open = false
		m.input.Blur()
		return func() tea.Msg { return ClosedMsg{} }

	case tea.KeyEnter:
		if len(filtered) > 0 && m.cursor < len(filtered) {
			entry := filtered[m.cursor]
			m.open = false
			m.input.Blur()
			return func() tea.Msg { return SelectedMsg{Entry: entry} }
		}
		m.open = false
		m.input.Blur()
		return func() tea.Msg { return ClosedMsg{} }

	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return nil

	case tea.KeyDown:
		if m.cursor < len(filtered)-1 {
			m.cursor++
		}
		return nil
	}

	// Forward to text input for filtering.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.cursor = 0 // reset cursor when filter changes
	return cmd
}

// View renders the palette as a centered popup.
func (m Model) View(width, height int) string {
	t := theme.Current()
	filtered := m.Filtered()

	border := t.ModalBorder.Width(50)

	var sections []string
	sections = append(sections, t.Title.MarginBottom(1).Render("Command Palette"))
	sections = append(sections, m.input.View())
	sections = append(sections, "")

	maxShow := 8
	if len(filtered) < maxShow {
		maxShow = len(filtered)
	}
	for i := 0; i < maxShow; i++ {
		entry := filtered[i]
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		line := cursor + entry.Label
		if i == m.cursor {
			line = t.Selected.Render(line)
		} else {
			line = t.DimText.Render(line)
		}
		sections = append(sections, line)
	}
	if len(filtered) > maxShow {
		sections = append(sections, t.DimText.Render("  ..."))
	}

	sections = append(sections, "")
	sections = append(sections, t.DimText.Render("enter: select  esc: close  type to filter"))

	popup := border.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
}
