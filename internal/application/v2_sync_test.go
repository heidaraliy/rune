package application

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
	runesync "github.com/heidaraliy/rune/internal/sync"
)

type recordingSyncClient struct {
	request runesync.SyncRequest
	report  runesync.Report
}

func (c *recordingSyncClient) Sync(_ context.Context, request runesync.SyncRequest) (runesync.Report, error) {
	c.request = request
	return c.report, nil
}

type recordingSyncTarget struct{}

func (recordingSyncTarget) ID() string { return "test:recording" }

func (recordingSyncTarget) Accept(context.Context, domain.Change) (domain.Change, *domain.Conflict, error) {
	return domain.Change{}, nil, nil
}

func (recordingSyncTarget) Changes(context.Context, string, int64, int) ([]domain.Change, error) {
	return nil, nil
}

func (recordingSyncTarget) PutArtifact(context.Context, domain.Artifact, []byte) error { return nil }

func (recordingSyncTarget) ReadArtifact(context.Context, domain.Artifact) ([]byte, error) {
	return nil, nil
}

func TestV2ServiceDelegatesSyncThroughClientBoundary(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := NewV2Service(store, "workspace")
	client := &recordingSyncClient{report: runesync.Report{Protocol: runesync.ProtocolVersion}}
	service.syncClient = client
	target := recordingSyncTarget{}
	if _, err := service.Sync(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	if client.request.Protocol != runesync.ProtocolVersion || client.request.WorkspaceID != "workspace" {
		t.Fatalf("request = %#v", client.request)
	}
	if client.request.Target == nil || client.request.Target.ID() != target.ID() {
		t.Fatalf("target = %#v", client.request.Target)
	}
}
