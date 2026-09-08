package transfer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/components/input"
	"ext.ocm.software/tui/internal/theme"
)

func (m *Model) render() string {
	switch m.step {
	case stepSource:
		return m.renderInput("Step 1/4: Source Component", "Enter the source component reference:", m.sourceInput)
	case stepTarget:
		return m.renderInput("Step 2/4: Target Repository", "Enter the target repository:", m.targetInput)
	case stepOptions:
		return m.renderOptions()
	case stepReview:
		return m.renderReview()
	case stepExecuting:
		return m.renderExecuting()
	case stepDone:
		return m.renderDone()
	}
	return ""
}

func (m *Model) renderInput(title, subtitle string, ti textinput.Model) string {
	p := input.Prompt{
		Title:    title,
		Subtitle: subtitle,
		Input:    ti,
		Err:      m.err,
		Help:     "enter: next  esc: back",
		Width:    m.width,
		Height:   m.height,
	}
	return p.View()
}

func (m *Model) renderOptions() string {
	t := theme.Current()

	var sections []string
	sections = append(sections, t.Title.MarginBottom(1).Render("Step 3/4: Transfer Options"))
	sections = append(sections, "")

	type row struct {
		label   string
		value   string
		toggle  bool
		checked bool
		action  bool
	}
	options := []row{
		{"Recursive", "", true, m.recursive, false},
		{"Copy all resources", "", true, m.copyResources, false},
		{"Upload as", uploadAsLabels[m.uploadAs], false, false, false},
		{"Build graph >>>", "", false, false, true},
	}

	for i, opt := range options {
		cursor := "  "
		if i == m.optionCursor {
			cursor = "> "
		}

		var line string
		switch {
		case opt.action:
			line = fmt.Sprintf("%s%s", cursor, opt.label)
		case opt.toggle:
			check := "[ ]"
			if opt.checked {
				check = "[x]"
			}
			line = fmt.Sprintf("%s%s %s", cursor, check, opt.label)
		default:
			line = fmt.Sprintf("%s    %s: %s", cursor, opt.label, opt.value)
		}

		if i == m.optionCursor {
			line = t.Selected.Render(line)
		}
		sections = append(sections, line)
	}

	sections = append(sections, "")
	if m.err != nil {
		sections = append(sections, t.ErrorText.MarginTop(1).Render(fmt.Sprintf("Error: %v", m.err)))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *Model) renderReview() string {
	t := theme.Current()

	var sections []string
	sections = append(sections, t.Title.MarginBottom(1).Render("Step 4/4: Review Transformation Graph"))

	count := 0
	if m.tgd != nil {
		count = len(m.tgd.Transformations)
	}
	sections = append(sections, fmt.Sprintf("%d transformations will be executed:", count))
	sections = append(sections, "")
	sections = append(sections, m.reviewView.View())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) renderExecuting() string {
	t := theme.Current()

	var sections []string

	progressText := ""
	if m.progressTotal > 0 {
		progressText = fmt.Sprintf(" (%d/%d)", m.progressCurrent, m.progressTotal)
	}
	sections = append(sections, t.Title.Render("Transferring..."+progressText))
	sections = append(sections, "")

	maxVisible := m.height - 5
	if maxVisible < 1 {
		maxVisible = 1
	}
	start := 0
	if len(m.progressLog) > maxVisible {
		start = len(m.progressLog) - maxVisible
	}
	for _, entry := range m.progressLog[start:] {
		sections = append(sections, "  "+styleLogEntry(t, entry))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) renderDone() string {
	t := theme.Current()

	var sections []string

	if m.execErr != nil {
		sections = append(sections, t.ErrorText.Bold(true).Render("Transfer failed"))
		sections = append(sections, fmt.Sprintf("\n%v", m.execErr))
	} else {
		sections = append(sections, t.SuccessText.Bold(true).Render("Transfer completed successfully"))
	}
	sections = append(sections, "")

	maxVisible := m.height - 6
	if maxVisible < 1 {
		maxVisible = 1
	}
	start := 0
	if len(m.progressLog) > maxVisible {
		start = len(m.progressLog) - maxVisible
	}
	for _, entry := range m.progressLog[start:] {
		sections = append(sections, "  "+styleLogEntry(t, entry))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func styleLogEntry(t *theme.Theme, entry string) string {
	switch {
	case strings.Contains(entry, "completed"):
		return t.SuccessText.Render(entry)
	case strings.Contains(entry, "failed"):
		return t.ErrorText.Render(entry)
	case strings.Contains(entry, "running"):
		return t.RunningText.Render(entry)
	default:
		return t.DimText.Render(entry)
	}
}
