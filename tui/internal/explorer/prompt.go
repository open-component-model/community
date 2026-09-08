package explorer

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"ext.ocm.software/tui/internal/components/input"
	"ext.ocm.software/tui/fetch"
)

// fetcherReadyMsg is sent when the FetcherFactory succeeds.
type fetcherReadyMsg struct {
	fetcher   fetch.ComponentFetcher
	component string
	version   string
	reference string
}

// fetcherErrMsg is sent when the FetcherFactory fails.
type fetcherErrMsg struct{ err error }

// updatePrompt handles messages in prompt mode.
func (m *Model) updatePrompt(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEnter:
			ref := strings.TrimSpace(m.input.Value())
			if ref == "" {
				return nil
			}
			factory := m.config.FetcherFactory
			if factory == nil {
				m.inputErr = fmt.Errorf("no repository connection configured")
				return nil
			}
			m.loading = true
			m.inputErr = nil
			m.input.Blur()
			return func() tea.Msg {
				fetcher, component, version, err := factory(context.Background(), ref)
				if err != nil {
					return fetcherErrMsg{fmt.Errorf("connecting to %s: %w", ref, err)}
				}
				return fetcherReadyMsg{fetcher: fetcher, component: component, version: version, reference: ref}
			}
		case tea.KeyEsc:
			return nil // OnBackPressed handles this
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.input.Width > msg.Width-4 {
			m.input.Width = msg.Width - 4
		}
		return nil

	case fetcherReadyMsg:
		m.loading = false
		m.reference = msg.reference
		m.fetcher = msg.fetcher
		m.initialComponent = msg.component
		m.initialVersion = msg.version
		m.mode = modeBrowse
		m.Resize(m.width, m.height)
		var initCmd tea.Cmd
		if msg.component != "" {
			if msg.version != "" {
				initCmd = m.fetchDescriptor(msg.component, msg.version)
			} else {
				initCmd = m.fetchVersions(msg.component)
			}
		}
		return initCmd

	case fetcherErrMsg:
		m.loading = false
		m.inputErr = msg.err
		m.input.Focus()
		return textinput.Blink
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// renderPrompt renders the reference input screen using the shared prompt component.
func (m *Model) renderPrompt() string {
	p := input.Prompt{
		Title:      "Explore Components",
		Subtitle:   "Enter a component reference:",
		Input:      m.input,
		Err:        m.inputErr,
		Loading:    m.loading,
		LoadingMsg: "Connecting...",
		Help:       "enter: connect  esc: back",
		Width:      m.width,
		Height:     m.height,
	}
	return p.View()
}
