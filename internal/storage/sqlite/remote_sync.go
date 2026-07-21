package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/heidaraliy/rune/internal/domain"
)

// ApplyRemoteChange applies one already-validated change from a peer. The
// operation id makes repeated delivery safe. A conflict is recorded without
// overwriting the local value and without requiring a second delivery.
func (s *Store) ApplyRemoteChange(ctx context.Context, change domain.Change) (domain.Change, *domain.Conflict, error) {
	if strings.TrimSpace(change.WorkspaceID) == "" {
		return domain.Change{}, nil, errors.New("remote change workspace id is required")
	}
	if change.Origin == "" {
		change.Origin = domain.ChangeOriginRemote
	}
	if change.Origin != domain.ChangeOriginRemote {
		return domain.Change{}, nil, fmt.Errorf("remote change origin must be %q", domain.ChangeOriginRemote)
	}
	if err := change.Validate(); err != nil {
		return domain.Change{}, nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Change{}, nil, fmt.Errorf("begin remote change: %w", err)
	}
	defer tx.Rollback()

	existing, err := scanChangeByOperationTx(ctx, tx, change.WorkspaceID, change.OperationID)
	if err == nil {
		return existing, nil, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.Change{}, nil, err
	}

	var conflict *domain.Conflict
	switch change.Kind {
	case "entity.created", "entity.imported", "entity.updated", "entity.deleted", "entity.restored":
		conflict, err = s.applyRemoteEntityTx(ctx, tx, change)
	case "entity.status":
		conflict, err = s.applyRemoteStatusTx(ctx, tx, change)
	case "link.created":
		conflict, err = s.applyRemoteLinkTx(ctx, tx, change)
	case "run.created", "run.status":
		conflict, err = s.applyRemoteRunTx(ctx, tx, change)
	case "run.event":
		conflict, err = s.applyRemoteRunEventTx(ctx, tx, change)
	case "artifact.created":
		conflict, err = s.applyRemoteArtifactTx(ctx, tx, change)
	default:
		return domain.Change{}, nil, fmt.Errorf("unsupported remote change kind %q", change.Kind)
	}
	if err != nil {
		return domain.Change{}, nil, err
	}
	stored, err := s.insertRemoteChangeTx(ctx, tx, change)
	if err != nil {
		return domain.Change{}, nil, err
	}
	if conflict != nil {
		if err := s.insertConflictTx(ctx, tx, *conflict); err != nil {
			return domain.Change{}, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Change{}, nil, fmt.Errorf("commit remote change: %w", err)
	}
	return stored, conflict, nil
}

func scanChangeByOperationTx(ctx context.Context, tx *sql.Tx, workspaceID, operationID string) (domain.Change, error) {
	var change domain.Change
	var created string
	err := tx.QueryRowContext(ctx, `SELECT cursor, id, workspace_id, operation_id, actor_id,
		device_id, kind, entity_id, revision, payload, created_at, origin
		FROM sync_changes WHERE workspace_id=? AND operation_id=?`, workspaceID, operationID).Scan(
		&change.Cursor, &change.ID, &change.WorkspaceID, &change.OperationID, &change.ActorID,
		&change.DeviceID, &change.Kind, &change.EntityID, &change.Revision, &change.Payload,
		&created, &change.Origin)
	if err != nil {
		return domain.Change{}, err
	}
	change.CreatedAt, err = parseTime(created)
	if err != nil {
		return domain.Change{}, err
	}
	return change, nil
}

func (s *Store) insertRemoteChangeTx(ctx context.Context, tx *sql.Tx, change domain.Change) (domain.Change, error) {
	if change.ID == "" {
		var err error
		change.ID, err = domain.NewID()
		if err != nil {
			return domain.Change{}, err
		}
	}
	if change.CreatedAt.IsZero() {
		change.CreatedAt = s.now()
	}
	change.Origin = domain.ChangeOriginRemote
	if err := change.Validate(); err != nil {
		return domain.Change{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO sync_changes(
		id, workspace_id, operation_id, actor_id, device_id, kind, entity_id,
		revision, payload, created_at, origin
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, change.ID, change.WorkspaceID,
		change.OperationID, change.ActorID, change.DeviceID, change.Kind, change.EntityID,
		change.Revision, change.Payload, formatTime(change.CreatedAt), change.Origin)
	if err != nil {
		return domain.Change{}, fmt.Errorf("record remote sync change: %w", err)
	}
	cursor, err := result.LastInsertId()
	if err != nil {
		return domain.Change{}, fmt.Errorf("read remote sync cursor: %w", err)
	}
	change.Cursor = cursor
	return change, nil
}

func (s *Store) applyRemoteEntityTx(ctx context.Context, tx *sql.Tx, txChange domain.Change) (*domain.Conflict, error) {
	var incoming domain.Entity
	if err := json.Unmarshal([]byte(txChange.Payload), &incoming); err != nil {
		return nil, fmt.Errorf("decode remote entity change: %w", err)
	}
	if incoming.WorkspaceID != txChange.WorkspaceID {
		return nil, errors.New("remote entity belongs to another workspace")
	}
	if err := incoming.Normalize(); err != nil {
		return nil, err
	}
	if err := validateEntityReferencesTx(ctx, tx, incoming); err != nil {
		return nil, err
	}
	current, err := scanEntityByIDTx(ctx, tx, txChange.WorkspaceID, incoming.ID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := insertEntityTx(ctx, tx, incoming); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sameValue(current, incoming) {
		return nil, nil
	}
	if current.Revision+1 != incoming.Revision {
		return newConflict(txChange, incoming.ID, current.Revision, current, incoming)
	}
	if err := updateEntityTx(ctx, tx, incoming); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *Store) applyRemoteStatusTx(ctx context.Context, tx *sql.Tx, txChange domain.Change) (*domain.Conflict, error) {
	var payload struct {
		EntityID string           `json:"entity_id"`
		Status   domain.Status    `json:"status"`
		State    domain.RuneState `json:"state"`
		Revision int64            `json:"revision"`
	}
	if err := json.Unmarshal([]byte(txChange.Payload), &payload); err != nil {
		return nil, fmt.Errorf("decode remote entity status: %w", err)
	}
	if payload.EntityID == "" {
		payload.EntityID = txChange.EntityID
	}
	if payload.EntityID == "" || payload.Revision < 1 {
		return nil, errors.New("remote entity status requires an entity id and positive revision")
	}
	status, err := domain.NormalizeStatus(string(payload.Status))
	if err != nil {
		return nil, err
	}
	current, err := scanEntityByIDTx(ctx, tx, txChange.WorkspaceID, payload.EntityID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("remote status entity %s does not exist", domain.DisplayID(payload.EntityID))
	}
	if err != nil {
		return nil, err
	}
	state, err := domain.NormalizeRuneState(string(payload.State))
	if err != nil {
		return nil, err
	}
	if state == "" {
		state = domain.RuneStateFromStatus(status, current.Kind)
	} else if current.IsTask() && state != domain.RuneStateFromStatus(status, current.Kind) {
		return nil, fmt.Errorf("remote status %q conflicts with Rune state %q", status, state)
	}
	if current.Revision == payload.Revision && current.Status == payload.Status && current.State == state {
		return nil, nil
	}
	if current.Revision+1 != payload.Revision {
		return newConflict(txChange, current.ID, current.Revision, current, payload)
	}
	if !current.IsTask() {
		return nil, errors.New("remote status cannot apply to a note")
	}
	updatedAt := txChange.CreatedAt
	if updatedAt.IsZero() {
		updatedAt = s.now()
	}
	var finishedAt any
	if status == domain.StatusCompleted {
		finishedAt = formatTime(updatedAt)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE entities SET status=?, state=?, updated_at=?, finished_at=?, revision=? WHERE id=? AND workspace_id=? AND revision=?`,
		status, state, formatTime(updatedAt), finishedAt, payload.Revision, current.ID, txChange.WorkspaceID, current.Revision); err != nil {
		return nil, fmt.Errorf("apply remote entity status: %w", err)
	}
	return nil, nil
}

func (s *Store) applyRemoteLinkTx(ctx context.Context, tx *sql.Tx, txChange domain.Change) (*domain.Conflict, error) {
	var incoming domain.Link
	if err := json.Unmarshal([]byte(txChange.Payload), &incoming); err != nil {
		return nil, fmt.Errorf("decode remote link change: %w", err)
	}
	if incoming.WorkspaceID != txChange.WorkspaceID {
		return nil, errors.New("remote link belongs to another workspace")
	}
	if err := incoming.Validate(); err != nil {
		return nil, err
	}
	if err := validateLinkEndpointsTx(ctx, tx, incoming); err != nil {
		return nil, err
	}
	var current domain.Link
	var created string
	err := tx.QueryRowContext(ctx, `SELECT id, workspace_id, from_id, to_id, kind, created_at, revision
		FROM links WHERE workspace_id=? AND id=?`, txChange.WorkspaceID, incoming.ID).Scan(
		&current.ID, &current.WorkspaceID, &current.FromID, &current.ToID, &current.Kind, &created, &current.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		if err := insertLinkTx(ctx, tx, incoming); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read remote link: %w", err)
	}
	current.CreatedAt, err = parseTime(created)
	if err != nil {
		return nil, err
	}
	if sameValue(current, incoming) {
		return nil, nil
	}
	return newConflict(txChange, incoming.ID, current.Revision, current, incoming)
}

func (s *Store) applyRemoteRunTx(ctx context.Context, tx *sql.Tx, txChange domain.Change) (*domain.Conflict, error) {
	var incoming domain.Run
	if err := json.Unmarshal([]byte(txChange.Payload), &incoming); err != nil {
		return nil, fmt.Errorf("decode remote run change: %w", err)
	}
	if incoming.WorkspaceID != txChange.WorkspaceID {
		return nil, errors.New("remote run belongs to another workspace")
	}
	if err := incoming.Validate(); err != nil {
		return nil, err
	}
	if err := validateRunReferenceTx(ctx, tx, incoming); err != nil {
		return nil, err
	}
	current, err := scanRunByIDTx(ctx, tx, txChange.WorkspaceID, incoming.ID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := insertRun(ctx, tx, incoming); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sameValue(current, incoming) {
		return nil, nil
	}
	if current.Revision+1 != incoming.Revision {
		return newConflict(txChange, incoming.ID, current.Revision, current, incoming)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE runs SET status=?, model=?, permission_policy=?, context_snapshot=?, context_artifact_id=?, summary=?, error_text=?, started_at=?, finished_at=?, revision=? WHERE id=? AND workspace_id=? AND revision=?`,
		incoming.Status, incoming.Model, incoming.PermissionPolicy, incoming.ContextSnapshot, incoming.ContextArtifactID,
		incoming.Summary, incoming.Error, formatOptionalTime(incoming.StartedAt), formatOptionalTime(incoming.FinishedAt),
		incoming.Revision, incoming.ID, txChange.WorkspaceID, current.Revision); err != nil {
		return nil, fmt.Errorf("apply remote run: %w", err)
	}
	return nil, nil
}

func (s *Store) applyRemoteRunEventTx(ctx context.Context, tx *sql.Tx, txChange domain.Change) (*domain.Conflict, error) {
	var incoming domain.RunEvent
	if err := json.Unmarshal([]byte(txChange.Payload), &incoming); err != nil {
		return nil, fmt.Errorf("decode remote run event: %w", err)
	}
	if err := incoming.Validate(); err != nil {
		return nil, err
	}
	if err := validateRunEventReferenceTx(ctx, tx, txChange.WorkspaceID, incoming); err != nil {
		return nil, err
	}
	var current domain.RunEvent
	var created string
	err := tx.QueryRowContext(ctx, `SELECT id, run_id, sequence, kind, payload, created_at
		FROM run_events WHERE run_id=? AND sequence=?`, incoming.RunID, incoming.Sequence).Scan(
		&current.ID, &current.RunID, &current.Sequence, &current.Kind, &current.Payload, &created)
	if errors.Is(err, sql.ErrNoRows) {
		if err := insertRunEvent(ctx, tx, incoming, incoming.CreatedAt); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read remote run event: %w", err)
	}
	current.CreatedAt, err = parseTime(created)
	if err != nil {
		return nil, err
	}
	if sameValue(current, incoming) {
		return nil, nil
	}
	return newConflict(txChange, incoming.RunID, current.Sequence, current, incoming)
}

func (s *Store) applyRemoteArtifactTx(ctx context.Context, tx *sql.Tx, txChange domain.Change) (*domain.Conflict, error) {
	var incoming domain.Artifact
	if err := json.Unmarshal([]byte(txChange.Payload), &incoming); err != nil {
		return nil, fmt.Errorf("decode remote artifact change: %w", err)
	}
	if incoming.WorkspaceID != txChange.WorkspaceID {
		return nil, errors.New("remote artifact belongs to another workspace")
	}
	if err := incoming.Validate(); err != nil {
		return nil, err
	}
	if err := validateArtifactReferencesTx(ctx, tx, incoming); err != nil {
		return nil, err
	}
	current, err := scanArtifactByIDTx(ctx, tx, txChange.WorkspaceID, incoming.ID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := insertArtifact(ctx, tx, incoming); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sameValue(current, incoming) {
		return nil, nil
	}
	return newConflict(txChange, incoming.ID, current.Revision, current, incoming)
}

func scanEntityByIDTx(ctx context.Context, tx *sql.Tx, workspaceID, id string) (domain.Entity, error) {
	return scanEntity(tx.QueryRowContext(ctx, `SELECT id, kind, workspace_id, project, title, body, heading, tags_json,
		properties_json, facets_json, status, state, priority, parent_id, source_note_id, legacy_id,
		legacy_source, created_at, updated_at, finished_at, revision, deleted_at, sibling_order
		FROM entities WHERE workspace_id=? AND id=?`, workspaceID, id))
}

func scanRunByIDTx(ctx context.Context, tx *sql.Tx, workspaceID, id string) (domain.Run, error) {
	return scanRun(tx.QueryRowContext(ctx, `SELECT id, workspace_id, task_id, provider, model, status,
		permission_policy, context_snapshot, context_artifact_id, summary, error_text,
		created_at, started_at, finished_at, revision FROM runs WHERE workspace_id=? AND id=?`, workspaceID, id))
}

func scanArtifactByIDTx(ctx context.Context, tx *sql.Tx, workspaceID, id string) (domain.Artifact, error) {
	return scanArtifact(tx.QueryRowContext(ctx, `SELECT id, workspace_id, run_id, entity_id, kind, name,
		media_type, size_bytes, sha256, storage_key, retention, secret_state, created_at, revision
		FROM artifacts WHERE workspace_id=? AND id=?`, workspaceID, id))
}

func validateEntityReferencesTx(ctx context.Context, tx *sql.Tx, entity domain.Entity) error {
	for field, id := range map[string]string{"parent_id": entity.ParentID, "source_note_id": entity.SourceNoteID} {
		if strings.TrimSpace(id) == "" {
			continue
		}
		var workspaceID string
		err := tx.QueryRowContext(ctx, "SELECT workspace_id FROM entities WHERE id=? AND deleted_at IS NULL", id).Scan(&workspaceID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("remote %s %s does not exist", field, id)
		}
		if err != nil {
			return fmt.Errorf("check remote %s %s: %w", field, id, err)
		}
		if workspaceID != entity.WorkspaceID {
			return fmt.Errorf("remote %s %s belongs to workspace %s, not %s", field, id, workspaceID, entity.WorkspaceID)
		}
	}
	if entity.ParentID != "" {
		seen := map[string]bool{entity.ID: true}
		parentID := entity.ParentID
		for parentID != "" {
			if seen[parentID] {
				return errors.New("remote Rune parent hierarchy cannot contain cycles")
			}
			seen[parentID] = true
			var next sql.NullString
			if err := tx.QueryRowContext(ctx, "SELECT parent_id FROM entities WHERE id=? AND workspace_id=? AND deleted_at IS NULL", parentID, entity.WorkspaceID).Scan(&next); err != nil {
				return fmt.Errorf("check remote Rune parent hierarchy: %w", err)
			}
			parentID = next.String
		}
	}
	return nil
}

func validateLinkEndpointsTx(ctx context.Context, tx *sql.Tx, link domain.Link) error {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM entities WHERE workspace_id=? AND id IN (?, ?)", link.WorkspaceID, link.FromID, link.ToID).Scan(&count); err != nil {
		return fmt.Errorf("check remote link endpoints: %w", err)
	}
	if count != 2 {
		return errors.New("remote link endpoints must exist in the same workspace")
	}
	return nil
}

func validateRunReferenceTx(ctx context.Context, tx *sql.Tx, run domain.Run) error {
	var workspaceID string
	if err := tx.QueryRowContext(ctx, "SELECT workspace_id FROM entities WHERE id=? AND kind=?", run.TaskID, domain.KindTask).Scan(&workspaceID); err != nil {
		return fmt.Errorf("check remote run task: %w", err)
	}
	if workspaceID != run.WorkspaceID {
		return errors.New("remote run task belongs to another workspace")
	}
	return nil
}

func validateRunEventReferenceTx(ctx context.Context, tx *sql.Tx, workspaceID string, event domain.RunEvent) error {
	var runWorkspace string
	if err := tx.QueryRowContext(ctx, "SELECT workspace_id FROM runs WHERE id=?", event.RunID).Scan(&runWorkspace); err != nil {
		return fmt.Errorf("check remote run event: %w", err)
	}
	if runWorkspace != workspaceID {
		return errors.New("remote run event belongs to another workspace")
	}
	return nil
}

func validateArtifactReferencesTx(ctx context.Context, tx *sql.Tx, artifact domain.Artifact) error {
	if artifact.RunID != "" {
		var workspaceID, taskID string
		if err := tx.QueryRowContext(ctx, "SELECT workspace_id, task_id FROM runs WHERE id=?", artifact.RunID).Scan(&workspaceID, &taskID); err != nil {
			return fmt.Errorf("check remote artifact run: %w", err)
		}
		if workspaceID != artifact.WorkspaceID {
			return errors.New("remote artifact run belongs to another workspace")
		}
		if artifact.EntityID != "" && artifact.EntityID != taskID {
			return errors.New("remote artifact entity does not match the run task")
		}
	}
	if artifact.EntityID != "" {
		var workspaceID string
		if err := tx.QueryRowContext(ctx, "SELECT workspace_id FROM entities WHERE id=?", artifact.EntityID).Scan(&workspaceID); err != nil {
			return fmt.Errorf("check remote artifact entity: %w", err)
		}
		if workspaceID != artifact.WorkspaceID {
			return errors.New("remote artifact entity belongs to another workspace")
		}
	}
	return nil
}

func insertEntityTx(ctx context.Context, tx *sql.Tx, entity domain.Entity) error {
	var err error
	entity, err = assignSiblingOrderTx(ctx, tx, entity)
	if err != nil {
		return err
	}
	tags, properties, facets, err := encodeMetadata(entity)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO entities(
		id, kind, workspace_id, project, title, body, heading, tags_json,
		properties_json, facets_json, status, state, priority, parent_id, source_note_id,
		legacy_id, legacy_source, created_at, updated_at, finished_at,
		revision, deleted_at, sibling_order
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entity.ID, entity.Kind, entity.WorkspaceID, entity.Project, entity.Title, entity.Body,
		entity.Heading, tags, properties, facets, entity.Status, entity.State, entity.Priority, nullString(entity.ParentID),
		nullString(entity.SourceNoteID), entity.LegacyID, entity.LegacySource, formatTime(entity.CreatedAt),
		formatTime(entity.UpdatedAt), formatOptionalTime(entity.FinishedAt), entity.Revision,
		formatOptionalTime(entity.DeletedAt), entity.SiblingOrder)
	if err != nil {
		return fmt.Errorf("insert remote entity: %w", err)
	}
	return nil
}

func updateEntityTx(ctx context.Context, tx *sql.Tx, entity domain.Entity) error {
	tags, properties, facets, err := encodeMetadata(entity)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE entities SET kind=?, project=?, title=?, body=?, heading=?, tags_json=?,
		properties_json=?, facets_json=?, status=?, state=?, priority=?, parent_id=?, sibling_order=?, source_note_id=?, legacy_id=?, legacy_source=?,
		created_at=?, updated_at=?, finished_at=?, revision=?, deleted_at=? WHERE id=? AND workspace_id=? AND revision=?`,
		entity.Kind, entity.Project, entity.Title, entity.Body, entity.Heading, tags, properties, facets, entity.Status, entity.State,
		entity.Priority, nullString(entity.ParentID), entity.SiblingOrder, nullString(entity.SourceNoteID), entity.LegacyID, entity.LegacySource,
		formatTime(entity.CreatedAt), formatTime(entity.UpdatedAt), formatOptionalTime(entity.FinishedAt), entity.Revision,
		formatOptionalTime(entity.DeletedAt), entity.ID, entity.WorkspaceID, entity.Revision-1)
	if err != nil {
		return fmt.Errorf("update remote entity: %w", err)
	}
	return nil
}

func insertLinkTx(ctx context.Context, tx *sql.Tx, link domain.Link) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO links(id, workspace_id, from_id, to_id, kind, created_at, revision)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, link.ID, link.WorkspaceID, link.FromID, link.ToID, link.Kind,
		formatTime(link.CreatedAt), link.Revision)
	if err != nil {
		return fmt.Errorf("insert remote link: %w", err)
	}
	return nil
}

func sameValue(left, right any) bool {
	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftPayload) == string(rightPayload)
}

func newConflict(change domain.Change, entityID string, localRevision int64, local, remote any) (*domain.Conflict, error) {
	localPayload, err := json.Marshal(local)
	if err != nil {
		return nil, fmt.Errorf("encode local sync conflict: %w", err)
	}
	remotePayload := change.Payload
	if remotePayload == "" {
		encoded, err := json.Marshal(remote)
		if err != nil {
			return nil, fmt.Errorf("encode remote sync conflict: %w", err)
		}
		remotePayload = string(encoded)
	}
	id, err := domain.NewID()
	if err != nil {
		return nil, err
	}
	return &domain.Conflict{
		ID:             id,
		WorkspaceID:    change.WorkspaceID,
		EntityID:       entityID,
		Kind:           change.Kind,
		LocalRevision:  localRevision,
		RemoteRevision: change.Revision,
		LocalPayload:   string(localPayload),
		RemotePayload:  remotePayload,
		Status:         "open",
		CreatedAt:      change.CreatedAt,
	}, nil
}

func (s *Store) insertConflictTx(ctx context.Context, tx *sql.Tx, conflict domain.Conflict) error {
	if conflict.CreatedAt.IsZero() {
		conflict.CreatedAt = s.now()
	}
	if err := conflict.Validate(); err != nil {
		return err
	}
	return insertConflict(ctx, tx, conflict)
}

type conflictExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertConflict(ctx context.Context, executor conflictExecutor, conflict domain.Conflict) error {
	_, err := executor.ExecContext(ctx, `INSERT INTO sync_conflicts(
		id, workspace_id, entity_id, kind, local_revision, remote_revision,
		local_payload, remote_payload, status, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, conflict.ID, conflict.WorkspaceID, conflict.EntityID,
		conflict.Kind, conflict.LocalRevision, conflict.RemoteRevision, conflict.LocalPayload,
		conflict.RemotePayload, conflict.Status, formatTime(conflict.CreatedAt))
	if err != nil {
		return fmt.Errorf("record remote sync conflict: %w", err)
	}
	return nil
}
