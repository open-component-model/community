package tui

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/open-component-model/community/odgcli/internal/views"
	"github.com/open-component-model/community/odgcli/pkg/github"
	"github.com/open-component-model/community/odgcli/pkg/odg"
)

// Application holds all state for the tview TUI.
type Application struct {
	TV            *tview.Application
	Pages         *tview.Pages
	MainTreeView  *tview.TreeView
	Clients       ApplicationClients
	LoadingModal  *views.LoadingModal
	StatusBar     *views.StatusBar
	DetailsPane   *views.DetailsPane
	filterForUser bool
	rootComponent string
	ctx           context.Context
	cancel        context.CancelFunc
}

// ApplicationClients bundles the API clients used by the TUI.
type ApplicationClients struct {
	GHClient  *github.Client
	ODGClient *odg.Client
}

// Run creates and starts the interactive TUI application.
func Run(odgClient *odg.Client, ghClient *github.Client, rootComponent string) error {
	// Suppress log output from libraries that write to stderr,
	// which would bypass tcell's alternate screen buffer and shift the TUI.
	log.SetOutput(io.Discard)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tv := tview.NewApplication()
	pages := tview.NewPages()

	app := &Application{
		TV:    tv,
		Pages: pages,
		Clients: ApplicationClients{
			GHClient:  ghClient,
			ODGClient: odgClient,
		},
		LoadingModal:  views.NewLoadingModal(tv, pages),
		StatusBar:     views.NewStatusBar(),
		filterForUser: true,
		rootComponent: rootComponent,
		ctx:           ctx,
		cancel:        cancel,
	}

	app.createMainUI()
	return nil
}

func (a *Application) loadArtifactDetails(artifact odg.ArtefactEntry) {
	var findings []odg.Finding

	err := a.doWithModal(func() error {
		var err error
		findings, err = a.Clients.ODGClient.GetRescorings(a.ctx, artifact.Artefact)
		return err
	}, fmt.Sprintf("Loading CVEs for %s in %s@%s...", artifact.Artefact.Info.Name, artifact.Artefact.ComponentName,
		artifact.Artefact.ComponentVersion), 50*time.Millisecond)

	if err != nil {
		go a.showErrorModal(err)
		return
	}

	// filter findings
	var filteredFindings []odg.Finding
	for _, finding := range findings {
		if finding.Finding.Severity == "NONE" {
			continue
		}
		if len(finding.ApplicableRescorings) != 0 {
			continue
		}
		filteredFindings = append(filteredFindings, finding)
	}

	go func() {
		a.TV.QueueUpdateDraw(func() {
			if len(filteredFindings) == 0 {
				a.DetailsPane.SetNoFindings(artifact.Artefact.Info.Name)
			} else {
				currentArtifact := a.MainTreeView.GetCurrentNode()
				currentArtifact.ClearChildren()
				currentArtifact.Expand()
				for _, finding := range filteredFindings {
					findingNode := views.CreateTreeNode(fmt.Sprintf("%s (%s)", finding.Finding.CVE, finding.Finding.Severity), false).SetReference(views.FindingContext{
						Finding:  finding,
						Artefact: artifact.Artefact,
					})
					currentArtifact.AddChild(findingNode)
				}
				a.MainTreeView.SetCurrentNode(currentArtifact.GetChildren()[0])
			}
		})
	}()
}

func (a *Application) showErrorModal(err error) {
	errorModal := tview.NewModal().
		SetText("An error occurred:" + err.Error()).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.Pages.RemovePage("error")
			a.TV.SetFocus(a.MainTreeView)
		})
	a.TV.QueueUpdateDraw(func() {
		a.Pages.AddPage("error", errorModal, true, true)
	})
}

func (a *Application) showFindingDetails(findingCtx views.FindingContext) {
	comments, commentsErr := a.getCommentsForCVE(a.ctx, findingCtx.Finding.Finding.CVE)

	if commentsErr != nil {
		go a.showErrorModal(commentsErr)
	}

	// Resolve author display names.
	var resolved []views.ResolvedComment
	if commentsErr == nil {
		for _, comment := range comments {
			author, err := a.Clients.GHClient.ResolveUsername(a.ctx, comment.Author)
			if err != nil {
				author = comment.Author
			}
			resolved = append(resolved, views.ResolvedComment{
				Comment:       comment,
				DisplayAuthor: author,
			})
		}
	}

	a.TV.QueueUpdateDraw(func() {
		a.DetailsPane.ShowFinding(views.FindingContext{
			Finding:  findingCtx.Finding,
			Artefact: findingCtx.Artefact,
		}, resolved)
	})
}

// getCommentsForCVE queries rescoring entries for the given CVE and transforms
// them into Comment structs for display.
func (a *Application) getCommentsForCVE(ctx context.Context, cve string) ([]odg.Comment, error) {
	var comments []odg.Comment
	for item, err := range a.Clients.ODGClient.QueryMetadataBySearchExpression(ctx, []odg.MetadataQueryCriterion{
		{Type: "artefact-metadata", Attr: "type", Op: "eq", Value: "rescorings"},
		{Type: "artefact-metadata", Attr: "data.finding.cve", Op: "eq", Value: cve},
	}, 50, []odg.MetadataQuerySort{
		{Field: "meta.creation_date", Order: "desc"},
		{Field: "id", Order: "desc"},
	}) {
		if err != nil {
			return nil, err
		}
		if item.Data.Comment == "" {
			continue
		}

		createdAt, err := time.Parse(time.RFC3339, item.Meta.CreationDate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse creation date %q: %w", item.Meta.CreationDate, err)
		}

		componentVersion := ""
		if item.Artefact.ComponentVersion != nil {
			componentVersion = *item.Artefact.ComponentVersion
		}

		comments = append(comments, odg.Comment{
			Author:           item.Data.User.Username,
			Content:          item.Data.Comment,
			CreatedAt:        createdAt,
			ComponentName:    item.Artefact.ComponentName,
			ComponentVersion: componentVersion,
			ArtefactName:     item.Artefact.Info.Name,
			ArtefactVersion:  item.Artefact.Info.Version,
			Severity:         item.Data.Severity,
		})
	}

	return comments, nil
}

// openInEditor writes the finding details to a temporary file and opens it in $EDITOR.
func (a *Application) openInEditor(findingCtx views.FindingContext) {
	finding := findingCtx.Finding
	artefact := findingCtx.Artefact

	matchingRules := finding.MatchingRules
	matchingRules = slices.DeleteFunc(matchingRules, func(rule string) bool {
		return rule == "original-severity"
	})

	dueDate := "N/A"
	if finding.DueDate != nil {
		dueDate = *finding.DueDate
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "CVE:                 %s\n", finding.Finding.CVE)
	fmt.Fprintf(&sb, "Package:             %s\n", finding.Finding.PackageName)
	fmt.Fprintf(&sb, "Component:           %s (%s)\n", artefact.ComponentName, artefact.ComponentVersion)
	fmt.Fprintf(&sb, "Artefact:            %s (%s)\n", artefact.Info.Name, artefact.Info.Version)
	fmt.Fprintf(&sb, "Original Severity:   %s\n", finding.Finding.Severity)
	fmt.Fprintf(&sb, "Suggested Severity:  %s\n", finding.Severity)
	fmt.Fprintf(&sb, "Discovery Date:      %s\n", finding.DiscoveryDate)
	fmt.Fprintf(&sb, "Due Date:            %s\n", dueDate)
	fmt.Fprintf(&sb, "Matching Rules:      %s\n", matchingRules)
	sb.WriteString("\n")
	sb.WriteString("Description:\n")
	sb.WriteString(finding.Finding.Summary)
	sb.WriteString("\n\n")

	comments, err := a.getCommentsForCVE(a.ctx, finding.Finding.CVE)
	if err != nil {
		fmt.Fprintf(&sb, "Error loading comments: %s\n", err.Error())
	} else if len(comments) == 0 {
		sb.WriteString("No rescoring comments for this CVE\n")
	} else {
		fmt.Fprintf(&sb, "Rescorings for %s in other components (%d total):\n\n", finding.Finding.CVE, len(comments))
		for _, comment := range comments {
			author, err := a.Clients.GHClient.ResolveUsername(a.ctx, comment.Author)
			if err != nil {
				author = comment.Author
			}
			fmt.Fprintf(&sb, "- Author:     %s\n", author)
			fmt.Fprintf(&sb, "  Created at: %s\n", comment.CreatedAt.Format(time.RFC1123))
			fmt.Fprintf(&sb, "  Component:  %s (%s)\n", comment.ComponentName, comment.ComponentVersion)
			fmt.Fprintf(&sb, "  Artefact:   %s@%s\n", comment.ArtefactName, comment.ArtefactVersion)
			fmt.Fprintf(&sb, "  Severity:   %s\n", comment.Severity)
			fmt.Fprintf(&sb, "  Comment:\n    %s\n\n", comment.Content)
		}
	}

	tmpFile, err := os.CreateTemp("", "cve-details-*.txt")
	if err != nil {
		go a.showErrorModal(fmt.Errorf("failed to create temp file: %w", err))
		return
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck

	if _, err := tmpFile.WriteString(sb.String()); err != nil {
		tmpFile.Close() //nolint:errcheck
		go a.showErrorModal(fmt.Errorf("failed to write temp file: %w", err))
		return
	}
	tmpFile.Close() //nolint:errcheck

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	var editorErr error
	a.TV.Suspend(func() {
		cmd := exec.Command(editor, tmpFile.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		editorErr = cmd.Run()
	})
	a.TV.ForceDraw()
	if editorErr != nil {
		go a.showErrorModal(fmt.Errorf("editor exited with error: %w", editorErr))
	}
}

func (a *Application) updateFilterStatus() {
	text := "filtering for user: "
	if a.filterForUser {
		username, err := a.Clients.GHClient.LoggedInUsername(a.ctx)
		if err != nil {
			text += "[red]error[white] (press 'f' to toggle)"
		} else {
			text += fmt.Sprintf("[green]%s[white] (press 'f' to toggle)", username)
		}
	} else {
		text += "[red]OFF[white] (press 'f' to toggle)"
	}
	a.StatusBar.SetMessage("filter", text)
}

func (a *Application) createMainUI() {
	cveTreeView := tview.NewTreeView()
	views.WrapWithFocusBorders(cveTreeView.Box)
	cveTreeView.SetBorderPadding(1, 1, 2, 2)
	cveTreeView.SetBorder(true).SetTitle("CVEs")

	// Clear details pane when scrolling
	var detailsTimer *time.Timer
	cveTreeView.SetChangedFunc(func(node *tview.TreeNode) {
		a.DetailsPane.Clear()

		// load new details data after debounce
		if detailsTimer != nil {
			detailsTimer.Stop()
		}
		detailsTimer = time.AfterFunc(150*time.Millisecond, func() {
			if findingCtx, ok := node.GetReference().(views.FindingContext); ok {
				a.showFindingDetails(findingCtx)
			}
		})
	})
	cveTreeView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'l':
				cveTreeView.SetCurrentNode(cveTreeView.GetCurrentNode().SetExpanded(true))
			case 'h':
				if cveTreeView.GetCurrentNode().IsExpanded() {
					cveTreeView.SetCurrentNode(cveTreeView.GetCurrentNode().SetExpanded(false))
					return nil
				}
				// if current node is already collapsed, move to parent
				cveTreeView.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
					if node == cveTreeView.GetCurrentNode() && parent != nil {
						parent.SetExpanded(false)
						return false // stop walking
					}
					return true // continue walking
				})
			case 'f':
				a.filterForUser = !a.filterForUser
				a.updateFilterStatus()
				a.loadCVEs(a.filterForUser)
				return event
			case 'e':
				node := cveTreeView.GetCurrentNode()
				if node == nil {
					return nil
				}
				if findingCtx, ok := node.GetReference().(views.FindingContext); ok {
					go a.openInEditor(findingCtx)
				}
				return nil
			default:
				return event

			}
		default:
			return event
		}

		return event
	})
	cveTreeView.SetSelectedFunc(func(node *tview.TreeNode) {
		// do nothing if node has no reference (e.g. it's the root node) or component node
		if node.GetReference() == nil {
			return
		}

		artifact, ok := node.GetReference().(odg.ArtefactEntry)
		if !ok {
			return
		}

		go a.loadArtifactDetails(artifact)

	})
	a.LoadingModal.SetOnHideFocus(cveTreeView)
	a.MainTreeView = cveTreeView
	a.updateFilterStatus()
	a.StatusBar.SetMessage("keybinds", "up/down or j/k: Navigate | l: Expand | h: Collapse | Enter: Load CVEs | e: Open in $EDITOR")

	a.DetailsPane = views.NewDetailsPane()

	flex := tview.NewFlex().SetDirection(tview.FlexColumnCSS).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRowCSS).
			AddItem(cveTreeView, 0, 1, false).
			AddItem(a.DetailsPane.Primitive(), 0, 3, false), 0, 1, false).
		AddItem(a.StatusBar.Primitive(), 3, 1, false)

	a.Pages.AddPage("root", flex, true, true)

	a.loadCVEs(a.filterForUser)

	if err := a.TV.SetRoot(a.Pages, true).SetFocus(cveTreeView).Run(); err != nil {
		panic(err)
	}
}

// filterSummary removes components and artefacts that have no vulnerability findings.
// Only artefacts with at least one finding/vulnerability entry that is not CLEAN or UNKNOWN are kept.
// Components with no remaining artefacts are removed entirely.
// TODO: inquire if there is a better way to filter the compliance summary (e.g. server-side filtering)
func filterSummary(summary *odg.ComplianceSummaryResponse) *odg.ComplianceSummaryResponse {
	filtered := &odg.ComplianceSummaryResponse{
		ComplianceSummary: []odg.ComplianceSummaryItem{},
	}
	for _, comp := range summary.ComplianceSummary {
		if !slices.ContainsFunc(comp.Entries, func(entry odg.Entry) bool {
			return entry.Type == "finding/vulnerability" && entry.Value > 0 && entry.Categorisation != "CLEAN" && entry.Categorisation != "UNKNOWN"
		}) {
			continue
		}

		filteredComp := odg.ComplianceSummaryItem{
			ComponentID: comp.ComponentID,
			Entries:     []odg.Entry{},
			Artefacts:   []odg.ArtefactEntry{},
		}
		for _, artifact := range comp.Artefacts {
			if !slices.ContainsFunc(artifact.Entries, func(entry odg.Entry) bool {
				return entry.Type == "finding/vulnerability" && entry.Value > 0 && entry.Categorisation != "CLEAN" && entry.Categorisation != "UNKNOWN"
			}) {
				continue
			}
			filteredComp.Artefacts = append(filteredComp.Artefacts, artifact)
		}

		if len(filteredComp.Artefacts) == 0 {
			continue
		}

		filtered.ComplianceSummary = append(filtered.ComplianceSummary, filteredComp)

	}

	return filtered
}

func (a *Application) loadCVEs(filterByUser bool) {
	go a.LoadingModal.Show("Loading Compliance Summary...")
	go func() {
		odgClient := a.Clients.ODGClient
		summary, err := odgClient.GetComplianceSummary(a.ctx, a.rootComponent, "greatest")
		if err != nil {
			a.TV.QueueUpdateDraw(func() {
				a.Pages.RemovePage("loading")
				a.Pages.RemovePage("loadingBG")
				errorModal := tview.NewModal().
					SetText("Error loading data: " + err.Error()).
					AddButtons([]string{"OK"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						a.Pages.RemovePage("error")
					})
				a.Pages.AddPage("error", errorModal, true, true)
			})
			return
		}
		filteredSummary := filterSummary(summary)
		a.TV.QueueUpdateDraw(func() {
			a.loadCVEsIntoTree(filteredSummary, filterByUser)
		})
		go a.LoadingModal.Hide()
	}()
}

// artefactLabel builds a display label for an artefact node.
// If the artefact has a non-empty extra ID, it is appended as key=value pairs.
func artefactLabel(info odg.ArtefactInfo) string {
	label := info.Name + "@" + info.Version
	if len(info.ExtraID) > 0 {
		// Sort keys for stable output
		keys := make([]string, 0, len(info.ExtraID))
		for k := range info.ExtraID {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, info.ExtraID[k]))
		}
		label += " (" + strings.Join(parts, ", ") + ")"
	}
	return label
}

func (a *Application) loadCVEsIntoTree(summary *odg.ComplianceSummaryResponse, filterByUser bool) {
	rootNode := views.CreateTreeNode(a.rootComponent, true)
	rootNode.ClearChildren()

	username, err := a.Clients.GHClient.LoggedInUsername(a.ctx)
	if err != nil {
		panic("Could not get logged in username: " + err.Error())
	}
	for _, comp := range summary.ComplianceSummary {
		responsibles, err := a.Clients.ODGClient.GetResponsibles(a.ctx, comp.ComponentID.Name, nil)
		if err != nil {
			// TODO: handle error properly, e.g. show in UI
			continue
		}

		// Only show components for which the user is responsible
		if filterByUser && !slices.ContainsFunc(responsibles, func(r odg.Responsible) bool { return r.Username == username }) {
			continue
		}

		// If this item is the root component itself, add its artefacts
		// directly to the root node instead of creating a redundant child.
		if comp.ComponentID.Name == a.rootComponent {
			for _, artefact := range comp.Artefacts {
				artefactNode := views.CreateTreeNode(artefactLabel(artefact.Artefact.Info), false).SetReference(artefact)
				rootNode.AddChild(artefactNode)
			}
			continue
		}

		compNode := views.CreateTreeNode(comp.ComponentID.Name+"@"+comp.ComponentID.Version, false)
		for _, artefact := range comp.Artefacts {
			artefactNode := views.CreateTreeNode(artefactLabel(artefact.Artefact.Info), false).SetReference(artefact)
			compNode.AddChild(artefactNode)
		}
		rootNode.AddChild(compNode)
	}
	cves := a.Pages.GetPage("root").(*tview.Flex).GetItem(0).(*tview.Flex).GetItem(0).(*tview.TreeView)
	cves.SetRoot(rootNode).SetCurrentNode(rootNode)
}

func (a *Application) doWithModal(action func() error, loadingText string, after time.Duration) error {
	done := make(chan struct{})
	var actionErr error
	go func() {
		actionErr = action()
		close(done)
	}()

	// Wait for either completion or timeout
	for {
		select {
		case <-done:
			// Completed within desired interval, no loading modal needed
			return actionErr
		case <-time.After(after):
			// Taking longer than desired interval, show loading modal
			a.LoadingModal.Show(loadingText)
			<-done // Wait for completion
			a.LoadingModal.Hide()
			return actionErr
		}
	}
}
