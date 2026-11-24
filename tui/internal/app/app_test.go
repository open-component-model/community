package app

import (
	"context"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	"ext.ocm.software/tui/internal/component/list"
	"ext.ocm.software/tui/internal/view/explore"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

// send applies msg and follows the component messages its command produces.
// Other commands, such as cursor blink timers, are not run.
func send(m Model, msg tea.Msg) Model {
	next, cmd := m.Update(msg)
	m = next.(Model)
	if _, isKey := msg.(tea.KeyMsg); !isKey || cmd == nil {
		return m
	}
	switch out := cmd().(type) {
	case list.ChosenMsg:
		return send(m, out)
	}
	return m
}

func TestApp(t *testing.T) {
	open := func(context.Context, string) (explore.Repository, string, string, error) { return nil, "", "", nil }
	views := []View{
		{Name: explore.Name, Label: "Explore components", Hint: "browse component versions", Open: func(ref string) Page { return explore.New(open, ref) }},
		{Name: "other", Label: "Transfer component versions", Hint: "copy component versions", Open: func(ref string) Page { return explore.New(open, ref) }},
	}
	start := func(height int) Model { return send(New(views, nil), tea.WindowSizeMsg{Width: 80, Height: height}) }
	colon := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")}
	enter := tea.KeyMsg{Type: tea.KeyEnter}

	t.Run("menu with logo", func(t *testing.T) {
		golden.RequireEqual(t, start(30).View())
	})
	t.Run("menu without logo when short", func(t *testing.T) {
		r := require.New(t)
		view := start(14).View()
		r.NotContains(view, "_.--'--._")
		r.Contains(view, "Explore components", "the menu stays usable on small terminals")
	})
	t.Run("palette lists every command", func(t *testing.T) {
		golden.RequireEqual(t, send(start(30), colon).View())
	})
	t.Run("palette opens a view and esc closes it", func(t *testing.T) {
		r := require.New(t)
		r.False(send(send(start(30), colon), tea.KeyMsg{Type: tea.KeyEsc}).paletteOpen)
		m := send(send(start(30), colon), enter)
		r.False(m.paletteOpen)
		r.NotNil(m.page)
	})
	t.Run("colon is text while a page takes input", func(t *testing.T) {
		r := require.New(t)
		m := send(start(30), enter)
		r.NotNil(m.page)
		r.False(send(m, colon).paletteOpen, "':' is part of a reference while the prompt has focus")
	})
}
