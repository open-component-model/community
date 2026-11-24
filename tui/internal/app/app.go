// Package app is the root model: a menu, a command palette and the active page.
package app

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ext.ocm.software/tui/internal/component/list"
	"ext.ocm.software/tui/internal/ui"
)

// Page is a full-screen view. It leaves itself by sending ui.ExitMsg.
type Page interface {
	tea.Model
	ui.Editor
}

// View is a top-level command: it appears in the menu and the palette and can be opened by name.
type View struct {
	Name  string
	Label string
	Hint  string
	// Open creates the page; arg is optional input such as a reference.
	Open func(arg string) Page
}

const (
	cmdMenu = "Menu"
	cmdQuit = "Quit"
)

var (
	quit    = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	closeIt = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))
)

// Model is the root model.
type Model struct {
	views []View
	menu  list.Model
	help  help.Model

	page        Page
	palette     list.Model
	paletteOpen bool

	start *ui.OpenMsg
	size  tea.WindowSizeMsg
}

// New creates the app with its views. A non-nil start message opens a view right away.
func New(views []View, start *ui.OpenMsg) Model {
	items := make([]list.Item, len(views))
	for i, v := range views {
		items[i] = list.Item{Label: v.Label, Hint: v.Hint}
	}
	return Model{views: views, menu: list.New("menu", items...), help: ui.NewHelp(), start: start}
}

func (m Model) Init() tea.Cmd {
	if m.start == nil {
		return nil
	}
	start := *m.start
	return func() tea.Msg { return start }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.size = msg
		m.menu, _ = m.menu.Update(tea.WindowSizeMsg{Width: 64, Height: min(len(m.views)*3+1, max(msg.Height-12, 3))})
	case ui.ExitMsg:
		m.page = nil
		return m, nil
	case ui.OpenMsg:
		for _, v := range m.views {
			if v.Name == msg.View {
				return m.show(v.Open(msg.Arg))
			}
		}
		return m, nil
	case list.ChosenMsg:
		if msg.ID == "menu" || msg.ID == "palette" {
			m.paletteOpen = false
			return m.run(msg.Item.Label)
		}
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmds []tea.Cmd
	if m.paletteOpen {
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.page != nil {
		page, cmd := m.page.Update(msg)
		m.page = page.(Page)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case msg.String() == "ctrl+c":
		return m, tea.Quit
	case m.paletteOpen && key.Matches(msg, closeIt):
		m.paletteOpen = false
		return m, nil
	case m.paletteOpen:
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd
	case key.Matches(msg, ui.Command) && (m.page == nil || !m.page.Editing()):
		items := make([]list.Item, 0, len(m.views)+2)
		for _, v := range m.views {
			items = append(items, list.Item{Label: v.Label})
		}
		m.palette = list.New("palette", append(items, list.Item{Label: cmdMenu}, list.Item{Label: cmdQuit})...).Filtering()
		m.palette, _ = m.palette.Update(tea.WindowSizeMsg{Width: 44, Height: 8})
		m.paletteOpen = true
		return m, nil
	case m.page != nil:
		page, cmd := m.page.Update(msg)
		m.page = page.(Page)
		return m, cmd
	case key.Matches(msg, quit):
		return m, tea.Quit
	}
	m.menu, cmd = m.menu.Update(msg)
	return m, cmd
}

// run executes a menu or palette entry by its label.
func (m Model) run(label string) (tea.Model, tea.Cmd) {
	switch label {
	case cmdQuit:
		return m, tea.Quit
	case cmdMenu:
		m.page = nil
		return m, nil
	}
	for _, v := range m.views {
		if v.Label == label {
			return m.show(v.Open(""))
		}
	}
	return m, nil
}

func (m Model) show(page Page) (tea.Model, tea.Cmd) {
	sized, cmd := page.Update(m.size)
	m.page = sized.(Page)
	return m, tea.Batch(page.Init(), cmd)
}

func (m Model) View() string {
	switch {
	case m.size.Width == 0:
		return "Initializing..."
	case m.paletteOpen:
		return ui.Center(m.size.Width, m.size.Height, ui.Box.Padding(0, 1).Render(lipgloss.JoinVertical(lipgloss.Left,
			ui.Title.Render("Commands"), m.palette.View(), m.help.ShortHelpView([]key.Binding{closeIt}))))
	case m.page != nil:
		return m.page.View()
	}
	keys := append(m.menu.Bindings(), ui.Command, quit)
	rest := []string{ui.Badge.Render("OCM") + ui.Title.PaddingLeft(1).Render("Open Component Model"), "", m.menu.View(), "", m.help.ShortHelpView(keys)}
	content := lipgloss.JoinVertical(lipgloss.Center, rest...)
	// Show the logo whenever it fits above the menu.
	if withLogo := lipgloss.JoinVertical(lipgloss.Center, append([]string{ui.Logo(), ""}, rest...)...); lipgloss.Height(withLogo) <= m.size.Height {
		content = withLogo
	}
	return lipgloss.Place(m.size.Width, m.size.Height, lipgloss.Center, lipgloss.Center, content)
}
