// Package list wraps the bubbles list in the brand theme and reports choices as messages.
package list

import (
	"github.com/charmbracelet/bubbles/key"
	blist "github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/ui"
)

// Item is a list entry with an optional one-line hint below it.
// Key identifies the item when its label changes, e.g. a toggled checkbox.
type Item struct{ Key, Label, Hint string }

func (i Item) Title() string       { return i.Label }
func (i Item) Description() string { return i.Hint }
func (i Item) FilterValue() string { return i.Label }

// ChosenMsg reports that Item was chosen in the list with ID.
type ChosenMsg struct {
	ID   string
	Item Item
}

// Model is a navigable, optionally filterable list.
type Model struct {
	ID   string
	list blist.Model
}

var choose = key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter", "select"))

// New creates a list. The ID tells parents with several lists which one sent a ChosenMsg.
func New(id string, items ...Item) Model {
	d := blist.NewDefaultDelegate()
	d.Styles.NormalTitle = lipgloss.NewStyle().Foreground(ui.Text).Padding(0, 0, 0, 2)
	d.Styles.NormalDesc = lipgloss.NewStyle().Foreground(ui.Muted).Padding(0, 0, 0, 2)
	d.Styles.SelectedTitle = lipgloss.NewStyle().Bold(true).Foreground(ui.Accent).
		Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(ui.Accent).Padding(0, 0, 0, 1)
	d.Styles.SelectedDesc = d.Styles.SelectedTitle.Bold(false).Foreground(ui.Muted)
	d.Styles.DimmedTitle, d.Styles.DimmedDesc = d.Styles.NormalTitle, d.Styles.NormalDesc
	d.Styles.FilterMatch = lipgloss.NewStyle().Underline(true).Foreground(ui.BrandCyan)
	d.ShowDescription = false
	for _, it := range items {
		d.ShowDescription = d.ShowDescription || it.Hint != ""
	}
	if !d.ShowDescription {
		d.SetHeight(1)
		d.SetSpacing(0)
	}

	l := blist.New(toItems(items), d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ui.Accent)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ui.BrandCyan)
	l.Styles.ActivePaginationDot = lipgloss.NewStyle().Foreground(ui.Accent).SetString("•")
	l.Styles.InactivePaginationDot = lipgloss.NewStyle().Foreground(ui.Divider).SetString("•")
	return Model{ID: id, list: l}
}

func toItems(items []Item) []blist.Item {
	out := make([]blist.Item, len(items))
	for i, it := range items {
		out[i] = it
	}
	return out
}

// Filtering returns the list with its fuzzy filter input open, as used by the command palette.
func (m Model) Filtering() Model {
	m.list.SetFilteringEnabled(true)
	m.list.FilterInput.Prompt = "› "
	// Run the empty filter once so every item is listed before the first keystroke.
	m.list.SetFilterText("")
	m.list.SetFilterState(blist.Filtering)
	return m
}

// SetItems replaces the items and keeps the cursor position.
func (m Model) SetItems(items ...Item) Model {
	m.list.SetItems(toItems(items))
	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		typing := m.list.FilterState() == blist.Filtering
		if key.Matches(msg, choose) && (!typing || msg.String() != " ") {
			if it, ok := m.list.SelectedItem().(Item); ok {
				chosen := ChosenMsg{ID: m.ID, Item: it}
				return m, func() tea.Msg { return chosen }
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string { return m.list.View() }

// Bindings returns the list keys for help footers.
func (m Model) Bindings() []key.Binding {
	return []key.Binding{m.list.KeyMap.CursorUp, m.list.KeyMap.CursorDown, choose}
}
