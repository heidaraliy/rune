package runesync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

const batchSize = 100

type Report struct {
	Protocol  string            `json:"protocol,omitempty"`
	Status    domain.SyncStatus `json:"status"`
	Pushed    int               `json:"pushed"`
	Pulled    int               `json:"pulled"`
	Conflicts []domain.Conflict `json:"conflicts,omitempty"`
}

type Engine struct {
	Store       *sqlite.Store
	Artifacts   *artifacts.Store
	WorkspaceID string
	Peer        Peer
}

func (e Engine) Sync(ctx context.Context) (Report, error) {
	if e.Store == nil {
		return Report{}, errors.New("sync store is required")
	}
	if e.Peer == nil {
		return Report{}, errors.New("sync peer is required")
	}
	if e.WorkspaceID == "" {
		e.WorkspaceID = "local"
	}
	state, err := e.Store.SyncState(ctx, e.WorkspaceID)
	if err != nil {
		return Report{}, err
	}
	if state.RemoteID != e.Peer.ID() {
		state = domain.SyncState{WorkspaceID: e.WorkspaceID, RemoteID: e.Peer.ID()}
		if _, err := e.Store.SetSyncState(ctx, state); err != nil {
			return Report{}, err
		}
	}
	report := Report{}
	state, report.Conflicts, report.Pushed, err = e.push(ctx, state)
	if err != nil {
		return Report{}, err
	}
	state, report.Conflicts, report.Pulled, err = e.pull(ctx, state, report.Conflicts)
	if err != nil {
		return Report{}, err
	}
	status, err := e.Store.SyncStatus(ctx, e.WorkspaceID)
	if err != nil {
		return Report{}, err
	}
	report.Status = status
	return report, nil
}

func (e Engine) push(ctx context.Context, state domain.SyncState) (domain.SyncState, []domain.Conflict, int, error) {
	var conflicts []domain.Conflict
	pushed := 0
	after := state.PushedCursor
	for {
		changes, err := e.Store.ListPendingChanges(ctx, e.WorkspaceID, after, batchSize)
		if err != nil {
			return state, conflicts, pushed, err
		}
		if len(changes) == 0 {
			break
		}
		for _, change := range changes {
			if err := e.pushArtifact(ctx, change); err != nil {
				return state, conflicts, pushed, err
			}
			_, conflict, err := e.Peer.Accept(ctx, change)
			if err != nil {
				return state, conflicts, pushed, err
			}
			if conflict != nil {
				if err := e.Store.RecordSyncConflict(ctx, *conflict); err != nil {
					return state, conflicts, pushed, err
				}
				conflicts = append(conflicts, *conflict)
			}
			after = change.Cursor
			pushed++
		}
		state.PushedCursor = after
		state, err = e.Store.SetSyncState(ctx, state)
		if err != nil {
			return state, conflicts, pushed, err
		}
	}
	return state, conflicts, pushed, nil
}

func (e Engine) pull(ctx context.Context, state domain.SyncState, conflicts []domain.Conflict) (domain.SyncState, []domain.Conflict, int, error) {
	pulled := 0
	after := state.PulledCursor
	for {
		changes, err := e.Peer.Changes(ctx, e.WorkspaceID, after, batchSize)
		if err != nil {
			return state, conflicts, pulled, err
		}
		if len(changes) == 0 {
			break
		}
		for _, change := range changes {
			if hasConflict(conflicts, change) {
				after = change.Cursor
				pulled++
				continue
			}
			if err := e.pullArtifact(ctx, change); err != nil {
				return state, conflicts, pulled, err
			}
			_, conflict, err := e.Store.ApplyRemoteChange(ctx, change)
			if err != nil {
				return state, conflicts, pulled, err
			}
			if conflict != nil {
				conflicts = append(conflicts, *conflict)
			}
			after = change.Cursor
			pulled++
		}
		state.PulledCursor = after
		state, err = e.Store.SetSyncState(ctx, state)
		if err != nil {
			return state, conflicts, pulled, err
		}
	}
	return state, conflicts, pulled, nil
}

func hasConflict(conflicts []domain.Conflict, change domain.Change) bool {
	for _, conflict := range conflicts {
		if conflict.EntityID == change.EntityID && conflict.Kind == change.Kind && conflict.RemoteRevision == change.Revision {
			return true
		}
	}
	return false
}

func (e Engine) pushArtifact(ctx context.Context, change domain.Change) error {
	if change.Kind != "artifact.created" {
		return nil
	}
	if e.Artifacts == nil {
		return errors.New("local artifact store is required to sync artifacts")
	}
	var artifact domain.Artifact
	if err := json.Unmarshal([]byte(change.Payload), &artifact); err != nil {
		return fmt.Errorf("decode artifact sync change: %w", err)
	}
	content, err := e.Artifacts.Read(ctx, artifact.StorageKey)
	if err != nil {
		return fmt.Errorf("read artifact %s for sync: %w", domain.DisplayID(artifact.ID), err)
	}
	return e.Peer.PutArtifact(ctx, artifact, content)
}

func (e Engine) pullArtifact(ctx context.Context, change domain.Change) error {
	if change.Kind != "artifact.created" {
		return nil
	}
	if e.Artifacts == nil {
		return errors.New("local artifact store is required to receive artifacts")
	}
	var artifact domain.Artifact
	if err := json.Unmarshal([]byte(change.Payload), &artifact); err != nil {
		return fmt.Errorf("decode remote artifact change: %w", err)
	}
	content, err := e.Peer.ReadArtifact(ctx, artifact)
	if err != nil {
		return fmt.Errorf("read remote artifact %s: %w", domain.DisplayID(artifact.ID), err)
	}
	blob, err := e.Artifacts.Put(ctx, content)
	if err != nil {
		return err
	}
	if blob.SHA256 != artifact.SHA256 || blob.SizeBytes != artifact.SizeBytes || blob.StorageKey != artifact.StorageKey {
		return fmt.Errorf("remote artifact %s metadata does not match content", domain.DisplayID(artifact.ID))
	}
	return nil
}
