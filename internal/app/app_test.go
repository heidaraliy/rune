package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	termansi "github.com/charmbracelet/x/ansi"
	"github.com/heidaraliy/rune/internal/core"
)

func TestModelTogglesTaskAndCyclesDoneFilter(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	if _, err := store.Add(scope, core.AddOptions{Title: "fix stuns"}); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.items) != 1 {
		t.Fatalf("items = %d, want 1", len(model.items))
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(Model)
	if model.status == "" || model.statusRev == 0 {
		t.Fatalf("status toast not set after toggle: %q rev=%d", model.status, model.statusRev)
	}
	if len(model.items) != 0 {
		t.Fatalf("open items after toggle = %d, want 0", len(model.items))
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model = updated.(Model)
	if model.filter != filterDone || len(model.items) != 1 || !model.items[0].Done {
		t.Fatalf("done filter/items = %d/%#v", model.filter, model.items)
	}
}

func TestModelSearchAndView(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	if _, err := store.Add(scope, core.AddOptions{
		Title: "combat snapshot",
		Body:  "this selected note has enough text that it should wrap into the available right pane and reveal the tail-marker",
		Tags:  []string{"netcode"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(scope, core.AddOptions{Title: "map polish", Tags: []string{"content"}}); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	model = updated.(Model)
	model.query = "combat"
	if err := model.reload(); err != nil {
		t.Fatal(err)
	}
	if len(model.items) != 1 || model.items[0].Title != "combat snapshot" {
		t.Fatalf("search items = %#v", model.items)
	}
	if view := plainText(model.View()); !strings.Contains(view, "project: lune") || !strings.Contains(view, "combat snapshot") {
		t.Fatalf("view = %q", view)
	}
	if view := plainText(model.View()); !strings.Contains(view, "╭") || !strings.Contains(view, "╰") {
		t.Fatalf("view did not render rounded section borders: %q", view)
	}
	if detail := model.renderRight(36, 18); !strings.Contains(detail, "ID") ||
		!strings.Contains(detail, "Status") ||
		!strings.Contains(detail, "Note") ||
		!strings.Contains(detail, "Details") ||
		!strings.Contains(detail, "Source  projects/lune.md") ||
		!strings.Contains(detail, "tail-marker") ||
		strings.Contains(detail, "…") {
		t.Fatalf("right pane did not wrap full selected note: %q", detail)
	}
}

func TestViewRowsDoNotExceedTerminalWidth(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	if _, err := store.Add(scope, core.AddOptions{Title: "start exploring fps and performance improvements"}); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 18})
	model = updated.(Model)

	viewLines := strings.Split(model.View(), "\n")
	if len(viewLines) != model.height {
		t.Fatalf("view line count = %d, want %d: %q", len(viewLines), model.height, plainText(model.View()))
	}
	for row, line := range viewLines {
		if got := lipgloss.Width(line); got > model.width {
			t.Fatalf("row %d width = %d, want <= %d: %q", row, got, model.width, line)
		}
	}
}

func TestPaneWidthsUseFiftyFiftyBodyLayout(t *testing.T) {
	left, mid, right := paneWidths(202)
	if left != 0 || mid != 101 || right != 101 {
		t.Fatalf("paneWidths(202) = %d/%d/%d, want 0/101/101", left, mid, right)
	}
}

func TestMiddlePaneRendersNestedTaskDepth(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	doc, err := core.ParseDocument("todo.md", "project", "lune", []byte(strings.Join([]string{
		"# lune todo",
		"",
		"- [ ] parent",
		"<!-- rune:id=aaaa0000 type=task tags= created=2026-05-14T00:00:00Z -->",
		"    - [ ] child",
		"<!-- rune:id=bbbb0000 type=task tags= created=2026-05-14T00:00:00Z -->",
		"        - [ ] grandchild",
		"<!-- rune:id=cccc0000 type=task tags= created=2026-05-14T00:00:00Z -->",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	path := core.ProjectPath(home, "lune")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(doc.Lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	rendered := model.renderMiddle(60, 10)
	if !strings.Contains(rendered, "parent") ||
		!strings.Contains(rendered, "  └─ bbb") ||
		!strings.Contains(rendered, "    └─ ccc") {
		t.Fatalf("nested middle render = %q", rendered)
	}
	detail := model.renderRight(60, 14)
	if !strings.Contains(detail, "Children") ||
		!strings.Contains(detail, "[ ] child") ||
		!strings.Contains(detail, "  [ ] grandchild") {
		t.Fatalf("nested right render = %q", detail)
	}
}

func TestDepthIDStyleVariesByDepth(t *testing.T) {
	if depthIDStyle(0).GetForeground() == depthIDStyle(1).GetForeground() {
		t.Fatal("depth 0 and depth 1 should use different id colors")
	}
	if !depthIDStyle(0).GetBold() {
		t.Fatal("root item ids should be bold")
	}
	if depthIDStyle(1).GetBold() {
		t.Fatal("nested item ids should not be bold")
	}
}

func TestHeaderExposesViewAndFilterStateWithoutSidebar(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	for _, title := range []string{"first task", "second task", "third task"} {
		if _, err := store.Add(scope, core.AddOptions{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 96, Height: 20})
	model = updated.(Model)
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyDown},
		{Type: tea.KeyDown},
		{Type: tea.KeyUp},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
		view := plainText(model.View())
		if !strings.Contains(view, "Views") || !strings.Contains(view, "project") || !strings.Contains(view, "Filters") {
			t.Fatalf("header controls disappeared after %q: %q", key.String(), view)
		}
		for _, unwanted := range []string{"inbox", "Store", home, "global  all"} {
			if strings.Contains(view, unwanted) {
				t.Fatalf("view kept sidebar-era text %q after %q: %q", unwanted, key.String(), view)
			}
		}
	}
}

func TestStatusToastClearsOnlyCurrentRevision(t *testing.T) {
	model := Model{width: 80}
	var cmd tea.Cmd
	model, cmd = model.setStatus("Added abc.")
	if cmd == nil {
		t.Fatal("expected status clear command")
	}
	if !strings.Contains(model.renderFooter(), "Added abc.") {
		t.Fatalf("footer did not show status: %q", model.renderFooter())
	}

	updated, _ := model.Update(statusClearMsg{revision: model.statusRev - 1})
	model = updated.(Model)
	if model.status != "Added abc." {
		t.Fatalf("stale clear removed status: %q", model.status)
	}

	updated, _ = model.Update(statusClearMsg{revision: model.statusRev})
	model = updated.(Model)
	if model.status != "" {
		t.Fatalf("current clear left status = %q", model.status)
	}
	if !strings.Contains(plainText(model.renderFooter()), "spc done") {
		t.Fatalf("footer did not restore controls: %q", model.renderFooter())
	}
}

func TestNavigationDoesNotScheduleRenderLoopCommands(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "rune"}
	for i := 0; i < 25; i++ {
		if _, err := store.Add(scope, core.AddOptions{Title: fmt.Sprintf("task %02d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	if cmd := model.Init(); cmd != nil {
		t.Fatal("initial model scheduled a command")
	}

	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: 100, Height: 18},
		tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyPgDown},
		tea.KeyMsg{Type: tea.KeyUp},
	} {
		updated, cmd := model.Update(msg)
		model = updated.(Model)
		if cmd != nil {
			t.Fatalf("%T scheduled an unexpected command", msg)
		}
	}
}

func TestEnterTogglesSelectedTaskLikeSpace(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	if _, err := store.Add(scope, core.AddOptions{Title: "toggle me"}); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if len(model.items) != 0 {
		t.Fatalf("enter did not toggle item out of open filter: %#v", model.items)
	}
	all, _, err := store.Items(scope, core.ListOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || !all[0].Done {
		t.Fatalf("item after enter = %#v, want done", all)
	}
}

func TestArchiveRequiresConfirmation(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	item, err := store.Add(scope, core.AddOptions{Title: "done task"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetDone(scope, item.ID, true, false, false); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	model.width = 100
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model = updated.(Model)
	if model.mode != modeConfirm || model.confirm.action != confirmArchive {
		t.Fatalf("archive mode/action = %v/%v, want confirm/archive", model.mode, model.confirm.action)
	}
	footer := plainText(model.renderFooter())
	if !strings.Contains(footer, "archive 1 done item(s)?") || !strings.Contains(footer, "y/enter confirm") {
		t.Fatalf("confirm footer = %q", model.renderFooter())
	}
	if _, err := os.Stat(core.ArchivePath(home, store.Now())); !os.IsNotExist(err) {
		t.Fatalf("archive was written before confirmation: %v", err)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.mode != modeNormal {
		t.Fatalf("mode after cancel = %v, want normal", model.mode)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = updated.(Model)
	if model.mode != modeNormal {
		t.Fatalf("mode after confirm = %v, want normal", model.mode)
	}
	if _, err := os.Stat(core.ArchivePath(home, store.Now())); err != nil {
		t.Fatalf("archive was not written after confirmation: %v", err)
	}
}

func TestCodexLaunchRequiresConfirmation(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune", CWD: t.TempDir()}
	if _, err := store.Add(scope, core.AddOptions{Title: "codex task", Body: "launch detail"}); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	model.width = 100
	oldLaunchCodex := launchCodex
	t.Cleanup(func() { launchCodex = oldLaunchCodex })
	var launchedCWD, launchedPrompt string
	launches := 0
	launchCodex = func(cwd, prompt string) tea.Cmd {
		launches++
		launchedCWD = cwd
		launchedPrompt = prompt
		return func() tea.Msg { return codexFinishedMsg{} }
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("codex key launched before confirmation")
	}
	if model.mode != modeConfirm || model.confirm.action != confirmCodex {
		t.Fatalf("codex mode/action = %v/%v, want confirm/codex", model.mode, model.confirm.action)
	}
	footer := plainText(model.renderFooter())
	if !strings.Contains(footer, "start Codex for") || !strings.Contains(footer, "y/enter confirm") {
		t.Fatalf("codex confirm footer = %q", model.renderFooter())
	}
	if launches != 0 {
		t.Fatalf("launches before confirm = %d", launches)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.mode != modeNormal || !strings.Contains(model.status, "Codex launch cancelled.") {
		t.Fatalf("after cancel mode/status = %v/%q", model.mode, model.status)
	}
	if launches != 0 {
		t.Fatalf("launches after cancel = %d", launches)
	}

	model.status = ""
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(Model)
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.mode != modeNormal {
		t.Fatalf("mode after confirm = %v, want normal", model.mode)
	}
	if cmd == nil {
		t.Fatal("codex confirm did not return launch command")
	}
	if launches != 1 {
		t.Fatalf("launches after confirm = %d", launches)
	}
	if launchedCWD != scope.CWD {
		t.Fatalf("codex cwd = %q, want %q", launchedCWD, scope.CWD)
	}
	if !strings.Contains(launchedPrompt, "# Rune Ticket: codex task") || !strings.Contains(launchedPrompt, "launch detail") {
		t.Fatalf("codex prompt = %q", launchedPrompt)
	}
	msg := cmd()
	updated, _ = model.Update(msg)
	model = updated.(Model)
	if !strings.Contains(model.status, "Codex closed.") {
		t.Fatalf("status after codex command = %q", model.status)
	}
}

func TestMiddlePaneAutoscrollsAndPages(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	for i := 0; i < 40; i++ {
		if _, err := store.Add(scope, core.AddOptions{Title: fmt.Sprintf("task %02d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 14})
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updated.(Model)
	if model.selected == 0 || model.scrollTop == 0 {
		t.Fatalf("page down did not advance selected/scroll: selected=%d scrollTop=%d", model.selected, model.scrollTop)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	model = updated.(Model)
	rows, selectedRow := model.middleRows(58)
	if selectedRow < model.scrollTop || selectedRow >= model.scrollTop+model.bodyContentHeight() {
		t.Fatalf("selected row not visible: selectedRow=%d scrollTop=%d bodyContentHeight=%d rows=%d", selectedRow, model.scrollTop, model.bodyContentHeight(), len(rows))
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	model = updated.(Model)
	if model.selected >= 2*model.pageSize() {
		t.Fatalf("ctrl+u did not page upward: selected=%d page=%d", model.selected, model.pageSize())
	}
}

func TestModelAddsBelowAndAboveSelectedItem(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	if _, err := store.Add(scope, core.AddOptions{Title: "anchor"}); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	model, _ = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model.input.SetValue("below anchor")
	model, _ = press(model, tea.KeyMsg{Type: tea.KeyEnter})

	model.selected = 0
	model, _ = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	model.input.SetValue("above anchor")
	model, _ = press(model, tea.KeyMsg{Type: tea.KeyEnter})

	got := make([]string, len(model.items))
	for idx, item := range model.items {
		got[idx] = item.Title
	}
	want := []string{"above anchor", "anchor", "below anchor"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
}

func TestTopBarAndFooterExposeNewControls(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	if _, err := store.Add(scope, core.AddOptions{Title: "open task"}); err != nil {
		t.Fatal(err)
	}
	done, err := store.Add(scope, core.AddOptions{Title: "done task"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetDone(scope, done.ID, true, false, false); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	model.width = 100
	top := plainText(model.renderTop())
	topLines := strings.Split(top, "\n")
	if len(topLines) != 5 {
		t.Fatalf("top bar line count = %d, want 5: %q", len(topLines), top)
	}
	if !strings.HasPrefix(topLines[0], "╭") || !strings.HasSuffix(topLines[0], "╮") ||
		!strings.HasPrefix(topLines[len(topLines)-1], "╰") || !strings.HasSuffix(topLines[len(topLines)-1], "╯") {
		t.Fatalf("top bar did not render rounded border: %q", top)
	}
	if !strings.Contains(top, "'||''|") ||
		!strings.Contains(top, "project: lune") ||
		!strings.Contains(top, "Views") ||
		!strings.Contains(top, "project lune") ||
		!strings.Contains(top, "Filters") ||
		!strings.Contains(top, "open") ||
		!strings.Contains(top, "todo: 1") ||
		!strings.Contains(top, "done: 1") {
		t.Fatalf("top bar = %q", top)
	}
	narrow := model
	narrow.width = 80
	narrow.scope.Project = "tui-header-controls-split"
	narrowTop := plainText(narrow.renderTop())
	if !strings.Contains(narrowTop, "Views") || !strings.Contains(narrowTop, "Filters") {
		t.Fatalf("narrow top bar dropped controls: %q", narrowTop)
	}
	footer := plainText(model.renderFooter())
	footerLines := strings.Split(footer, "\n")
	if len(footerLines) != 4 {
		t.Fatalf("footer line count = %d, want 4: %q", len(footerLines), footer)
	}
	for idx, line := range footerLines {
		if got := lipgloss.Width(line); got != model.width {
			t.Fatalf("footer line %d width = %d, want %d: %q", idx, got, model.width, footer)
		}
	}
	for _, want := range []string{"pg ^u/^d page", "a below", "A above", "y yank", "c codex", "t top", "x archive"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("footer missing %q: %q", want, footer)
		}
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	model = updated.(Model)
	if got := model.renderTop(); got != "" {
		t.Fatalf("top bar after hide = %q", got)
	}
}

func TestFooterHeightAndColorsReflectExpandedControls(t *testing.T) {
	model := Model{width: 90, height: 24}
	if got := model.footerHeight(); got != 4 {
		t.Fatalf("normal footer height = %d, want 4", got)
	}
	if got := model.bodyHeight(); got != 15 {
		t.Fatalf("body height = %d, want 15", got)
	}
	if got := model.bodyContentHeight(); got != 13 {
		t.Fatalf("body content height = %d, want 13", got)
	}
	if footerKeyStyleFor("/").GetForeground() != electricBlue {
		t.Fatalf("search key color = %q, want electric blue %q", footerKeyStyleFor("/").GetForeground(), electricBlue)
	}
	if footerKeyStyleFor("q").GetForeground() == footerKeyStyleFor("e").GetForeground() {
		t.Fatal("quit and edit footer keys should be color-coded differently")
	}
	if headingStyle.GetForeground() != electricBlue {
		t.Fatalf("heading color = %q, want electric blue %q", headingStyle.GetForeground(), electricBlue)
	}
	if sectionBoxStyle.GetBorderTopForeground() != cosmicViolet {
		t.Fatalf("section border color = %q, want violet %q", sectionBoxStyle.GetBorderTopForeground(), cosmicViolet)
	}

	model.status = "Saved."
	if got := model.footerHeight(); got != 3 {
		t.Fatalf("status footer height = %d, want 3", got)
	}
}

func TestYankCopiesTicketContextForProjectAgent(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	content := strings.Join([]string{
		"# lune todo",
		"",
		"- [ ] parent context",
		"<!-- rune:id=aaaa0000 type=task tags= created=2026-05-14T00:00:00Z -->",
		"    - [ ] selected ticket",
		"<!-- rune:id=bbbb0000 type=task tags=combat,bug created=2026-05-14T00:00:00Z -->",
		"      details for agent",
		"        - [ ] child detail",
		"<!-- rune:id=cccc0000 type=task tags= created=2026-05-14T00:00:00Z -->",
		"    - [ ] sibling should stay out",
		"<!-- rune:id=dddd0000 type=task tags= created=2026-05-14T00:00:00Z -->",
	}, "\n")
	path := core.ProjectPath(home, "lune")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	for idx, item := range model.items {
		if item.Title == "selected ticket" {
			model.selected = idx
			break
		}
	}
	oldWriteClipboard := writeClipboard
	oldTmuxSession := tmuxSession
	oldWriteTmuxBuffer := writeTmuxBuffer
	t.Cleanup(func() {
		writeClipboard = oldWriteClipboard
		tmuxSession = oldTmuxSession
		writeTmuxBuffer = oldWriteTmuxBuffer
	})
	var copied string
	writeClipboard = func(value string) error {
		copied = value
		return nil
	}
	tmuxSession = func() bool { return false }
	writeTmuxBuffer = func(string, string) error {
		t.Fatal("tmux buffer should not be loaded outside tmux")
		return nil
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = updated.(Model)

	for _, want := range []string{
		"# Rune Ticket: selected ticket",
		"- ID: bbb",
		"- Status: open",
		"- Heading: lune todo",
		"- Tags: #bug #combat",
		"- Source: projects/lune.md:5",
		"- [ ] parent context",
		"    - [ ] selected ticket",
		"      details for agent",
		"        - [ ] child detail",
		"implement this ticket, $lune-agent\n",
	} {
		if !strings.Contains(copied, want) {
			t.Fatalf("copied ticket missing %q:\n%s", want, copied)
		}
	}
	if strings.Contains(copied, "sibling should stay out") {
		t.Fatalf("copied ticket included sibling context:\n%s", copied)
	}
	if !strings.Contains(model.status, "Yanked bbb for $lune-agent.") {
		t.Fatalf("status = %q", model.status)
	}
}

func TestYankCopiesTicketToTmuxBufferWhenAvailable(t *testing.T) {
	home := t.TempDir()
	store := core.NewStore(home)
	scope := core.Scope{Home: home, Project: "lune"}
	if _, err := store.Add(scope, core.AddOptions{Title: "tmux handoff", Body: "detail for tmux"}); err != nil {
		t.Fatal(err)
	}
	model, err := New(store, scope)
	if err != nil {
		t.Fatal(err)
	}
	oldWriteClipboard := writeClipboard
	oldTmuxSession := tmuxSession
	oldWriteTmuxBuffer := writeTmuxBuffer
	t.Cleanup(func() {
		writeClipboard = oldWriteClipboard
		tmuxSession = oldTmuxSession
		writeTmuxBuffer = oldWriteTmuxBuffer
	})
	var copied string
	writeClipboard = func(value string) error {
		copied = value
		return nil
	}
	tmuxSession = func() bool { return true }
	var tmuxName, tmuxText string
	writeTmuxBuffer = func(name, value string) error {
		tmuxName = name
		tmuxText = value
		return nil
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = updated.(Model)

	if !strings.Contains(model.status, "tmux buffer ready") {
		t.Fatalf("status = %q", model.status)
	}
	if tmuxName != "rune-ticket" {
		t.Fatalf("tmux buffer = %q", tmuxName)
	}
	if copied == "" || copied != tmuxText {
		t.Fatalf("clipboard/tmux mismatch:\nclipboard=%q\ntmux=%q", copied, tmuxText)
	}
	if !strings.Contains(tmuxText, "# Rune Ticket: tmux handoff") || !strings.Contains(tmuxText, "detail for tmux") {
		t.Fatalf("tmux ticket = %q", tmuxText)
	}
}

func press(model Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	updated, cmd := model.Update(msg)
	return updated.(Model), cmd
}

func plainText(value string) string {
	return termansi.Strip(value)
}
