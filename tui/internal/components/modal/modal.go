// Package modal provides a centered modal dialog component.
package modal

import (
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/theme"
)

// Modal renders a centered bordered popup.
type Modal struct {
	Title   string
	Content string
	Footer  string
	Width   int // inner width, 0 = auto (50)
}

// Render places the modal centered in the given dimensions.
func (m Modal) Render(width, height int) string {
	t := theme.Current()

	innerWidth := m.Width
	if innerWidth == 0 {
		innerWidth = 50
	}

	border := t.ModalBorder.Width(innerWidth)

	var sections []string
	if m.Title != "" {
		sections = append(sections, t.Title.MarginBottom(1).Render(m.Title))
	}
	if m.Content != "" {
		sections = append(sections, m.Content)
	}
	if m.Footer != "" {
		sections = append(sections, "")
		sections = append(sections, t.DimText.Render(m.Footer))
	}

	popup := border.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
}
