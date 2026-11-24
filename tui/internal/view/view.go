// Package view registers the top-level commands of the TUI.
package view

import (
	"ext.ocm.software/tui/internal/app"
	"ext.ocm.software/tui/internal/ocm"
	"ext.ocm.software/tui/internal/view/explore"
	"ext.ocm.software/tui/internal/view/transfer"
)

// All returns every command, backed by the OCM runtime.
func All(rt *ocm.Runtime) []app.View {
	return []app.View{
		{Name: explore.Name, Label: "Explore components", Hint: "browse versions, resources, references and signatures", Open: func(ref string) app.Page {
			return explore.New(explore.Connect(rt), ref)
		}},
		{Name: transfer.Name, Label: "Transfer component versions", Hint: "copy a component version to another registry or CTF", Open: func(source string) app.Page {
			return transfer.New(transfer.Backend(rt), source)
		}},
	}
}
