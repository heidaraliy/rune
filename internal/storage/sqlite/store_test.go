package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestOpenUpgradesSlice4DatabaseToSlice5Schema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rune-v2.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)",
		`CREATE TABLE entities (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			body TEXT NOT NULL DEFAULT '',
			heading TEXT NOT NULL DEFAULT '',
			tags_json TEXT NOT NULL DEFAULT '[]',
			properties_json TEXT NOT NULL DEFAULT '{}',
			status TEXT NOT NULL DEFAULT '',
			priority INTEGER NOT NULL DEFAULT 0,
			parent_id TEXT,
			source_note_id TEXT,
			legacy_id TEXT NOT NULL DEFAULT '',
			legacy_source TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			finished_at TEXT,
			revision INTEGER NOT NULL DEFAULT 1,
			deleted_at TEXT
		)`,
		`INSERT INTO entities(id, kind, workspace_id, title, created_at, updated_at)
			VALUES ('old-a', 'note', 'local', 'old a', '2026-07-01T00:00:00Z', '2026-07-01T00:00:00Z'),
			       ('old-b', 'note', 'local', 'old b', '2026-07-02T00:00:00Z', '2026-07-02T00:00:00Z')`,
		"INSERT INTO schema_migrations(version, applied_at) VALUES (4, '2026-07-01T00:00:00Z')",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("schema version = %d, want 5", version)
	}
	var firstOrder, secondOrder int
	if err := store.db.QueryRow("SELECT sibling_order FROM entities WHERE id='old-a'").Scan(&firstOrder); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT sibling_order FROM entities WHERE id='old-b'").Scan(&secondOrder); err != nil {
		t.Fatal(err)
	}
	if firstOrder != 1 || secondOrder != 2 {
		t.Fatalf("migrated sibling orders = %d, %d; want 1, 2", firstOrder, secondOrder)
	}
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
	if got.SiblingOrder != 1 {
		t.Fatalf("task sibling order = %d, want 1", got.SiblingOrder)
	}
}

func TestRuneQueryFiltersAndOrdersChildren(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	parent, err := store.Create(ctx, domain.Rune{Kind: domain.KindNote, WorkspaceID: "local", Title: "parent"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Create(ctx, domain.Rune{Kind: domain.KindTask, WorkspaceID: "local", ParentID: domain.RuneReference(parent.ID), Title: "first", Status: domain.StatusReady})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(ctx, domain.Rune{Kind: domain.KindTask, WorkspaceID: "local", ParentID: parent.ID, Title: "second", Status: domain.StatusReady})
	if err != nil {
		t.Fatal(err)
	}
	children, err := store.List(ctx, domain.RuneQuery{WorkspaceID: "local", ParentID: domain.RuneReference(parent.ID), SortBy: domain.RuneSortOrder, Reverse: false})
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 || children[0].ID != first.ID || children[1].ID != second.ID {
		t.Fatalf("children = %#v", children)
	}
	if children[0].SiblingOrder != 1 || children[1].SiblingOrder != 2 {
		t.Fatalf("child order = %#v", children)
	}
	limited, err := store.List(ctx, domain.RuneQuery{WorkspaceID: "local", ParentID: parent.ID, SortBy: domain.RuneSortOrder, Limit: 1, Offset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0].ID != second.ID {
		t.Fatalf("limited children = %#v", limited)
	}
}

func TestRuneParentReferencesRejectCyclesAndAcceptStableRefs(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	parent, err := store.Create(ctx, domain.Rune{Kind: domain.KindNote, WorkspaceID: "local", Title: "parent"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := store.Create(ctx, domain.Rune{Kind: domain.KindTask, WorkspaceID: "local", ParentID: parent.ID, Title: "child", Status: domain.StatusReady})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.Get(ctx, domain.RuneReference(child.ID), "local"); err != nil || got.ID != child.ID {
		t.Fatalf("get by Rune ref = %#v, err=%v", got, err)
	}
	parentID := child.ID
	if _, err := store.Update(ctx, parent.ID, "local", domain.RuneUpdate{ExpectedRevision: parent.Revision, ParentID: &parentID}); err == nil || !strings.Contains(err.Error(), "cycles") {
		t.Fatalf("cycle update error = %v", err)
	}
}

func TestSetTaskStatusUsesOptimisticRevisionAndStatusTimestamps(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	task, err := store.Create(ctx, domain.Entity{Kind: domain.KindTask, WorkspaceID: "local", Title: "ship", Status: domain.StatusDraft})
	if err != nil {
		t.Fatal(err)
	}
	status := domain.StatusCompleted
	updated, err := store.SetTaskStatus(ctx, task.ID, "local", status, 1)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Status != domain.StatusCompleted || updated.FinishedAt == nil {
		t.Fatalf("updated = %#v", updated)
	}
	changes, err := store.ListChanges(ctx, "local", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || changes[0].Kind != "entity.created" || changes[1].Kind != "entity.status" {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestUpdateEditsAuthoredFieldsAndRecordsRevision(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	note, err := store.Create(ctx, domain.Entity{Kind: domain.KindNote, WorkspaceID: "local", Title: "before", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	title := "after"
	body := "new body"
	updated, err := store.Update(ctx, note.ID, "local", domain.Update{ExpectedRevision: 1, Title: &title, Body: &body})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != title || updated.Body != body || updated.Revision != 2 {
		t.Fatalf("updated = %#v", updated)
	}
	changes, err := store.ListChanges(ctx, "local", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || changes[1].Kind != "entity.updated" || changes[1].Revision != 2 {
		t.Fatalf("changes = %#v", changes)
	}
	if _, err := store.Update(ctx, note.ID, "local", domain.Update{ExpectedRevision: 1, Title: &title}); err == nil || !strings.Contains(err.Error(), "revision conflict") {
		t.Fatalf("stale edit should conflict: %v", err)
	}
}

func TestDeleteAndRestoreUseReversibleTombstones(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	note, err := store.Create(ctx, domain.Entity{Kind: domain.KindNote, WorkspaceID: "local", Title: "keep me", Body: "preserve this"})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := store.Delete(ctx, note.ID, "local", 1)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.DeletedAt == nil || deleted.Revision != 2 {
		t.Fatalf("deleted = %#v", deleted)
	}
	if _, err := store.Get(ctx, note.ID, "local"); err == nil {
		t.Fatal("tombstoned item should be hidden from normal get")
	}
	withDeleted, err := store.GetIncludingDeleted(ctx, note.ID, "local")
	if err != nil || withDeleted.DeletedAt == nil || withDeleted.Body != note.Body {
		t.Fatalf("tombstone = %#v, err=%v", withDeleted, err)
	}
	restored, err := store.Restore(ctx, note.ID, "local", 2)
	if err != nil {
		t.Fatal(err)
	}
	if restored.DeletedAt != nil || restored.Revision != 3 || restored.Title != note.Title || restored.Body != note.Body {
		t.Fatalf("restored = %#v", restored)
	}
	changes, err := store.ListChanges(ctx, "local", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 || changes[1].Kind != "entity.deleted" || changes[2].Kind != "entity.restored" {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestApplyRemoteChangeIsIdempotentAndConflictSafe(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	note, err := store.Create(ctx, domain.Entity{Kind: domain.KindNote, WorkspaceID: "local", Title: "shared"})
	if err != nil {
		t.Fatal(err)
	}

	remote := note
	remote.Title = "remote"
	remote.Revision = 2
	remote.UpdatedAt = time.Now().UTC()
	payload, err := json.Marshal(remote)
	if err != nil {
		t.Fatal(err)
	}
	change := domain.Change{
		ID:          "remote-change-1",
		WorkspaceID: "local",
		OperationID: "remote-operation-1",
		ActorID:     "other-actor",
		DeviceID:    "other-device",
		Kind:        "entity.updated",
		EntityID:    note.ID,
		Revision:    remote.Revision,
		Payload:     string(payload),
		CreatedAt:   remote.UpdatedAt,
		Origin:      domain.ChangeOriginRemote,
	}
	if _, conflict, err := store.ApplyRemoteChange(ctx, change); err != nil || conflict != nil {
		t.Fatalf("apply remote change = conflict %v, err %v", conflict, err)
	}
	updated, err := store.Get(ctx, note.ID, "local")
	if err != nil || updated.Title != "remote" || updated.Revision != 2 {
		t.Fatalf("updated entity = %#v, err=%v", updated, err)
	}
	if _, conflict, err := store.ApplyRemoteChange(ctx, change); err != nil || conflict != nil {
		t.Fatalf("replay remote change = conflict %v, err %v", conflict, err)
	}
	changes, err := store.ListChanges(ctx, "local", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || changes[1].Origin != domain.ChangeOriginRemote {
		t.Fatalf("changes after replay = %#v", changes)
	}

	title := "local"
	if _, err := store.Update(ctx, note.ID, "local", domain.Update{ExpectedRevision: 2, Title: &title}); err != nil {
		t.Fatal(err)
	}
	remote.Title = "competing remote"
	remote.Revision = 3
	remote.UpdatedAt = time.Now().UTC()
	payload, err = json.Marshal(remote)
	if err != nil {
		t.Fatal(err)
	}
	conflictChange := change
	conflictChange.ID = "remote-change-2"
	conflictChange.OperationID = "remote-operation-2"
	conflictChange.Revision = remote.Revision
	conflictChange.Payload = string(payload)
	conflictChange.CreatedAt = remote.UpdatedAt
	_, conflict, err := store.ApplyRemoteChange(ctx, conflictChange)
	if err != nil || conflict == nil {
		t.Fatalf("conflicting remote change = %#v, err=%v", conflict, err)
	}
	local, err := store.Get(ctx, note.ID, "local")
	if err != nil || local.Title != "local" || local.Revision != 3 {
		t.Fatalf("local entity after conflict = %#v, err=%v", local, err)
	}
	conflicts, err := store.ListConflicts(ctx, "local")
	if err != nil || len(conflicts) != 1 {
		t.Fatalf("conflicts = %#v, err=%v", conflicts, err)
	}
}

func TestSyncStatusAndConflictsAreVisibleWithoutMutatingEntities(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	note, err := store.Create(ctx, domain.Entity{Kind: domain.KindNote, WorkspaceID: "local", Title: "immutable note", Body: "original"})
	if err != nil {
		t.Fatal(err)
	}
	conflict, err := store.RecordConflict(ctx, domain.Conflict{
		WorkspaceID:    "local",
		EntityID:       note.ID,
		Kind:           "entity.created",
		LocalRevision:  1,
		RemoteRevision: 1,
		LocalPayload:   `{"title":"immutable note","body":"original"}`,
		RemotePayload:  `{"title":"immutable note","body":"remote variant"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := store.SyncStatus(ctx, "local")
	if err != nil {
		t.Fatal(err)
	}
	if status.LocalCursor != 1 || status.PendingChanges != 1 || status.OpenConflicts != 1 || status.RemoteState != "not-configured" {
		t.Fatalf("sync status = %#v", status)
	}
	conflicts, err := store.ListConflicts(ctx, "local")
	if err != nil || len(conflicts) != 1 || conflicts[0].ID != conflict.ID {
		t.Fatalf("conflicts = %#v, err=%v", conflicts, err)
	}
	got, err := store.Get(ctx, note.ID, "local")
	if err != nil || got.Title != note.Title || got.Body != note.Body || got.Revision != note.Revision {
		t.Fatalf("note changed while recording conflict: %#v, err=%v", got, err)
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

func TestQueueRunTransitionsTaskAndPersistsEventsAndArtifacts(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	task, err := store.Create(ctx, domain.Entity{Kind: domain.KindTask, WorkspaceID: "local", Title: "execute me", Status: domain.StatusReady})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	artifactID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("b", 64)
	run, artifact, err := store.QueueRun(ctx, domain.Run{ID: runID, WorkspaceID: "local", TaskID: task.ID, Provider: "fake", Status: domain.RunStatusQueued, PermissionPolicy: domain.PermissionReadOnly, ContextSnapshot: `{"schema":"rune.context.v1"}`}, &domain.Artifact{
		ID: artifactID, WorkspaceID: "local", RunID: runID, EntityID: task.ID, Kind: "context", Name: "context.json", MediaType: "application/json", SizeBytes: 32, SHA256: hash, StorageKey: "sha256/bb/" + hash, Retention: "permanent", SecretState: "clear",
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.ContextArtifactID != artifact.ID || artifact.ID != artifactID {
		t.Fatalf("run/artifact = %#v / %#v", run, artifact)
	}
	queuedTask, err := store.Get(ctx, task.ID, "local")
	if err != nil || queuedTask.Status != domain.StatusQueued || queuedTask.Revision != 2 {
		t.Fatalf("queued task = %#v, err=%v", queuedTask, err)
	}
	if _, err := store.SetTaskStatus(ctx, task.ID, "local", domain.StatusReady, 0); err == nil {
		t.Fatal("active run should own task status")
	}
	for _, status := range []domain.RunStatus{domain.RunStatusRunning, domain.RunStatusReview, domain.RunStatusCompleted} {
		if _, err := store.SetRunStatus(ctx, run.ID, "local", status, "done", ""); err != nil {
			t.Fatalf("status %s: %v", status, err)
		}
	}
	got, err := store.GetRun(ctx, run.ID, "local")
	if err != nil || got.Status != domain.RunStatusCompleted || got.FinishedAt == nil || got.Revision != 4 {
		t.Fatalf("completed run = %#v, err=%v", got, err)
	}
	events, err := store.ListRunEvents(ctx, run.ID, "local")
	if err != nil || len(events) != 4 {
		t.Fatalf("events = %#v, err=%v", events, err)
	}
	for index, event := range events {
		if event.Sequence != int64(index+1) {
			t.Fatalf("event sequence = %#v", events)
		}
	}
	artifacts, err := store.ListArtifacts(ctx, "local", run.ID)
	if err != nil || len(artifacts) != 1 || artifacts[0].Kind != "context" {
		t.Fatalf("artifacts = %#v, err=%v", artifacts, err)
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
