package v2

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	termansi "github.com/charmbracelet/x/ansi"
	"github.com/heidaraliy/rune/internal/domain"
)

type Service interface {
	Create(context.Context, domain.Entity) (domain.Entity, error)
	Get(context.Context, string) (domain.Entity, error)
	List(context.Context, domain.ListOptions) ([]domain.Entity, error)
	Links(context.Context, string) ([]domain.Link, error)
	QueueRun(context.Context, string, string, string, domain.PermissionPolicy) (domain.Run, error)
	ExecuteRun(context.Context, string) (domain.Run, error)
	CancelRun(context.Context, string) (domain.Run, error)
	Runs(context.Context, domain.RunListOptions) ([]domain.Run, error)
	RunEvents(context.Context, string) ([]domain.RunEvent, error)
	Artifacts(context.Context, string) ([]domain.Artifact, error)
	AllArtifacts(context.Context) ([]domain.Artifact, error)
	ReadArtifact(context.Context, string) (domain.Artifact, []byte, error)
}

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
)

const statusTTL = 2500 * time.Millisecond

type statusClearMsg struct {
	revision int
}

type Model struct {
	service     Service
	workspaceID string
	project     string

	entities  []domain.Entity
	runs      []domain.Run
	artifacts []domain.Artifact
	links     []domain.Link
	events    []domain.RunEvent

	selected       int
	activeView     view
	inputMode      inputMode
	captureKind    domain.Kind
	input          textinput.Model
	query          string
	help           bool
	width          int
	height         int
	status         string
	statusRevision int
	detailError    string
	artifactBody   string
}

func New(service Service, workspaceID, project string) (Model, error) {
	if service == nil {
		return Model{}, errors.New("v2 TUI service is required")
	}
	if strings.TrimSpace(workspaceID) == "" {
		workspaceID = "local"
	}
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 4096
	m := Model{
		service:     service,
		workspaceID: workspaceID,
		project:     project,
		input:       input,
	}
	if err := m.reload(); err != nil {
		return Model{}, err
	}
	return m, nil
}

func (m Model) Init() tea.Cmd {
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
		if err := m.reload(); err != nil {
			return m.setStatus(err.Error())
		}
	}
	return m, nil
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = inputNone
		m.input.Blur()
		m.input.SetValue("")
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.input.Value())
		if m.inputMode == inputSearch {
			m.query = value
			m.inputMode = inputNone
			m.input.Blur()
			if err := m.reload(); err != nil {
				return m.setStatus(err.Error())
			}
			return m.setStatus(searchStatus(value))
		}
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
	entities, err := m.service.List(context.Background(), domain.ListOptions{Project: m.project, Query: m.query})
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
	header := m.renderHeader(m.width)
	footer := m.renderFooter(m.width)
	bodyHeight := max(1, height-3)
	body := m.renderBody(m.width, bodyHeight)
	lines := append(strings.Split(header, "\n"), strings.Split(body, "\n")...)
	lines = append(lines, footer)
	return fitScreen(lines, m.width, height)
}

func (m Model) renderHeader(width int) string {
	project := m.project
	if project == "" {
		project = "all projects"
	}
	line := fmt.Sprintf("Rune 2  ·  workspace:%s  ·  project:%s", m.workspaceID, project)
	tabs := []string{
		m.tabLabel("1 workspace", viewWorkspace),
		m.tabLabel("2 runs", viewRuns),
		m.tabLabel("3 artifacts", viewArtifacts),
		m.tabLabel("4 sync", viewSync),
	}
	return strings.Join([]string{truncate(line, width), truncate(strings.Join(tabs, "  "), width)}, "\n")
}

func (m Model) tabLabel(label string, tab view) string {
	if m.activeView == tab {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("[" + label + "]")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(label)
}

func (m Model) renderBody(width, height int) string {
	if m.help {
		return strings.Join(fitLines([]string{
			"Rune 2 keyboard guide",
			"",
			"j/k or arrows  move selection",
			"1-4           switch workspace, runs, artifacts, sync",
			"a             capture a task",
			"n             capture a note",
			"/             search workspace",
			"q             queue selected task",
			"x             execute selected queued run",
			"c             cancel selected run",
			"r             refresh from SQLite",
			"Q / ctrl+c    quit",
			"",
			"esc           close this guide or clear search",
		}, width, height), "\n")
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
		return stackSections(m.renderEntityList(), m.renderEntityDetail(width, height), width, height)
	}
	left, right := splitColumns(width)
	return joinColumns(m.renderEntityList(), m.renderEntityDetail(right.width, height), left.width)
}

func (m Model) renderRuns(width, height int) string {
	if width < 64 {
		return stackSections(m.renderRunList(), m.renderRunDetail(width, height), width, height)
	}
	left, right := splitColumns(width)
	return joinColumns(m.renderRunList(), m.renderRunDetail(right.width, height), left.width)
}

func (m Model) renderArtifacts(width, height int) string {
	if width < 64 {
		return stackSections(m.renderArtifactList(), m.renderArtifactDetail(width, height), width, height)
	}
	left, right := splitColumns(width)
	return joinColumns(m.renderArtifactList(), m.renderArtifactDetail(right.width, height), left.width)
}

func (m Model) renderSync(width, height int) string {
	return strings.Join(fitLines([]string{
		"LOCAL WORKSPACE",
		"",
		"Sync is not connected in Slice 4.",
		"",
		"SQLite is authoritative while offline.",
		"Revision-aware sync and visible conflicts arrive in Slice 5.",
		"",
		"No remote state is being implied or overwritten.",
	}, width, height), "\n")
}

func (m Model) renderEntityList() []string {
	lines := []string{fmt.Sprintf("WORKSPACE  %d entities", len(m.entities))}
	if strings.TrimSpace(m.query) != "" {
		lines = append(lines, "search: "+m.query)
	}
	for index, entity := range m.entities {
		marker := " "
		if index == m.selected {
			marker = ">"
		}
		kind := "note"
		status := "     "
		if entity.IsTask() {
			kind = "task"
			status = string(entity.Status)
		}
		line := fmt.Sprintf("%s %-4s %-9s %-8s %s", marker, domain.DisplayID(entity.ID), kind, status, entity.Title)
		lines = append(lines, line)
	}
	if len(m.entities) == 0 {
		lines = append(lines, "", "No notes or tasks in this view.")
	}
	return lines
}

func (m Model) renderEntityDetail(width, height int) []string {
	entity := m.currentEntity()
	if entity == nil {
		return fitLines([]string{"Select a note or task."}, width, height)
	}
	lines := []string{
		strings.ToUpper(string(entity.Kind)) + "  " + entity.ID,
		"status: " + statusOrNote(*entity),
		fmt.Sprintf("revision: %d", entity.Revision),
	}
	if entity.Project != "" {
		lines = append(lines, "project: "+entity.Project)
	}
	lines = append(lines, "", entity.Title)
	if entity.Body != "" {
		lines = append(lines, "", "Details")
		lines = append(lines, strings.Split(entity.Body, "\n")...)
	}
	if len(m.links) > 0 {
		lines = append(lines, "", "Links")
		for _, link := range m.links {
			other := link.ToID
			if other == entity.ID {
				other = link.FromID
			}
			lines = append(lines, fmt.Sprintf("%s -> %s", link.Kind, domain.DisplayID(other)))
		}
	}
	if m.detailError != "" {
		lines = append(lines, "", "Error: "+m.detailError)
	}
	return fitLines(lines, width, height)
}

func (m Model) renderRunList() []string {
	lines := []string{fmt.Sprintf("RUNS  %d attempts", len(m.runs))}
	for index, run := range m.runs {
		marker := " "
		if index == m.selected {
			marker = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %-8s %-9s task %s %s", marker, domain.DisplayID(run.ID), run.Status, domain.DisplayID(run.TaskID), run.Provider))
	}
	if len(m.runs) == 0 {
		lines = append(lines, "", "No runs yet. Queue a task with q.")
	}
	return lines
}

func (m Model) renderRunDetail(width, height int) []string {
	run := m.currentRun()
	if run == nil {
		return fitLines([]string{"Select a run."}, width, height)
	}
	lines := []string{
		"RUN  " + run.ID,
		"status: " + string(run.Status),
		"task: " + domain.DisplayID(run.TaskID),
		"provider: " + run.Provider + "  model: " + run.Model,
		"permission: " + string(run.PermissionPolicy),
		fmt.Sprintf("revision: %d", run.Revision),
	}
	if run.Summary != "" {
		lines = append(lines, "", "Summary", run.Summary)
	}
	if run.Error != "" {
		lines = append(lines, "", "Error", run.Error)
	}
	if run.ContextArtifactID != "" {
		lines = append(lines, "", "Context artifact", domain.DisplayID(run.ContextArtifactID))
	}
	if len(m.events) > 0 {
		lines = append(lines, "", "Events")
		for _, event := range m.events {
			lines = append(lines, fmt.Sprintf("%d  %-8s %s", event.Sequence, event.Kind, event.Payload))
		}
	}
	return fitLines(lines, width, height)
}

func (m Model) renderArtifactList() []string {
	lines := []string{fmt.Sprintf("ARTIFACTS  %d files", len(m.artifacts))}
	for index, artifact := range m.artifacts {
		marker := " "
		if index == m.selected {
			marker = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %-8s %-9s %-18s %dB", marker, domain.DisplayID(artifact.ID), artifact.Kind, artifact.Name, artifact.SizeBytes))
	}
	if len(m.artifacts) == 0 {
		lines = append(lines, "", "No artifacts yet.")
	}
	return lines
}

func (m Model) renderArtifactDetail(width, height int) []string {
	artifact := m.currentArtifact()
	if artifact == nil {
		return fitLines([]string{"Select an artifact."}, width, height)
	}
	lines := []string{
		"ARTIFACT  " + artifact.ID,
		"kind: " + artifact.Kind + "  name: " + artifact.Name,
		"type: " + artifact.MediaType,
		fmt.Sprintf("size: %d bytes  revision: %d", artifact.SizeBytes, artifact.Revision),
		"sha256: " + artifact.SHA256,
		"retention: " + artifact.Retention + "  secret: " + artifact.SecretState,
	}
	if artifact.RunID != "" {
		lines = append(lines, "run: "+domain.DisplayID(artifact.RunID))
	}
	if m.artifactBody != "" {
		lines = append(lines, "", "Content")
		lines = append(lines, strings.Split(m.artifactBody, "\n")...)
	}
	if m.detailError != "" {
		lines = append(lines, "", "Content unavailable: "+m.detailError)
	}
	return fitLines(lines, width, height)
}

func (m Model) renderFooter(width int) string {
	var text string
	if m.inputMode == inputCapture {
		text = m.input.View() + "  enter capture · esc cancel"
	} else if m.inputMode == inputSearch {
		text = m.input.View() + "  enter search · esc cancel"
	} else if m.status != "" {
		text = m.status
	} else {
		text = "j/k move · 1-4 views · a task · n note · / search · ? help · Q quit"
		if m.activeView == viewWorkspace {
			text += " · q queue"
		}
		if m.activeView == viewRuns {
			text += " · x execute · c cancel"
		}
	}
	return truncate(text, width)
}

func (m Model) currentCount() int {
	switch m.activeView {
	case viewWorkspace:
		return len(m.entities)
	case viewRuns:
		return len(m.runs)
	case viewArtifacts:
		return len(m.artifacts)
	default:
		return 0
	}
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

func statusOrNote(entity domain.Entity) string {
	if entity.IsTask() {
		return string(entity.Status)
	}
	return "note"
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
	left := max(28, width/2)
	right := max(1, width-left-1)
	return columnLayout{width: left, content: max(1, left)}, columnLayout{width: right, content: right}
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

func stackSections(first, second []string, width, height int) string {
	lines := make([]string, 0, len(first)+len(second)+1)
	lines = append(lines, first...)
	lines = append(lines, "")
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
