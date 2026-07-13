package application

import (
	"context"
	"errors"

	runesync "github.com/heidaraliy/rune/internal/sync"
)

func (s V2Service) Sync(ctx context.Context, peer runesync.Peer) (runesync.Report, error) {
	if s.Store == nil {
		return runesync.Report{}, errors.New("v2 sync store is required")
	}
	return runesync.Engine{
		Store:       s.Store,
		Artifacts:   s.ArtifactStore,
		WorkspaceID: s.WorkspaceID,
		Peer:        peer,
	}.Sync(ctx)
}
