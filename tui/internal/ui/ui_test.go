package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

func TestFrameFillsWindow(t *testing.T) {
	tests := []struct {
		name     string
		size     tea.WindowSizeMsg
		wantLogo bool
	}{
		{"small window has a one-line header", tea.WindowSizeMsg{Width: 80, Height: 20}, false},
		{"narrow tall window has a one-line header", tea.WindowSizeMsg{Width: 60, Height: 40}, false},
		{"large window shows the logo in the header", tea.WindowSizeMsg{Width: 120, Height: 40}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			out := Frame(tt.size, "status", "body", HelpView(tt.size.Width, key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "do"))))
			r.Equal(tt.size.Height, lipgloss.Height(out), "header, separator, body and footer fill the window")
			r.Equal(tt.wantLogo, strings.Contains(out, logo[3]))
			r.Contains(out, "status")
			r.Contains(out, "x do")
			r.Contains(out, ": commands", "every footer offers the command palette")

			full := strings.TrimSuffix(strings.Repeat("x\n", Body(tt.size).Height), "\n")
			r.Equal(tt.size.Height, lipgloss.Height(Frame(tt.size, "", full, "")), "a full body must not push the footer out")
		})
	}
}

func TestWindow(t *testing.T) {
	tests := []struct {
		n, cursor, height, start, end int
	}{
		{n: 10, cursor: 0, height: 4, start: 0, end: 4},
		{n: 10, cursor: 3, height: 4, start: 0, end: 4},
		{n: 10, cursor: 4, height: 4, start: 1, end: 5},
		{n: 10, cursor: 9, height: 4, start: 6, end: 10},
		{n: 2, cursor: 1, height: 4, start: 0, end: 2},
	}
	for _, tt := range tests {
		r := require.New(t)
		start, end := Window(tt.n, tt.cursor, tt.height)
		r.Equal([2]int{tt.start, tt.end}, [2]int{start, end}, "n=%d cursor=%d height=%d", tt.n, tt.cursor, tt.height)
	}
}

func TestLogo(t *testing.T) {
	r := require.New(t)
	r.Len(logo, LogoHeight)
	for _, line := range logo {
		r.Len(line, len(logo[0]), "all logo lines have the same width so it aligns as a block")
	}
	r.Equal(LogoHeight, lipgloss.Height(Logo()))
}
