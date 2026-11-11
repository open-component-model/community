package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func WrapWithFocusBorders(p *tview.Box) {
	p.SetFocusFunc(func() {
		p.SetBorderColor(tcell.ColorBlue)
	})
	p.SetBlurFunc(func() {
		p.SetBorderColor(tcell.ColorWhite)
	})
}

func CreateTreeNode(text string, expanded bool) *tview.TreeNode {
	node := tview.NewTreeNode(text).SetExpanded(expanded)
	node.SetSelectedTextStyle(tcell.Style{}.Foreground(tcell.ColorDarkCyan))
	return node
}
