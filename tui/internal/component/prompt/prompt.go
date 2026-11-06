// Package prompt is a centered, titled text input component.
package prompt

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"ext.ocm.software/tui/internal/ui"
)

// SubmitMsg reports a non-empty value entered in the prompt with ID.
type SubmitMsg struct {
	ID    string
	Value string
}

// Model is a text input with a title, a subtitle and a status or error line.
type Model struct {
	ID       string
	Title    string
	Subtitle string
	Status   string
	Err      error
	input    textinput.Model
	width    int
	height   int
}

// New creates a focused prompt.
func New(id, title, subtitle, placeholder string) Model {
	in := textinput.New()
	in.Placeholder = placeholder
	in.CharLimit = 512
	in.Width = 80
	in.Focus()
	return Model{ID: id, Title: title, Subtitle: subtitle, input: in}
}

// Value returns the trimmed input.
func (m Model) Value() string { return strings.TrimSpace(m.input.Value()) }

// SetValue replaces the input.
func (m Model) SetValue(v string) Model {
	m.input.SetValue(v)
	return m
}

// Focus enables typing; an unfocused prompt ignores keys.
func (m Model) Focus() (Model, tea.Cmd) {
	return m, m.input.Focus()
}

// Blur disables typing.
func (m Model) Blur() Model {
	m.input.Blur()
	return m
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = min(80, max(msg.Width-4, 10))
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			if !m.input.Focused() || m.Value() == "" {
				return m, nil
			}
			submit := SubmitMsg{ID: m.ID, Value: m.Value()}
			return m, func() tea.Msg { return submit }
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return ui.Center(m.width, m.height,
		ui.Title.MarginBottom(1).Render(m.Title), ui.Dim.MarginBottom(1).Render(m.Subtitle),
		m.input.View(), ui.Dim.Render(m.Status), ui.Errorf(m.Err))
}
