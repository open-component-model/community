package tree

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"ext.ocm.software/tui/internal/theme"
)

// NodeExpandedMsg is sent when a node is expanded by the user.
// The domain model should handle this to trigger data fetching if needed.
type NodeExpandedMsg struct{ Node *Node }

// CursorChangedMsg is sent when the cursor moves to a different node.
type CursorChangedMsg struct{ Node *Node }

// Model handles tree navigation state. Domain views embed this and
// delegate key handling to it.
type Model struct {
	Keys    KeyMap
	Roots   []*Node
	Cursor  int
	Visible []*Node
}

// New creates a new tree model.
func New() Model {
	return Model{
		Keys: DefaultKeyMap(),
	}
}

// SetRoots replaces the tree roots and recomputes visible nodes.
func (m *Model) SetRoots(roots []*Node) {
	m.Roots = roots
	m.Visible = Flatten(m.Roots)
	if m.Cursor >= len(m.Visible) {
		m.Cursor = len(m.Visible) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}

// Rebuild recomputes visible nodes from the current roots.
func (m *Model) Rebuild() {
	m.Visible = Flatten(m.Roots)
	if m.Cursor >= len(m.Visible) {
		m.Cursor = len(m.Visible) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}

// Selected returns the currently selected node, or nil if the tree is empty.
func (m Model) Selected() *Node {
	if len(m.Visible) == 0 || m.Cursor >= len(m.Visible) {
		return nil
	}
	return m.Visible[m.Cursor]
}

// Update handles tree navigation keys. Returns true if the key was handled.
func (m *Model) Update(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.Keys.Up):
		if m.Cursor > 0 {
			m.Cursor--
			return cursorChanged(m), true
		}
		return nil, true

	case key.Matches(msg, m.Keys.Down):
		if m.Cursor < len(m.Visible)-1 {
			m.Cursor++
			return cursorChanged(m), true
		}
		return nil, true

	case key.Matches(msg, m.Keys.PageUp):
		m.Cursor -= 10
		if m.Cursor < 0 {
			m.Cursor = 0
		}
		return cursorChanged(m), true

	case key.Matches(msg, m.Keys.PageDown):
		m.Cursor += 10
		if m.Cursor >= len(m.Visible) {
			m.Cursor = len(m.Visible) - 1
		}
		if m.Cursor < 0 {
			m.Cursor = 0
		}
		return cursorChanged(m), true

	case key.Matches(msg, m.Keys.Expand):
		if len(m.Visible) > 0 {
			node := m.Visible[m.Cursor]
			if node.Expandable && !node.Expanded {
				node.Expanded = true
				m.Visible = Flatten(m.Roots)
				return func() tea.Msg { return NodeExpandedMsg{Node: node} }, true
			}
		}
		return nil, true

	case key.Matches(msg, m.Keys.Collapse):
		if len(m.Visible) > 0 {
			node := m.Visible[m.Cursor]
			if node.Expandable && node.Expanded {
				node.Expanded = false
				m.Visible = Flatten(m.Roots)
				return cursorChanged(m), true
			}
		}
		return nil, true
	}

	return nil, false
}

func cursorChanged(m *Model) tea.Cmd {
	node := m.Selected()
	if node == nil {
		return nil
	}
	return func() tea.Msg { return CursorChangedMsg{Node: node} }
}

// Render renders the tree pane with the given height.
func (m Model) Render(height int, focused bool) string {
	if len(m.Visible) == 0 {
		return "No items."
	}

	var lines []string

	scrollOffset := 0
	if m.Cursor >= height {
		scrollOffset = m.Cursor - height + 1
	}

	end := scrollOffset + height
	if end > len(m.Visible) {
		end = len(m.Visible)
	}

	for i := scrollOffset; i < end; i++ {
		node := m.Visible[i]
		line := RenderNode(node, i == m.Cursor)

		if i == m.Cursor {
			t := theme.Current()
			if focused {
				line = t.Cursor.Render(line)
			} else {
				line = t.CursorDim.Render(line)
			}
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
