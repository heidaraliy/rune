package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/markdown"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rune-v2.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestOpenMigratesAndPersistsTypedEntities(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	note, err := store.Create(ctx, domain.Entity{Kind: domain.KindNote, WorkspaceID: "local", Project: "rune", Title: "architecture", Body: "details"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := store.Create(ctx, domain.Entity{Kind: domain.KindTask, WorkspaceID: "local", Project: "rune", Title: "build store", Status: domain.StatusReady, ParentID: note.ID})
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.List(ctx, domain.ListOptions{WorkspaceID: "local", Project: "rune"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Title != "build store" {
		t.Fatalf("items = %#v", items)
	}
	got, err := store.Get(ctx, domain.DisplayID(task.ID), "local")
	if err != nil || got.ID != task.ID {
		t.Fatalf("get = %#v, err=%v", got, err)
	}
	if got.Revision != 1 || got.ParentID != note.ID {
		t.Fatalf("task = %#v", got)
	}
}

func TestUpdateUsesOptimisticRevisionAndStatusTimestamps(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	task, err := store.Create(ctx, domain.Entity{Kind: domain.KindTask, WorkspaceID: "local", Title: "ship", Status: domain.StatusDraft})
	if err != nil {
		t.Fatal(err)
	}
	status := domain.StatusCompleted
	updated, err := store.Update(ctx, task.ID, "local", domain.Update{ExpectedRevision: 1, Status: &status})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Status != domain.StatusCompleted || updated.FinishedAt == nil {
		t.Fatalf("updated = %#v", updated)
	}
	if _, err := store.Update(ctx, task.ID, "local", domain.Update{ExpectedRevision: 1, Title: stringUpdate("stale")}); err == nil {
		t.Fatal("stale revision should fail")
	}
}

func TestLinksAreTypedAndDuplicateSafe(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	a, err := store.Create(ctx, domain.Entity{Kind: domain.KindNote, WorkspaceID: "local", Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.Create(ctx, domain.Entity{Kind: domain.KindTask, WorkspaceID: "local", Title: "b"})
	if err != nil {
		t.Fatal(err)
	}
	link := domain.Link{WorkspaceID: "local", FromID: a.ID, ToID: b.ID, Kind: "references"}
	if _, err := store.CreateLink(ctx, link); err != nil {
		t.Fatal(err)
	}
	links, err := store.ListLinks(ctx, "local", a.ID)
	if err != nil || len(links) != 1 || links[0].Kind != "references" {
		t.Fatalf("links = %#v, err=%v", links, err)
	}
	if _, err := store.CreateLink(ctx, link); err == nil {
		t.Fatal("duplicate link should fail")
	}
}

func TestCreateRejectsCrossWorkspaceReferences(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	parent, err := store.Create(ctx, domain.Entity{Kind: domain.KindNote, WorkspaceID: "one", Title: "parent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, domain.Entity{Kind: domain.KindTask, WorkspaceID: "two", Title: "child", ParentID: parent.ID}); err == nil {
		t.Fatal("cross-workspace parent should fail")
	}
}

func TestImportIsSourcePreservingAndIdempotent(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "ideas.md")
	original := "# ideas\n\n- [ ] first\n<!-- rune:id=first001 type=task created=2026-07-01T00:00:00Z -->\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle, err := markdown.ReadFile(path, "ideas", time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Import(ctx, "local", bundle)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Import(ctx, "local", bundle)
	if err != nil {
		t.Fatal(err)
	}
	if first.Created != 1 || first.Skipped != 0 || second.Created != 0 || second.Skipped != 1 {
		t.Fatalf("reports = %#v / %#v", first, second)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatal("import changed source file")
	}
	items, err := store.List(ctx, domain.ListOptions{WorkspaceID: "local"})
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %#v, err=%v", items, err)
	}
}

func stringUpdate(value string) *string { return &value }
