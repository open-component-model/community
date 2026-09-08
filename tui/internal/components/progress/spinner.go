// Package progress provides progress indicator components.
package progress

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Frames are the spinner animation characters.
var Frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// TickMsg drives the spinner animation.
type TickMsg struct{}

// Tick returns a command that fires a TickMsg after 100ms.
func Tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return TickMsg{}
	})
}

// Frame returns the spinner character for the given frame index.
func Frame(index int) string {
	return Frames[index%len(Frames)]
}
