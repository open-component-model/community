package views

import (
	"strings"
	"sync"

	"github.com/rivo/tview"
)

// StatusBar is a tview.TextView wrapper that manages multiple named message
// segments separated by " | ". Any part of the application that holds a
// reference to the StatusBar can update individual segments independently,
// and the displayed text is re-rendered automatically.
type StatusBar struct {
	tv *tview.TextView

	mu       sync.Mutex
	keys     []string          // insertion-ordered segment keys
	segments map[string]string // key -> display text
}

// NewStatusBar creates a StatusBar with a bordered TextView titled "Key Bindings".
func NewStatusBar() *StatusBar {
	tv := tview.NewTextView().SetDynamicColors(true)
	tv.SetBorder(true).SetTitle("Key Bindings").SetBorderPadding(0, 0, 1, 1)

	return &StatusBar{
		tv:       tv,
		segments: make(map[string]string),
	}
}

// SetMessage sets or updates the segment identified by key.
func (s *StatusBar) SetMessage(key, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.segments[key]; !exists {
		s.keys = append(s.keys, key)
	}
	s.segments[key] = text
	s.render()
}

// RemoveMessage removes a segment by key.
func (s *StatusBar) RemoveMessage(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.segments[key]; !exists {
		return
	}
	delete(s.segments, key)
	filtered := s.keys[:0]
	for _, k := range s.keys {
		if k != key {
			filtered = append(filtered, k)
		}
	}
	s.keys = filtered
	s.render()
}

// Primitive returns the top-level tview primitive for embedding in layouts.
func (s *StatusBar) Primitive() tview.Primitive {
	return s.tv
}

// render rebuilds the displayed text. Must be called with s.mu held.
func (s *StatusBar) render() {
	parts := make([]string, 0, len(s.keys))
	for _, k := range s.keys {
		if text := s.segments[k]; text != "" {
			parts = append(parts, text)
		}
	}
	s.tv.SetText(strings.Join(parts, " | "))
}
