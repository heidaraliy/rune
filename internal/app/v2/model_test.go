package v2

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/heidaraliy/rune/internal/application"
	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
	runesync "github.com/heidaraliy/rune/internal/sync"
)

func testModel(t *testing.T) (Model, application.V2Service) {
	t.Helper()
	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	blobs, err := artifacts.Open(filepath.Join(root, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewV2ExecutionService(store, "local", blobs)
	model, err := New(service, "local", "rune")
	if err != nil {
		t.Fatal(err)
	}
	return model, service
}

func press(model Model, msg tea.Msg) Model {
	updated, _ := model.Update(msg)
	return updated.(Model)
}

func runCommand(t *testing.T, model Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("command is nil")
	}
	updated, _ := model.Update(cmd())
	return updated.(Model)
}

func TestModelRendersWorkspaceLinksWithinCompactWidth(t *testing.T) {
	model, service := testModel(t)
	ctx := context.Background()
	note, err := service.Create(ctx, domain.Entity{
		Kind:     domain.KindNote,
		Project:  "rune",
		Title:    "architecture map",
		Body:     "shared context",
		FacetSet: []domain.RuneFacet{"proposal"},
		FacetProperties: map[domain.RuneFacet]map[string]string{
			"proposal": {"decision": "pending"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.Create(ctx, domain.Entity{Kind: domain.KindTask, Project: "rune", Title: "connect ideas", Status: domain.StatusReady})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Link(ctx, task.ID, note.ID, "references"); err != nil {
		t.Fatal(err)
	}
	otherTask, err := service.Create(ctx, domain.Entity{Kind: domain.KindTask, Project: "other", Title: "other project run", Status: domain.StatusReady})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.QueueRun(ctx, otherTask.ID, "fake", "local", domain.PermissionReadOnly); err != nil {
		t.Fatal(err)
	}
	if err := model.reloadKeeping(task.ID); err != nil {
		t.Fatal(err)
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if len(model.runs) != 0 {
		t.Fatalf("runs from another project leaked into view: %#v", model.runs)
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if err := model.reloadKeeping(note.ID); err != nil {
		t.Fatal(err)
	}
	model = press(model, tea.WindowSizeMsg{Width: 72, Height: 20})
	view := model.View()
	for _, want := range []string{"rune", "conne", "archi", "references", "proposal", "decision=pending", "workspace local"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	lines := strings.Split(view, "\n")
	if len(lines) != 20 {
		t.Fatalf("view lines = %d, want 20", len(lines))
	}
	for index, line := range lines {
		if got := lipgloss.Width(line); got > 72 {
			t.Fatalf("line %d width = %d, want <= 72: %q", index, got, line)
		}
	}
	model = press(model, tea.WindowSizeMsg{Width: 52, Height: 20})
	narrow := model.View()
	if !strings.Contains(narrow, "references") {
		t.Fatalf("narrow view hid links:\n%s", narrow)
	}
	for index, line := range strings.Split(narrow, "\n") {
		if got := lipgloss.Width(line); got > 52 {
			t.Fatalf("narrow line %d width = %d, want <= 52: %q", index, got, line)
		}
	}
}

func TestModelVisualLayoutFitsCommonTerminalWidths(t *testing.T) {
	model, service := testModel(t)
	ctx := context.Background()
	task, err := service.Create(ctx, domain.Entity{
		Kind:    domain.KindTask,
		Project: "rune",
		Title:   "polish the terminal workspace",
		Body:    strings.Repeat("Keep the inbox calm, readable, and useful. ", 8),
		Status:  domain.StatusReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, domain.Entity{
		Kind:    domain.KindNote,
		Project: "rune",
		Title:   "shared visual language",
		Body:    "The terminal and browser should feel like the same workspace.",
	}); err != nil {
		t.Fatal(err)
	}
	run, err := service.QueueRun(ctx, task.ID, "fake", "local", domain.PermissionReadOnly)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteRun(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	if err := model.reloadKeeping(task.ID); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		width  int
		height int
	}{
		{name: "wide", width: 120, height: 32},
		{name: "desktop", width: 96, height: 24},
		{name: "split", width: 72, height: 24},
		{name: "compact", width: 52, height: 20},
	} {
		t.Run(test.name, func(t *testing.T) {
			sized := press(model, tea.WindowSizeMsg{Width: test.width, Height: test.height})
			view := sized.View()
			if !strings.Contains(view, "YOUR RUNES") || !strings.Contains(view, "Lifecycle") {
				t.Fatalf("workspace view lost its hierarchy:\n%s", view)
			}
			if test.width >= 64 && !strings.Contains(view, "WHAT SHOULD RUNE REMEMBER?") {
				t.Fatalf("workspace view lost quick capture:\n%s", view)
			}
			if test.width >= 80 && !strings.Contains(view, "'||''|") {
				t.Fatalf("workspace view lost the Rune wordmark:\n%s", view)
			}
			if lines := strings.Split(view, "\n"); len(lines) != test.height {
				t.Fatalf("view lines = %d, want %d:\n%s", len(lines), test.height, view)
			}
			for index, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > test.width {
					t.Fatalf("line %d width = %d, want <= %d: %q", index, got, test.width, line)
				}
			}
		})
	}

	model = press(model, tea.WindowSizeMsg{Width: 96, Height: 24})
	for _, test := range []struct {
		key  rune
		want string
	}{
		{key: '2', want: "EXECUTION"},
		{key: '3', want: "ARTIFACTS"},
		{key: '4', want: "LOCAL SYNC"},
	} {
		model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{test.key}})
		view := model.View()
		if !strings.Contains(view, test.want) {
			t.Fatalf("view %q missing %q:\n%s", string(test.key), test.want, view)
		}
	}
}

func TestModelCaptureQueueRunAndInspectArtifactFlow(t *testing.T) {
	model, service := testModel(t)
	model = press(model, tea.WindowSizeMsg{Width: 96, Height: 24})
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("build local loop")})
	model = press(model, tea.KeyMsg{Type: tea.KeyEnter})
	if len(model.entities) != 1 || model.entities[0].Title != "build local loop" {
		t.Fatalf("captured entities = %#v", model.entities)
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if model.entities[0].Status != domain.StatusQueued {
		t.Fatalf("queued entity = %#v", model.entities[0])
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if len(model.runs) != 1 || model.runs[0].Status != domain.RunStatusQueued {
		t.Fatalf("queued runs = %#v", model.runs)
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if model.runs[0].Status != domain.RunStatusCompleted || !strings.Contains(model.runs[0].Summary, "build local loop") {
		t.Fatalf("completed runs = %#v", model.runs)
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if len(model.artifacts) != 2 {
		t.Fatalf("artifacts = %#v", model.artifacts)
	}
	view := model.View()
	if !strings.Contains(view, "context.json") || !strings.Contains(view, "result.md") {
		t.Fatalf("artifact view =\n%s", view)
	}
	if _, err := service.Get(context.Background(), model.entities[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestModelEditsAndConfirmsReversibleTombstones(t *testing.T) {
	model, _ := testModel(t)
	model = press(model, tea.WindowSizeMsg{Width: 96, Height: 24})
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("original title")})
	model = press(model, tea.KeyMsg{Type: tea.KeyEnter})

	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = press(model, tea.KeyMsg{Type: tea.KeyCtrlA})
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("edited title")})
	model = press(model, tea.KeyMsg{Type: tea.KeyEnter})
	if len(model.entities) != 1 || model.entities[0].Title != "edited title" || model.entities[0].Revision != 2 {
		t.Fatalf("edited entity = %#v", model.entities)
	}

	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	model = press(model, tea.KeyMsg{Type: tea.KeyCtrlA})
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("edited body")})
	model = press(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.entities[0].Body != "edited body" || model.entities[0].Revision != 3 {
		t.Fatalf("edited body entity = %#v", model.entities[0])
	}

	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if model.entities[0].State != domain.StateReady || model.entities[0].Revision != 4 {
		t.Fatalf("advanced Rune state = %#v", model.entities[0])
	}

	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !strings.Contains(model.View(), "Tombstone selected item?") {
		t.Fatalf("delete confirmation missing:\n%s", model.View())
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if model.entities[0].DeletedAt != nil {
		t.Fatal("delete cancellation changed entity")
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if model.entities[0].DeletedAt == nil || !strings.Contains(model.View(), "reversible tombstone") {
		t.Fatalf("tombstoned entity = %#v\n%s", model.entities, model.View())
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if model.entities[0].DeletedAt != nil || model.entities[0].Title != "edited title" {
		t.Fatalf("restored entity = %#v", model.entities[0])
	}
}

func TestModelSearchHelpAndSyncViewAreDiscoverable(t *testing.T) {
	model, service := testModel(t)
	model = press(model, tea.WindowSizeMsg{Width: 96, Height: 24})
	note, err := service.Create(context.Background(), domain.Entity{Kind: domain.KindNote, Project: "rune", Title: "searchable note"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), domain.Entity{Kind: domain.KindTask, Project: "rune", Title: "other task", Status: domain.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	if err := model.reload(); err != nil {
		t.Fatal(err)
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("searchable")})
	model = press(model, tea.KeyMsg{Type: tea.KeyEnter})
	if len(model.entities) != 1 || model.entities[0].Title != "searchable note" {
		t.Fatalf("search results = %#v", model.entities)
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !strings.Contains(model.View(), "Rune command guide") || !strings.Contains(model.View(), "queue selected task") {
		t.Fatalf("help view =\n%s", model.View())
	}
	if _, err := service.RecordConflict(context.Background(), domain.Conflict{
		EntityID:       note.ID,
		Kind:           "entity.created",
		LocalRevision:  1,
		RemoteRevision: 1,
		LocalPayload:   `{"title":"searchable note"}`,
		RemotePayload:  `{"title":"remote note"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := model.reload(); err != nil {
		t.Fatal(err)
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if !strings.Contains(model.View(), "LOCAL SYNC") || !strings.Contains(model.View(), "not-configured") || !strings.Contains(model.View(), "none") || !strings.Contains(model.View(), "pushed") || !strings.Contains(model.View(), "Edits are revision-checked") || !strings.Contains(model.View(), domain.DisplayID(note.ID)) || !strings.Contains(model.View(), "LOCAL PAYLOAD") || !strings.Contains(model.View(), "remote note") {
		t.Fatalf("sync view =\n%s", model.View())
	}
}

func TestModelSyncNowUsesInjectedTargetAndReportsCompletion(t *testing.T) {
	_, service := testModel(t)
	if _, err := service.Create(context.Background(), domain.Entity{
		Kind:    domain.KindNote,
		Project: "rune",
		Title:   "sync from TUI",
	}); err != nil {
		t.Fatal(err)
	}
	peer, err := runesync.OpenFilePeer(filepath.Join(t.TempDir(), "remote"))
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	model, err := NewWithOptions(service, "local", "rune", Options{SyncTarget: peer})
	if err != nil {
		t.Fatal(err)
	}
	model = press(model, tea.WindowSizeMsg{Width: 96, Height: 24})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = updated.(Model)
	if !model.syncBusy {
		t.Fatal("sync did not enter the busy state")
	}
	model = runCommand(t, model, cmd)
	if model.syncBusy {
		t.Fatal("sync remained busy after completion")
	}
	if model.syncError != "" || !strings.Contains(model.status, "Synced") {
		t.Fatalf("sync result status=%q error=%q", model.status, model.syncError)
	}
	if model.sync.RemoteID != peer.ID() || model.sync.PushedCursor == 0 {
		t.Fatalf("sync status = %#v, peer=%q", model.sync, peer.ID())
	}

	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	view := model.View()
	if !strings.Contains(view, "ready") || !strings.Contains(view, "remote id") || !strings.Contains(view, "file:") {
		t.Fatalf("configured sync view =\n%s", view)
	}
}

func TestModelSyncNowReportsOfflineWithoutLosingLocalState(t *testing.T) {
	_, service := testModel(t)
	peer, err := runesync.OpenFilePeer(filepath.Join(t.TempDir(), "remote"))
	if err != nil {
		t.Fatal(err)
	}
	model, err := NewWithOptions(service, "local", "rune", Options{SyncTarget: peer})
	if err != nil {
		t.Fatal(err)
	}
	model = press(model, tea.WindowSizeMsg{Width: 96, Height: 24})
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = updated.(Model)
	model = runCommand(t, model, cmd)
	if !strings.Contains(model.status, "Sync offline:") || model.syncError == "" {
		t.Fatalf("offline sync status=%q error=%q", model.status, model.syncError)
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if !strings.Contains(model.View(), "offline") || !strings.Contains(model.View(), "last error:") {
		t.Fatalf("offline sync view =\n%s", model.View())
	}
}

func TestModelAutoSyncRunsOnceAtStartupAndKindFilterCycles(t *testing.T) {
	_, service := testModel(t)
	if _, err := service.Create(context.Background(), domain.Entity{Kind: domain.KindTask, Project: "rune", Title: "task"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), domain.Entity{Kind: domain.KindNote, Project: "rune", Title: "note"}); err != nil {
		t.Fatal(err)
	}
	peer, err := runesync.OpenFilePeer(filepath.Join(t.TempDir(), "remote"))
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	model, err := NewWithOptions(service, "local", "rune", Options{SyncTarget: peer, AutoSync: true})
	if err != nil {
		t.Fatal(err)
	}
	model = press(model, tea.WindowSizeMsg{Width: 96, Height: 24})
	if !model.syncBusy {
		t.Fatal("auto-sync did not mark startup sync busy")
	}
	model = runCommand(t, model, model.Init())
	if model.syncBusy || !strings.Contains(model.status, "Synced") {
		t.Fatalf("auto-sync result status=%q busy=%v", model.status, model.syncBusy)
	}

	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if len(model.entities) != 1 || !model.entities[0].IsTask() || !strings.Contains(model.View(), "filter: tasks") {
		t.Fatalf("task filter entities=%#v view=\n%s", model.entities, model.View())
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if len(model.entities) != 1 || model.entities[0].Kind != domain.KindNote || !strings.Contains(model.View(), "filter: notes") {
		t.Fatalf("note filter entities=%#v view=\n%s", model.entities, model.View())
	}
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if len(model.entities) != 2 || !strings.Contains(model.View(), "filter: all") {
		t.Fatalf("all filter entities=%#v view=\n%s", model.entities, model.View())
	}
}
