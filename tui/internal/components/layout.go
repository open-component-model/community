package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/theme"
)

// Hotkey describes a key binding to show in the footer.
type Hotkey struct {
	Key  key.Binding
	Show bool
}

// NewHotkey creates a visible hotkey.
func NewHotkey(k key.Binding) Hotkey {
	return Hotkey{Key: k, Show: true}
}

// Layout renders a standard view layout with title bar, content, and footer.
type Layout struct {
	Title      string
	StatusInfo string
	Content    string
	Hotkeys    []Hotkey
	Width      int
	Height     int
}

// ContentHeight returns the available height for the content area,
// accounting for the title bar (1 line), separator (1 line), and footer (1 line).
func (l Layout) ContentHeight() int {
	h := l.Height - 3
	if h < 1 {
		h = 1
	}
	return h
}

// Render produces the full layout string.
func (l Layout) Render() string {
	t := theme.Current()

	titleBar := StatusBar(l.Width, l.Title, l.StatusInfo)
	separator := t.Separator(l.Width)

	contentStyle := lipgloss.NewStyle().
		Width(l.Width).
		Height(l.ContentHeight())
	content := contentStyle.Render(l.Content)

	footer := l.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, separator, content, footer)
}

func (l Layout) renderFooter() string {
	t := theme.Current()

	var parts []string
	for _, hk := range l.Hotkeys {
		if !hk.Show {
			continue
		}
		help := hk.Key.Help()
		if help.Key == "" {
			continue
		}
		parts = append(parts, help.Key+": "+help.Desc)
	}
	parts = append(parts, ":: command")

	return t.Help.Render(strings.Join(parts, "  "))
}
