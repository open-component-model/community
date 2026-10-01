// Package transfer is the component version transfer wizard page.
package transfer

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"sigs.k8s.io/yaml"

	transferv1alpha1 "ocm.software/open-component-model/bindings/go/transfer/v1alpha1/spec"
	transformv1alpha1 "ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"

	"ext.ocm.software/tui/internal/component/list"
	"ext.ocm.software/tui/internal/component/prompt"
	"ext.ocm.software/tui/internal/ui"
)

// Transferer builds and runs transfers.
type Transferer interface {
	BuildTransfer(ctx context.Context, source, target string, opts Options) (*transformv1alpha1.TransformationGraphDefinition, error)
	Transfer(ctx context.Context, tgd *transformv1alpha1.TransformationGraphDefinition, progress chan<- Progress) error
}

// Name identifies the transfer view in ui.OpenMsg and on the command line.
const Name = "transfer"

type step int

const (
	stepSource step = iota
	stepTarget
	stepOptions
	stepReview
	stepRunning
	stepDone
)

var stepNames = []string{"source", "target", "options", "review", "running", "done"}

// Progress is one state change of a running transfer.
type Progress struct {
	Line            string
	Finished, Total int
}

const (
	optRecursive     = "recursive"
	optCopyResources = "copy"
	optUploadAs      = "upload"
	optBuild         = "build"
)

var keys = struct{ Next, Back, Run, Scroll key.Binding }{
	Next:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Run:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run transfer")),
	Scroll: key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑↓/jk", "scroll")),
}

type (
	graphMsg struct {
		tgd *transformv1alpha1.TransformationGraphDefinition
		err error
	}
	progressMsg Progress
	doneMsg     struct{ err error }
)

// Model is the transfer wizard: source, target, options, review, then the running transfer.
type Model struct {
	transferer Transferer
	step       step

	source  prompt.Model
	target  prompt.Model
	options list.Model
	opts    Options
	review  viewport.Model
	tgd     *transformv1alpha1.TransformationGraphDefinition

	progress <-chan Progress
	done     <-chan error
	log      []string
	bar      progress.Model
	ratio    float64

	size tea.WindowSizeMsg
	err  error
}

// New creates the wizard. A non-empty source skips the source step.
func New(t Transferer, source string) Model {
	m := Model{
		transferer: t,
		source:     prompt.New("source", "Step 1/4: Source", "Enter the source component reference:", "ghcr.io/source-org/ocm//ocm.software/mycomponent:1.0.0"),
		target:     prompt.New("target", "Step 2/4: Target", "Enter the target repository:", "ghcr.io/target-org/ocm"),
		opts:       Options{UploadAs: transferv1alpha1.UploadAsLocalBlob},
		review:     viewport.New(0, 0),
		bar:        progress.New(progress.WithGradient(ui.GradientFrom, ui.GradientTo)),
	}
	m.options = list.New("options", m.optionItems()...)
	m.target = m.target.Blur()
	if source != "" {
		m.source = m.source.SetValue(source)
		m, _ = m.goTo(stepTarget)
	}
	return m
}

func (m Model) Init() tea.Cmd { return m.source.Init() }

// Editing reports whether keys go to a text prompt.
func (m Model) Editing() bool { return m.step == stepSource || m.step == stepTarget }

func (m Model) optionItems() []list.Item {
	check := func(b bool) string {
		if b {
			return "[x] "
		}
		return "[ ] "
	}
	return []list.Item{
		{Key: optRecursive, Label: check(m.opts.Recursive) + "Recursive", Hint: "also transfer referenced component versions"},
		{Key: optCopyResources, Label: check(m.opts.CopyResources) + "Copy all resources", Hint: "copy external resources by value, not only local blobs"},
		{Key: optUploadAs, Label: "Upload as " + string(m.opts.UploadAs), Hint: "store copied resources as local blobs or OCI artifacts"},
		{Key: optBuild, Label: "Build transformation graph →", Hint: "review the plan before anything is transferred"},
	}
}

// goTo switches the wizard step and moves focus to that step's prompt.
func (m Model) goTo(s step) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.step, m.err = s, nil
	m.source, m.target = m.source.Blur(), m.target.Blur()
	switch s {
	case stepSource:
		m.source, cmd = m.source.Focus()
	case stepTarget:
		m.target, cmd = m.target.Focus()
	}
	return m, cmd
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.size = msg
		body := ui.Body(msg)
		m.source, _ = m.source.Update(body)
		m.target, _ = m.target.Update(body)
		m.review.Width, m.review.Height = body.Width-4, max(body.Height-2, 1)
		m.options, _ = m.options.Update(tea.WindowSizeMsg{Width: min(body.Width, 70), Height: min(len(m.optionItems())*3+1, max(body.Height-4, 3))})
		m.bar.Width = min(body.Width-4, 60)
		return m, nil

	case prompt.SubmitMsg:
		return m.goTo(m.step + 1)

	case list.ChosenMsg:
		switch msg.Item.Key {
		case optRecursive:
			m.opts.Recursive = !m.opts.Recursive
		case optCopyResources:
			m.opts.CopyResources = !m.opts.CopyResources
		case optUploadAs:
			m.opts.UploadAs = map[transferv1alpha1.UploadType]transferv1alpha1.UploadType{
				transferv1alpha1.UploadAsLocalBlob:   transferv1alpha1.UploadAsOciArtifact,
				transferv1alpha1.UploadAsOciArtifact: transferv1alpha1.UploadAsLocalBlob,
			}[m.opts.UploadAs]
		case optBuild:
			m.err = nil
			t, opts, source, target := m.transferer, m.opts, m.source.Value(), m.target.Value()
			return m, func() tea.Msg {
				tgd, err := t.BuildTransfer(context.Background(), source, target, opts)
				return graphMsg{tgd, err}
			}
		}
		m.options = m.options.SetItems(m.optionItems()...)
		return m, nil

	case graphMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		rendered, err := yaml.Marshal(msg.tgd)
		if err != nil {
			m.err = err
			return m, nil
		}
		m, cmd = m.goTo(stepReview)
		m.tgd = msg.tgd
		m.review.SetContent(string(rendered))
		m.review.GotoTop()
		return m, cmd

	case progressMsg:
		m.log = append(m.log, msg.Line)
		if msg.Total > 0 {
			m.ratio = float64(msg.Finished) / float64(msg.Total)
		}
		return m, m.wait()

	case doneMsg:
		m.step, m.err = stepDone, msg.err
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			return m.back()
		}
		switch m.step {
		case stepSource:
			m.source, cmd = m.source.Update(msg)
		case stepTarget:
			m.target, cmd = m.target.Update(msg)
		case stepOptions:
			m.options, cmd = m.options.Update(msg)
		case stepReview:
			if msg.Type == tea.KeyEnter {
				return m.run()
			}
			m.review, cmd = m.review.Update(msg)
		}
		return m, cmd
	}

	// Cursor blinks and other component messages go to the active prompt.
	switch m.step {
	case stepSource:
		m.source, cmd = m.source.Update(msg)
	case stepTarget:
		m.target, cmd = m.target.Update(msg)
	}
	return m, cmd
}

func (m Model) back() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepTarget, stepOptions, stepReview:
		return m.goTo(m.step - 1)
	case stepRunning:
		return m, nil
	}
	return m, ui.Exit
}

func (m Model) run() (tea.Model, tea.Cmd) {
	m, _ = m.goTo(stepRunning)
	progress, done := make(chan Progress, 16), make(chan error, 1)
	m.progress, m.done = progress, done
	t, tgd := m.transferer, m.tgd
	go func() { done <- t.Transfer(context.Background(), tgd, progress) }()
	return m, m.wait()
}

// wait delivers the next progress line, or the result once progress is closed.
func (m Model) wait() tea.Cmd {
	progress, done := m.progress, m.done
	return func() tea.Msg {
		if p, ok := <-progress; ok {
			return progressMsg(p)
		}
		return doneMsg{<-done}
	}
}

func (m Model) View() string {
	return ui.Frame(m.size, "transfer: "+stepNames[m.step], m.body(), m.help())
}

func (m Model) body() string {
	body := ui.Body(m.size)
	switch m.step {
	case stepSource:
		return m.source.View()
	case stepTarget:
		return m.target.View()
	case stepOptions:
		return ui.Center(body.Width, body.Height, ui.Title.MarginBottom(1).Render("Step 3/4: Options"), m.options.View(), "", ui.Errorf(m.err))
	case stepReview:
		header := fmt.Sprintf("Step 4/4: Review (%d transformations)", len(m.tgd.Transformations))
		return lipgloss.JoinVertical(lipgloss.Left, ui.Title.Render(header), "", m.review.View())
	}

	title := ui.Title.Render("Transferring...")
	if m.step == stepDone && m.err != nil {
		title = ui.Error.Bold(true).Render("Transfer failed: " + m.err.Error())
	} else if m.step == stepDone {
		title = ui.Success.Bold(true).Render("Transfer completed")
	}
	start, end := ui.Window(len(m.log), len(m.log)-1, max(body.Height-4, 1))
	lines := []string{title, "", m.bar.ViewAs(m.ratio), ""}
	for _, l := range m.log[start:end] {
		style := ui.Dim
		switch {
		case strings.HasSuffix(l, "completed"):
			style = ui.Success
		case strings.Contains(l, "failed"):
			style = ui.Error
		case strings.HasSuffix(l, "running"):
			style = ui.Running
		}
		lines = append(lines, style.Render(l))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) help() string {
	w := m.size.Width
	switch m.step {
	case stepOptions:
		return ui.HelpView(w, append(m.options.Bindings(), keys.Back)...)
	case stepReview:
		return ui.HelpView(w, keys.Run, keys.Scroll, keys.Back)
	case stepRunning:
		return ""
	case stepDone:
		return ui.HelpView(w, key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "menu")))
	}
	return ui.HelpView(w, keys.Next, keys.Back)
}
