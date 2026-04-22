// Package splitpane provides a two-pane horizontal layout component.
package splitpane

import (
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/theme"
)

// SplitPane renders a horizontal split layout with a divider.
type SplitPane struct {
	Left   string
	Right  string
	Ratio  float64 // left pane ratio (0.5 = 50/50)
	Width  int
	Height int
}

// New creates a split pane with 50/50 ratio.
func New(width, height int) SplitPane {
	return SplitPane{Ratio: 0.5, Width: width, Height: height}
}

// LeftWidth returns the left pane width.
func (s SplitPane) LeftWidth() int {
	return int(float64(s.Width) * s.Ratio)
}

// RightWidth returns the right pane width (minus divider).
func (s SplitPane) RightWidth() int {
	return s.Width - s.LeftWidth() - 1 // 1 for divider
}

// Render produces the split layout.
func (s SplitPane) Render() string {
	t := theme.Current()

	leftView := lipgloss.NewStyle().
		Width(s.LeftWidth()).
		Height(s.Height).
		Render(s.Left)

	rightView := lipgloss.NewStyle().
		Width(s.RightWidth()).
		Height(s.Height).
		PaddingLeft(1).
		Render(s.Right)

	divider := t.Divider.Render("│")

	return lipgloss.JoinHorizontal(lipgloss.Top, leftView, divider, rightView)
}
