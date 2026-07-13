package application

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

func TestLocalExecutionQueuesRunsAndStoresContextAndResult(t *testing.T) {
	root := t.TempDir()
	db, err := sqlite.Open(filepath.Join(root, "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	blobs, err := artifacts.Open(filepath.Join(root, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewV2ExecutionService(db, "local", blobs)
	task, err := service.Create(context.Background(), domain.Entity{Kind: domain.KindTask, Project: "rune", Title: "local run", Body: "inspect output"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.QueueRun(context.Background(), task.ID, "fake", "local", domain.PermissionReadOnly)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != domain.RunStatusQueued || run.ContextArtifactID == "" || !strings.Contains(run.ContextSnapshot, "rune.context.v1") {
		t.Fatalf("queued run = %#v", run)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != domain.RunStatusCompleted || completed.Summary == "" {
		t.Fatalf("completed run = %#v", completed)
	}
	items, err := service.Artifacts(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("artifacts = %#v", items)
	}
	var resultID string
	for _, item := range items {
		if item.Kind == "result" {
			resultID = item.ID
		}
	}
	if resultID == "" {
		t.Fatalf("missing result artifact: %#v", items)
	}
	artifact, content, err := service.ReadArtifact(context.Background(), resultID)
	if err != nil || artifact.Kind != "result" || !strings.Contains(string(content), "local run") {
		t.Fatalf("result = %#v, content=%q, err=%v", artifact, content, err)
	}
	events, err := service.RunEvents(context.Background(), run.ID)
	if err != nil || len(events) != 5 {
		t.Fatalf("events = %#v, err=%v", events, err)
	}
	changes, err := service.Changes(context.Background(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	seenKinds := make(map[string]bool)
	for _, change := range changes {
		seenKinds[change.Kind] = true
	}
	for _, kind := range []string{"entity.created", "run.created", "artifact.created", "entity.status", "run.status", "run.event"} {
		if !seenKinds[kind] {
			t.Fatalf("sync ledger missing %q: %#v", kind, changes)
		}
	}
}

func TestLocalExecutionRecordsProviderFailure(t *testing.T) {
	root := t.TempDir()
	db, err := sqlite.Open(filepath.Join(root, "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	blobs, err := artifacts.Open(filepath.Join(root, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewV2ExecutionService(db, "local", blobs)
	task, err := service.Create(context.Background(), domain.Entity{Kind: domain.KindTask, Title: "failure path"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.QueueRun(context.Background(), task.ID, "fake-fail", "local", domain.PermissionReadOnly)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := service.ExecuteRun(context.Background(), run.ID)
	if err == nil || failed.Status != domain.RunStatusFailed {
		t.Fatalf("failed run = %#v, err=%v", failed, err)
	}
	task, err = service.Get(context.Background(), task.ID)
	if err != nil || task.Status != domain.StatusFailed {
		t.Fatalf("failed task = %#v, err=%v", task, err)
	}
}
