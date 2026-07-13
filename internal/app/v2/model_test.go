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

func TestModelSearchHelpAndSyncViewAreDiscoverable(t *testing.T) {
	model, service := testModel(t)
	model = press(model, tea.WindowSizeMsg{Width: 96, Height: 24})
	if _, err := service.Create(context.Background(), domain.Entity{Kind: domain.KindNote, Project: "rune", Title: "searchable note"}); err != nil {
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
	model = press(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if !strings.Contains(model.View(), "Sync is not connected in Slice 4") || !strings.Contains(model.View(), "Slice 5") {
		t.Fatalf("sync view =\n%s", model.View())
	}
}
