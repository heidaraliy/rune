package v2

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	termansi "github.com/charmbracelet/x/ansi"
	"github.com/heidaraliy/rune/internal/app/theme"
	"github.com/heidaraliy/rune/internal/application"
	"github.com/heidaraliy/rune/internal/domain"
	runesync "github.com/heidaraliy/rune/internal/sync"
)

// Service is retained as a local name for compatibility with existing TUI
// tests, but the contract is owned by the application package and shared with
// CLI, app, and future sync clients.
type Service = application.RuneClient

type view int

const (
	viewWorkspace view = iota
	viewRuns
	viewArtifacts
	viewSync
)

type inputMode int

const (
	inputNone inputMode = iota
	inputCapture
	inputSearch
	inputEditTitle
	inputEditBody
	inputDeleteConfirm
)

const statusTTL = 2500 * time.Millisecond

const syncTimeout = 35 * time.Second

type statusClearMsg struct {
	revision int
}

type syncFinishedMsg struct {
	report runesync.Report
	err    error
}

// Options configures optional capabilities for the structured TUI. The CLI
// owns construction and lifetime of the target; the model only consumes the
// abstract sync contract.
type Options struct {
	SyncTarget runesync.SyncTarget
	AutoSync   bool
}

type Model struct {
	service     Service
	workspaceID string
	project     string
	syncClient  application.SyncClient
	syncTarget  runesync.SyncTarget
	autoSync    bool
	syncBusy    bool
	syncError   string

	entities  []domain.Entity
	runs      []domain.Run
	artifacts []domain.Artifact
	links     []domain.Link
	events    []domain.RunEvent
	sync      domain.SyncStatus
	conflicts []domain.Conflict

	selected       int
	activeView     view
	inputMode      inputMode
	captureKind    domain.Kind
	input          textinput.Model
	editRevision   int64
	query          string
	kindFilter     domain.Kind
	help           bool
	width          int
	height         int
	status         string
	statusRevision int
	detailError    string
	artifactBody   string
}

func New(service Service, workspaceID, project string) (Model, error) {
	return NewWithOptions(service, workspaceID, project, Options{})
}

func NewWithOptions(service Service, workspaceID, project string, options Options) (Model, error) {
	if service == nil {
		return Model{}, errors.New("v2 TUI service is required")
	}
	if strings.TrimSpace(workspaceID) == "" {
		workspaceID = "local"
	}
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 4096
	syncClient, _ := service.(application.SyncClient)
	if options.SyncTarget != nil && syncClient == nil {
		return Model{}, errors.New("v2 TUI service does not support sync")
	}
	if options.AutoSync && options.SyncTarget == nil {
		return Model{}, errors.New("v2 TUI auto-sync requires a sync target")
	}
	m := Model{
		service:     service,
		workspaceID: workspaceID,
		project:     project,
		syncClient:  syncClient,
		syncTarget:  options.SyncTarget,
		autoSync:    options.AutoSync,
		input:       input,
	}
	if err := m.reload(); err != nil {
		return Model{}, err
	}
	m.syncBusy = m.autoSync
	return m, nil
}

func (m Model) Init() tea.Cmd {
	if m.syncBusy {
		return m.syncCmd()
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case statusClearMsg:
		if msg.revision == m.statusRevision {
			m.status = ""
		}
		return m, nil
	case syncFinishedMsg:
		return m.finishSync(msg)
	case tea.KeyMsg:
		if m.inputMode != inputNone {
			return m.updateInput(msg)
		}
		return m.updateNormal(msg)
	default:
		return m, nil
	}
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "Q":
		return m, tea.Quit
	case "j", "down":
		m.moveSelection(1)
	case "k", "up":
		m.moveSelection(-1)
	case "1":
		m.switchView(viewWorkspace)
	case "2":
		m.switchView(viewRuns)
	case "3":
		m.switchView(viewArtifacts)
	case "4":
		m.switchView(viewSync)
	case "r":
		if err := m.reload(); err != nil {
			return m.setStatus(err.Error())
		}
		return m.setStatus("Refreshed.")
	case "y":
		return m.startSync()
	case "f":
		return m.cycleKindFilter()
	case "/":
		m.inputMode = inputSearch
		m.input.Placeholder = "search workspace..."
		m.input.SetValue(m.query)
		m.input.CursorEnd()
		m.input.Focus()
	case "a":
		m.startCapture(domain.KindTask)
	case "n":
		m.startCapture(domain.KindNote)
	case "e":
		m.startEdit(inputEditTitle)
	case "E":
		m.startEdit(inputEditBody)
	case "s":
		return m.advanceSelectedState()
	case "d":
		m.startDelete()
	case "u":
		return m.restoreSelected()
	case "q":
		if m.activeView == viewWorkspace {
			return m.queueSelected()
		}
	case "x":
		if m.activeView == viewRuns {
			return m.executeSelected()
		}
	case "c":
		if m.activeView == viewRuns {
			return m.cancelSelected()
		}
	case "?":
		m.help = !m.help
	case "esc":
		m.help = false
		m.query = ""
		m.kindFilter = ""
		if err := m.reload(); err != nil {
			return m.setStatus(err.Error())
		}
	}
	return m, nil
}

func (m Model) startSync() (tea.Model, tea.Cmd) {
	if m.syncClient == nil {
		return m.setStatus("Sync is unavailable for this client.")
	}
	if m.syncTarget == nil {
		return m.setStatus("Sync is not configured. Start TUI with --remote.")
	}
	if m.syncBusy {
		return m.setStatus("Sync already in progress.")
	}
	m.syncBusy = true
	m.syncError = ""
	return m, m.syncCmd()
}

func (m Model) syncCmd() tea.Cmd {
	client := m.syncClient
	target := m.syncTarget
	return func() tea.Msg {
		if client == nil || target == nil {
			return syncFinishedMsg{err: errors.New("sync is not configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
		defer cancel()
		report, err := client.Sync(ctx, target)
		return syncFinishedMsg{report: report, err: err}
	}
}

func (m Model) finishSync(msg syncFinishedMsg) (tea.Model, tea.Cmd) {
	m.syncBusy = false
	if msg.err != nil {
		m.syncError = msg.err.Error()
		if err := m.reload(); err != nil {
			return m.setStatus("Sync offline: " + msg.err.Error() + "; refresh failed: " + err.Error())
		}
		return m.setStatus("Sync offline: " + msg.err.Error())
	}
	m.syncError = ""
	if err := m.reload(); err != nil {
		return m.setStatus("Synced, but refresh failed: " + err.Error())
	}
	message := fmt.Sprintf("Synced · pushed %d · pulled %d.", msg.report.Pushed, msg.report.Pulled)
	if m.sync.OpenConflicts > 0 {
		message += fmt.Sprintf(" %d conflict(s) need review.", m.sync.OpenConflicts)
	}
	return m.setStatus(message)
}

func (m Model) cycleKindFilter() (tea.Model, tea.Cmd) {
	if m.activeView != viewWorkspace {
		return m.setStatus("Kind filters are available from the workspace view.")
	}
	switch m.kindFilter {
	case "":
		m.kindFilter = domain.KindTask
	case domain.KindTask:
		m.kindFilter = domain.KindNote
	default:
		m.kindFilter = ""
	}
	m.selected = 0
	if err := m.reload(); err != nil {
		return m.setStatus(err.Error())
	}
	return m.setStatus("Filter: " + kindFilterLabel(m.kindFilter) + ".")
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.inputMode == inputDeleteConfirm {
		switch msg.String() {
		case "y", "Y":
			return m.confirmDelete()
		case "n", "N", "esc":
			m.inputMode = inputNone
			return m.setStatus("Delete canceled.")
		default:
			return m, nil
		}
	}
	if (m.inputMode == inputEditTitle || m.inputMode == inputEditBody) && msg.String() == "ctrl+a" {
		m.input.SetValue("")
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.inputMode = inputNone
		m.input.Blur()
		m.input.SetValue("")
		return m, nil
	case "enter":
		if m.inputMode == inputSearch {
			value := strings.TrimSpace(m.input.Value())
			m.query = value
			m.inputMode = inputNone
			m.input.Blur()
			if err := m.reload(); err != nil {
				return m.setStatus(err.Error())
			}
			return m.setStatus(searchStatus(value))
		}
		if m.inputMode == inputEditTitle || m.inputMode == inputEditBody {
			value := m.input.Value()
			if m.inputMode == inputEditTitle {
				value = strings.TrimSpace(value)
				if value == "" {
					return m.setStatus("Edit needs a title.")
				}
			}
			update := domain.Update{ExpectedRevision: m.editRevision}
			if m.inputMode == inputEditTitle {
				update.Title = &value
			} else {
				update.Body = &value
			}
			entity, err := m.service.Update(context.Background(), m.selectedID(), update)
			if err != nil {
				return m.setStatus(err.Error())
			}
			m.inputMode = inputNone
			m.input.Blur()
			m.input.SetValue("")
			if err := m.reloadKeeping(entity.ID); err != nil {
				return m.setStatus(err.Error())
			}
			return m.setStatus(fmt.Sprintf("Updated %s.", domain.DisplayID(entity.ID)))
		}
		value := strings.TrimSpace(m.input.Value())
		if value == "" {
			return m.setStatus("Capture needs a title.")
		}
		entity, err := m.service.Create(context.Background(), domain.Entity{
			Kind:        m.captureKind,
			WorkspaceID: m.workspaceID,
			Project:     m.project,
			Title:       value,
			Status:      captureStatus(m.captureKind),
		})
		if err != nil {
			return m.setStatus(err.Error())
		}
		m.inputMode = inputNone
		m.input.Blur()
		m.input.SetValue("")
		if err := m.reloadKeeping(entity.ID); err != nil {
			return m.setStatus(err.Error())
		}
		return m.setStatus(fmt.Sprintf("Captured %s %s.", entity.Kind, domain.DisplayID(entity.ID)))
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.inputMode == inputSearch {
		m.query = m.input.Value()
		if err := m.reload(); err != nil {
			return m.setStatus(err.Error())
		}
	}
	return m, cmd
}

func (m *Model) startCapture(kind domain.Kind) {
	m.captureKind = kind
	m.inputMode = inputCapture
	m.input.Placeholder = capturePlaceholder(kind)
	m.input.SetValue("")
	m.input.Focus()
	m.help = false
}

func (m *Model) startEdit(field inputMode) {
	entity := m.currentEntity()
	if entity == nil {
		m.setStatus("Select a note or task to edit.")
		return
	}
	if entity.DeletedAt != nil {
		m.setStatus("Restore the tombstone before editing.")
		return
	}
	m.inputMode = field
	m.editRevision = entity.Revision
	m.input.Prompt = "> "
	if field == inputEditBody {
		m.input.Placeholder = "edit body..."
		m.input.SetValue(entity.Body)
	} else {
		m.input.Placeholder = "edit title..."
		m.input.SetValue(entity.Title)
	}
	m.input.CursorEnd()
	m.input.Focus()
	m.help = false
}

func (m *Model) startDelete() {
	entity := m.currentEntity()
	if entity == nil {
		m.setStatus("Select a note or task to delete.")
		return
	}
	if entity.DeletedAt != nil {
		m.setStatus("Item is already deleted; press u to restore it.")
		return
	}
	m.inputMode = inputDeleteConfirm
	m.editRevision = entity.Revision
	m.input.Blur()
	m.help = false
}

func (m Model) advanceSelectedState() (tea.Model, tea.Cmd) {
	entity := m.currentEntity()
	if entity == nil {
		return m.setStatus("Select a note or task to change state.")
	}
	if entity.DeletedAt != nil {
		return m.setStatus("Restore the tombstone before changing state.")
	}
	state := entity.State
	if state == "" {
		state = domain.RuneStateFromStatus(entity.Status, entity.Kind)
	}
	next := domain.NextRuneState(state)
	updated, err := m.service.Update(context.Background(), entity.ID, domain.Update{ExpectedRevision: entity.Revision, State: &next})
	if err != nil {
		return m.setStatus(err.Error())
	}
	if err := m.reloadKeeping(updated.ID); err != nil {
		return m.setStatus(err.Error())
	}
	return m.setStatus(fmt.Sprintf("State: %s.", updated.State))
}

func (m Model) confirmDelete() (tea.Model, tea.Cmd) {
	entity := m.currentEntity()
	if entity == nil {
		return m.setStatus("Select a note or task to delete.")
	}
	deleted, err := m.service.Delete(context.Background(), entity.ID, m.editRevision)
	if err != nil {
		return m.setStatus(err.Error())
	}
	m.inputMode = inputNone
	if err := m.reloadKeeping(deleted.ID); err != nil {
		return m.setStatus(err.Error())
	}
	return m.setStatus("Tombstoned " + domain.DisplayID(deleted.ID) + ". Press u to restore.")
}

func (m Model) restoreSelected() (tea.Model, tea.Cmd) {
	if m.activeView != viewWorkspace {
		return m.setStatus("Restore is available from the workspace view.")
	}
	entity := m.currentEntity()
	if entity == nil {
		return m.setStatus("Select a tombstone to restore.")
	}
	if entity.DeletedAt == nil {
		return m.setStatus("Selected item is not deleted.")
	}
	restored, err := m.service.Restore(context.Background(), entity.ID, entity.Revision)
	if err != nil {
		return m.setStatus(err.Error())
	}
	if err := m.reloadKeeping(restored.ID); err != nil {
		return m.setStatus(err.Error())
	}
	return m.setStatus("Restored " + domain.DisplayID(restored.ID) + ".")
}

func (m *Model) switchView(next view) {
	if m.activeView == next {
		return
	}
	m.activeView = next
	m.help = false
	m.selected = 0
	m.detailError = ""
	m.artifactBody = ""
	_ = m.refreshDetail()
}

func (m *Model) moveSelection(delta int) {
	count := m.currentCount()
	if count == 0 {
		m.selected = 0
		return
	}
	m.selected = max(0, min(count-1, m.selected+delta))
	_ = m.refreshDetail()
}

func (m *Model) queueSelected() (tea.Model, tea.Cmd) {
	entity := m.currentEntity()
	if entity == nil {
		return m.setStatus("Select a task to queue.")
	}
	if !entity.IsTask() {
		return m.setStatus("Only tasks can be queued.")
	}
	if _, err := m.service.QueueRun(context.Background(), entity.ID, "fake", "local", domain.PermissionReadOnly); err != nil {
		return m.setStatus(err.Error())
	}
	if err := m.reloadKeeping(entity.ID); err != nil {
		return m.setStatus(err.Error())
	}
	return m.setStatus("Queued task for the local agent.")
}

func (m *Model) executeSelected() (tea.Model, tea.Cmd) {
	run := m.currentRun()
	if run == nil {
		return m.setStatus("Select a run to execute.")
	}
	if _, err := m.service.ExecuteRun(context.Background(), run.ID); err != nil {
		_ = m.reloadKeeping(run.ID)
		return m.setStatus(err.Error())
	}
	if err := m.reloadKeeping(run.ID); err != nil {
		return m.setStatus(err.Error())
	}
	return m.setStatus("Run completed and is ready for inspection.")
}

func (m *Model) cancelSelected() (tea.Model, tea.Cmd) {
	run := m.currentRun()
	if run == nil {
		return m.setStatus("Select a run to cancel.")
	}
	if _, err := m.service.CancelRun(context.Background(), run.ID); err != nil {
		return m.setStatus(err.Error())
	}
	if err := m.reloadKeeping(run.ID); err != nil {
		return m.setStatus(err.Error())
	}
	return m.setStatus("Run canceled.")
}

func (m *Model) reload() error {
	return m.reloadKeeping(m.selectedID())
}

func (m *Model) reloadKeeping(keepID string) error {
	entities, err := m.service.List(context.Background(), domain.ListOptions{
		Project:        m.project,
		Kind:           m.kindFilter,
		Query:          m.query,
		IncludeDeleted: true,
	})
	if err != nil {
		return fmt.Errorf("load v2 workspace: %w", err)
	}
	runs, err := m.service.Runs(context.Background(), domain.RunListOptions{})
	if err != nil {
		return fmt.Errorf("load v2 runs: %w", err)
	}
	artifacts, err := m.service.AllArtifacts(context.Background())
	if err != nil {
		return fmt.Errorf("load v2 artifacts: %w", err)
	}
	syncStatus, err := m.service.SyncStatus(context.Background())
	if err != nil {
		return fmt.Errorf("load v2 sync status: %w", err)
	}
	conflicts, err := m.service.Conflicts(context.Background())
	if err != nil {
		return fmt.Errorf("load v2 sync conflicts: %w", err)
	}
	if m.project != "" {
		projectTasks, err := m.service.List(context.Background(), domain.ListOptions{Project: m.project, Kind: domain.KindTask})
		if err != nil {
			return fmt.Errorf("load v2 project task scope: %w", err)
		}
		taskIDs := make(map[string]struct{}, len(projectTasks))
		for _, task := range projectTasks {
			taskIDs[task.ID] = struct{}{}
		}
		projectRuns := make([]domain.Run, 0, len(runs))
		runIDs := make(map[string]struct{}, len(runs))
		for _, run := range runs {
			if _, ok := taskIDs[run.TaskID]; ok {
				projectRuns = append(projectRuns, run)
				runIDs[run.ID] = struct{}{}
			}
		}
		runs = projectRuns
		projectArtifacts := make([]domain.Artifact, 0, len(artifacts))
		for _, artifact := range artifacts {
			if _, ok := runIDs[artifact.RunID]; ok {
				projectArtifacts = append(projectArtifacts, artifact)
			}
		}
		artifacts = projectArtifacts
	}
	m.entities = entities
	m.runs = runs
	m.artifacts = artifacts
	m.sync = syncStatus
	m.conflicts = conflicts
	m.selected = indexForID(m.currentIDs(), keepID)
	m.detailError = ""
	m.artifactBody = ""
	return m.refreshDetail()
}

func (m *Model) refreshDetail() error {
	m.links = nil
	m.events = nil
	m.detailError = ""
	m.artifactBody = ""
	switch m.activeView {
	case viewWorkspace:
		if entity := m.currentEntity(); entity != nil {
			links, err := m.service.Links(context.Background(), entity.ID)
			if err != nil {
				m.detailError = err.Error()
				return err
			}
			m.links = links
		}
	case viewRuns:
		if run := m.currentRun(); run != nil {
			events, err := m.service.RunEvents(context.Background(), run.ID)
			if err != nil {
				m.detailError = err.Error()
				return err
			}
			m.events = events
		}
	case viewArtifacts:
		if artifact := m.currentArtifact(); artifact != nil {
			_, content, err := m.service.ReadArtifact(context.Background(), artifact.ID)
			if err != nil {
				m.detailError = err.Error()
				return nil
			}
			m.artifactBody = string(content)
		}
	}
	return nil
}

func (m Model) setStatus(message string) (tea.Model, tea.Cmd) {
	if message == "" {
		m.status = ""
		return m, nil
	}
	m.status = message
	m.statusRevision++
	revision := m.statusRevision
	return m, tea.Tick(statusTTL, func(time.Time) tea.Msg { return statusClearMsg{revision: revision} })
}

func (m Model) View() string {
	if m.width <= 0 {
		return "Rune 2 is starting..."
	}
	height := m.height
	if height <= 0 {
		height = 24
	}
	headerLines := strings.Split(m.renderHeader(m.width), "\n")
	footerLines := strings.Split(m.renderFooter(m.width), "\n")
	bodyHeight := max(1, height-len(headerLines)-len(footerLines))
	bodyLines := strings.Split(m.renderBody(m.width, bodyHeight), "\n")
	lines := append(headerLines, bodyLines...)
	lines = append(lines, footerLines...)
	return fitScreen(lines, m.width, height)
}

func (m Model) renderHeader(width int) string {
	project := m.project
	if project == "" {
		project = "all projects"
	}
	openCount, doneCount := m.entityStats()
	meta := theme.TopMetaStyle.Render("  ·  ") + theme.TopLabelStyle.Render("workspace ") + theme.ProjectStyle.Render(m.workspaceID) + theme.TopMetaStyle.Render("  ·  project ") + theme.ProjectStyle.Render(project)
	if width < 60 {
		meta = theme.TopMetaStyle.Render("  ·  ") + theme.TopLabelStyle.Render("workspace ") + theme.ProjectStyle.Render(m.workspaceID) + theme.TopMetaStyle.Render("  ·  ") + theme.TodoStyle.Render(fmt.Sprintf("%d open", openCount))
	} else if width < 84 {
		meta = theme.TopMetaStyle.Render("  ·  ") + theme.TopLabelStyle.Render("workspace ") + theme.ProjectStyle.Render(m.workspaceID) + theme.TopMetaStyle.Render("  ·  ") + theme.ProjectStyle.Render(project) + theme.TopMetaStyle.Render("  ·  ") + theme.TodoStyle.Render(fmt.Sprintf("%d open", openCount))
	} else {
		meta += theme.TopMetaStyle.Render("  ·  ") + theme.TodoStyle.Render(fmt.Sprintf("%d open", openCount)) + theme.TopMetaStyle.Render("  ") + theme.DoneCountStyle.Render(fmt.Sprintf("%d done", doneCount))
	}
	tabs := []string{
		m.tabLabel("1 inbox", viewWorkspace),
		m.tabLabel("2 runs", viewRuns),
		m.tabLabel("3 artifacts", viewArtifacts),
		m.tabLabel("4 sync", viewSync),
	}
	innerWidth := max(1, width-2)
	if width < 80 {
		brand := theme.TopStyle.Render(" ") + theme.LogoStyle.Render("RUNE") + theme.TopMetaStyle.Render("  shared workspace")
		lines := []string{
			headerLine(innerWidth, brand, meta),
			theme.TopStyle.Render(" ") + strings.Join(tabs, theme.TopStyle.Render("  ")),
		}
		return renderStyledBoxString(width, 4, lines, theme.TopBoxStyle, theme.TopStyle)
	}
	lines := make([]string, 0, len(runeWordmark)+1)
	for index, wordmarkLine := range runeWordmark {
		right := ""
		if index == 0 {
			right = meta
		} else if index == 1 {
			right = theme.TopMetaStyle.Render("shared workspace")
		}
		lines = append(lines, headerLine(innerWidth, theme.TopStyle.Render(" ")+theme.LogoStyle.Render(wordmarkLine), right))
	}
	lines = append(lines, theme.TopStyle.Render(" ")+strings.Join(tabs, theme.TopStyle.Render("  ")))
	return renderStyledBoxString(width, len(lines)+2, lines, theme.TopBoxStyle, theme.TopStyle)
}

func (m Model) tabLabel(label string, tab view) string {
	if m.activeView == tab {
		return theme.SelectedStyle.Render(" " + label + " ")
	}
	return theme.TopMetaStyle.Render(label)
}

func headerLine(width int, left, right string) string {
	if width <= 0 {
		return ""
	}
	left = clipStyled(left, width)
	right = clipStyled(right, width)
	remaining := width - lipgloss.Width(left) - lipgloss.Width(right)
	if remaining < 1 {
		return clipStyled(left+" "+right, width)
	}
	return left + strings.Repeat(" ", remaining) + right
}

func (m Model) renderBody(width, height int) string {
	if m.help {
		lines := []string{
			theme.HeadingStyle.Render("Rune command guide"),
			"",
			theme.TopLabelStyle.Render("j/k or arrows") + "  move selection",
			theme.TopLabelStyle.Render("1-4") + "           switch inbox, runs, artifacts, sync",
			theme.TopLabelStyle.Render("y") + "             sync now (when a remote is configured)",
			theme.TopLabelStyle.Render("f") + "             cycle all, task, and note filters",
			theme.TopLabelStyle.Render("a") + "             capture a task",
			theme.TopLabelStyle.Render("n") + "             capture a note",
			theme.TopLabelStyle.Render("e/E") + "           edit title/body",
			theme.TopLabelStyle.Render("s") + "             advance selected Rune state",
			theme.TopLabelStyle.Render("d/u") + "           tombstone/restore",
			theme.TopLabelStyle.Render("/") + "             search workspace",
			theme.TopLabelStyle.Render("q") + "             queue selected task",
			theme.TopLabelStyle.Render("x/c") + "           execute/cancel run",
			theme.TopLabelStyle.Render("r") + "             refresh from SQLite",
			theme.TopLabelStyle.Render("Q / ctrl+c") + "   quit",
			"",
			theme.TopMetaStyle.Render("esc") + "           close this guide or clear search",
		}
		return strings.Join(renderStyledBox(width, height, lines, v2PanelStyle, theme.SurfaceStyle), "\n")
	}
	switch m.activeView {
	case viewWorkspace:
		return m.renderWorkspace(width, height)
	case viewRuns:
		return m.renderRuns(width, height)
	case viewArtifacts:
		return m.renderArtifacts(width, height)
	default:
		return m.renderSync(width, height)
	}
}

func (m Model) renderWorkspace(width, height int) string {
	if width < 64 {
		listHeight := compactListHeight(height)
		detailHeight := max(1, height-listHeight-1)
		return stackPanels(
			renderStyledBox(width, listHeight, m.renderStyledEntityList(max(1, width-2), true), v2PanelStyle, theme.SurfaceStyle),
			renderStyledBox(width, detailHeight, m.renderEntityDetail(max(1, width-2), detailHeight, true), v2PanelStyle, theme.SurfaceStyle),
			width,
			height,
		)
	}
	left, right := splitColumns(width)
	return joinColumns(
		renderStyledBox(left.width, height, m.renderStyledEntityList(left.content, false), v2PanelStyle, theme.SurfaceStyle),
		renderStyledBox(right.width, height, m.renderEntityDetail(right.content, height, width < 80), v2PanelStyle, theme.SurfaceStyle),
		left.width,
	)
}

func (m Model) renderRuns(width, height int) string {
	if width < 64 {
		listHeight := compactListHeight(height)
		detailHeight := max(1, height-listHeight-1)
		return stackPanels(
			renderStyledBox(width, listHeight, m.renderStyledRunList(max(1, width-2)), v2PanelStyle, theme.SurfaceStyle),
			renderStyledBox(width, detailHeight, m.renderRunDetail(max(1, width-2), detailHeight), v2PanelStyle, theme.SurfaceStyle),
			width,
			height,
		)
	}
	left, right := splitColumns(width)
	return joinColumns(
		renderStyledBox(left.width, height, m.renderStyledRunList(left.content), v2PanelStyle, theme.SurfaceStyle),
		renderStyledBox(right.width, height, m.renderRunDetail(right.content, height), v2PanelStyle, theme.SurfaceStyle),
		left.width,
	)
}

func (m Model) renderArtifacts(width, height int) string {
	if width < 64 {
		listHeight := compactListHeight(height)
		detailHeight := max(1, height-listHeight-1)
		return stackPanels(
			renderStyledBox(width, listHeight, m.renderStyledArtifactList(max(1, width-2)), v2PanelStyle, theme.SurfaceStyle),
			renderStyledBox(width, detailHeight, m.renderArtifactDetail(max(1, width-2), detailHeight), v2PanelStyle, theme.SurfaceStyle),
			width,
			height,
		)
	}
	left, right := splitColumns(width)
	return joinColumns(
		renderStyledBox(left.width, height, m.renderStyledArtifactList(left.content), v2PanelStyle, theme.SurfaceStyle),
		renderStyledBox(right.width, height, m.renderArtifactDetail(right.content, height), v2PanelStyle, theme.SurfaceStyle),
		left.width,
	)
}

func (m Model) renderSync(width, height int) string {
	if width < 64 {
		listHeight := compactListHeight(height)
		detailHeight := max(1, height-listHeight-1)
		return stackPanels(
			renderStyledBox(width, listHeight, m.renderStyledSyncList(max(1, width-2)), v2PanelStyle, theme.SurfaceStyle),
			renderStyledBox(width, detailHeight, m.renderSyncDetail(max(1, width-2), detailHeight), v2PanelStyle, theme.SurfaceStyle),
			width,
			height,
		)
	}
	left, right := splitColumns(width)
	return joinColumns(
		renderStyledBox(left.width, height, m.renderStyledSyncList(left.content), v2PanelStyle, theme.SurfaceStyle),
		renderStyledBox(right.width, height, m.renderSyncDetail(right.content, height), v2PanelStyle, theme.SurfaceStyle),
		left.width,
	)
}

func (m Model) renderStyledSyncList(width int) []string {
	remoteID := m.configuredRemoteID()
	remoteDisplay := truncate(remoteID, max(8, width/2))
	remoteState := m.sync.RemoteState
	if m.syncTarget != nil && remoteState == "not-configured" {
		remoteState = "configured"
	}
	connection := "not configured"
	if m.syncTarget != nil {
		connection = "ready"
	}
	if m.syncBusy {
		connection = "syncing"
	} else if m.syncError != "" {
		connection = "offline"
	}
	lines := []string{
		renderTwoColumnLine(width, theme.HeadingStyle.Render("LOCAL SYNC"), lifecycleBadge(connection)),
		renderRule(width),
		renderTwoColumnLine(width, theme.TopLabelStyle.Render("remote"), theme.ProjectStyle.Render(remoteState)),
		renderTwoColumnLine(width, theme.TopLabelStyle.Render("remote id"), theme.TopMetaStyle.Render(remoteDisplay)),
		renderTwoColumnLine(width, theme.TopLabelStyle.Render("local cursor"), fmt.Sprintf("%d", m.sync.LocalCursor)),
		renderTwoColumnLine(width, theme.TopLabelStyle.Render("pushed"), fmt.Sprintf("%d", m.sync.PushedCursor)),
		renderTwoColumnLine(width, theme.TopLabelStyle.Render("pulled"), fmt.Sprintf("%d", m.sync.PulledCursor)),
		renderTwoColumnLine(width, theme.TopLabelStyle.Render("pending"), fmt.Sprintf("%d", m.sync.PendingChanges)),
		renderTwoColumnLine(width, theme.TopLabelStyle.Render("conflicts"), fmt.Sprintf("%d", m.sync.OpenConflicts)),
		"",
		theme.TopMetaStyle.Render("Edits are revision-checked · SQLite authoritative offline"),
	}
	if m.syncTarget == nil {
		lines = append(lines, theme.TopMetaStyle.Render("Set --remote to connect a file or HTTP(S) peer."))
	} else {
		lines = append(lines, theme.TopMetaStyle.Render("Press y to sync now; --auto-sync runs once at startup."))
	}
	if m.syncError != "" {
		lines = append(lines, v2DangerStyle.Render("last error: "+m.syncError))
	}
	if len(m.conflicts) > 0 {
		lines = append(lines, "", theme.HeadingStyle.Render("CONFLICTS"))
		for index, conflict := range m.conflicts {
			marker := "  "
			if index == m.selected {
				marker = v2MarkerStyle.Render("▸ ")
			}
			row := marker + domain.DisplayID(conflict.ID) + "  " + conflict.Kind
			row = truncate(row, width)
			if index == m.selected {
				row = v2SelectedRowStyle.Render(padToWidth(clipStyled(row, width), width))
			}
			lines = append(lines, row)
		}
	}
	return lines
}

func (m Model) renderSyncDetail(width, height int) []string {
	conflict := m.currentConflict()
	if conflict == nil {
		return []string{theme.HeadingStyle.Render("Shared state"), theme.TopMetaStyle.Render("Select a conflict to inspect both payloads.")}
	}
	lines := []string{
		renderTwoColumnLine(width, v2DangerStyle.Render("CONFLICT · "+domain.DisplayID(conflict.ID)), lifecycleBadge(conflict.Status)),
		theme.TopMetaStyle.Render("entity " + domain.DisplayID(conflict.EntityID) + fmt.Sprintf("  ·  local %d / remote %d", conflict.LocalRevision, conflict.RemoteRevision)),
		renderRule(width),
		"",
		theme.TopLabelStyle.Render("LOCAL PAYLOAD"),
	}
	for _, line := range wrapPlainText(conflict.LocalPayload, width) {
		lines = append(lines, v2BodyStyle.Render(line))
	}
	lines = append(lines, "", theme.TopLabelStyle.Render("REMOTE PAYLOAD"))
	for _, line := range wrapPlainText(conflict.RemotePayload, width) {
		lines = append(lines, v2BodyStyle.Render(line))
	}
	return fitLines(lines, width, height)
}

func (m Model) configuredRemoteID() string {
	if m.syncTarget != nil && strings.TrimSpace(m.syncTarget.ID()) != "" {
		return m.syncTarget.ID()
	}
	if strings.TrimSpace(m.sync.RemoteID) != "" {
		return m.sync.RemoteID
	}
	return "none"
}

func (m Model) renderStyledEntityList(width int, compact bool) []string {
	if compact {
		heading := renderTwoColumnLine(width, theme.HeadingStyle.Render("YOUR RUNES"), theme.TopMetaStyle.Render(fmt.Sprintf("%d Runes", len(m.entities))))
		if len(m.entities) == 0 {
			return []string{heading, theme.TopMetaStyle.Render("Nothing here yet.")}
		}
		selected := max(0, min(len(m.entities)-1, m.selected))
		row := renderEntityRow(m.entities[selected], width, true)
		if len(row) > 0 {
			return []string{heading, row[0]}
		}
		return []string{heading}
	}
	lines := []string{
		theme.HeadingStyle.Render("WHAT SHOULD RUNE REMEMBER?"),
		theme.TopMetaStyle.Render("a task · n note · / search"),
		renderDashedRule(width),
		renderTwoColumnLine(width, theme.HeadingStyle.Render("YOUR RUNES"), theme.TopMetaStyle.Render(fmt.Sprintf("%d Runes", len(m.entities)))),
		theme.TopMetaStyle.Render("filter: " + kindFilterLabel(m.kindFilter)),
	}
	if strings.TrimSpace(m.query) != "" {
		lines = append(lines, theme.TopMetaStyle.Render("search / "+m.query))
	}
	lines = append(lines, renderRule(width))
	for index, entity := range m.entities {
		lines = append(lines, renderEntityRow(entity, width, index == m.selected)...)
		if index < len(m.entities)-1 {
			lines = append(lines, "")
		}
	}
	if len(m.entities) == 0 {
		lines = append(lines, "", theme.HeadingStyle.Render("Nothing here yet."), theme.TopMetaStyle.Render("Capture a note or task to give it shape."))
	}
	return lines
}

func renderEntityRow(entity domain.Entity, width int, selected bool) []string {
	marker := "  "
	if selected {
		marker = v2MarkerStyle.Render("▸ ")
	}
	titleStyle := v2TitleStyle
	if selected {
		titleStyle = titleStyle.Foreground(theme.CosmicViolet)
	}
	state := runeStateOrLegacy(entity)
	if entity.DeletedAt != nil {
		state = "deleted"
	}
	top := renderTwoColumnLine(width, marker+titleStyle.Render(entity.Title), theme.DimStyle.Render(domain.DisplayID(entity.ID)))
	snippet := v2MetaStyle.Render(compactPreview(entity.Body, "No body yet — open to add details.", width))
	meta := kindBadge(string(entity.Kind)) + " " + lifecycleBadge(state)
	if date := shortDate(entity.UpdatedAt); date != "" {
		meta += " " + theme.TopMetaStyle.Render(date)
	}
	rows := []string{top, snippet, meta}
	if selected {
		for index := range rows {
			rows[index] = v2SelectedRowStyle.Render(padToWidth(clipStyled(rows[index], width), width))
		}
	}
	return rows
}

func (m Model) renderEntityDetail(width, height int, compact bool) []string {
	entity := m.currentEntity()
	if entity == nil {
		return []string{theme.HeadingStyle.Render("Select a Rune"), theme.TopMetaStyle.Render("Capture a thought or choose one from the inbox.")}
	}
	state := runeStateOrLegacy(*entity)
	if entity.DeletedAt != nil {
		state = "deleted"
	}
	meta := "workspace note"
	if entity.Project != "" {
		meta = "project:" + entity.Project
	}
	if date := shortDate(entity.UpdatedAt); date != "" {
		meta += "  ·  " + date
	}
	lines := []string{
		renderTwoColumnLine(width, kindBadge(string(entity.Kind))+theme.TopMetaStyle.Render(" · "+domain.DisplayID(entity.ID)), theme.DimStyle.Render(fmt.Sprintf("rev %d", entity.Revision))),
		v2TitleStyle.Render(entity.Title),
		theme.TopMetaStyle.Render(meta),
		renderRule(width),
		renderTwoColumnLine(width, theme.TopLabelStyle.Render("Lifecycle"), lifecycleBadge(state)),
	}
	if entity.DeletedAt != nil {
		lines = append(lines, v2DangerStyle.Render("reversible tombstone"))
	}
	if compact {
		lines = append(lines[:3], lines[4:]...)
		if entity.Body != "" {
			bodyLines := wrapPlainText(entity.Body, width)
			if len(bodyLines) > 0 {
				lines = append(lines, v2BodyStyle.Render(bodyLines[0]))
			}
		} else {
			lines = append(lines, theme.TopMetaStyle.Render("No body yet — press E to add details."))
		}
		for _, link := range m.links {
			other := link.ToID
			if other == entity.ID {
				other = link.FromID
			}
			lines = append(lines, theme.TopLabelStyle.Render("link: ")+theme.ProjectStyle.Render(link.Kind)+theme.TopMetaStyle.Render(" → "+domain.DisplayID(other)))
		}
		properties := runePropertyLines(*entity)
		if len(properties) > 1 {
			for _, property := range properties[1:] {
				lines = append(lines, theme.TopMetaStyle.Render(property))
			}
		}
		return fitLines(lines, width, height)
	}
	lines = append(lines, "", theme.TopLabelStyle.Render("Body"))
	if entity.Body != "" {
		for _, line := range wrapPlainText(entity.Body, width) {
			lines = append(lines, v2BodyStyle.Render(line))
		}
	} else {
		lines = append(lines, theme.TopMetaStyle.Render("No body yet — press E to add details."))
	}
	if len(m.links) > 0 {
		lines = append(lines, "", theme.TopLabelStyle.Render("Connections"))
		for _, link := range m.links {
			other := link.ToID
			if other == entity.ID {
				other = link.FromID
			}
			lines = append(lines, theme.ProjectStyle.Render(link.Kind)+theme.TopMetaStyle.Render(" → "+domain.DisplayID(other)))
		}
	}
	properties := runePropertyLines(*entity)
	if len(properties) > 0 {
		lines = append(lines, "", theme.TopLabelStyle.Render("Properties"))
		for _, property := range properties[1:] {
			lines = append(lines, theme.TopMetaStyle.Render(property))
		}
	}
	if m.detailError != "" {
		lines = append(lines, "", v2DangerStyle.Render("Error: "+m.detailError))
	}
	return fitLines(lines, width, height)
}

func (m Model) renderStyledRunList(width int) []string {
	lines := []string{
		renderTwoColumnLine(width, theme.HeadingStyle.Render("EXECUTION"), theme.TopMetaStyle.Render(fmt.Sprintf("%d attempts", len(m.runs)))),
		theme.TopMetaStyle.Render("local agent activity"),
		renderRule(width),
	}
	for index, run := range m.runs {
		status := lifecycleBadge(string(run.Status))
		top := renderTwoColumnLine(width, theme.ProjectStyle.Render("run "+domain.DisplayID(run.ID)), status)
		meta := theme.TopMetaStyle.Render("task " + domain.DisplayID(run.TaskID) + " · " + run.Provider + " / " + run.Model)
		rows := []string{top, meta}
		if index == m.selected {
			for row := range rows {
				rows[row] = v2SelectedRowStyle.Render(padToWidth(clipStyled(rows[row], width), width))
			}
		}
		lines = append(lines, rows...)
		if index < len(m.runs)-1 {
			lines = append(lines, "")
		}
	}
	if len(m.runs) == 0 {
		lines = append(lines, "", theme.HeadingStyle.Render("No agent runs yet."), theme.TopMetaStyle.Render("Queue a task with q."))
	}
	return lines
}

func (m Model) renderRunDetail(width, height int) []string {
	run := m.currentRun()
	if run == nil {
		return []string{theme.HeadingStyle.Render("Select a run"), theme.TopMetaStyle.Render("Queue a task to create an execution attempt.")}
	}
	lines := []string{
		renderTwoColumnLine(width, theme.HeadingStyle.Render("RUN · "+domain.DisplayID(run.ID)), theme.DimStyle.Render(fmt.Sprintf("rev %d", run.Revision))),
		renderTwoColumnLine(width, theme.TopLabelStyle.Render("Status"), lifecycleBadge(string(run.Status))),
		theme.TopMetaStyle.Render("task " + domain.DisplayID(run.TaskID) + "  ·  " + run.Provider + " / " + run.Model),
		theme.TopMetaStyle.Render("permission: " + string(run.PermissionPolicy)),
		renderRule(width),
	}
	if run.Summary != "" {
		lines = append(lines, "", theme.TopLabelStyle.Render("Summary"))
		for _, line := range wrapPlainText(run.Summary, width) {
			lines = append(lines, v2BodyStyle.Render(line))
		}
	}
	if run.Error != "" {
		lines = append(lines, "", v2DangerStyle.Render("Error"))
		for _, line := range wrapPlainText(run.Error, width) {
			lines = append(lines, v2DangerStyle.Render(line))
		}
	}
	if run.ContextArtifactID != "" {
		lines = append(lines, "", theme.TopLabelStyle.Render("Context artifact"), theme.ProjectStyle.Render(domain.DisplayID(run.ContextArtifactID)))
	}
	if len(m.events) > 0 {
		lines = append(lines, "", theme.TopLabelStyle.Render("Events"))
		for _, event := range m.events {
			lines = append(lines, theme.TopMetaStyle.Render(fmt.Sprintf("%d  %-8s %s", event.Sequence, event.Kind, event.Payload)))
		}
	}
	return fitLines(lines, width, height)
}

func (m Model) renderStyledArtifactList(width int) []string {
	lines := []string{
		renderTwoColumnLine(width, theme.HeadingStyle.Render("ARTIFACTS"), theme.TopMetaStyle.Render(fmt.Sprintf("%d files", len(m.artifacts)))),
		theme.TopMetaStyle.Render("durable run outputs"),
		renderRule(width),
	}
	for index, artifact := range m.artifacts {
		top := renderTwoColumnLine(width, theme.ProjectStyle.Render(artifact.Name), theme.DimStyle.Render(domain.DisplayID(artifact.ID)))
		meta := theme.TopMetaStyle.Render(artifact.Kind + " · " + formatBytes(artifact.SizeBytes))
		rows := []string{top, meta}
		if index == m.selected {
			for row := range rows {
				rows[row] = v2SelectedRowStyle.Render(padToWidth(clipStyled(rows[row], width), width))
			}
		}
		lines = append(lines, rows...)
		if index < len(m.artifacts)-1 {
			lines = append(lines, "")
		}
	}
	if len(m.artifacts) == 0 {
		lines = append(lines, "", theme.HeadingStyle.Render("No artifacts yet."), theme.TopMetaStyle.Render("Completed runs will leave durable outputs here."))
	}
	return lines
}

func formatBytes(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	if value < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(value)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(value)/(1024*1024))
}

func (m Model) renderArtifactDetail(width, height int) []string {
	artifact := m.currentArtifact()
	if artifact == nil {
		return []string{theme.HeadingStyle.Render("Select an artifact"), theme.TopMetaStyle.Render("Run outputs and durable files appear here.")}
	}
	lines := []string{
		renderTwoColumnLine(width, theme.HeadingStyle.Render("ARTIFACT · "+domain.DisplayID(artifact.ID)), theme.DimStyle.Render(fmt.Sprintf("rev %d", artifact.Revision))),
		v2TitleStyle.Render(artifact.Name),
		theme.TopMetaStyle.Render(artifact.Kind + "  ·  " + artifact.MediaType + "  ·  " + formatBytes(artifact.SizeBytes)),
		renderRule(width),
		theme.TopLabelStyle.Render("sha256") + theme.TopMetaStyle.Render(" "+artifact.SHA256),
		theme.TopLabelStyle.Render("retention") + theme.TopMetaStyle.Render(" "+artifact.Retention+"  ·  secret "+artifact.SecretState),
	}
	if artifact.RunID != "" {
		lines = append(lines, theme.TopLabelStyle.Render("run")+theme.TopMetaStyle.Render(" "+domain.DisplayID(artifact.RunID)))
	}
	if m.artifactBody != "" {
		lines = append(lines, "", theme.TopLabelStyle.Render("Content"))
		for _, line := range wrapPlainText(m.artifactBody, width) {
			lines = append(lines, v2BodyStyle.Render(line))
		}
	}
	if m.detailError != "" {
		lines = append(lines, "", v2DangerStyle.Render("Content unavailable: "+m.detailError))
	}
	return fitLines(lines, width, height)
}

func (m Model) renderFooter(width int) string {
	var text string
	if m.inputMode == inputCapture {
		text = m.input.View() + "  enter capture · esc cancel"
	} else if m.inputMode == inputSearch {
		text = m.input.View() + "  enter search · esc cancel"
	} else if m.inputMode == inputEditTitle {
		text = m.input.View() + "  enter save title · esc cancel"
	} else if m.inputMode == inputEditBody {
		text = m.input.View() + "  enter save body · esc cancel"
	} else if m.inputMode == inputDeleteConfirm {
		text = "Tombstone selected item? y confirm · n/esc cancel"
	} else if m.status != "" {
		return renderStyledBoxString(width, 3, []string{theme.StatusStyle.Render(" " + m.status)}, theme.FooterBoxStyle.Background(theme.CosmicViolet), theme.StatusStyle)
	} else {
		text = "j/k move · 1-4 views · a/n capture · e/E edit · s state · d/u tombstone/restore · / search · ? commands · Q quit"
		if m.activeView == viewWorkspace {
			text += " · q queue"
		}
		if m.activeView == viewRuns {
			text += " · x execute · c cancel"
		}
	}
	if m.inputMode == inputNone && width < 64 {
		text = "j/k · 1-4 · a/n · e/E edit · d/u tomb/restore · ? commands · Q quit"
		return renderStyledBoxString(width, 3, []string{theme.FooterTextStyle.Render(" " + truncate(text, max(1, width-2)))}, theme.FooterBoxStyle, theme.FooterBarStyle)
	}
	lines := []string{text}
	if m.inputMode == inputNone {
		lines = wrapPlainText(text, max(1, width-4))
	}
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, theme.FooterTextStyle.Render(" "+line))
	}
	return renderStyledBoxString(width, len(rendered)+2, rendered, theme.FooterBoxStyle, theme.FooterBarStyle)
}

func (m Model) currentCount() int {
	switch m.activeView {
	case viewWorkspace:
		return len(m.entities)
	case viewRuns:
		return len(m.runs)
	case viewArtifacts:
		return len(m.artifacts)
	case viewSync:
		return len(m.conflicts)
	default:
		return 0
	}
}

func (m Model) entityStats() (open, done int) {
	for _, entity := range m.entities {
		state := runeStateOrLegacy(entity)
		if state == string(domain.StateComplete) || entity.Status == domain.StatusCompleted {
			done++
			continue
		}
		open++
	}
	return open, done
}

func (m Model) selectedID() string {
	ids := m.currentIDs()
	if m.selected < 0 || m.selected >= len(ids) {
		return ""
	}
	return ids[m.selected]
}

func (m Model) currentIDs() []string {
	switch m.activeView {
	case viewWorkspace:
		ids := make([]string, len(m.entities))
		for i := range m.entities {
			ids[i] = m.entities[i].ID
		}
		return ids
	case viewRuns:
		ids := make([]string, len(m.runs))
		for i := range m.runs {
			ids[i] = m.runs[i].ID
		}
		return ids
	case viewArtifacts:
		ids := make([]string, len(m.artifacts))
		for i := range m.artifacts {
			ids[i] = m.artifacts[i].ID
		}
		return ids
	case viewSync:
		ids := make([]string, len(m.conflicts))
		for i := range m.conflicts {
			ids[i] = m.conflicts[i].ID
		}
		return ids
	default:
		return nil
	}
}

func (m Model) currentEntity() *domain.Entity {
	if m.activeView != viewWorkspace || m.selected < 0 || m.selected >= len(m.entities) {
		return nil
	}
	return &m.entities[m.selected]
}

func (m Model) currentRun() *domain.Run {
	if m.activeView != viewRuns || m.selected < 0 || m.selected >= len(m.runs) {
		return nil
	}
	return &m.runs[m.selected]
}

func (m Model) currentArtifact() *domain.Artifact {
	if m.activeView != viewArtifacts || m.selected < 0 || m.selected >= len(m.artifacts) {
		return nil
	}
	return &m.artifacts[m.selected]
}

func (m Model) currentConflict() *domain.Conflict {
	if m.activeView != viewSync || m.selected < 0 || m.selected >= len(m.conflicts) {
		return nil
	}
	return &m.conflicts[m.selected]
}

func runeStateOrLegacy(entity domain.Entity) string {
	if entity.State != "" {
		return string(entity.State)
	}
	return string(domain.RuneStateFromStatus(entity.Status, entity.Kind))
}

func runePropertyLines(entity domain.Entity) []string {
	lines := make([]string, 0, len(entity.Properties)+len(entity.FacetProperties))
	keys := make([]string, 0, len(entity.Properties))
	for key := range entity.Properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("property: %s=%s", key, entity.Properties[key]))
	}
	facetNames := make([]string, 0, len(entity.FacetProperties))
	for facet := range entity.FacetProperties {
		facetNames = append(facetNames, string(facet))
	}
	sort.Strings(facetNames)
	for _, facetName := range facetNames {
		values := entity.FacetProperties[domain.RuneFacet(facetName)]
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("property: %s.%s=%s", facetName, key, values[key]))
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return append([]string{"properties:"}, lines...)
}

func captureStatus(kind domain.Kind) domain.Status {
	if kind == domain.KindTask {
		return domain.StatusDraft
	}
	return ""
}

func capturePlaceholder(kind domain.Kind) string {
	if kind == domain.KindNote {
		return "capture a note..."
	}
	return "capture a task..."
}

func searchStatus(query string) string {
	if query == "" {
		return "Showing all workspace entities."
	}
	return "Search: " + query
}

func kindFilterLabel(kind domain.Kind) string {
	switch kind {
	case domain.KindTask:
		return "tasks"
	case domain.KindNote:
		return "notes"
	default:
		return "all"
	}
}

func indexForID(ids []string, id string) int {
	if id != "" {
		for index, candidate := range ids {
			if candidate == id {
				return index
			}
		}
	}
	return 0
}

type columnLayout struct {
	width   int
	content int
}

func splitColumns(width int) (columnLayout, columnLayout) {
	if width < 64 {
		return columnLayout{width: width, content: width}, columnLayout{width: 0, content: 0}
	}
	left := max(32, width*38/100)
	right := max(1, width-left-1)
	return columnLayout{width: left, content: max(1, left-2)}, columnLayout{width: right, content: max(1, right-2)}
}

func compactListHeight(height int) int {
	return max(1, min(4, height/3))
}

func joinColumns(left []string, right []string, leftWidth int) string {
	if leftWidth <= 0 {
		return strings.Join(left, "\n")
	}
	rows := max(len(left), len(right))
	lines := make([]string, 0, rows)
	for index := 0; index < rows; index++ {
		leftLine := ""
		if index < len(left) {
			leftLine = left[index]
		}
		rightLine := ""
		if index < len(right) {
			rightLine = right[index]
		}
		lines = append(lines, padToWidth(leftLine, leftWidth)+" "+rightLine)
	}
	return strings.Join(lines, "\n")
}

func stackPanels(first, second []string, width, height int) string {
	lines := make([]string, 0, len(first)+len(second)+1)
	lines = append(lines, first...)
	if len(first) > 0 && len(second) > 0 {
		lines = append(lines, theme.SurfaceStyle.Render(" "))
	}
	lines = append(lines, second...)
	return strings.Join(fitLines(lines, width, height), "\n")
}

func fitLines(lines []string, width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	out := make([]string, 0, min(height, len(lines)))
	for _, line := range lines {
		out = append(out, truncate(line, width))
		if len(out) == height {
			return out
		}
	}
	for len(out) < height {
		out = append(out, "")
	}
	return out
}

func fitScreen(lines []string, width, height int) string {
	return strings.Join(fitLines(lines, width, height), "\n")
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return termansi.Truncate(value, width, "")
	}
	return termansi.Truncate(value, width, "…")
}

func padToWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = truncate(value, width)
	missing := width - lipgloss.Width(value)
	if missing > 0 {
		return value + strings.Repeat(" ", missing)
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
