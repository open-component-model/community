package transfer

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"ext.ocm.software/tui/fetch"
)

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch m.step {
	case stepSource:
		return m.handleSourceKey(msg)
	case stepTarget:
		return m.handleTargetKey(msg)
	case stepOptions:
		return m.handleOptionsKey(msg)
	case stepReview:
		return m.handleReviewKey(msg)
	case stepDone:
		return nil
	}
	return nil
}

func (m *Model) handleSourceKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEnter:
		if strings.TrimSpace(m.sourceInput.Value()) == "" {
			return nil
		}
		m.step = stepTarget
		m.sourceInput.Blur()
		m.targetInput.Focus()
		return textinput.Blink
	}
	var cmd tea.Cmd
	m.sourceInput, cmd = m.sourceInput.Update(msg)
	return cmd
}

func (m *Model) handleTargetKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEnter:
		if strings.TrimSpace(m.targetInput.Value()) == "" {
			return nil
		}
		m.step = stepOptions
		m.targetInput.Blur()
		return nil
	}
	var cmd tea.Cmd
	m.targetInput, cmd = m.targetInput.Update(msg)
	return cmd
}

func (m *Model) handleOptionsKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.optionCursor > 0 {
			m.optionCursor--
		}
	case key.Matches(msg, m.keys.Down):
		if m.optionCursor < 3 {
			m.optionCursor++
		}
	case msg.Type == tea.KeySpace:
		m.toggleOption()
	case msg.Type == tea.KeyEnter:
		if m.optionCursor == 3 {
			m.err = nil
			return m.buildGraph()
		}
		m.toggleOption()
	}
	return nil
}

func (m *Model) toggleOption() {
	switch m.optionCursor {
	case 0:
		m.recursive = !m.recursive
	case 1:
		m.copyResources = !m.copyResources
	case 2:
		m.uploadAs = (m.uploadAs + 1) % len(uploadAsLabels)
	}
}

func (m *Model) handleReviewKey(msg tea.KeyMsg) tea.Cmd {
	if key.Matches(msg, m.keys.Submit) {
		if m.executor == nil {
			m.err = fmt.Errorf("no transfer executor configured")
			return nil
		}
		m.step = stepExecuting
		progressCh := make(chan fetch.TransferProgress, 16)
		doneCh := make(chan error, 1)
		m.progressCh = progressCh
		m.doneCh = doneCh
		tgd := m.tgd
		executor := m.executor
		go func() {
			doneCh <- executor.Execute(context.Background(), tgd, progressCh)
		}()
		return waitForProgress(progressCh, doneCh)
	}
	var cmd tea.Cmd
	m.reviewView, cmd = m.reviewView.Update(msg)
	return cmd
}

func (m *Model) buildGraph() tea.Cmd {
	if m.executor == nil {
		m.err = fmt.Errorf("no transfer executor configured")
		return nil
	}
	source := strings.TrimSpace(m.sourceInput.Value())
	target := strings.TrimSpace(m.targetInput.Value())
	opts := fetch.TransferOptions{
		Recursive:     m.recursive,
		CopyResources: m.copyResources,
		UploadAs:      uploadAsLabels[m.uploadAs],
	}
	executor := m.executor
	return func() tea.Msg {
		tgd, err := executor.BuildGraph(context.Background(), source, target, opts)
		if err != nil {
			return graphErrMsg{err}
		}
		return graphBuiltMsg{tgd: tgd}
	}
}
