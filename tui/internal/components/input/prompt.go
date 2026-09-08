// Package input provides reusable text input components.
package input

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/theme"
)

// Prompt is a centered text input with title, subtitle, error display, and help text.
type Prompt struct {
	Title      string
	Subtitle   string
	Input      textinput.Model
	Err        error
	Loading    bool
	LoadingMsg string
	Help       string
	Width      int
	Height     int
}

// New creates a new prompt with sensible defaults.
func New(placeholder string, width int) Prompt {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 512
	ti.Width = 80
	if width > 4 && ti.Width > width-4 {
		ti.Width = width - 4
	}
	ti.Focus()

	return Prompt{
		Input: ti,
		Width: width,
	}
}

// Update forwards messages to the text input.
func (p *Prompt) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.Input, cmd = p.Input.Update(msg)
	return cmd
}

// Value returns the trimmed input value.
func (p Prompt) Value() string {
	return p.Input.Value()
}

// Focus activates the input for typing.
func (p *Prompt) Focus() tea.Cmd {
	p.Input.Focus()
	return textinput.Blink
}

// Blur deactivates the input.
func (p *Prompt) Blur() {
	p.Input.Blur()
}

// Resize updates the prompt dimensions.
func (p *Prompt) Resize(width, height int) {
	p.Width = width
	p.Height = height
	if p.Input.Width > width-4 {
		p.Input.Width = width - 4
	}
}

// View renders the prompt.
func (p Prompt) View() string {
	t := theme.Current()

	var sections []string
	if p.Title != "" {
		sections = append(sections, t.Title.MarginBottom(1).Render(p.Title))
	}
	if p.Subtitle != "" {
		sections = append(sections, t.Subtitle.MarginBottom(1).Render(p.Subtitle))
	}
	sections = append(sections, p.Input.View())

	if p.Loading {
		msg := "Loading..."
		if p.LoadingMsg != "" {
			msg = p.LoadingMsg
		}
		sections = append(sections, t.Subtitle.Render(msg))
	}

	if p.Err != nil {
		sections = append(sections, t.ErrorText.MarginTop(1).Render(fmt.Sprintf("Error: %v", p.Err)))
	}

	if p.Help != "" {
		sections = append(sections, t.Help.MarginTop(1).Render(p.Help))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return lipgloss.Place(p.Width, p.Height, lipgloss.Center, lipgloss.Center, content)
}
