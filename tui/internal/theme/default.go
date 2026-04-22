package theme

import "github.com/charmbracelet/lipgloss"

// Default returns the default OCM TUI theme.
func Default() *Theme {
	primary := lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	subtle := lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}
	text := lipgloss.AdaptiveColor{Light: "#333333", Dark: "#DDDDDD"}
	errColor := lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#FF4444"}
	success := lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#44FF44"}
	warning := lipgloss.AdaptiveColor{Light: "#CCAA00", Dark: "#FFCC44"}
	running := lipgloss.AdaptiveColor{Light: "#0077CC", Dark: "#55AAFF"}

	return &Theme{
		// Colors
		Primary: primary,
		Subtle:  subtle,
		Text:    text,
		Error:   errColor,
		Success: success,
		Warning: warning,
		Running: running,

		// Text styles
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(primary),

		Subtitle: lipgloss.NewStyle().
			Foreground(subtle),

		Help: lipgloss.NewStyle().
			Foreground(subtle),

		ErrorText: lipgloss.NewStyle().
			Foreground(errColor),

		SuccessText: lipgloss.NewStyle().
			Foreground(success),

		RunningText: lipgloss.NewStyle().
			Foreground(running),

		DimText: lipgloss.NewStyle().
			Foreground(subtle),

		// Interactive
		Cursor: lipgloss.NewStyle().
			Bold(true).
			Reverse(true),

		CursorDim: lipgloss.NewStyle().
			Bold(true),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(primary),

		// Layout
		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary).
			Padding(0, 1),

		Divider: lipgloss.NewStyle().
			Foreground(subtle),

		// Status bar
		StatusTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(primary).
			PaddingRight(1),

		StatusHint: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#BBBBBB", Dark: "#555555"}).
			PaddingRight(1),

		StatusRef: lipgloss.NewStyle().
			Foreground(subtle),
	}
}
