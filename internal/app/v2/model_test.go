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

func TestModelRendersWorkspaceLinksWithinCompactWidth(t *testing.T) {
	model, service := testModel(t)
	ctx := context.Background()
	note, err := service.Create(ctx, domain.Entity{Kind: domain.KindNote, Project: "rune", Title: "architecture map", Body: "shared context"})
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
	model = press(model, tea.WindowSizeMsg{Width: 72, Height: 20})
	view := model.View()
	for _, want := range []string{"Rune 2", "connect ideas", "archi", "references", "workspace:local"} {
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
	if !strings.Contains(model.View(), "Rune 2 keyboard guide") || !strings.Contains(model.View(), "queue selected task") {
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
	if !strings.Contains(model.View(), "LOCAL SYNC") || !strings.Contains(model.View(), "remote: not-configured") || !strings.Contains(model.View(), "remote id: none") || !strings.Contains(model.View(), "pushed cursor: 0") || !strings.Contains(model.View(), "Edits are revision-checked") || !strings.Contains(model.View(), "CONFLICTS") || !strings.Contains(model.View(), domain.DisplayID(note.ID)) {
		t.Fatalf("sync view =\n%s", model.View())
	}
}
