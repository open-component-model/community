package prompt

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func typed(m Model, s string) Model {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return m
}

func enter(m Model) tea.Msg {
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestSubmit(t *testing.T) {
	tests := []struct {
		name  string
		setup func(Model) Model
		want  tea.Msg
	}{
		{"typed value is submitted trimmed", func(m Model) Model { return typed(m, "  ghcr.io/x//c:1  ") }, SubmitMsg{ID: "ref", Value: "ghcr.io/x//c:1"}},
		{"set value is submitted", func(m Model) Model { return m.SetValue("ctf::/tmp/ctf") }, SubmitMsg{ID: "ref", Value: "ctf::/tmp/ctf"}},
		{"empty input is not submitted", func(m Model) Model { return typed(m, "   ") }, nil},
		{"blurred prompt is not submitted", func(m Model) Model { return typed(m, "x").Blur() }, nil},
		{"blurred prompt ignores typing until focused again", func(m Model) Model {
			m, _ = typed(m.Blur(), "lost").Focus()
			return typed(m, "kept")
		}, SubmitMsg{ID: "ref", Value: "kept"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			r.Equal(tt.want, enter(tt.setup(New("ref", "Title", "Subtitle", "placeholder"))))
		})
	}
}

func TestView(t *testing.T) {
	r := require.New(t)
	m, _ := New("ref", "Explore Components", "Enter a component reference:", "").Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m.Status, m.Err = "Connecting...", errors.New("boom")
	view := m.View()
	for _, want := range []string{"Explore Components", "Enter a component reference:", "Connecting...", "Error: boom"} {
		r.Contains(view, want)
	}
}
