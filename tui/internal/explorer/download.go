package explorer

import (
	"context"
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ext.ocm.software/tui/internal/components/progress"
	"ext.ocm.software/tui/internal/theme"
)

// downloadDoneMsg is sent when a download completes.
type downloadDoneMsg struct {
	resourceName string
	outputPath   string
}

// startDownload initiates a resource download if a resource node is selected.
func (m *Model) startDownload() tea.Cmd {
	if !m.focusTree || m.downloader == nil || m.downloading || len(m.visible) == 0 {
		return nil
	}
	node := m.visible[m.cursor]
	if node.Kind != NodeResource || node.Resource == nil {
		return nil
	}
	component, version := m.findResourceContext(node)
	if component == "" || version == "" {
		return nil
	}

	m.downloading = true
	m.downloadResName = node.Resource.Name
	m.spinnerFrame = 0
	m.downloadStatus = fmt.Sprintf("%s Downloading %s...", progress.Frame(0), node.Resource.Name)

	return tea.Batch(
		m.doDownload(component, version, node.Resource),
		progress.Tick(),
	)
}

func (m *Model) doDownload(component, version string, res *descriptor.Resource) tea.Cmd {
	downloader := m.downloader
	ref := m.reference
	return func() tea.Msg {
		outputPath, err := downloader.DownloadResource(context.Background(), ref, component, version, res, ".")
		if err != nil {
			return errMsg{fmt.Errorf("downloading resource %s: %w", res.Name, err)}
		}
		if abs, err := filepath.Abs(outputPath); err == nil {
			outputPath = abs
		}
		return downloadDoneMsg{resourceName: res.Name, outputPath: outputPath}
	}
}

// renderDownloadModal renders the download progress/result as a centered modal.
func (m *Model) renderDownloadModal() string {
	t := theme.Current()

	border := t.ModalBorder.Padding(1, 2).Width(60)

	var sections []string

	if m.downloading {
		sections = append(sections, t.Title.MarginBottom(1).Render("Download in Progress"))
		sections = append(sections, m.downloadStatus)
	} else if m.err != nil {
		sections = append(sections, t.Title.MarginBottom(1).Render("Download Failed"))
		sections = append(sections, t.ErrorText.Render(m.downloadStatus))
		sections = append(sections, "")
		sections = append(sections, t.DimText.Render("press any key to dismiss"))
	} else {
		sections = append(sections, t.SuccessText.Render("Download Complete"))
		sections = append(sections, "")
		sections = append(sections, m.downloadStatus)
		sections = append(sections, "")
		sections = append(sections, t.DimText.Render("press any key to dismiss"))
	}

	popup := border.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}
