package runesync

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

type emptySyncTarget struct{}

func (emptySyncTarget) ID() string { return "test:empty" }

func (emptySyncTarget) Accept(context.Context, domain.Change) (domain.Change, *domain.Conflict, error) {
	return domain.Change{}, nil, nil
}

func (emptySyncTarget) Changes(context.Context, string, int64, int) ([]domain.Change, error) {
	return nil, nil
}

func (emptySyncTarget) PutArtifact(context.Context, domain.Artifact, []byte) error { return nil }

func (emptySyncTarget) ReadArtifact(context.Context, domain.Artifact) ([]byte, error) {
	return nil, nil
}

func TestEmbeddedClientUsesVersionedRequestAndReport(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	report, err := (EmbeddedClient{Store: store}).Sync(context.Background(), SyncRequest{
		WorkspaceID: "workspace",
		Target:      emptySyncTarget{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Protocol != ProtocolVersion {
		t.Fatalf("protocol = %q, want %q", report.Protocol, ProtocolVersion)
	}
	if report.Status.WorkspaceID != "workspace" || report.Status.RemoteID != "test:empty" {
		t.Fatalf("status = %#v", report.Status)
	}
}

func TestEmbeddedClientRejectsUnknownProtocol(t *testing.T) {
	_, err := (EmbeddedClient{}).Sync(context.Background(), SyncRequest{Protocol: "sync.v0"})
	if err == nil {
		t.Fatal("unknown protocol unexpectedly accepted")
	}
}
