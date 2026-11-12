// Package explore is the component version browser page.
package explore

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"

	"ext.ocm.software/tui/internal/component/prompt"
	"ext.ocm.software/tui/internal/component/tree"
	"ext.ocm.software/tui/internal/ui"
	"ext.ocm.software/tui/internal/view/transfer"
)

// Repository is the part of an OCM repository the explorer reads from.
type Repository interface {
	ListComponentVersions(ctx context.Context, component string) ([]string, error)
	GetComponentVersion(ctx context.Context, component, version string) (*descriptor.Descriptor, error)
	DownloadResource(ctx context.Context, component, version string, res *descriptor.Resource, dir string) (string, error)
}

// OpenFunc connects to the repository of a reference and returns the component and version it names.
type OpenFunc func(ctx context.Context, reference string) (repo Repository, component, version string, err error)

var keys = struct{ Focus, Download, Transfer, Back, Connect, Dismiss key.Binding }{
	Focus:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "details")),
	Download: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "download")),
	Transfer: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "transfer")),
	Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Connect:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "connect")),
	Dismiss:  key.NewBinding(key.WithKeys("any"), key.WithHelp("any key", "dismiss")),
}

// Name identifies the explorer view in ui.OpenMsg and on the command line.
const Name = "explore"

type (
	openedMsg struct {
		repo               Repository
		component, version string
	}
	openFailedMsg struct{ err error }
	versionsMsg   struct {
		component string
		versions  []string
	}
	loadedMsg struct {
		node tree.Node
		desc *descriptor.Descriptor
	}
	loadFailedMsg struct {
		id  string
		err error
	}
	downloadedMsg struct {
		path string
		err  error
	}
)

// Model is the explorer page: a reference prompt, then a tree with a detail pane.
type Model struct {
	open      OpenFunc
	prompt    prompt.Model
	reference string
	repo      Repository

	tree     tree.Model
	treeSize tea.WindowSizeMsg
	detail   viewport.Model
	shown    string

	spinner     spinner.Model
	downloading bool
	download    string

	size tea.WindowSizeMsg
	err  error
}

// New creates an explorer at the reference prompt; a non-empty reference connects right away.
func New(open OpenFunc, reference string) Model {
	m := Model{
		open:    open,
		prompt:  prompt.New("reference", "Explore Components", "Enter a component reference:", "ghcr.io/open-component-model/ocm//ocm.software/ocmcli:0.23.0"),
		detail:  viewport.New(0, 0),
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(ui.Title)),
	}
	m.prompt = m.prompt.SetValue(reference)
	return m
}

func (m Model) Init() tea.Cmd {
	if ref := m.prompt.Value(); ref != "" {
		return func() tea.Msg { return prompt.SubmitMsg{ID: "reference", Value: ref} }
	}
	return m.prompt.Init()
}

// Editing reports whether keys go to the reference prompt.
func (m Model) Editing() bool { return m.repo == nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.size = msg
		body := ui.Body(msg)
		m.prompt, _ = m.prompt.Update(body)
		m.treeSize = tea.WindowSizeMsg{Width: body.Width / 2, Height: body.Height}
		m.tree, _ = m.tree.Update(m.treeSize)
		m.detail.Width, m.detail.Height = body.Width-body.Width/2-2, body.Height
		return m, nil

	case prompt.SubmitMsg:
		m.reference = msg.Value
		m.prompt.Status, m.prompt.Err = "Connecting...", nil
		m.prompt = m.prompt.Blur()
		open, ref := m.open, msg.Value
		return m, func() tea.Msg {
			repo, component, version, err := open(context.Background(), ref)
			if err != nil {
				return openFailedMsg{err}
			}
			return openedMsg{repo, component, version}
		}

	case openFailedMsg:
		m.prompt.Status, m.prompt.Err = "", msg.err
		m.prompt, cmd = m.prompt.Focus()
		return m, cmd

	case openedMsg:
		m.repo = msg.repo
		if msg.version == "" {
			m = m.setTree(componentNode(msg.component))
			repo, component := msg.repo, msg.component
			return m, func() tea.Msg {
				versions, err := repo.ListComponentVersions(context.Background(), component)
				if err != nil {
					return loadFailedMsg{component, fmt.Errorf("listing versions of %s: %w", component, err)}
				}
				return versionsMsg{component, versions}
			}
		}
		root := componentNode(msg.component, msg.version)
		version := root.Children[0]
		version.Expanded, version.Loading = true, true
		root.Children[0] = version
		return m.setTree(root), m.load(version)

	case versionsMsg:
		return m.setTree(componentNode(msg.component, msg.versions...)), nil

	case tree.ExpandMsg:
		m.err = nil
		return m, m.load(msg.Node)

	case loadedMsg:
		it := data(msg.node)
		if !it.reference {
			it.detail = versionDetail(msg.desc)
		}
		m.tree = m.tree.SetChildren(msg.node.ID, descriptorChildren(msg.node, msg.desc)).
			Modify(msg.node.ID, func(n tree.Node) tree.Node { n.Data = it; return n })
		return m.syncDetail(), nil

	case loadFailedMsg:
		m.tree, m.err = m.tree.LoadFailed(msg.id), msg.err
		return m.syncDetail(), nil

	case downloadedMsg:
		m.downloading = false
		m.download = ui.Errorf(msg.err)
		if msg.err == nil {
			m.download = ui.Success.Render("Downloaded to:") + "\n" + msg.path
		}
		return m, nil

	case spinner.TickMsg:
		if m.downloading {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.repo == nil {
		m.prompt, cmd = m.prompt.Update(msg)
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case m.repo == nil && msg.Type == tea.KeyEsc:
		return m, ui.Exit
	case m.repo == nil:
		m.prompt, cmd = m.prompt.Update(msg)
		return m, cmd
	case m.downloading:
		return m, nil
	case m.download != "":
		m.download = ""
		return m, nil
	}

	n, _ := m.tree.Selected()
	it := data(n)
	switch {
	case key.Matches(msg, keys.Back):
		if !m.tree.Focused {
			m.tree.Focused = true
			return m, nil
		}
		var collapsed bool
		if m.tree, collapsed = m.tree.Collapse(); !collapsed {
			return m, ui.Exit
		}
	case key.Matches(msg, keys.Focus):
		m.tree.Focused = !m.tree.Focused
		return m, nil
	case key.Matches(msg, keys.Download):
		if it.resource != nil {
			return m.startDownload(it)
		}
	case key.Matches(msg, keys.Transfer):
		if it.version != "" {
			repo, _, _ := strings.Cut(m.reference, "//")
			source := fmt.Sprintf("%s//%s:%s", repo, it.component, it.version)
			return m, func() tea.Msg { return ui.OpenMsg{View: transfer.Name, Arg: source} }
		}
	default:
		if m.tree.Focused {
			m.tree, cmd = m.tree.Update(msg)
		} else {
			m.detail, cmd = m.detail.Update(msg)
		}
	}
	return m.syncDetail(), cmd
}

func (m Model) setTree(root tree.Node) Model {
	m.tree, _ = tree.New(root).Update(m.treeSize)
	return m.syncDetail()
}

// syncDetail shows the selected node in the detail pane when the selection changed.
func (m Model) syncDetail() Model {
	n, ok := m.tree.Selected()
	if ok && n.ID+data(n).detail != m.shown {
		m.shown = n.ID + data(n).detail
		m.detail.SetContent(data(n).detail)
		m.detail.GotoTop()
	}
	return m
}

func (m Model) load(n tree.Node) tea.Cmd {
	repo, it := m.repo, data(n)
	return func() tea.Msg {
		desc, err := repo.GetComponentVersion(context.Background(), it.component, it.version)
		if err != nil {
			return loadFailedMsg{n.ID, fmt.Errorf("fetching %s:%s: %w", it.component, it.version, err)}
		}
		return loadedMsg{n, desc}
	}
}

func (m Model) startDownload(it item) (tea.Model, tea.Cmd) {
	m.downloading, m.download = true, "Downloading "+it.resource.Name+"..."
	repo := m.repo
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		path, err := repo.DownloadResource(context.Background(), it.component, it.version, it.resource, ".")
		return downloadedMsg{path, err}
	})
}

func (m Model) View() string {
	return ui.Frame(m.size, m.status(), m.body(), m.help())
}

func (m Model) body() string {
	body := ui.Body(m.size)
	switch {
	case m.repo == nil:
		return m.prompt.View()
	case m.download != "":
		text := m.download
		if m.downloading {
			text = m.spinner.View() + " " + text
		}
		return ui.Center(body.Width, body.Height, ui.Box.Width(60).Render(text))
	}
	left := m.tree.View()
	if m.err != nil {
		left = ui.Errorf(m.err) + "\n" + left
	}
	divider := lipgloss.NewStyle().Foreground(ui.Divider).Render(strings.TrimSuffix(strings.Repeat("│\n", body.Height), "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.treeSize.Width).Height(body.Height).Render(left),
		divider, lipgloss.NewStyle().PaddingLeft(1).Render(m.detail.View()))
}

func (m Model) status() string {
	if m.repo == nil {
		return "enter component reference"
	}
	return m.reference
}

func (m Model) help() string {
	w := m.size.Width
	switch {
	case m.repo == nil:
		return ui.HelpView(w, keys.Connect, keys.Back)
	case m.download != "" && !m.downloading:
		return ui.HelpView(w, keys.Dismiss)
	case !m.tree.Focused:
		return ui.HelpView(w, tree.Keys.Up, tree.Keys.Down, key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab/esc", "tree")))
	}
	n, _ := m.tree.Selected()
	it := data(n)
	download, transfer := keys.Download, keys.Transfer
	download.SetEnabled(it.resource != nil)
	transfer.SetEnabled(it.version != "")
	return ui.HelpView(w, tree.Keys.Up, tree.Keys.Down, tree.Keys.Expand, tree.Keys.Collapse, keys.Focus, download, transfer, keys.Back)
}
