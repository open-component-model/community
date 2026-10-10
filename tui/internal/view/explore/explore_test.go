package explore

import (
	"context"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/runtime"

	"ext.ocm.software/tui/internal/component/prompt"
	"ext.ocm.software/tui/internal/ui"
	"ext.ocm.software/tui/internal/view/transfer"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

type fakeRepo map[string]*descriptor.Descriptor

func (f fakeRepo) ListComponentVersions(context.Context, string) ([]string, error) {
	return []string{"1.0.0"}, nil
}

func (f fakeRepo) GetComponentVersion(_ context.Context, component, version string) (*descriptor.Descriptor, error) {
	return f[component+":"+version], nil
}

func (f fakeRepo) DownloadResource(context.Context, string, string, *descriptor.Resource, string) (string, error) {
	return "", nil
}

func cv(name, version string, refs ...descriptor.Reference) *descriptor.Descriptor {
	d := &descriptor.Descriptor{}
	d.Component.Name, d.Component.Version, d.Component.References = name, version, refs
	d.Component.Resources = []descriptor.Resource{{ElementMeta: descriptor.ElementMeta{ObjectMeta: descriptor.ObjectMeta{Name: name + "-bin"}}, Type: "executable"}}
	return d
}

// send feeds msg to the model and then every message its commands produce, returning the last one.
func send(m Model, msg tea.Msg) (Model, tea.Msg) {
	for {
		next, cmd := m.Update(msg)
		m = next.(Model)
		if cmd == nil {
			return m, msg
		}
		if msg = cmd(); msg == nil {
			return m, nil
		}
	}
}

func press(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestExplorer(t *testing.T) {
	r := require.New(t)
	repo := fakeRepo{
		"acme.org/app:1.0.0": cv("acme.org/app", "1.0.0", descriptor.Reference{ElementMeta: descriptor.ElementMeta{ObjectMeta: descriptor.ObjectMeta{Name: "lib", Version: "2.0.0"}}, Component: "acme.org/lib"}),
		"acme.org/lib:2.0.0": cv("acme.org/lib", "2.0.0"),
	}
	m := New(func(context.Context, string) (Repository, string, string, error) {
		return repo, "acme.org/app", "1.0.0", nil
	}, "")
	m, _ = send(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	m, _ = send(m, prompt.SubmitMsg{ID: "reference", Value: "ghcr.io/acme//acme.org/app:1.0.0"})
	r.False(m.Editing())

	// The selected version: tree on the left, its details on the right.
	m, _ = send(m, press("j"))
	golden.RequireEqual(t, m.View())

	// Expanding a reference loads the referenced component below it instead of replacing the tree.
	for _, k := range []string{"j", "j", "enter", "j", "enter", "j", "enter", "j"} {
		m, _ = send(m, press(k))
	}
	view := m.tree.View()
	r.Contains(view, "▾ acme.org/app")
	r.Contains(view, "▾ lib → acme.org/lib:2.0.0")

	// A resource below a reference transfers the referenced component version.
	n, ok := m.tree.Selected()
	r.True(ok)
	r.Equal("acme.org/lib-bin [executable]", n.Label)
	_, msg := send(m, press("t"))
	r.Equal(ui.OpenMsg{View: transfer.Name, Arg: "ghcr.io/acme//acme.org/lib:2.0.0"}, msg)
}

func TestElementLabel(t *testing.T) {
	r := require.New(t)
	r.Equal("cli [executable]", elementLabel("cli", "executable", nil))
	r.Equal("cli [executable] (arch=amd64, os=linux)", elementLabel("cli", "executable",
		runtime.Identity{"name": "cli", "os": "linux", "arch": "amd64"}))
}
