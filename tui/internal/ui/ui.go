// Package ui holds the styles, layout helpers and messages shared by pages and components.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colors follow the ocm.software brand (website/assets/scss/common/_variables-custom.scss).
// Adaptive colors pick the light or dark theme value from the terminal background.
var (
	BrandBlue = lipgloss.Color("#257ddc") // --brand-blue-dark
	BrandMid  = lipgloss.Color("#1d65b4") // --brand-blue-mid
	BrandCyan = lipgloss.Color("#4cc9f0") // --brand-cyan
	// GradientFrom and GradientTo are the stops of --brand-gradient.
	GradientFrom, GradientTo = "#4cc9f0", "#4361ee"

	Accent  = lipgloss.AdaptiveColor{Light: "#1d65b4", Dark: "#4cc9f0"} // --c-accent
	Text    = lipgloss.AdaptiveColor{Light: "#0a0f12", Dark: "#dfdfd6"} // --lp-c-text-1
	Muted   = lipgloss.AdaptiveColor{Light: "#5a6a73", Dark: "#98989f"} // --lp-c-text-2/3
	Divider = lipgloss.AdaptiveColor{Light: "#e2e2de", Dark: "#2e2e32"} // --lp-c-divider
	Soft    = lipgloss.AdaptiveColor{Light: "#eef3f6", Dark: "#202127"} // --lp-c-bg-elv
	Red     = lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ff6b6b"}
	Green   = lipgloss.AdaptiveColor{Light: "#2e7d32", Dark: "#69db7c"}

	Title     = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	Dim       = lipgloss.NewStyle().Foreground(Muted)
	Error     = lipgloss.NewStyle().Foreground(Red)
	Success   = lipgloss.NewStyle().Foreground(Green)
	Running   = lipgloss.NewStyle().Foreground(BrandBlue)
	Cursor    = lipgloss.NewStyle().Bold(true).Foreground(Accent).Background(Soft)
	CursorDim = lipgloss.NewStyle().Foreground(Text).Background(Soft)
	Box       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(BrandBlue).Padding(1, 2)
	// Badge is the "OCM" mark in the status bar, styled like the site's brand button.
	Badge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(BrandMid).Padding(0, 1)
)

// NewHelp returns a help model in the brand colors.
func NewHelp() help.Model {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(Accent)
	h.Styles.ShortDesc = Dim
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(Divider)
	h.Styles.FullKey, h.Styles.FullDesc, h.Styles.FullSeparator = h.Styles.ShortKey, h.Styles.ShortDesc, h.Styles.ShortSeparator
	return h
}

// HelpView renders a one-line help footer for the bindings plus the global command binding.
func HelpView(width int, bindings ...key.Binding) string {
	h := NewHelp()
	h.Width = width
	return h.ShortHelpView(append(bindings, Command))
}

// Command is the global binding that opens the command palette; pages add it to their help.
var Command = key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "commands"))

// ExitMsg asks the app to close the current page and return to the menu.
type ExitMsg struct{}

// Exit is a tea.Cmd that sends ExitMsg.
func Exit() tea.Msg { return ExitMsg{} }

// OpenMsg asks the app to open the view registered under View, passing Arg to it.
type OpenMsg struct{ View, Arg string }

// Editor is implemented by pages; Editing reports whether keys are currently text input,
// so the app must not intercept global keys like ':'.
type Editor interface{ Editing() bool }

// tall reports whether the window has room for the header with the logo.
func tall(size tea.WindowSizeMsg) bool { return size.Height >= 30 && size.Width >= 80 }

func headerHeight(size tea.WindowSizeMsg) int {
	if tall(size) {
		return LogoHeight
	}
	return 1
}

// Body is the content area a page gets inside Frame for a window of the given size.
func Body(size tea.WindowSizeMsg) tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: size.Width, Height: max(size.Height-headerHeight(size)-2, 1)}
}

// Frame renders the header, a separator, the body and the rendered help footer.
// Tall windows get a k9s-style header with the logo on the right.
func Frame(size tea.WindowSizeMsg, status, body, help string) string {
	title := Badge.Render("OCM") + Title.PaddingLeft(1).Render("tui")
	var header string
	if tall(size) {
		info := lipgloss.JoinVertical(lipgloss.Left, title, "", Dim.Render(status))
		header = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(size.Width-lipgloss.Width(Logo())-2).Render(info), Logo())
	} else {
		gap := max(size.Width-lipgloss.Width(title)-lipgloss.Width(status), 0)
		header = title + strings.Repeat(" ", gap) + Dim.Render(status)
	}
	inner := Body(size)
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.NewStyle().Foreground(Divider).Render(strings.Repeat("─", size.Width)),
		lipgloss.NewStyle().Width(inner.Width).Height(inner.Height).MaxHeight(inner.Height).Render(body),
		help)
}

// Center joins lines and places them in the middle of a width x height area.
func Center(width, height int, lines ...string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// Window returns the [start, end) range of n lines that keeps cursor visible in height lines.
func Window(n, cursor, height int) (int, int) {
	start := max(cursor-height+1, 0)
	return start, min(start+height, n)
}

// Errorf renders an error line, or nothing for a nil error.
func Errorf(err error) string {
	if err == nil {
		return ""
	}
	return Error.Render(fmt.Sprintf("Error: %v", err))
}
