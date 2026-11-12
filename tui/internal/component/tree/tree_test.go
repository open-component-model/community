package tree

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func keyMsg(k string) tea.KeyMsg {
	if k == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func press(m Model, keys ...string) (Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		m, cmd = m.Update(keyMsg(k))
	}
	return m, cmd
}

func fixture() Model {
	m := New(
		Node{ID: "a", Label: "a", Expanded: true, Children: []Node{{ID: "a/1", Label: "a1"}, {ID: "a/2", Label: "a2"}}},
		Node{ID: "b", Label: "b", Children: []Node{{ID: "b/1", Label: "b1"}}},
		Node{ID: "c", Label: "c", Lazy: true},
	)
	m, _ = m.Update(tea.WindowSizeMsg{Height: 10})
	return m
}

func TestNavigate(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want string
	}{
		{"starts at the first node", nil, "a"},
		{"j walks visible nodes in order", []string{"j", "j", "j"}, "b"},
		{"collapsed children are skipped", []string{"j", "j", "j", "j"}, "c"},
		{"cursor stops at the last node", []string{"j", "j", "j", "j", "j", "j"}, "c"},
		{"k stops at the first node", []string{"j", "k", "k"}, "a"},
		{"expanding shows children", []string{"j", "j", "j", "enter", "j"}, "b/1"},
		{"h collapses the selected node", []string{"h", "j"}, "b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			m, _ := press(fixture(), tt.keys...)
			n, ok := m.Selected()
			r.True(ok)
			r.Equal(tt.want, n.ID)
		})
	}
}

func TestExpand(t *testing.T) {
	t.Run("lazy node asks the parent for children and shows loading", func(t *testing.T) {
		r := require.New(t)
		before, _ := press(fixture(), "j", "j", "j", "j")
		m, cmd := press(before, "enter")
		r.Equal(ExpandMsg{Node: Node{ID: "c", Label: "c", Lazy: true, Expanded: true, Loading: true}}, cmd())
		r.Contains(m.View(), "… c")

		m = m.SetChildren("c", []Node{{ID: "c/1", Label: "c1"}})
		r.Contains(m.View(), "▾ c")
		r.Contains(m.View(), "c1")
	})
	t.Run("node with children expands without a message", func(t *testing.T) {
		r := require.New(t)
		_, cmd := press(fixture(), "j", "j", "j", "enter")
		r.Nil(cmd)
	})
	t.Run("updates never change the previous model", func(t *testing.T) {
		r := require.New(t)
		before := fixture()
		_, _ = press(before, "h")
		after := before.Modify("a/2", func(n Node) Node { n.Label = "changed"; return n })
		r.Contains(after.View(), "changed")
		r.Contains(before.View(), "a2")
		r.NotContains(before.View(), "changed")
	})
	t.Run("collapse reports whether anything was collapsed", func(t *testing.T) {
		r := require.New(t)
		m, collapsed := fixture().Collapse()
		r.True(collapsed)
		r.NotContains(m.View(), "a1")
		_, collapsed = m.Collapse()
		r.False(collapsed)
	})
	t.Run("failed load collapses the node again", func(t *testing.T) {
		r := require.New(t)
		m, _ := press(fixture(), "j", "j", "j", "j", "enter")
		m = m.LoadFailed("c")
		n, _ := m.Selected()
		r.False(n.Loading)
		r.False(n.Expanded)
	})
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	r := require.New(t)
	var roots []Node
	for _, id := range []string{"n1", "n2", "n3", "n4", "n5"} {
		roots = append(roots, Node{ID: id, Label: id})
	}
	m, _ := New(roots...).Update(tea.WindowSizeMsg{Height: 2})
	m, _ = press(m, "j", "j", "j")
	r.Contains(m.View(), "n4")
	r.NotContains(m.View(), "n1")
}
