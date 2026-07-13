package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/heidaraliy/rune/internal/domain"
)

func (s *Store) SyncState(ctx context.Context, workspaceID string) (domain.SyncState, error) {
	state := domain.SyncState{WorkspaceID: workspaceID}
	var updated string
	err := s.db.QueryRowContext(ctx, `SELECT remote_id, pushed_cursor, pulled_cursor, updated_at
		FROM sync_state WHERE workspace_id=?`, workspaceID).Scan(&state.RemoteID, &state.PushedCursor, &state.PulledCursor, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return domain.SyncState{}, fmt.Errorf("read v2 sync state: %w", err)
	}
	state.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return domain.SyncState{}, err
	}
	return state, nil
}

func (s *Store) ConfigureSync(ctx context.Context, workspaceID, remoteID string) (domain.SyncState, error) {
	state, err := s.SyncState(ctx, workspaceID)
	if err != nil {
		return domain.SyncState{}, err
	}
	state.RemoteID = strings.TrimSpace(remoteID)
	return s.SetSyncState(ctx, state)
}

func (s *Store) SetSyncState(ctx context.Context, state domain.SyncState) (domain.SyncState, error) {
	if strings.TrimSpace(state.WorkspaceID) == "" {
		return domain.SyncState{}, fmt.Errorf("sync workspace id is required")
	}
	if state.PushedCursor < 0 || state.PulledCursor < 0 {
		return domain.SyncState{}, fmt.Errorf("sync cursors cannot be negative")
	}
	state.UpdatedAt = s.now()
	_, err := s.db.ExecContext(ctx, `INSERT INTO sync_state(
		workspace_id, remote_id, pushed_cursor, pulled_cursor, updated_at
	) VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(workspace_id) DO UPDATE SET remote_id=excluded.remote_id,
		pushed_cursor=excluded.pushed_cursor, pulled_cursor=excluded.pulled_cursor,
		updated_at=excluded.updated_at`, state.WorkspaceID, state.RemoteID, state.PushedCursor,
		state.PulledCursor, formatTime(state.UpdatedAt))
	if err != nil {
		return domain.SyncState{}, fmt.Errorf("write v2 sync state: %w", err)
	}
	return state, nil
}
