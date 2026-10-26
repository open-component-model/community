// Package integration runs each TUI command end to end against a real OCI registry.
// Like bindings/go/cli/integration in the OCM repository, it needs Docker.
package integration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	"ext.ocm.software/tui/integration/internal"
	"ext.ocm.software/tui/internal/app"
	"ext.ocm.software/tui/internal/ocm"
	"ext.ocm.software/tui/internal/ui"
	"ext.ocm.software/tui/internal/view"
	"ext.ocm.software/tui/internal/view/explore"
	"ext.ocm.software/tui/internal/view/transfer"
)

const (
	component = "ocm.software/tui-integration"
	version   = "v1.0.0"
	resource  = "greeting"
	content   = "hello from the tui"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

// run starts the real app, bootstrapped like cmd/main, directly in the given view.
func run(t *testing.T, start ui.OpenMsg) *teatest.TestModel {
	t.Helper()
	rt, err := ocm.Bootstrap(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rt.Shutdown(t.Context())) })

	tm := teatest.NewTestModel(t, app.New(view.All(rt), &start), teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() {
		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
	})
	return tm
}

// waitFor blocks until the screen output contains text.
func waitFor(t *testing.T, tm *teatest.TestModel, text string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool { return bytes.Contains(out, []byte(text)) },
		teatest.WithDuration(60*time.Second), teatest.WithCheckInterval(100*time.Millisecond))
}

func press(tm *teatest.TestModel, keys ...string) {
	for _, k := range keys {
		switch k {
		case "enter":
			tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
		default:
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		}
	}
}

// Test_Integration covers the happy path of every command against one registry.
func Test_Integration(t *testing.T) {
	reg := internal.StartRegistry(t)
	reg.PushComponentVersion(t, component, version, resource, []byte(content))
	reg.UseConfig(t)

	t.Run("explore: browse and download", func(t *testing.T) {
		r := require.New(t)
		downloads := t.TempDir()
		t.Chdir(downloads)

		tm := run(t, ui.OpenMsg{View: explore.Name, Arg: reg.Reference(component, version)})
		waitFor(t, tm, "Resources (1)")

		// component > version > Resources: expand the group, select the resource and download it.
		press(tm, "j", "j", "enter", "j")
		waitFor(t, tm, resource+" [plainText]")
		press(tm, "d")
		waitFor(t, tm, "Downloaded to:")

		files, err := filepath.Glob(filepath.Join(downloads, "*name="+resource+"*"))
		r.NoError(err)
		r.Len(files, 1)
		data, err := os.ReadFile(files[0])
		r.NoError(err)
		r.Equal(content, string(data))
	})

	t.Run("transfer: registry to CTF", func(t *testing.T) {
		r := require.New(t)
		target := "ctf::" + filepath.Join(t.TempDir(), "ctf")

		tm := run(t, ui.OpenMsg{View: transfer.Name, Arg: reg.Reference(component, version)})
		waitFor(t, tm, "Step 2/4")
		tm.Type(target)
		press(tm, "enter")
		waitFor(t, tm, "Build transformation graph")
		press(tm, "j", "j", "j", "enter")
		waitFor(t, tm, "Step 4/4")
		press(tm, "enter")
		waitFor(t, tm, "Transfer completed")

		// The component version and its resource are in the CTF.
		rt, err := ocm.Bootstrap(t.Context())
		r.NoError(err)
		repo, _, _, err := explore.Connect(rt)(t.Context(), target+"//"+component)
		r.NoError(err)
		desc, err := repo.GetComponentVersion(t.Context(), component, version)
		r.NoError(err)
		r.Len(desc.Component.Resources, 1)
		path, err := repo.DownloadResource(t.Context(), component, version, &desc.Component.Resources[0], t.TempDir())
		r.NoError(err)
		data, err := os.ReadFile(path)
		r.NoError(err)
		r.Equal(content, string(data))
	})
}
