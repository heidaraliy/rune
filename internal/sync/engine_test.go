package runesync

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

func TestFilePeerSyncsEntitiesArtifactsAndConflicts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	peer, err := OpenFilePeer(filepath.Join(root, "remote"))
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	storeA, err := sqlite.Open(filepath.Join(root, "a", "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer storeA.Close()
	blobsA, err := artifacts.Open(filepath.Join(root, "a", "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	entity, err := storeA.Create(ctx, domain.Entity{Kind: domain.KindNote, WorkspaceID: "local", Title: "shared idea", Body: "from A"})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("artifact from A")
	blob, err := blobsA.Put(ctx, content)
	if err != nil {
		t.Fatal(err)
	}
	artifactID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storeA.CreateArtifact(ctx, domain.Artifact{
		ID:          artifactID,
		WorkspaceID: "local",
		EntityID:    entity.ID,
		Kind:        "note",
		Name:        "evidence.txt",
		MediaType:   "text/plain",
		SizeBytes:   blob.SizeBytes,
		SHA256:      blob.SHA256,
		StorageKey:  blob.StorageKey,
		Retention:   "normal",
		SecretState: "clear",
		CreatedAt:   time.Now().UTC(),
		Revision:    1,
	}); err != nil {
		t.Fatal(err)
	}

	report, err := (Engine{Store: storeA, Artifacts: blobsA, WorkspaceID: "local", Peer: peer}).Sync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Pushed != 2 || report.Pulled != 2 || report.Status.PendingChanges != 0 {
		t.Fatalf("first sync report = %#v", report)
	}

	storeB, err := sqlite.Open(filepath.Join(root, "b", "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer storeB.Close()
	blobsB, err := artifacts.Open(filepath.Join(root, "b", "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	report, err = (Engine{Store: storeB, Artifacts: blobsB, WorkspaceID: "local", Peer: peer}).Sync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Pushed != 0 || report.Pulled != 2 {
		t.Fatalf("second client sync report = %#v", report)
	}
	shared, err := storeB.Get(ctx, entity.ID, "local")
	if err != nil || shared.Title != entity.Title || shared.Body != entity.Body {
		t.Fatalf("shared entity on B = %#v, err=%v", shared, err)
	}
	remoteArtifact, err := storeB.GetArtifact(ctx, artifactID, "local")
	if err != nil {
		t.Fatal(err)
	}
	gotContent, err := blobsB.Read(ctx, remoteArtifact.StorageKey)
	if err != nil || string(gotContent) != string(content) {
		t.Fatalf("shared artifact = %q, err=%v", gotContent, err)
	}

	titleA := "A wins?"
	if _, err := storeA.Update(ctx, entity.ID, "local", domain.Update{ExpectedRevision: 1, Title: &titleA}); err != nil {
		t.Fatal(err)
	}
	titleB := "B wins?"
	if _, err := storeB.Update(ctx, entity.ID, "local", domain.Update{ExpectedRevision: 1, Title: &titleB}); err != nil {
		t.Fatal(err)
	}
	if report, err = (Engine{Store: storeA, Artifacts: blobsA, WorkspaceID: "local", Peer: peer}).Sync(ctx); err != nil || report.Pushed != 1 {
		t.Fatalf("A update sync report = %#v, err=%v", report, err)
	}
	report, err = (Engine{Store: storeB, Artifacts: blobsB, WorkspaceID: "local", Peer: peer}).Sync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Conflicts) != 1 || report.Status.OpenConflicts != 1 {
		t.Fatalf("B conflict sync report = %#v", report)
	}
	localB, err := storeB.Get(ctx, entity.ID, "local")
	if err != nil || localB.Title != titleB || localB.Revision != 2 {
		t.Fatalf("B local value after conflict = %#v, err=%v", localB, err)
	}
}

func TestFilePeerSyncsRunLifecycle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	peer, err := OpenFilePeer(filepath.Join(root, "remote"))
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	storeA, err := sqlite.Open(filepath.Join(root, "a", "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer storeA.Close()
	task, err := storeA.Create(ctx, domain.Entity{Kind: domain.KindTask, WorkspaceID: "local", Title: "run me", Status: domain.StatusDraft})
	if err != nil {
		t.Fatal(err)
	}
	run, _, err := storeA.QueueRun(ctx, domain.Run{
		WorkspaceID:      "local",
		TaskID:           task.ID,
		Provider:         "fake",
		PermissionPolicy: domain.PermissionReadOnly,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report, err := (Engine{Store: storeA, WorkspaceID: "local", Peer: peer}).Sync(ctx); err != nil || report.Pushed != 4 {
		t.Fatalf("queue sync report = %#v, err=%v", report, err)
	}

	storeB, err := sqlite.Open(filepath.Join(root, "b", "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer storeB.Close()
	if report, err := (Engine{Store: storeB, WorkspaceID: "local", Peer: peer}).Sync(ctx); err != nil || report.Pulled != 4 {
		t.Fatalf("remote queue sync report = %#v, err=%v", report, err)
	}
	remoteTask, err := storeB.Get(ctx, task.ID, "local")
	if err != nil || remoteTask.Status != domain.StatusQueued {
		t.Fatalf("remote task = %#v, err=%v", remoteTask, err)
	}
	remoteRun, err := storeB.GetRun(ctx, run.ID, "local")
	if err != nil || remoteRun.Status != domain.RunStatusQueued {
		t.Fatalf("remote run = %#v, err=%v", remoteRun, err)
	}
	events, err := storeB.ListRunEvents(ctx, run.ID, "local")
	if err != nil || len(events) != 1 {
		t.Fatalf("remote run events = %#v, err=%v", events, err)
	}

	if _, err := storeA.SetRunStatus(ctx, run.ID, "local", domain.RunStatusRunning, "", ""); err != nil {
		t.Fatal(err)
	}
	if report, err := (Engine{Store: storeA, WorkspaceID: "local", Peer: peer}).Sync(ctx); err != nil || report.Pushed != 3 {
		t.Fatalf("running sync report = %#v, err=%v", report, err)
	}
	if report, err := (Engine{Store: storeB, WorkspaceID: "local", Peer: peer}).Sync(ctx); err != nil || report.Pulled != 3 {
		t.Fatalf("remote running sync report = %#v, err=%v", report, err)
	}
	remoteRun, err = storeB.GetRun(ctx, run.ID, "local")
	if err != nil || remoteRun.Status != domain.RunStatusRunning {
		t.Fatalf("remote running state = %#v, err=%v", remoteRun, err)
	}
	remoteTask, err = storeB.Get(ctx, task.ID, "local")
	if err != nil || remoteTask.Status != domain.StatusRunning {
		t.Fatalf("remote running task = %#v, err=%v", remoteTask, err)
	}
}
