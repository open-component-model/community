package explorer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"ext.ocm.software/tui/internal/components/progress"
	"ext.ocm.software/tui/internal/components/splitpane"
	"ext.ocm.software/tui/internal/theme"
)

// updateBrowse handles messages in browse mode.
func (m *Model) updateBrowse(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Resize(msg.Width, msg.Height)
		return nil

	case versionsMsg:
		m.loading = false
		node := &Node{Kind: NodeComponent, Label: msg.component, Depth: 0}
		for _, v := range msg.versions {
			node.Children = append(node.Children, &Node{Kind: NodeVersion, Label: v, Depth: 1})
		}
		node.Expanded = true
		m.roots = []*Node{node}
		m.visible = Flatten(m.roots)
		m.updateDetail()
		return nil

	case descriptorMsg:
		m.loading = false
		found := false
		for _, root := range m.roots {
			for _, child := range root.Children {
				if child.Kind == NodeVersion && child.Label == msg.version {
					child.Descriptor = msg.descriptor
					child.Children = BuildVersionNodes(msg.descriptor, child.Depth+1)[0].Children
					child.Expanded = true
					child.Loading = false
					found = true
					break
				}
			}
		}
		if !found && msg.descriptor != nil {
			node := &Node{Kind: NodeComponent, Label: msg.descriptor.Component.Name, Depth: 0, Expanded: true}
			versionNodes := BuildVersionNodes(msg.descriptor, 1)
			versionNodes[0].Expanded = true
			node.Children = versionNodes
			m.roots = []*Node{node}
		}
		m.visible = Flatten(m.roots)
		m.updateDetail()
		return nil

	case progress.TickMsg:
		if m.downloading {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(progress.Frames)
			m.downloadStatus = fmt.Sprintf("%s Downloading %s...", progress.Frame(m.spinnerFrame), m.downloadResName)
			return progress.Tick()
		}
		return nil

	case downloadDoneMsg:
		m.downloading = false
		m.downloadStatus = fmt.Sprintf("Downloaded %s to:\n%s", msg.resourceName, msg.outputPath)
		return nil

	case errMsg:
		m.loading = false
		if m.downloading {
			m.downloading = false
			m.downloadStatus = fmt.Sprintf("Download failed: %v", msg.err)
		}
		m.err = msg.err
		return nil

	case tea.KeyMsg:
		if m.downloading {
			return nil
		}
		if m.downloadStatus != "" {
			m.downloadStatus = ""
			return nil
		}
		return m.handleKey(msg)
	}

	if !m.focusTree {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return cmd
	}

	return nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.focusTree && m.cursor > 0 {
			m.cursor--
			m.updateDetail()
		}

	case key.Matches(msg, m.keys.Down):
		if m.focusTree && m.cursor < len(m.visible)-1 {
			m.cursor++
			m.updateDetail()
		}

	case key.Matches(msg, m.keys.PageUp):
		if m.focusTree {
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateDetail()
		}

	case key.Matches(msg, m.keys.PageDown):
		if m.focusTree {
			m.cursor += 10
			if m.cursor >= len(m.visible) {
				m.cursor = len(m.visible) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateDetail()
		}

	case key.Matches(msg, m.keys.Expand):
		if m.focusTree && len(m.visible) > 0 {
			node := m.visible[m.cursor]
			if node.IsExpandable() && !node.Expanded {
				node.Expanded = true
				if node.Kind == NodeVersion && len(node.Children) == 0 && node.Descriptor == nil {
					node.Loading = true
					m.visible = Flatten(m.roots)
					component := m.findComponentName(node)
					return m.fetchDescriptor(component, node.Label)
				}
				if node.Kind == NodeReference && len(node.Children) == 0 && node.Reference != nil {
					node.Loading = true
					m.visible = Flatten(m.roots)
					return m.fetchDescriptor(node.Reference.Component, node.Reference.Version)
				}
				m.visible = Flatten(m.roots)
				m.updateDetail()
			}
		}

	case key.Matches(msg, m.keys.Collapse):
		if m.focusTree && len(m.visible) > 0 {
			node := m.visible[m.cursor]
			if node.IsExpandable() && node.Expanded {
				node.Expanded = false
				m.visible = Flatten(m.roots)
				m.updateDetail()
			}
		}

	case key.Matches(msg, m.keys.Download):
		return m.startDownload()

	case key.Matches(msg, m.keys.Transfer):
		if m.focusTree && len(m.visible) > 0 {
			node := m.visible[m.cursor]
			component, version := m.findTransferContext(node)
			if component != "" && version != "" {
				ref := m.reference
				return func() tea.Msg {
					return TransferRequestMsg{Reference: ref, Component: component, Version: version}
				}
			}
		}
	}

	return nil
}

// renderBrowse renders the browse mode split pane view.
func (m *Model) renderBrowse() string {
	if !m.ready {
		return "Initializing..."
	}

	sp := splitpane.New(m.width, m.height)
	sp.Left = m.renderTree(m.height)
	sp.Right = m.detail.View()

	base := sp.Render()

	if m.downloadStatus != "" {
		return m.renderDownloadModal()
	}

	return base
}

func (m *Model) renderTree(height int) string {
	if m.loading && len(m.visible) == 0 {
		return "Loading..."
	}
	if m.err != nil && len(m.visible) == 0 {
		return fmt.Sprintf("Error: %v", m.err)
	}

	t := theme.Current()
	var lines []string

	scrollOffset := 0
	if m.cursor >= height {
		scrollOffset = m.cursor - height + 1
	}
	end := scrollOffset + height
	if end > len(m.visible) {
		end = len(m.visible)
	}

	for i := scrollOffset; i < end; i++ {
		node := m.visible[i]
		line := RenderNode(node, i == m.cursor)
		if i == m.cursor {
			if m.focusTree {
				line = t.Cursor.Render(line)
			} else {
				line = t.CursorDim.Render(line)
			}
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// --- Tree context helpers ---

func (m *Model) findComponentName(node *Node) string {
	for _, root := range m.roots {
		if root.Kind == NodeComponent {
			for _, child := range root.Children {
				if child == node {
					return root.Label
				}
			}
			if containsNode(root, node) {
				return root.Label
			}
		}
	}
	return m.initialComponent
}

func containsNode(parent, target *Node) bool {
	for _, child := range parent.Children {
		if child == target {
			return true
		}
		if containsNode(child, target) {
			return true
		}
	}
	return false
}

func (m *Model) findTransferContext(node *Node) (component, version string) {
	if node.Kind == NodeVersion {
		return m.findComponentName(node), node.Label
	}
	for _, root := range m.roots {
		if root.Kind != NodeComponent {
			continue
		}
		for _, vNode := range root.Children {
			if vNode.Kind != NodeVersion {
				continue
			}
			if vNode == node || containsNode(vNode, node) {
				return root.Label, vNode.Label
			}
		}
	}
	return "", ""
}

func (m *Model) findResourceContext(target *Node) (component, version string) {
	for _, root := range m.roots {
		if root.Kind != NodeComponent {
			continue
		}
		for _, vNode := range root.Children {
			if vNode.Kind != NodeVersion {
				continue
			}
			if containsNode(vNode, target) {
				return root.Label, vNode.Label
			}
		}
	}
	return "", ""
}
