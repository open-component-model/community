// Package theme provides centralized styling for the TUI.
// All components reference theme.Current() instead of hardcoding colors.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines all colors and pre-built styles for the TUI.
type Theme struct {
	// Base colors
	Primary lipgloss.AdaptiveColor
	Subtle  lipgloss.AdaptiveColor
	Text    lipgloss.AdaptiveColor
	Error   lipgloss.AdaptiveColor
	Success lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Running lipgloss.AdaptiveColor

	// Pre-built text styles
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Help        lipgloss.Style
	ErrorText   lipgloss.Style
	SuccessText lipgloss.Style
	RunningText lipgloss.Style
	DimText     lipgloss.Style

	// Interactive styles
	Cursor    lipgloss.Style // focused cursor line (bold + reverse)
	CursorDim lipgloss.Style // unfocused cursor line (bold only)
	Selected  lipgloss.Style // selected/highlighted item

	// Layout styles
	ModalBorder lipgloss.Style
	Divider     lipgloss.Style // vertical pane divider character style

	// Status bar styles
	StatusTitle lipgloss.Style
	StatusHint  lipgloss.Style
	StatusRef   lipgloss.Style
}

// Separator renders a horizontal separator line of the given width.
func (t Theme) Separator(width int) string {
	return t.DimText.Render(repeat("─", width))
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}

// current holds the active theme. Changed via SetTheme.
var current = Default()

// Current returns the active theme.
func Current() *Theme {
	return current
}

// SetTheme replaces the active theme.
func SetTheme(t *Theme) {
	current = t
}
