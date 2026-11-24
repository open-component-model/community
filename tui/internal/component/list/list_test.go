package list

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

// press applies keys in order like the Bubble Tea runtime would: messages produced by
// commands, such as async filter results, are fed back into Update. It returns the list
// and the ChosenMsg emitted by the last key, if any.
func press(m Model, keys ...string) (Model, tea.Msg) {
	var chosen tea.Msg
	for _, k := range keys {
		chosen = nil
		queue := []tea.Msg{keyMsg(k)}
		for len(queue) > 0 {
			var cmd tea.Cmd
			m, cmd = m.Update(queue[0])
			queue = queue[1:]
			for _, out := range run(cmd) {
				if c, ok := out.(ChosenMsg); ok {
					chosen = c
				} else {
					queue = append(queue, out)
				}
			}
		}
	}
	return m, chosen
}

// run executes cmd and batched commands, dropping any that do not finish quickly (timers).
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
	case <-time.After(50 * time.Millisecond):
		return nil
	}
}

var items = []Item{{Key: "a", Label: "Alpha"}, {Key: "b", Label: "Beta"}, {Key: "c", Label: "Gamma"}}

func sized(m Model) Model {
	m, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	return m
}

func TestChoose(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want Item
	}{
		{"enter chooses the first item", []string{"enter"}, items[0]},
		{"j moves down", []string{"j", "enter"}, items[1]},
		{"space chooses like enter", []string{"j", "j", "space"}, items[2]},
		{"cursor stops at the end", []string{"j", "j", "j", "j", "enter"}, items[2]},
		{"k moves up", []string{"j", "j", "k", "enter"}, items[1]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			_, msg := press(sized(New("id", items...)), tt.keys...)
			r.Equal(ChosenMsg{ID: "id", Item: tt.want}, msg)
		})
	}
}

func TestFiltering(t *testing.T) {
	r := require.New(t)
	m := sized(New("palette", items...).Filtering())
	for _, it := range items {
		r.Contains(m.View(), it.Label, "all items are listed before typing")
	}
	_, msg := press(m, "enter")
	r.Equal(ChosenMsg{ID: "palette", Item: items[0]}, msg, "enter without typing chooses the first item")

	m, _ = press(m, "g", "a", "m")
	r.Contains(m.View(), "Gamma")
	r.NotContains(m.View(), "Alpha")

	// Space is part of the filter text while typing, not a choice.
	_, msg = press(m, "space")
	r.Nil(msg)

	_, msg = press(m, "enter")
	r.Equal(ChosenMsg{ID: "palette", Item: items[2]}, msg)
}

func TestSetItemsKeepsKeys(t *testing.T) {
	r := require.New(t)
	m, _ := press(sized(New("options", items...)), "j")
	m = m.SetItems(Item{Key: "a", Label: "[x] Alpha"}, Item{Key: "b", Label: "[x] Beta"})
	_, msg := press(m, "enter")
	r.Equal(ChosenMsg{ID: "options", Item: Item{Key: "b", Label: "[x] Beta"}}, msg)
}

func TestHintsAreShown(t *testing.T) {
	r := require.New(t)
	r.Contains(sized(New("menu", Item{Label: "Explore", Hint: "browse things"})).View(), "browse things")
	r.NotContains(sized(New("menu", items...)).View(), "\n\n", "items without hints use one line each")
}
