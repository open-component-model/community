package components

import (
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/theme"
)

// StatusBar renders a top status bar with title, global hint, and reference info.
func StatusBar(width int, title, reference string) string {
	t := theme.Current()

	left := t.StatusTitle.Render(title)
	hint := t.StatusHint.Render(":: command palette")
	right := t.StatusRef.Render(reference)

	gap := width - lipgloss.Width(left) - lipgloss.Width(hint) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	bar := lipgloss.NewStyle().
		Width(width).
		Render(left + hint + lipgloss.NewStyle().Width(gap).Render("") + right)

	return bar
}
