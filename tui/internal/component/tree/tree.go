// Package tree is an expandable tree component with lazily loaded children.
package tree

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"ext.ocm.software/tui/internal/ui"
)

// Node is a tree item. IDs must be unique within the tree; Data is owned by the parent.
type Node struct {
	ID       string
	Label    string
	Data     any
	Children []Node
	// Lazy nodes have children that the parent loads when ExpandMsg arrives.
	Lazy     bool
	Expanded bool
	Loading  bool
}

func (n Node) expandable() bool { return n.Lazy || len(n.Children) > 0 }

// Keys are the tree bindings; parents show them in their help.
var Keys = struct{ Up, Down, PageUp, PageDown, Expand, Collapse key.Binding }{
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
	PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),
	Expand:   key.NewBinding(key.WithKeys("enter", "right", "l"), key.WithHelp("enter", "expand")),
	Collapse: key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("h", "collapse")),
}

// ExpandMsg asks the parent to load the children of a lazy node and pass them to SetChildren.
type ExpandMsg struct{ Node Node }

// Model shows the visible part of a tree with a cursor.
type Model struct {
	roots   []Node
	cursor  int
	height  int
	Focused bool
}

// New creates a focused tree.
func New(roots ...Node) Model {
	return Model{roots: roots, Focused: true}
}

type row struct {
	node  Node
	depth int
}

func flatten(nodes []Node, depth int) []row {
	var rows []row
	for _, n := range nodes {
		rows = append(rows, row{n, depth})
		if n.Expanded {
			rows = append(rows, flatten(n.Children, depth+1)...)
		}
	}
	return rows
}

func (m Model) rows() []row { return flatten(m.roots, 0) }

// Selected returns the node under the cursor.
func (m Model) Selected() (Node, bool) {
	rows := m.rows()
	if m.cursor < len(rows) {
		return rows[m.cursor].node, true
	}
	return Node{}, false
}

// Modify returns a tree in which the node with id is replaced by f(node).
func (m Model) Modify(id string, f func(Node) Node) Model {
	m.roots = modify(m.roots, id, f)
	m.cursor = max(min(m.cursor, len(m.rows())-1), 0)
	return m
}

func modify(nodes []Node, id string, f func(Node) Node) []Node {
	out := slices.Clone(nodes)
	for i, n := range out {
		if n.ID == id {
			out[i] = f(n)
		} else if len(n.Children) > 0 {
			out[i].Children = modify(n.Children, id, f)
		}
	}
	return out
}

// SetChildren fills a lazy node with its loaded children and expands it.
func (m Model) SetChildren(id string, children []Node) Model {
	return m.Modify(id, func(n Node) Node {
		n.Children, n.Lazy, n.Loading, n.Expanded = children, false, false, true
		return n
	})
}

// LoadFailed collapses a lazy node whose children could not be loaded.
func (m Model) LoadFailed(id string) Model {
	return m.Modify(id, func(n Node) Node {
		n.Loading, n.Expanded = false, false
		return n
	})
}

// Collapse collapses the selected node and reports whether there was anything to collapse.
func (m Model) Collapse() (Model, bool) {
	n, ok := m.Selected()
	if !ok || !n.Expanded {
		return m, false
	}
	return m.Modify(n.ID, func(n Node) Node { n.Expanded = false; return n }), true
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case tea.KeyMsg:
		last := len(m.rows()) - 1
		switch {
		case key.Matches(msg, Keys.Up):
			m.cursor = max(m.cursor-1, 0)
		case key.Matches(msg, Keys.Down):
			m.cursor = max(min(m.cursor+1, last), 0)
		case key.Matches(msg, Keys.PageUp):
			m.cursor = max(m.cursor-10, 0)
		case key.Matches(msg, Keys.PageDown):
			m.cursor = max(min(m.cursor+10, last), 0)
		case key.Matches(msg, Keys.Collapse):
			m, _ = m.Collapse()
		case key.Matches(msg, Keys.Expand):
			n, ok := m.Selected()
			if !ok || !n.expandable() || n.Expanded {
				return m, nil
			}
			load := n.Lazy && len(n.Children) == 0
			n.Expanded, n.Loading = true, load
			m = m.Modify(n.ID, func(Node) Node { return n })
			if load {
				return m, func() tea.Msg { return ExpandMsg{Node: n} }
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	rows := m.rows()
	start, end := ui.Window(len(rows), m.cursor, max(m.height, 1))
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		n := rows[i].node
		prefix := "  "
		switch {
		case n.Loading:
			prefix = "… "
		case n.expandable() && n.Expanded:
			prefix = "▾ "
		case n.expandable():
			prefix = "▸ "
		}
		line := strings.Repeat("  ", rows[i].depth) + prefix + n.Label
		if i == m.cursor && m.Focused {
			line = ui.Cursor.Render(line)
		} else if i == m.cursor {
			line = ui.CursorDim.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
