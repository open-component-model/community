package transfer

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"ext.ocm.software/tui/internal/components"
	"ext.ocm.software/tui/fetch"
	transformv1alpha1 "ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"
)

// Wizard steps
type step int

const (
	stepSource step = iota
	stepTarget
	stepOptions
	stepReview
	stepExecuting
	stepDone
)

// Messages
type graphBuiltMsg struct {
	tgd *transformv1alpha1.TransformationGraphDefinition
}
type graphErrMsg struct{ err error }
type transferProgressMsg struct {
	progress fetch.TransferProgress
}
type transferDoneMsg struct{ err error }

// Model is the transfer wizard state.
type Model struct {
	executor fetch.TransferExecutor
	keys     KeyMap
	step     step

	// Inputs
	sourceInput textinput.Model
	targetInput textinput.Model

	// Options
	recursive     bool
	copyResources bool
	uploadAs      int
	optionCursor  int

	// Review
	tgd        *transformv1alpha1.TransformationGraphDefinition
	reviewView viewport.Model

	// Execution
	progressCh      <-chan fetch.TransferProgress
	doneCh          <-chan error
	progressLog     []string
	progressCurrent int
	progressTotal   int
	execErr         error

	// Layout
	width  int
	height int
	ready  bool
	err    error
}

var uploadAsLabels = []string{"default", "localBlob", "ociArtifact"}

// Config holds dependencies for the transfer view.
type Config struct {
	Executor fetch.TransferExecutor
}

// New creates a new transfer wizard model.
func New(executor fetch.TransferExecutor) Model {
	src := textinput.New()
	src.Placeholder = "ghcr.io/source-org/ocm//ocm.software/mycomponent:1.0.0"
	src.CharLimit = 512
	src.Width = 80
	src.Focus()

	tgt := textinput.New()
	tgt.Placeholder = "ghcr.io/target-org/ocm"
	tgt.CharLimit = 512
	tgt.Width = 80

	return Model{
		executor:    executor,
		keys:        DefaultKeyMap(),
		step:        stepSource,
		sourceInput: src,
		targetInput: tgt,
	}
}

// NewView creates a new transfer view.
func NewView(cfg Config, width, height int) *Model {
	m := New(cfg.Executor)
	m.width = width
	m.height = height
	m.ready = true
	return &m
}

// --- View interface ---

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		for _, inp := range []*textinput.Model{&m.sourceInput, &m.targetInput} {
			if inp.Width > msg.Width-4 {
				inp.Width = msg.Width - 4
			}
		}
		if m.step == stepReview {
			m.reviewView.Width = msg.Width - 4
			m.reviewView.Height = msg.Height - 8
		}
		return nil

	case graphBuiltMsg:
		m.tgd = msg.tgd
		m.step = stepReview
		m.err = nil
		rendered, _ := yaml.Marshal(msg.tgd)
		m.reviewView = viewport.New(m.width-4, m.height-8)
		m.reviewView.SetContent(string(rendered))
		return nil

	case graphErrMsg:
		m.err = msg.err
		m.step = stepOptions
		return nil

	case transferProgressMsg:
		if msg.progress.IsLog {
			m.progressLog = append(m.progressLog, "  "+msg.progress.Step)
		} else {
			m.progressLog = append(m.progressLog, msg.progress.Step)
			m.progressCurrent = msg.progress.Current
			m.progressTotal = msg.progress.Total
		}
		return waitForProgress(m.progressCh, m.doneCh)

	case transferDoneMsg:
		m.step = stepDone
		m.execErr = msg.err
		return nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to active input or viewport.
	switch m.step {
	case stepSource:
		var cmd tea.Cmd
		m.sourceInput, cmd = m.sourceInput.Update(msg)
		return cmd
	case stepTarget:
		var cmd tea.Cmd
		m.targetInput, cmd = m.targetInput.Update(msg)
		return cmd
	case stepReview:
		var cmd tea.Cmd
		m.reviewView, cmd = m.reviewView.Update(msg)
		return cmd
	}

	return nil
}

func (m *Model) Render() string {
	return m.render()
}

func (m *Model) StatusInfo() string {
	stepNames := []string{"source", "target", "options", "review", "executing", "done"}
	if int(m.step) < len(stepNames) {
		return "transfer: " + stepNames[m.step]
	}
	return "transfer"
}

func (m *Model) Hotkeys() []components.Hotkey {
	switch m.step {
	case stepSource, stepTarget:
		return []components.Hotkey{
			components.NewHotkey(key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next"))),
			components.NewHotkey(key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))),
		}
	case stepOptions:
		return []components.Hotkey{
			components.NewHotkey(key.NewBinding(key.WithKeys("space"), key.WithHelp("space/enter", "toggle"))),
			components.NewHotkey(m.keys.Up),
			components.NewHotkey(m.keys.Down),
			components.NewHotkey(key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))),
		}
	case stepReview:
		return []components.Hotkey{
			components.NewHotkey(m.keys.Submit),
			components.NewHotkey(key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))),
		}
	case stepDone:
		return []components.Hotkey{
			components.NewHotkey(key.NewBinding(key.WithKeys("any"), key.WithHelp("any key", "continue"))),
		}
	default:
		return nil
	}
}

func (m *Model) OnBackPressed() int {
	switch m.step {
	case stepSource:
		return 1
	case stepTarget:
		m.step = stepSource
		m.targetInput.Blur()
		m.sourceInput.Focus()
		return 0
	case stepOptions:
		m.step = stepTarget
		m.targetInput.Focus()
		return 0
	case stepReview:
		m.step = stepOptions
		return 0
	case stepDone:
		return 1
	default:
		return 1
	}
}

func (m *Model) Resize(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

// Step returns the current wizard step.
func (m Model) Step() step { return m.step }

// IsDone returns true when the transfer is complete.
func (m Model) IsDone() bool { return m.step == stepDone }

// SetSource pre-fills the source input and advances to the target step.
func (m *Model) SetSource(source string) {
	m.sourceInput.SetValue(source)
	m.sourceInput.Blur()
	m.step = stepTarget
	m.targetInput.Focus()
}

func waitForProgress(progressCh <-chan fetch.TransferProgress, doneCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case p, ok := <-progressCh:
			if !ok {
				return transferDoneMsg{err: <-doneCh}
			}
			return transferProgressMsg{progress: p}
		case err := <-doneCh:
			for range progressCh {
			}
			return transferDoneMsg{err: err}
		}
	}
}
