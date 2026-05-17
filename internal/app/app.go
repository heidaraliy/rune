package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	termansi "github.com/charmbracelet/x/ansi"
	"github.com/heidaraliy/rune/internal/core"
	"github.com/heidaraliy/rune/internal/handoff"
)

type mode int

const (
	modeNormal mode = iota
	modeSearch
	modeAdd
	modeConfirm
)

type filterMode int

const (
	filterOpen filterMode = iota
	filterAll
	filterDone
)

type confirmAction int

const (
	confirmNone confirmAction = iota
	confirmArchive
	confirmCodex
)

type confirmation struct {
	action        confirmAction
	prompt        string
	itemDisplayID string
	codexCWD      string
	codexPrompt   string
}

type Model struct {
	store     core.Store
	scope     core.Scope
	docs      []*core.Document
	allItems  []*core.Item
	items     []*core.Item
	selected  int
	scrollTop int
	width     int
	height    int
	mode      mode
	filter    filterMode
	query     string
	input     textinput.Model
	status    string
	statusRev int
	collapsed map[string]bool
	addAnchor string
	addAbove  bool
	topHidden bool
	confirm   confirmation
}

const statusTTL = 2500 * time.Millisecond

var (
	writeClipboard  = clipboard.WriteAll
	tmuxSession     = handoff.IsTmuxSession
	writeTmuxBuffer = handoff.LoadTmuxBuffer
	launchCodex     = openCodex
)

type statusClearMsg struct {
	revision int
}

func statusClearCmd(revision int) tea.Cmd {
	return tea.Tick(statusTTL, func(time.Time) tea.Msg {
		return statusClearMsg{revision: revision}
	})
}

func (m Model) setStatus(message string) (Model, tea.Cmd) {
	if message == "" {
		m.status = ""
		return m, nil
	}
	m.status = message
	m.statusRev++
	return m, statusClearCmd(m.statusRev)
}

func New(store core.Store, scope core.Scope) (Model, error) {
	in := textinput.New()
	in.Prompt = "> "
	in.CharLimit = 4096
	collapsed, err := loadCollapsedState(scope.Home)
	if err != nil {
		return Model{}, err
	}
	m := Model{
		store:     store,
		scope:     scope,
		input:     in,
		collapsed: collapsed,
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
		m.ensureSelectedVisible()
	case tea.KeyMsg:
		switch m.mode {
		case modeSearch:
			return m.updateSearch(msg)
		case modeAdd:
			return m.updateAdd(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		default:
			return m.updateNormal(msg)
		}
	case statusClearMsg:
		if msg.revision == m.statusRev {
			m.status = ""
		}
	case editorFinishedMsg:
		if msg.err != nil {
			return m.setStatus(msg.err.Error())
		} else {
			_ = m.reload()
			return m.setStatus("Editor closed.")
		}
	case codexFinishedMsg:
		if msg.err != nil {
			return m.setStatus("Codex failed: " + msg.err.Error())
		}
		return m.setStatus("Codex closed.")
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.moveSelection(1)
	case "k", "up":
		m.moveSelection(-1)
	case "pgdown", "pagedown", "ctrl+d":
		m.moveSelection(m.pageSize())
	case "pgup", "pageup", "ctrl+u":
		m.moveSelection(-m.pageSize())
	case "H", "h", "left":
		return m.collapseCurrent()
	case "L", "l", "right":
		return m.expandCurrent()
	case "t":
		m.topHidden = !m.topHidden
		m.ensureSelectedVisible()
		if m.topHidden {
			return m.setStatus("Top bar hidden.")
		}
		return m.setStatus("Top bar shown.")
	case "g":
		m.scope.Global = !m.scope.Global
		m.selected = 0
		m.scrollTop = 0
		if err := m.reload(); err != nil {
			return m.setStatus(err.Error())
		} else if m.scope.Global {
			return m.setStatus("Global view.")
		} else {
			return m.setStatus("Project view.")
		}
	case "f":
		m.filter = (m.filter + 1) % 3
		m.selected = 0
		m.scrollTop = 0
		if err := m.reload(); err != nil {
			return m.setStatus(err.Error())
		}
		return m.setStatus("Showing " + m.filterLabel() + " items.")
	case "/":
		m.mode = modeSearch
		m.input.SetValue(m.query)
		m.input.Placeholder = "search..."
		m.input.Focus()
	case "a":
		m.mode = modeAdd
		m.addAbove = false
		m.addAnchor = ""
		if item := m.current(); item != nil {
			m.addAnchor = item.ID
		}
		m.input.SetValue("")
		m.input.Placeholder = "new task below..."
		m.input.Focus()
	case "A":
		m.mode = modeAdd
		m.addAbove = true
		m.addAnchor = ""
		if item := m.current(); item != nil {
			m.addAnchor = item.ID
		}
		m.input.SetValue("")
		m.input.Placeholder = "new task above..."
		m.input.Focus()
	case " ", "enter":
		return m.toggleCurrent()
	case "x":
		count := m.doneCountInScope()
		if count == 0 {
			return m.setStatus("No completed items to archive.")
		}
		m = m.enterConfirm(confirmation{
			action: confirmArchive,
			prompt: fmt.Sprintf("archive %d done item(s)?", count),
		})
		return m, nil
	case "e":
		if item := m.current(); item != nil {
			return m, openEditor(item)
		}
	case "c":
		if item := m.current(); item != nil {
			m = m.enterConfirm(confirmation{
				action:        confirmCodex,
				prompt:        "start Codex for " + item.DisplayID + "?",
				itemDisplayID: item.DisplayID,
				codexCWD:      m.scope.CWD,
				codexPrompt:   m.yankTicketText(item),
			})
			return m, nil
		}
	case "y":
		if item := m.current(); item != nil {
			options := m.yankTicketOptions(item)
			text := core.YankTicketTextWithOptions(item, m.scope.Home, options)
			result, err := handoff.YankTicket(text, writeClipboard, tmuxSession(), writeTmuxBuffer)
			if err != nil {
				return m.setStatus("Yank failed: " + err.Error())
			}
			return m.setStatus(handoff.YankStatus(item.DisplayID, options.Agent, result))
		}
	}
	return m, nil
}

func (m Model) enterConfirm(confirm confirmation) Model {
	m.mode = modeConfirm
	m.confirm = confirm
	return m
}

func (m Model) clearConfirm() Model {
	m.mode = modeNormal
	m.confirm = confirmation{}
	return m
}

func (m Model) toggleCurrent() (tea.Model, tea.Cmd) {
	if item := m.current(); item != nil && item.Type == core.ItemTask {
		updated, err := m.store.SetDone(m.scope, item.DisplayID, false, true, m.scope.Global)
		if err != nil {
			return m.setStatus(err.Error())
		}
		_ = m.reload()
		return m.setStatus("Toggled " + updated.DisplayID + ".")
	}
	return m, nil
}

func (m Model) collapseCurrent() (tea.Model, tea.Cmd) {
	item := m.current()
	if item == nil {
		return m.setStatus("No item selected.")
	}
	if !m.hasChildren(item) {
		return m.setStatus("No nested items.")
	}
	if m.collapsed == nil {
		m.collapsed = make(map[string]bool)
	}
	if m.collapsed[item.ID] {
		return m.setStatus("Already collapsed " + item.DisplayID + ".")
	}
	m.collapsed[item.ID] = true
	selectedID := item.ID
	m.applyCollapsedItems()
	m.selectItem(selectedID)
	m.ensureSelectedVisible()
	if err := m.saveCollapsedState(); err != nil {
		return m.setStatus("Collapse not saved: " + err.Error())
	}
	return m.setStatus("Collapsed " + item.DisplayID + ".")
}

func (m Model) expandCurrent() (tea.Model, tea.Cmd) {
	item := m.current()
	if item == nil {
		return m.setStatus("No item selected.")
	}
	if !m.hasChildren(item) {
		return m.setStatus("No nested items.")
	}
	if !m.collapsed[item.ID] {
		return m.setStatus("Already unfurled " + item.DisplayID + ".")
	}
	delete(m.collapsed, item.ID)
	selectedID := item.ID
	m.applyCollapsedItems()
	m.selectItem(selectedID)
	m.ensureSelectedVisible()
	if err := m.saveCollapsedState(); err != nil {
		return m.setStatus("Unfurl not saved: " + err.Error())
	}
	return m.setStatus("Unfurled " + item.DisplayID + ".")
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		action := m.confirm.action
		m = m.clearConfirm()
		return m.setStatus(confirmCancelStatus(action))
	case "enter", "y", "Y":
		confirm := m.confirm
		m = m.clearConfirm()
		switch confirm.action {
		case confirmArchive:
			return m.confirmArchive()
		case confirmCodex:
			if strings.TrimSpace(confirm.codexPrompt) == "" {
				return m.setStatus("No item selected.")
			}
			m, _ = m.setStatus("Starting Codex for " + confirm.itemDisplayID + ".")
			return m, launchCodex(confirm.codexCWD, confirm.codexPrompt)
		default:
			return m, nil
		}
	}
	return m, nil
}

func (m Model) confirmArchive() (tea.Model, tea.Cmd) {
	count, path, err := m.store.ArchiveDone(m.scope)
	if err != nil {
		return m.setStatus(err.Error())
	}
	if count == 0 {
		return m.setStatus("No completed items to archive.")
	}
	_ = m.reload()
	return m.setStatus(fmt.Sprintf("Archived %d item(s) to %s.", count, path))
}

func confirmCancelStatus(action confirmAction) string {
	switch action {
	case confirmCodex:
		return "Codex launch cancelled."
	case confirmArchive:
		return "Archive cancelled."
	default:
		return "Cancelled."
	}
}

type codexFinishedMsg struct{ err error }

func openCodex(cwd, prompt string) tea.Cmd {
	cmd := exec.Command("codex", prompt)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return codexFinishedMsg{err: err}
	})
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.query = ""
		m.input.Blur()
		m.selected = 0
		m.scrollTop = 0
		_ = m.reload()
		return m.setStatus("Search cleared.")
	case "enter":
		m.mode = modeNormal
		m.query = m.input.Value()
		m.input.Blur()
		m.selected = 0
		m.scrollTop = 0
		_ = m.reload()
		if strings.TrimSpace(m.query) == "" {
			return m.setStatus("Search cleared.")
		}
		return m.setStatus("Search: " + m.query)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "enter":
		title := strings.TrimSpace(m.input.Value())
		m.mode = modeNormal
		m.input.Blur()
		if title == "" {
			return m.setStatus("Nothing added.")
		}
		var item *core.Item
		var err error
		if m.addAnchor != "" {
			item, err = m.store.AddNear(m.scope, m.addAnchor, m.addAbove, core.AddOptions{Title: title}, m.scope.Global)
		} else {
			item, err = m.store.Add(m.scope, core.AddOptions{Title: title})
		}
		if err != nil {
			return m.setStatus(err.Error())
		} else {
			_ = m.reload()
			m.selectItem(item.ID)
			m.ensureSelectedVisible()
			return m.setStatus("Added " + item.DisplayID + ".")
		}
	}
	var cmd tea.Cmd
	before := m.footerHeight()
	m.input, cmd = m.input.Update(msg)
	if m.footerHeight() != before {
		m.ensureSelectedVisible()
	}
	return m, cmd
}

func (m Model) filterLabel() string {
	switch m.filter {
	case filterAll:
		return "all"
	case filterDone:
		return "done"
	default:
		return "open"
	}
}

func (m *Model) reload() error {
	opts := core.ListOptions{Query: m.query}
	switch m.filter {
	case filterAll:
		opts.All = true
	case filterDone:
		opts.Done = true
	}
	items, docs, err := m.store.Items(m.scope, opts)
	if err != nil {
		return err
	}
	m.docs = docs
	m.allItems = items
	m.applyCollapsedItems()
	if m.selected >= len(m.items) {
		m.selected = len(m.items) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
	m.ensureSelectedVisible()
	return nil
}

func (m *Model) applyCollapsedItems() {
	m.items = visibleItems(m.allItems, m.collapsed)
}

func visibleItems(items []*core.Item, collapsed map[string]bool) []*core.Item {
	if len(items) == 0 {
		return nil
	}
	out := make([]*core.Item, 0, len(items))
	var hiddenDoc *core.Document
	hiddenDepth := -1
	for _, item := range items {
		if hiddenDoc != nil {
			if item.Doc == hiddenDoc && item.Depth > hiddenDepth {
				continue
			}
			hiddenDoc = nil
			hiddenDepth = -1
		}
		out = append(out, item)
		if collapsed[item.ID] && itemHasChildren(item) {
			hiddenDoc = item.Doc
			hiddenDepth = item.Depth
		}
	}
	return out
}

func (m *Model) moveSelection(delta int) {
	if len(m.items) == 0 {
		m.selected = 0
		m.scrollTop = 0
		return
	}
	m.selected = clamp(m.selected+delta, 0, len(m.items)-1)
	m.ensureSelectedVisible()
}

func (m Model) pageSize() int {
	return max(1, m.bodyContentHeight()-2)
}

func (m Model) topHeight() int {
	if m.topHidden {
		return 0
	}
	return 5
}

func (m Model) bodyHeight() int {
	if m.height <= 0 {
		return 1
	}
	return max(1, m.height-m.topHeight()-m.footerHeight())
}

func (m Model) bodyContentHeight() int {
	return max(1, m.bodyHeight()-2)
}

func (m Model) footerHeight() int {
	return m.footerContentHeight() + 2
}

func (m Model) footerContentHeight() int {
	if m.mode == modeAdd {
		return len(m.addInputRows(boxInnerWidth(m.width)))
	}
	if m.mode == modeSearch || m.mode == modeConfirm || m.status != "" {
		return 1
	}
	return 2
}

func (m Model) maxAddInputRows() int {
	if m.height <= 0 {
		return 8
	}
	available := m.height - m.topHeight() - 3 - 2
	return max(1, available)
}

func (m *Model) ensureSelectedVisible() {
	visible := m.bodyContentHeight()
	width := 80
	if m.width > 0 {
		_, midW, _ := paneWidths(m.width)
		width = paneContentWidth(midW)
	}
	rows, selectedRow := m.middleRows(width)
	if len(rows) == 0 {
		m.scrollTop = 0
		return
	}
	maxTop := max(0, len(rows)-visible)
	if selectedRow < m.scrollTop {
		m.scrollTop = selectedRow
	} else if selectedRow >= m.scrollTop+visible {
		m.scrollTop = selectedRow - visible + 1
	}
	m.scrollTop = clamp(m.scrollTop, 0, maxTop)
}

func (m *Model) selectItem(id string) {
	for idx, item := range m.items {
		if item.ID == id {
			m.selected = idx
			return
		}
	}
}

func (m Model) current() *core.Item {
	if m.selected < 0 || m.selected >= len(m.items) {
		return nil
	}
	return m.items[m.selected]
}

type editorFinishedMsg struct{ err error }

func openEditor(item *core.Item) tea.Cmd {
	return func() tea.Msg {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nvim"
		}
		cmd := exec.Command(editor, fmt.Sprintf("+%d", item.Line+1), item.Source)
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			return editorFinishedMsg{err: err}
		})()
	}
}

func (m Model) View() string {
	if m.width == 0 {
		return "Rune is starting..."
	}
	top := m.renderTop()
	footer := m.renderFooter()
	bodyHeight := m.bodyHeight()
	bodyContentHeight := m.bodyContentHeight()
	_, midW, rightW := paneWidths(m.width)
	mid := renderPane(m.renderMiddle(paneContentWidth(midW), bodyContentHeight), midW, bodyHeight)
	right := renderPane(m.renderRight(paneContentWidth(rightW), bodyContentHeight), rightW, bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, mid, right)
	parts := []string{body, footer}
	if top != "" {
		parts = append([]string{top}, parts...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

const (
	electricBlue  lipgloss.Color = "39"
	cosmicBase    lipgloss.Color = "#090812"
	cosmicSurface lipgloss.Color = "#151122"
	cosmicMuted   lipgloss.Color = "#52436f"
	cosmicText    lipgloss.Color = "#e8e3f4"
	cosmicBright  lipgloss.Color = "#f5f0ff"
	cosmicViolet  lipgloss.Color = "#c8a7ff"
	cosmicBlue    lipgloss.Color = "#9bbcff"
	cosmicCyan    lipgloss.Color = electricBlue
	cosmicAmber   lipgloss.Color = "#f4d889"
	cosmicGreen   lipgloss.Color = "#8fe6a7"
)

var (
	topStyle        = lipgloss.NewStyle().Foreground(cosmicText).Background(cosmicBase)
	logoStyle       = lipgloss.NewStyle().Bold(true).Foreground(cosmicViolet).Background(cosmicBase)
	topLabelStyle   = lipgloss.NewStyle().Bold(true).Foreground(cosmicBlue).Background(cosmicBase)
	projectStyle    = lipgloss.NewStyle().Bold(true).Foreground(cosmicViolet).Background(cosmicBase)
	todoStyle       = lipgloss.NewStyle().Bold(true).Foreground(cosmicAmber).Background(cosmicBase)
	doneCountStyle  = lipgloss.NewStyle().Bold(true).Foreground(cosmicGreen).Background(cosmicBase)
	topMetaStyle    = lipgloss.NewStyle().Foreground(cosmicMuted).Background(cosmicBase)
	selectedStyle   = lipgloss.NewStyle().Foreground(cosmicBase).Background(cosmicViolet).Bold(true)
	dimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	tagStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	doneStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	openStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	headingStyle    = lipgloss.NewStyle().Bold(true).Foreground(electricBlue)
	labelStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	codeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("221")).Background(lipgloss.Color("236"))
	footerBarStyle  = lipgloss.NewStyle().Foreground(cosmicText).Background(cosmicSurface)
	footerKeyStyle  = lipgloss.NewStyle().Bold(true).Foreground(cosmicCyan).Background(cosmicSurface)
	footerTextStyle = lipgloss.NewStyle().Foreground(cosmicBright).Background(cosmicSurface)
	footerSepStyle  = lipgloss.NewStyle().Foreground(cosmicMuted).Background(cosmicSurface)
	statusStyle     = lipgloss.NewStyle().Bold(true).Foreground(cosmicBase).Background(cosmicViolet)
	sectionBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cosmicViolet)
	topBoxStyle     = sectionBoxStyle.Background(cosmicBase)
	footerBoxStyle  = sectionBoxStyle.Background(cosmicSurface)
)

var depthColors = []lipgloss.Color{
	lipgloss.Color("252"),
	lipgloss.Color("111"),
	lipgloss.Color("151"),
	lipgloss.Color("222"),
	lipgloss.Color("218"),
	lipgloss.Color("183"),
	electricBlue,
	lipgloss.Color("214"),
	lipgloss.Color("245"),
}

func (m Model) renderTop() string {
	if m.topHidden {
		return ""
	}
	scope := "PROJECT"
	name := m.scope.Project
	if m.scope.Global {
		scope = "GLOBAL"
		name = "all notes"
	}
	if name == "" {
		name = "project"
	}
	openCount, doneCount := m.taskStats()
	logo := []string{
		"'||''| '||  ||` `||''|,  .|''|,",
		" ||     ||  ||   ||  ||  ||..||",
		".||.    `|..'|. .||  ||. `|...",
	}
	stats := []string{
		topStat("project", name, projectStyle),
		topStat("todo", fmt.Sprintf("%d", openCount), todoStyle),
		topStat("done", fmt.Sprintf("%d", doneCount), doneCountStyle),
	}
	if scope != "PROJECT" {
		stats[0] = topStat(strings.ToLower(scope), name, projectStyle)
	}
	innerWidth := boxInnerWidth(m.width)
	lines := make([]string, 0, len(logo))
	for i := range logo {
		left := topStyle.Render(" ") + logoStyle.Render(logo[i])
		center := ""
		switch i {
		case 0:
			center = m.headerControlLine("Views", []headerChoice{
				{label: "project", meta: name, active: !m.scope.Global},
				{label: "global", meta: "all", active: m.scope.Global},
			})
		case 1:
			center = m.headerControlLine("Filters", []headerChoice{
				{label: "open", active: m.filter == filterOpen},
				{label: "all", active: m.filter == filterAll},
				{label: "done", active: m.filter == filterDone},
			})
		case 2:
			if m.query != "" {
				center = topMetaStyle.Render("/ " + m.query)
			}
		}
		right := stats[i] + topStyle.Render(" ")
		lines = append(lines, renderTopLine(innerWidth, left, center, right))
	}
	return renderBox(topBoxStyle, m.width, m.topHeight(), lines, topStyle)
}

func renderTopLine(width int, left, center, right string) string {
	line := clipStyled(left, width)
	remaining := width - lipgloss.Width(line)
	if center != "" && remaining > 1 {
		line += topStyle.Render(" ")
		remaining--
		center = clipStyled(center, remaining)
		line += center
		remaining -= lipgloss.Width(center)
	}
	if right != "" && remaining > 1 {
		rightWidth := lipgloss.Width(right)
		if rightWidth >= remaining {
			line += topStyle.Render(" ") + clipStyled(right, remaining-1)
		} else {
			line += topStyle.Render(strings.Repeat(" ", remaining-rightWidth)) + right
		}
	}
	return renderSolidLine(topStyle, width, line)
}

type headerChoice struct {
	label  string
	meta   string
	active bool
}

func (m Model) headerControlLine(title string, choices []headerChoice) string {
	parts := []string{headingStyle.Background(cosmicBase).Render(title)}
	for _, choice := range choices {
		text := choice.label
		if choice.meta != "" && choice.active {
			text += " " + choice.meta
		}
		if choice.active {
			parts = append(parts, selectedStyle.Render(" "+text+" "))
		} else {
			parts = append(parts, topMetaStyle.Render(text))
		}
	}
	return topStyle.Render(" ") + strings.Join(parts, topStyle.Render("  ")) + topStyle.Render(" ")
}

func topStat(label string, value string, style lipgloss.Style) string {
	return topLabelStyle.Render(label+": ") + style.Render(value)
}

func (m Model) renderFooter() string {
	innerWidth := boxInnerWidth(m.width)
	if m.mode == modeAdd {
		return renderFooterBox(m.width, m.addInputRows(innerWidth), footerBarStyle)
	}
	if m.mode == modeSearch {
		content := footerBarStyle.Render(" " + m.input.View())
		return renderFooterBox(m.width, []string{renderSolidLine(footerBarStyle, innerWidth, content)}, footerBarStyle)
	}
	if m.mode == modeConfirm {
		content := footerBarStyle.Render(" "+m.confirm.prompt+"  ") +
			footerKeyStyleFor("y/enter").Render("y/enter") + footerBarStyle.Render(" confirm  ") +
			footerKeyStyleFor("n/esc").Render("n/esc") + footerBarStyle.Render(" cancel")
		return renderFooterBox(m.width, []string{renderSolidLine(footerBarStyle, innerWidth, content)}, footerBarStyle)
	}
	if m.status != "" {
		return renderFooterBox(m.width, []string{renderSolidLine(statusStyle, innerWidth, statusStyle.Render(" "+m.status))}, statusStyle)
	}
	return renderFooterBox(m.width, []string{
		renderFooterRow(innerWidth, []footerHint{
			{"j/k", "move"},
			{"pg ^u/^d", "page"},
			{"h/l", "fold"},
			{"spc", "done"},
			{"a", "below"},
			{"A", "above"},
			{"e", "edit"},
			{"y", "yank"},
			{"c", "codex"},
		}),
		renderFooterRow(innerWidth, []footerHint{
			{"/", "search"},
			{"f", "filter"},
			{"g", "global"},
			{"t", "top"},
			{"x", "archive"},
			{"q", "quit"},
		}),
	}, footerBarStyle)
}

func (m Model) addInputRows(width int) []string {
	rows, cursorRow := wrappedInputRows(m.input, width)
	if limit := m.maxAddInputRows(); len(rows) > limit {
		start := clamp(cursorRow-limit+1, 0, len(rows)-limit)
		rows = rows[start : start+limit]
	}
	for idx, row := range rows {
		rows[idx] = renderSolidLine(footerBarStyle, width, row)
	}
	return rows
}

func wrappedInputRows(input textinput.Model, width int) ([]string, int) {
	if width <= 0 {
		return []string{""}, 0
	}
	if input.Value() == "" {
		return []string{footerBarStyle.Render(" ") + input.View()}, 0
	}

	prefixWidth := lipgloss.Width(" " + input.Prompt)
	prefix := footerBarStyle.Render(" ") + input.PromptStyle.Render(input.Prompt)
	continuationPrefix := footerBarStyle.Render(strings.Repeat(" ", max(1, prefixWidth)))
	value := []rune(input.Value())
	position := clamp(input.Position(), 0, len(value))

	var rows []string
	row := prefix
	rowWidth := prefixWidth
	cursorRow := 0
	appendRow := func() {
		rows = append(rows, row)
		row = continuationPrefix
		rowWidth = prefixWidth
	}

	for idx := 0; idx <= len(value); idx++ {
		if idx == position {
			char := " "
			if idx < len(value) {
				char = string(value[idx])
			}
			cursor := input.Cursor
			cursor.SetChar(char)
			cell := cursor.View()
			cellWidth := max(1, lipgloss.Width(char))
			if rowWidth+cellWidth > width && rowWidth > prefixWidth {
				appendRow()
			}
			row += cell
			rowWidth += cellWidth
			cursorRow = len(rows)
			if idx < len(value) {
				continue
			}
			break
		}
		if idx >= len(value) {
			break
		}
		cell := input.TextStyle.Inline(true).Render(string(value[idx]))
		cellWidth := max(1, lipgloss.Width(string(value[idx])))
		if rowWidth+cellWidth > width && rowWidth > prefixWidth {
			appendRow()
		}
		row += cell
		rowWidth += cellWidth
	}
	rows = append(rows, row)
	return rows, cursorRow
}

type footerHint struct {
	key    string
	action string
}

func renderFooterRow(width int, hints []footerHint) string {
	pairs := make([]string, 0, len(hints))
	for _, hint := range hints {
		pairs = append(pairs, keyHint(hint.key, hint.action))
	}
	text := footerBarStyle.Render(" ") + strings.Join(pairs, footerSepStyle.Render(" | "))
	return renderSolidLine(footerBarStyle, width, text)
}

func renderFooterBox(width int, rows []string, fillStyle lipgloss.Style) string {
	return renderBox(footerBoxStyle, width, len(rows)+2, rows, fillStyle)
}

func keyHint(key, action string) string {
	return footerKeyStyleFor(key).Render(key) + footerBarStyle.Render(" ") + footerTextStyle.Render(action)
}

func footerKeyStyleFor(key string) lipgloss.Style {
	color := electricBlue
	switch key {
	case "j/k", "pg ^u/^d":
		color = lipgloss.Color("111")
	case "spc":
		color = cosmicAmber
	case "a", "A", "e", "y", "c", "y/enter":
		color = cosmicGreen
	case "g", "t", "h/l":
		color = cosmicViolet
	case "x", "q", "n/esc":
		color = lipgloss.Color("203")
	}
	return footerKeyStyle.Foreground(color)
}

func renderSolidLine(style lipgloss.Style, width int, content string) string {
	if width <= 0 {
		return ""
	}
	line := clipStyled(content, width)
	if missing := width - lipgloss.Width(line); missing > 0 {
		line += style.Render(strings.Repeat(" ", missing))
	}
	return line
}

func (m Model) yankTicketText(item *core.Item) string {
	return core.YankTicketTextWithOptions(item, m.scope.Home, m.yankTicketOptions(item))
}

func (m Model) yankTicketOptions(item *core.Item) core.YankOptions {
	return core.YankOptionsForItem(item)
}

func (m Model) taskStats() (int, int) {
	openCount := 0
	doneCount := 0
	for _, doc := range m.docs {
		for _, item := range doc.Items {
			if item.Type != core.ItemTask {
				continue
			}
			if item.Done {
				doneCount++
			} else {
				openCount++
			}
		}
	}
	return openCount, doneCount
}

func (m Model) doneCountInScope() int {
	count := 0
	for _, doc := range m.docs {
		for _, item := range doc.Items {
			if item.IsDone() {
				count++
			}
		}
	}
	return count
}

func (m Model) renderMiddle(width, height int) string {
	rows, _ := m.middleRows(width)
	if len(rows) == 0 {
		return fitLines([]string{dimStyle.Render("No items.")}, width, height)
	}
	maxTop := max(0, len(rows)-height)
	start := clamp(m.scrollTop, 0, maxTop)
	return fitLines(rows[start:], width, height)
}

func (m Model) middleRows(width int) ([]string, int) {
	var lines []string
	lastHeading := ""
	selectedRow := 0
	for idx, item := range m.items {
		if item.Heading != "" && item.Heading != lastHeading {
			lastHeading = item.Heading
			lines = append(lines, headingStyle.Render(item.Heading))
		}
		if idx == m.selected {
			selectedRow = len(lines)
		}
		box := "   "
		if item.Type == core.ItemTask {
			if item.Done {
				box = doneStyle.Render("[x]")
			} else {
				box = openStyle.Render("[ ]")
			}
		}
		tags := ""
		if len(item.Tags) > 0 {
			tags = " " + tagStyle.Render("#"+strings.Join(item.Tags, " #"))
		}
		caret := " "
		if m.hasChildren(item) {
			caret = "v"
			if m.collapsed[item.ID] {
				caret = ">"
			}
		}
		line := itemIndent(item.Depth) + depthIDStyle(item.Depth).Render(fmt.Sprintf("%-4s", item.DisplayID)) +
			fmt.Sprintf(" %s %s %s%s", caret, box, item.Title, tags)
		if idx == m.selected {
			line = selectedStyle.Render(padStyled(clipStyled(line, width), width))
		}
		lines = append(lines, line)
	}
	return lines, selectedRow
}

func itemIndent(depth int) string {
	if depth <= 0 {
		return ""
	}
	if depth > 8 {
		depth = 8
	}
	prefixStyle := depthIDStyle(depth)
	return strings.Repeat("  ", depth) + prefixStyle.Render("└─ ")
}

func depthIDStyle(depth int) lipgloss.Style {
	if depth < 0 {
		depth = 0
	}
	color := depthColors[depth%len(depthColors)]
	return lipgloss.NewStyle().Foreground(color).Bold(depth == 0)
}

func (m Model) renderRight(width, height int) string {
	item := m.current()
	if item == nil {
		return fitLines([]string{dimStyle.Render("Select an item.")}, width, height)
	}
	var lines []string
	status := "note"
	if item.Type == core.ItemTask {
		status = "open"
		if item.Done {
			status = "done"
		}
	}
	lines = append(lines, labelStyle.Render("ID")+" "+item.DisplayID+"  "+labelStyle.Render("Status")+" "+status)
	if len(item.Tags) > 0 {
		lines = appendWrappedStyled(lines, "Tags  #"+strings.Join(item.Tags, " #"), width, tagStyle)
	}
	if item.Source != "" {
		lines = appendWrappedStyled(lines, fmt.Sprintf("Source  %s:%d", m.sourceLabel(item), item.Line+1), width, dimStyle)
	}
	lines = append(lines, ruleLine(width), labelStyle.Render("Note"))
	lines = appendWrappedStyled(lines, item.Title, width, headingStyle)
	children := m.childrenOf(item)
	if len(children) > 0 {
		lines = append(lines, "", ruleLine(width), labelStyle.Render("Children"))
		for _, child := range children {
			box := " - "
			if child.Type == core.ItemTask {
				box = " [ ] "
				if child.Done {
					box = " [x] "
				}
			}
			prefix := strings.Repeat("  ", max(0, child.Depth-item.Depth-1)) + box
			for _, wrapped := range wrapPlainLine(prefix+child.Title, width) {
				lines = append(lines, wrapped)
			}
		}
	}
	body := item.Body()
	if body != "" {
		lines = append(lines, "", ruleLine(width), labelStyle.Render("Details"))
		for _, line := range strings.Split(body, "\n") {
			if line == "" {
				lines = append(lines, "")
				continue
			}
			for _, wrapped := range wrapPlainLine(line, width) {
				lines = append(lines, renderMarkdownLine(wrapped))
			}
		}
	}
	return fitLines(lines, width, height)
}

func (m Model) sourceLabel(item *core.Item) string {
	return core.SourceLabel(item, m.scope.Home)
}

func (m Model) childrenOf(parent *core.Item) []*core.Item {
	if parent == nil {
		return nil
	}
	var children []*core.Item
	if parent.Doc == nil {
		return nil
	}
	for _, item := range parent.Doc.Items {
		if item.Line <= parent.Line {
			continue
		}
		if item.Depth <= parent.Depth {
			break
		}
		children = append(children, item)
	}
	return children
}

func (m Model) hasChildren(item *core.Item) bool {
	return itemHasChildren(item)
}

func itemHasChildren(item *core.Item) bool {
	if item == nil || item.Doc == nil {
		return false
	}
	for _, candidate := range item.Doc.Items {
		if candidate.Line <= item.Line {
			continue
		}
		if candidate.Depth <= item.Depth {
			return false
		}
		return true
	}
	return false
}

func ruleLine(width int) string {
	if width <= 0 {
		return ""
	}
	return dimStyle.Render(strings.Repeat("─", width))
}

func appendWrappedStyled(lines []string, text string, width int, style lipgloss.Style) []string {
	for _, line := range wrapPlainLine(text, width) {
		lines = append(lines, style.Render(line))
	}
	return lines
}

func renderMarkdownLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return headingStyle.Render(line)
	}
	if strings.HasPrefix(trimmed, ">") {
		return dimStyle.Render(line)
	}
	if strings.Contains(line, "`") {
		parts := strings.Split(line, "`")
		var out strings.Builder
		for i, part := range parts {
			if i%2 == 1 {
				out.WriteString(codeStyle.Render(part))
			} else {
				out.WriteString(part)
			}
		}
		return out.String()
	}
	return line
}

func fitLines(lines []string, width, height int) string {
	if height < 1 {
		height = 1
	}
	out := make([]string, 0, height)
	for _, line := range lines {
		if len(out) >= height {
			break
		}
		out = append(out, clipStyled(line, width))
	}
	for len(out) < height {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func paneWidths(total int) (int, int, int) {
	if total <= 0 {
		return 0, 0, 0
	}
	mid := total / 2
	right := total - mid
	if total >= 8 {
		mid = max(4, mid)
		right = max(4, total-mid)
	}
	return 0, mid, right
}

func renderPane(content string, width, height int) string {
	inner := boxInnerWidth(width)
	contentWidth := paneContentWidth(width)
	contentHeight := max(1, height-2)
	lines := strings.Split(content, "\n")
	out := make([]string, 0, contentHeight)
	for i := 0; i < contentHeight; i++ {
		line := ""
		if i < len(lines) {
			line = clipStyled(lines[i], contentWidth)
		}
		out = append(out, " "+padStyled(line, contentWidth)+" ")
	}
	if inner < 2 {
		for i := range out {
			out[i] = clipStyled(out[i], inner)
		}
	}
	return renderBox(sectionBoxStyle, width, height, out, lipgloss.NewStyle())
}

func renderBox(style lipgloss.Style, width, height int, lines []string, fillStyle lipgloss.Style) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if width < 4 || height < 3 {
		joined := strings.Join(lines, " ")
		return renderSolidLine(fillStyle, width, joined)
	}
	innerWidth := boxInnerWidth(width)
	innerHeight := max(1, height-2)
	out := make([]string, 0, innerHeight)
	for _, line := range lines {
		if len(out) >= innerHeight {
			break
		}
		out = append(out, renderSolidLine(fillStyle, innerWidth, line))
	}
	for len(out) < innerHeight {
		out = append(out, fillStyle.Render(strings.Repeat(" ", innerWidth)))
	}
	return style.Width(innerWidth).Height(innerHeight).Render(strings.Join(out, "\n"))
}

func boxInnerWidth(width int) int {
	return max(1, width-2)
}

func paneContentWidth(width int) int {
	return max(1, width-4)
}

func wrapPlainLine(line string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	if lipgloss.Width(line) <= width {
		return []string{line}
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{line}
	}
	var lines []string
	current := ""
	for _, word := range words {
		if current == "" {
			for lipgloss.Width(word) > width {
				prefix, rest := splitByWidth(word, width)
				lines = append(lines, prefix)
				word = rest
			}
			current = word
			continue
		}
		next := current + " " + word
		if lipgloss.Width(next) <= width {
			current = next
			continue
		}
		lines = append(lines, current)
		for lipgloss.Width(word) > width {
			prefix, rest := splitByWidth(word, width)
			lines = append(lines, prefix)
			word = rest
		}
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitByWidth(value string, width int) (string, string) {
	if width <= 0 {
		return "", value
	}
	runes := []rune(value)
	cut := 0
	for cut < len(runes) && lipgloss.Width(string(runes[:cut+1])) <= width {
		cut++
	}
	if cut == 0 {
		cut = 1
	}
	return string(runes[:cut]), string(runes[cut:])
}

func padStyled(line string, width int) string {
	if width <= 0 {
		return ""
	}
	lineWidth := lipgloss.Width(line)
	if lineWidth >= width {
		return line
	}
	return line + strings.Repeat(" ", width-lineWidth)
}

func plainClip(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	runes := []rune(line)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func clipStyled(line string, width int) string {
	if width <= 0 {
		return ""
	}
	return termansi.Truncate(line, width, "")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
