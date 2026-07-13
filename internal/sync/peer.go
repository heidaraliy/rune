package runesync

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

type Peer interface {
	ID() string
	Accept(context.Context, domain.Change) (domain.Change, *domain.Conflict, error)
	Changes(context.Context, string, int64, int) ([]domain.Change, error)
	PutArtifact(context.Context, domain.Artifact, []byte) error
	ReadArtifact(context.Context, domain.Artifact) ([]byte, error)
}

type FilePeer struct {
	root      string
	store     *sqlite.Store
	artifacts *artifacts.Store
}

func OpenFilePeer(root string) (*FilePeer, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("sync remote path is required")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve sync remote path: %w", err)
	}
	root = filepath.Clean(absoluteRoot)
	store, err := sqlite.Open(filepath.Join(root, "rune-remote.db"))
	if err != nil {
		return nil, err
	}
	artifactStore, err := artifacts.Open(filepath.Join(root, "artifacts"))
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	return &FilePeer{root: root, store: store, artifacts: artifactStore}, nil
}

func (p *FilePeer) ID() string {
	if p == nil {
		return ""
	}
	return "file:" + filepath.Clean(p.root)
}

func (p *FilePeer) Close() error {
	if p == nil || p.store == nil {
		return nil
	}
	return p.store.Close()
}

func (p *FilePeer) Accept(ctx context.Context, change domain.Change) (domain.Change, *domain.Conflict, error) {
	if p == nil || p.store == nil {
		return domain.Change{}, nil, errors.New("sync peer is closed")
	}
	change.Origin = domain.ChangeOriginRemote
	return p.store.ApplyRemoteChange(ctx, change)
}

func (p *FilePeer) Changes(ctx context.Context, workspaceID string, afterCursor int64, limit int) ([]domain.Change, error) {
	if p == nil || p.store == nil {
		return nil, errors.New("sync peer is closed")
	}
	return p.store.ListChanges(ctx, workspaceID, afterCursor, limit)
}

func (p *FilePeer) PutArtifact(ctx context.Context, artifact domain.Artifact, content []byte) error {
	if p == nil || p.artifacts == nil {
		return errors.New("sync peer is closed")
	}
	if artifact.SecretState == "withheld" {
		return fmt.Errorf("withheld artifact %s cannot sync", domain.DisplayID(artifact.ID))
	}
	blob, err := p.artifacts.Put(ctx, content)
	if err != nil {
		return err
	}
	if blob.SHA256 != artifact.SHA256 || blob.SizeBytes != artifact.SizeBytes || blob.StorageKey != artifact.StorageKey {
		return fmt.Errorf("artifact %s metadata does not match content", domain.DisplayID(artifact.ID))
	}
	return nil
}

func (p *FilePeer) ReadArtifact(ctx context.Context, artifact domain.Artifact) ([]byte, error) {
	if p == nil || p.artifacts == nil {
		return nil, errors.New("sync peer is closed")
	}
	return p.artifacts.Read(ctx, artifact.StorageKey)
}
