package application

import (
	"context"
	"errors"

	runesync "github.com/heidaraliy/rune/internal/sync"
)

// SyncClient is the optional sync capability for an application client. It
// keeps sync targets and reports behind the versioned sync contract without
// forcing every Rune surface to know about the local engine.
type SyncClient interface {
	Sync(context.Context, runesync.SyncTarget) (runesync.Report, error)
}

func (s V2Service) Sync(ctx context.Context, target runesync.SyncTarget) (runesync.Report, error) {
	if s.Store == nil {
		return runesync.Report{}, errors.New("v2 sync store is required")
	}
	client := s.syncClient
	if client == nil {
		client = runesync.EmbeddedClient{Store: s.Store, Artifacts: s.ArtifactStore}
	}
	return client.Sync(ctx, runesync.SyncRequest{
		Protocol:    runesync.ProtocolVersion,
		WorkspaceID: s.WorkspaceID,
		Target:      target,
	})
}

var _ SyncClient = V2Service{}
