package explorer

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ext.ocm.software/tui/internal/components"
	"ext.ocm.software/tui/internal/components/progress"
	"ext.ocm.software/tui/fetch"
)

// --- Messages ---

type versionsMsg struct {
	component string
	versions  []string
}

type descriptorMsg struct {
	component  string
	version    string
	descriptor *descriptor.Descriptor
}

// TransferRequestMsg is emitted when the user requests a transfer from the explorer.
type TransferRequestMsg struct {
	Reference string // full reference including repo (e.g. "ghcr.io/org/repo//comp:ver")
	Component string
	Version   string
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// --- Mode ---

type explorerMode int

const (
	modePrompt explorerMode = iota
	modeBrowse
)

// Config holds the dependencies for the explorer view.
type Config struct {
	FetcherFactory fetch.FetcherFactory
	Downloader     fetch.ResourceDownloader
}

// Model is the explorer view state.
type Model struct {
	config Config
	mode   explorerMode

	// Prompt
	input    textinput.Model
	inputErr error

	// Browse
	fetcher    fetch.ComponentFetcher
	downloader fetch.ResourceDownloader
	keys       KeyMap
	reference  string

	// Tree
	roots   []*Node
	cursor  int
	visible []*Node

	// Detail pane
	detail viewport.Model

	// Layout
	width     int
	height    int
	treeWidth int
	focusTree bool
	ready     bool

	// Status
	err     error
	loading bool

	// Download
	downloading     bool
	downloadStatus  string
	downloadResName string
	spinnerFrame    int

	// Initial load
	initialComponent string
	initialVersion   string
}

// NewView creates a new explorer view starting at the prompt.
func NewView(cfg Config, width, height int) *Model {
	ti := textinput.New()
	ti.Placeholder = "ghcr.io/open-component-model/ocm//ocm.software/ocmcli:0.23.0"
	ti.CharLimit = 512
	ti.Width = 80
	if width > 4 && ti.Width > width-4 {
		ti.Width = width - 4
	}
	ti.Focus()

	m := &Model{
		config:    cfg,
		mode:      modePrompt,
		input:     ti,
		keys:      DefaultKeyMap(),
		focusTree: true,
		width:     width,
		height:    height,
	}
	if cfg.Downloader != nil {
		m.downloader = cfg.Downloader
	}
	return m
}

// --- View interface implementation ---

func (m *Model) Init() tea.Cmd {
	if m.mode == modePrompt {
		return textinput.Blink
	}
	if m.initialComponent == "" {
		return nil
	}
	if m.initialVersion != "" {
		return m.fetchDescriptor(m.initialComponent, m.initialVersion)
	}
	return m.fetchVersions(m.initialComponent)
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch m.mode {
	case modePrompt:
		return m.updatePrompt(msg)
	case modeBrowse:
		return m.updateBrowse(msg)
	}
	return nil
}

func (m *Model) Render() string {
	switch m.mode {
	case modePrompt:
		return m.renderPrompt()
	case modeBrowse:
		return m.renderBrowse()
	}
	return ""
}

func (m *Model) StatusInfo() string {
	if m.mode == modePrompt {
		return "enter component reference"
	}
	if m.downloading {
		return fmt.Sprintf("%s downloading...", progress.Frame(m.spinnerFrame))
	}
	return m.reference
}

func (m *Model) Hotkeys() []components.Hotkey {
	if m.mode == modePrompt {
		return []components.Hotkey{
			components.NewHotkey(key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "connect"))),
			components.NewHotkey(key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))),
		}
	}
	if m.downloadStatus != "" {
		if m.downloading {
			return nil
		}
		return []components.Hotkey{
			components.NewHotkey(key.NewBinding(key.WithKeys("any"), key.WithHelp("any key", "dismiss"))),
		}
	}
	hotkeys := []components.Hotkey{
		components.NewHotkey(m.keys.Up),
		components.NewHotkey(m.keys.Expand),
		components.NewHotkey(m.keys.Collapse),
		components.NewHotkey(key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane"))),
	}
	if m.downloader != nil {
		hotkeys = append(hotkeys, components.NewHotkey(m.keys.Download))
	}
	hotkeys = append(hotkeys, components.NewHotkey(m.keys.Transfer))
	return hotkeys
}

func (m *Model) OnBackPressed() int {
	if m.mode == modePrompt {
		return 1 // BackExit
	}
	if m.focusTree && len(m.visible) > 0 && m.cursor < len(m.visible) {
		node := m.visible[m.cursor]
		if node.IsExpandable() && node.Expanded {
			node.Expanded = false
			m.visible = Flatten(m.roots)
			m.updateDetail()
			return 0 // BackConsumed
		}
	}
	return 1 // BackExit
}

func (m *Model) Resize(width, height int) {
	m.width = width
	m.height = height
	m.treeWidth = width / 2
	if m.input.Width > width-4 {
		m.input.Width = width - 4
	}
	detailWidth := width - m.treeWidth - 1
	contentHeight := height - 3
	if !m.ready {
		m.detail = viewport.New(detailWidth, contentHeight)
		m.ready = true
	} else {
		m.detail.Width = detailWidth
		m.detail.Height = contentHeight
	}
}

// ToggleFocus switches focus between tree and detail panes.
func (m *Model) ToggleFocus() {
	m.focusTree = !m.focusTree
}

func (m *Model) updateDetail() {
	if len(m.visible) == 0 || m.cursor >= len(m.visible) {
		m.detail.SetContent("No items.")
		return
	}
	node := m.visible[m.cursor]
	m.detail.SetContent(NodeDetail(node))
	m.detail.GotoTop()
}

// --- Fetch commands ---

func (m *Model) fetchVersions(component string) tea.Cmd {
	fetcher := m.fetcher
	return func() tea.Msg {
		versions, err := fetcher.ListVersions(context.Background(), component)
		if err != nil {
			return errMsg{fmt.Errorf("listing versions for %s: %w", component, err)}
		}
		return versionsMsg{component: component, versions: versions}
	}
}

func (m *Model) fetchDescriptor(component, version string) tea.Cmd {
	fetcher := m.fetcher
	return func() tea.Msg {
		desc, err := fetcher.GetDescriptor(context.Background(), component, version)
		if err != nil {
			return errMsg{fmt.Errorf("fetching %s:%s: %w", component, version, err)}
		}
		return descriptorMsg{component: component, version: version, descriptor: desc}
	}
}
