package transfer

import (
	"context"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	transformv1alpha1 "ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"

	"ext.ocm.software/tui/internal/ui"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

type fakeTransferer struct {
	source, target string
	opts           Options
}

func (f *fakeTransferer) BuildTransfer(_ context.Context, source, target string, opts Options) (*transformv1alpha1.TransformationGraphDefinition, error) {
	f.source, f.target, f.opts = source, target, opts
	return &transformv1alpha1.TransformationGraphDefinition{
		Transformations: []transformv1alpha1.GenericTransformation{{}, {}},
	}, nil
}

func (f *fakeTransferer) Transfer(_ context.Context, _ *transformv1alpha1.TransformationGraphDefinition, progress chan<- Progress) error {
	defer close(progress)
	progress <- Progress{Line: "[1/2] get: completed", Finished: 1, Total: 2}
	progress <- Progress{Line: "[2/2] upload: completed", Finished: 2, Total: 2}
	return nil
}

// send applies msg and the messages its commands produce, like the Bubble Tea runtime.
// Commands that do not finish quickly (cursor blink timers) are dropped. It returns the
// model and a ui.ExitMsg if the page asked to leave.
func send(m Model, msg tea.Msg) (Model, tea.Msg) {
	queue := []tea.Msg{msg}
	for len(queue) > 0 {
		next, cmd := m.Update(queue[0])
		m, queue = next.(Model), queue[1:]
		for _, out := range run(cmd) {
			if exit, ok := out.(ui.ExitMsg); ok {
				return m, exit
			}
			queue = append(queue, out)
		}
	}
	return m, nil
}

func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if batch, ok := msg.(tea.BatchMsg); ok {
			var out []tea.Msg
			for _, c := range batch {
				out = append(out, run(c)...)
			}
			return out
		}
		if msg == nil {
			return nil
		}
		return []tea.Msg{msg}
	case <-time.After(200 * time.Millisecond):
		return nil
	}
}

func press(m Model, ks ...string) Model {
	for _, k := range ks {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		}
		m, _ = send(m, msg)
	}
	return m
}

func TestWizard(t *testing.T) {
	r := require.New(t)
	fake := &fakeTransferer{}
	m := New(fake, "ghcr.io/src//acme.org/app:1.0.0")
	m, _ = send(m, tea.WindowSizeMsg{Width: 100, Height: 24})
	r.Equal(stepTarget, m.step, "a pre-filled source skips the source step")
	r.True(m.Editing())
	m = press(m, "esc")
	r.Equal(stepSource, m.step, "esc goes back one step")
	m = press(m, "enter")
	r.Equal(stepTarget, m.step, "the pre-filled source is kept")

	m.target = m.target.SetValue("ctf::/tmp/ctf")
	m = press(m, "enter")
	r.Equal(stepOptions, m.step)
	r.False(m.Editing(), "':' opens the palette outside the text steps")
	golden.RequireEqual(t, m.View())

	// toggle recursive, switch upload type, build the graph
	m = press(m, "enter", "j", "j", "enter", "j", "enter")
	r.Equal(stepReview, m.step)
	r.Equal(Options{Recursive: true, UploadAs: "ociArtifact"}, fake.opts)
	r.Equal("ghcr.io/src//acme.org/app:1.0.0", fake.source)
	r.Equal("ctf::/tmp/ctf", fake.target)
	r.Contains(m.View(), "Review (2 transformations)")

	m = press(m, "enter")
	r.Equal(stepDone, m.step)
	r.NoError(m.err)
	r.InDelta(1.0, m.ratio, 0.001)
	r.Contains(m.View(), "Transfer completed")
	r.Contains(m.View(), "[2/2] upload: completed")

	_, msg := send(m, tea.KeyMsg{Type: tea.KeyEsc})
	r.Equal(ui.ExitMsg{}, msg, "esc after the transfer leaves the page")
}
