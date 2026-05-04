package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/open-component-model/community/odgcli/pkg/odg"
)

// FindingContext pairs a Finding with its parent artefact for display purposes.
type FindingContext struct {
	Finding  odg.Finding
	Artefact odg.Artefact
}

// ResolvedComment is a Comment with the author's display name already resolved.
type ResolvedComment struct {
	odg.Comment
	DisplayAuthor string
}

// DetailsPane displays metadata and descriptive text for a selected finding.
type DetailsPane struct {
	flex      *tview.Flex
	table     *tview.Table
	separator *tview.TextView
	text      *tview.TextView
}

// NewDetailsPane creates a new details pane with its internal layout.
func NewDetailsPane() *DetailsPane {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(false, false)
	table.SetBorderPadding(0, 0, 1, 1)

	separator := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	separator.SetBorderPadding(1, 1, 1, 1)

	text := tview.NewTextView().
		SetRegions(true).
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetWordWrap(true).
		SetMaxLines(500)
	text.SetBorderPadding(0, 0, 1, 1)

	flex := tview.NewFlex().SetDirection(tview.FlexColumnCSS).
		AddItem(table, 5, 0, false).
		AddItem(separator, 3, 0, false).
		AddItem(text, 0, 1, false)
	flex.SetTitle("Details").SetBorder(true).SetBorderPadding(1, 1, 1, 1)

	return &DetailsPane{
		flex:      flex,
		table:     table,
		separator: separator,
		text:      text,
	}
}

// Primitive returns the top-level tview primitive for embedding in layouts.
func (d *DetailsPane) Primitive() tview.Primitive {
	return d.flex
}

// Clear resets the details pane to an empty state.
func (d *DetailsPane) Clear() {
	d.flex.SetTitle("Details")
	d.flex.SetBorderColor(tcell.ColorWhite)
	d.table.Clear()
	d.separator.SetText("")
	d.text.SetText("")
}

// ShowFinding renders a finding with its resolved comments.
func (d *DetailsPane) ShowFinding(ctx FindingContext, comments []ResolvedComment) {
	finding := ctx.Finding
	artefact := ctx.Artefact

	matchingRules := make([]string, 0, len(finding.MatchingRules))
	for _, rule := range finding.MatchingRules {
		if rule != "original-severity" {
			matchingRules = append(matchingRules, rule)
		}
	}

	dueDate := "N/A"
	if finding.DueDate != nil {
		dueDate = *finding.DueDate
	}

	tableData := [][]string{
		{"CVE:", finding.Finding.CVE, "Package:", finding.Finding.PackageName},
		{"Component:", fmt.Sprintf("%s (%s)", artefact.ComponentName, artefact.ComponentVersion), "Artefact:", fmt.Sprintf("%s (%s)", artefact.Info.Name, artefact.Info.Version)},
		{"Original Severity:", finding.Finding.Severity, "Suggested Severity:", finding.Severity},
		{"Discovery Date:", finding.DiscoveryDate, "Due Date:", dueDate},
		{"Matching Rules:", fmt.Sprintf("%s", matchingRules), "", ""},
	}

	d.table.Clear()
	for row, cols := range tableData {
		for col, text := range cols {
			cell := tview.NewTableCell(text)
			if col%2 == 0 {
				cell.SetTextColor(tcell.ColorGreen)
				cell.SetExpansion(0)
			} else {
				cell.SetText(" " + text)
				cell.SetExpansion(1)
			}
			d.table.SetCell(row, col, cell)
		}
	}

	details := fmt.Sprintf("[green::b]Description:[white::-]\n%s\n\n\n", finding.Finding.Summary)
	details += d.formatComments(finding.Finding.CVE, comments)

	d.flex.SetTitle("Details for " + finding.Finding.CVE)
	d.flex.SetBorderColor(tcell.ColorWhite)
	d.separator.SetText("[gray]" + strings.Repeat("─", 200))
	d.text.SetText(details)
	d.text.ScrollToBeginning()
}

const maxComments = 25

func (d *DetailsPane) formatComments(cve string, comments []ResolvedComment) string {
	if len(comments) == 0 {
		return fmt.Sprintf("[green::b]Rescorings for %s in other components:[white::-]\n[green::d]No rescoring comments for this CVE", cve)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[green::b]Rescorings for %s in other components (%d total):[white::-]\n", cve, len(comments)))

	displayed := len(comments)
	if displayed > maxComments {
		displayed = maxComments
	}

	for _, comment := range comments[:displayed] {
		sb.WriteString(fmt.Sprintf(`
[green]- Author:[white] %s
[green]  Created at:[white] %s
[green]  Component:[white] %s (%s)
[green]  Artefact:[white] %s@%s
[green]  Severity:[white] %s
[green]  Comment:[white]
    %s
`, comment.DisplayAuthor, comment.CreatedAt.Format(time.RFC1123), comment.ComponentName, comment.ComponentVersion, comment.ArtefactName, comment.ArtefactVersion, comment.Severity, comment.Content))
	}

	if len(comments) > maxComments {
		sb.WriteString(fmt.Sprintf("\n[yellow::d]... and %d more comments not shown\n", len(comments)-maxComments))
	}

	return sb.String()
}

// SetNoFindings displays a message indicating no open findings for an artefact.
func (d *DetailsPane) SetNoFindings(artefactName string) {
	d.table.Clear()
	d.separator.SetText("")
	d.flex.SetTitle("Findings for " + artefactName)
	d.flex.SetBorderColor(tcell.ColorGreen)
	d.text.SetText("[green::b]No open findings.[white::-]\n\nAll findings for this artefact have either been rescored or have severity NONE.")
}
