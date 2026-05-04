package views

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// LoadingModal is a modal overlay with an animated loading spinner.
type LoadingModal struct {
	app         *tview.Application
	pages       *tview.Pages
	modal       *tview.Modal
	background  *tview.TextView
	text        string
	stopChan    chan struct{}
	isVisible   bool
	onHideFocus tview.Primitive
}

// NewLoadingModal creates a new LoadingModal.
func NewLoadingModal(app *tview.Application, pages *tview.Pages) *LoadingModal {
	return &LoadingModal{
		app:   app,
		pages: pages,
	}
}

// SetOnHideFocus sets the primitive to focus when the modal is hidden.
func (l *LoadingModal) SetOnHideFocus(p tview.Primitive) *LoadingModal {
	l.onHideFocus = p
	return l
}

// Show displays the loading modal and starts the spinner animation.
func (l *LoadingModal) Show(text string) {
	if l.isVisible {
		return
	}

	l.text = text
	l.isVisible = true
	l.stopChan = make(chan struct{})

	l.background = tview.NewTextView().
		SetTextColor(tcell.ColorBlue).
		SetText(strings.Repeat(". ", 100000))

	modal := tview.NewModal().
		SetBackgroundColor(tcell.ColorDefault).
		SetText(text + "\n\n⠋").
		AddButtons([]string{})
	modal.SetBorderStyle(tcell.StyleDefault)
	modal.SetBorder(true)
	modal.SetBorderColor(tcell.ColorBlue)
	l.modal = modal

	l.app.QueueUpdateDraw(func() {
		l.pages.AddPage("loadingBG", l.background, true, true)
		l.pages.AddPage("loading", l.modal, true, true)
	})

	// Start spinner animation in a goroutine
	go l.animateSpinner()
}

// Hide hides the loading modal and stops the spinner.
func (l *LoadingModal) Hide() {
	if !l.isVisible {
		return
	}

	if l.stopChan != nil {
		close(l.stopChan)
		l.stopChan = nil
	}

	l.isVisible = false
	l.modal = nil
	l.background = nil

	l.app.QueueUpdateDraw(func() {
		l.pages.RemovePage("loading")
		l.pages.RemovePage("loadingBG")
		if l.onHideFocus != nil {
			l.app.SetFocus(l.onHideFocus)
		}
	})
}

// IsVisible returns whether the modal is currently visible.
func (l *LoadingModal) IsVisible() bool {
	return l.isVisible
}

// animateSpinner runs the spinner animation loop
func (l *LoadingModal) animateSpinner() {
	spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frameIndex := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopChan:
			return
		case <-ticker.C:
			frame := spinnerFrames[frameIndex]
			frameIndex = (frameIndex + 1) % len(spinnerFrames)
			l.app.QueueUpdateDraw(func() {
				if l.modal != nil {
					l.modal.SetText(l.text + "\n\n" + frame)
				}
			})
		}
	}
}
