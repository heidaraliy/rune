package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/markdown"
	_ "modernc.org/sqlite"
)

const defaultWorkspaceID = "local"

type Store struct {
	db  *sql.DB
	now func() time.Time
}

type ImportReport struct {
	SourcePath string
	Created    int
	Skipped    int
	Warnings   []string
}

type AmbiguousIDError struct {
	Prefix  string
	Matches []domain.Entity
}

func (e *AmbiguousIDError) Error() string {
	ids := make([]string, 0, len(e.Matches))
	for _, match := range e.Matches {
		ids = append(ids, fmt.Sprintf("%s (%s)", domain.DisplayID(match.ID), match.Title))
	}
	return fmt.Sprintf("id prefix %q is ambiguous: %s", e.Prefix, strings.Join(ids, ", "))
}

func DefaultPath(home string) string {
	return filepath.Join(home, "rune-v2.db")
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("v2 database path is required")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create v2 database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open v2 database: %w", err)
	}
	store := &Store{db: db, now: func() time.Time { return time.Now().UTC() }}
	if err := store.configure(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) configure() error {
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("configure v2 database: %s: %w", statement, err)
		}
	}
	return nil
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create schema migrations: %w", err)
	}
	var version int
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version < 1 {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin schema migration: %w", err)
		}
		statements := []string{
			`CREATE TABLE entities (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL CHECK (kind IN ('note', 'task')),
			workspace_id TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			body TEXT NOT NULL DEFAULT '',
			heading TEXT NOT NULL DEFAULT '',
			tags_json TEXT NOT NULL DEFAULT '[]',
			properties_json TEXT NOT NULL DEFAULT '{}',
			status TEXT NOT NULL DEFAULT '',
			priority INTEGER NOT NULL DEFAULT 0,
			parent_id TEXT REFERENCES entities(id) ON DELETE SET NULL,
			source_note_id TEXT REFERENCES entities(id) ON DELETE SET NULL,
			legacy_id TEXT NOT NULL DEFAULT '',
			legacy_source TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			finished_at TEXT,
			revision INTEGER NOT NULL DEFAULT 1,
			deleted_at TEXT
		)`,
			`CREATE UNIQUE INDEX entities_legacy_identity ON entities(workspace_id, legacy_source, legacy_id) WHERE legacy_id <> ''`,
			`CREATE INDEX entities_workspace_updated ON entities(workspace_id, updated_at DESC)`,
			`CREATE INDEX entities_workspace_project ON entities(workspace_id, project)`,
			`CREATE TABLE links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			from_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
			to_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			created_at TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 1,
			UNIQUE(workspace_id, from_id, to_id, kind)
		)`,
			`CREATE INDEX links_workspace_from ON links(workspace_id, from_id)`,
			`CREATE INDEX links_workspace_to ON links(workspace_id, to_id)`,
			`INSERT INTO schema_migrations(version, applied_at) VALUES (1, ?)`,
		}
		for index, statement := range statements {
			if index == len(statements)-1 {
				if _, err := tx.ExecContext(ctx, statement, s.now().Format(time.RFC3339Nano)); err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("apply schema migration %d: %w", index+1, err)
				}
				continue
			}
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply schema migration %d: %w", index+1, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit schema migration: %w", err)
		}
		version = 1
	}
	if version >= 2 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin run schema migration: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			task_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			model TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'review', 'completed', 'blocked', 'failed', 'canceled')),
			permission_policy TEXT NOT NULL CHECK (permission_policy IN ('read-only', 'workspace-write', 'full')),
			context_snapshot TEXT NOT NULL DEFAULT '{}',
			context_artifact_id TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL DEFAULT '',
			error_text TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			started_at TEXT,
			finished_at TEXT,
			revision INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX runs_workspace_created ON runs(workspace_id, created_at DESC)`,
		`CREATE INDEX runs_workspace_task ON runs(workspace_id, task_id, created_at DESC)`,
		`CREATE TABLE run_events (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
			sequence INTEGER NOT NULL,
			kind TEXT NOT NULL,
			payload TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			UNIQUE(run_id, sequence)
		)`,
		`CREATE INDEX run_events_run_sequence ON run_events(run_id, sequence ASC)`,
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT REFERENCES runs(id) ON DELETE CASCADE,
			entity_id TEXT REFERENCES entities(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			media_type TEXT NOT NULL,
			size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
			sha256 TEXT NOT NULL,
			storage_key TEXT NOT NULL,
			retention TEXT NOT NULL CHECK (retention IN ('ephemeral', 'normal', 'permanent')),
			secret_state TEXT NOT NULL CHECK (secret_state IN ('clear', 'redacted', 'withheld')),
			created_at TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX artifacts_workspace_created ON artifacts(workspace_id, created_at DESC)`,
		`CREATE INDEX artifacts_run_created ON artifacts(run_id, created_at ASC)`,
		`INSERT INTO schema_migrations(version, applied_at) VALUES (2, ?)`,
	}
	for index, statement := range statements {
		if index == len(statements)-1 {
			if _, err := tx.ExecContext(ctx, statement, s.now().Format(time.RFC3339Nano)); err != nil {
				return fmt.Errorf("apply run schema migration %d: %w", index+1, err)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply run schema migration %d: %w", index+1, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit run schema migration: %w", err)
	}
	return nil
}

func (s *Store) Create(ctx context.Context, entity domain.Entity) (domain.Entity, error) {
	entity, err := s.prepareEntity(entity)
	if err != nil {
		return domain.Entity{}, err
	}
	tags, properties, err := encodeMetadata(entity)
	if err != nil {
		return domain.Entity{}, err
	}
	if err := s.validateEntityReferences(ctx, entity); err != nil {
		return domain.Entity{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO entities(
			id, kind, workspace_id, project, title, body, heading, tags_json,
			properties_json, status, priority, parent_id, source_note_id,
			legacy_id, legacy_source, created_at, updated_at, finished_at,
			revision, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entity.ID, entity.Kind, entity.WorkspaceID, entity.Project, entity.Title, entity.Body,
		entity.Heading, tags, properties, entity.Status, entity.Priority, nullString(entity.ParentID),
		nullString(entity.SourceNoteID), entity.LegacyID, entity.LegacySource,
		formatTime(entity.CreatedAt), formatTime(entity.UpdatedAt), formatOptionalTime(entity.FinishedAt),
		entity.Revision, formatOptionalTime(entity.DeletedAt))
	if err != nil {
		return domain.Entity{}, fmt.Errorf("create %s %s: %w", entity.Kind, entity.ID, err)
	}
	return entity, nil
}

func (s *Store) Get(ctx context.Context, prefix string, workspaceID string) (domain.Entity, error) {
	entities, err := s.list(ctx, domain.ListOptions{WorkspaceID: workspaceID, IncludeDeleted: false})
	if err != nil {
		return domain.Entity{}, err
	}
	prefix = strings.TrimSpace(prefix)
	var matches []domain.Entity
	for _, entity := range entities {
		if prefix == "" || strings.HasPrefix(entity.ID, prefix) {
			matches = append(matches, entity)
		}
	}
	switch len(matches) {
	case 0:
		return domain.Entity{}, fmt.Errorf("no v2 entity matches id %q", prefix)
	case 1:
		return matches[0], nil
	default:
		return domain.Entity{}, &AmbiguousIDError{Prefix: prefix, Matches: matches}
	}
}

func (s *Store) List(ctx context.Context, opts domain.ListOptions) ([]domain.Entity, error) {
	return s.list(ctx, opts)
}

func (s *Store) list(ctx context.Context, opts domain.ListOptions) ([]domain.Entity, error) {
	query := `SELECT id, kind, workspace_id, project, title, body, heading, tags_json,
		properties_json, status, priority, parent_id, source_note_id, legacy_id,
		legacy_source, created_at, updated_at, finished_at, revision, deleted_at
		FROM entities WHERE 1=1`
	args := make([]any, 0, 6)
	if strings.TrimSpace(opts.WorkspaceID) != "" {
		query += " AND workspace_id = ?"
		args = append(args, opts.WorkspaceID)
	}
	if strings.TrimSpace(opts.Project) != "" {
		query += " AND project = ?"
		args = append(args, opts.Project)
	}
	if opts.Kind != "" {
		query += " AND kind = ?"
		args = append(args, opts.Kind)
	}
	if opts.Status != "" {
		query += " AND status = ?"
		args = append(args, opts.Status)
	}
	if strings.TrimSpace(opts.Query) != "" {
		query += " AND (LOWER(title) LIKE LOWER(?) OR LOWER(body) LIKE LOWER(?))"
		needle := "%" + strings.ToLower(strings.TrimSpace(opts.Query)) + "%"
		args = append(args, needle, needle)
	}
	if !opts.IncludeDeleted {
		query += " AND deleted_at IS NULL"
	}
	query += " ORDER BY updated_at DESC, id ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list v2 entities: %w", err)
	}
	defer rows.Close()
	entities := make([]domain.Entity, 0)
	for rows.Next() {
		entity, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read v2 entities: %w", err)
	}
	return entities, nil
}

func (s *Store) Update(ctx context.Context, prefix, workspaceID string, update domain.Update) (domain.Entity, error) {
	current, err := s.Get(ctx, prefix, workspaceID)
	if err != nil {
		return domain.Entity{}, err
	}
	if update.ExpectedRevision != 0 && update.ExpectedRevision != current.Revision {
		return domain.Entity{}, fmt.Errorf("v2 entity %s revision conflict: expected %d, current %d", domain.DisplayID(current.ID), update.ExpectedRevision, current.Revision)
	}
	if update.Title != nil {
		current.Title = strings.TrimSpace(*update.Title)
	}
	if update.Body != nil {
		current.Body = *update.Body
	}
	if update.AppendBody != nil {
		if current.Body != "" && !strings.HasSuffix(current.Body, "\n") {
			current.Body += "\n"
		}
		current.Body += *update.AppendBody
	}
	if update.Heading != nil {
		current.Heading = strings.TrimSpace(*update.Heading)
	}
	if update.Tags != nil {
		current.Tags = domain.NormalizeTags(*update.Tags)
	}
	if update.Priority != nil {
		current.Priority = *update.Priority
	}
	if update.Status != nil {
		status, err := domain.NormalizeStatus(string(*update.Status))
		if err != nil {
			return domain.Entity{}, err
		}
		if !current.IsTask() {
			return domain.Entity{}, errors.New("notes cannot have task status")
		}
		if status == "" {
			status = domain.StatusDraft
		}
		if status != current.Status {
			var activeRuns int
			if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM runs WHERE task_id=? AND status IN (?, ?, ?)", current.ID, domain.RunStatusQueued, domain.RunStatusRunning, domain.RunStatusReview).Scan(&activeRuns); err != nil {
				return domain.Entity{}, fmt.Errorf("check active v2 runs: %w", err)
			}
			if activeRuns > 0 {
				return domain.Entity{}, fmt.Errorf("task %s is owned by an active run; use v2 run or cancel", domain.DisplayID(current.ID))
			}
		}
		current.Status = status
		if status == domain.StatusCompleted {
			finishedAt := s.now()
			current.FinishedAt = &finishedAt
		} else {
			current.FinishedAt = nil
		}
	}
	current.UpdatedAt = s.now()
	current.Revision++
	if err := current.Validate(); err != nil {
		return domain.Entity{}, err
	}
	tags, properties, err := encodeMetadata(current)
	if err != nil {
		return domain.Entity{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE entities SET title=?, body=?, heading=?, tags_json=?, properties_json=?, status=?, priority=?, updated_at=?, finished_at=?, revision=? WHERE id=? AND workspace_id=? AND revision=?`,
		current.Title, current.Body, current.Heading, tags, properties, current.Status, current.Priority,
		formatTime(current.UpdatedAt), formatOptionalTime(current.FinishedAt), current.Revision,
		current.ID, current.WorkspaceID, current.Revision-1)
	if err != nil {
		return domain.Entity{}, fmt.Errorf("update v2 entity: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return domain.Entity{}, fmt.Errorf("read v2 update result: %w", err)
	}
	if count != 1 {
		return domain.Entity{}, fmt.Errorf("v2 entity %s changed concurrently", domain.DisplayID(current.ID))
	}
	return current, nil
}

func (s *Store) CreateLink(ctx context.Context, link domain.Link) (domain.Link, error) {
	if strings.TrimSpace(link.ID) == "" {
		var err error
		link.ID, err = domain.NewID()
		if err != nil {
			return domain.Link{}, err
		}
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = s.now()
	}
	if link.Revision == 0 {
		link.Revision = 1
	}
	if err := link.Validate(); err != nil {
		return domain.Link{}, err
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entities WHERE workspace_id=? AND id IN (?, ?)", link.WorkspaceID, link.FromID, link.ToID).Scan(&count); err != nil {
		return domain.Link{}, fmt.Errorf("check v2 link endpoints: %w", err)
	}
	if count != 2 {
		return domain.Link{}, errors.New("link endpoints must exist in the same workspace")
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO links(id, workspace_id, from_id, to_id, kind, created_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?)",
		link.ID, link.WorkspaceID, link.FromID, link.ToID, link.Kind, formatTime(link.CreatedAt), link.Revision)
	if err != nil {
		return domain.Link{}, fmt.Errorf("create v2 link: %w", err)
	}
	return link, nil
}

func (s *Store) validateEntityReferences(ctx context.Context, entity domain.Entity) error {
	for field, id := range map[string]string{
		"parent_id":      entity.ParentID,
		"source_note_id": entity.SourceNoteID,
	} {
		if strings.TrimSpace(id) == "" {
			continue
		}
		var workspaceID string
		err := s.db.QueryRowContext(ctx, "SELECT workspace_id FROM entities WHERE id=? AND deleted_at IS NULL", id).Scan(&workspaceID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%s %s does not exist", field, id)
		}
		if err != nil {
			return fmt.Errorf("check %s %s: %w", field, id, err)
		}
		if workspaceID != entity.WorkspaceID {
			return fmt.Errorf("%s %s belongs to workspace %s, not %s", field, id, workspaceID, entity.WorkspaceID)
		}
	}
	return nil
}

func (s *Store) ListLinks(ctx context.Context, workspaceID, entityID string) ([]domain.Link, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, workspace_id, from_id, to_id, kind, created_at, revision FROM links WHERE workspace_id=? AND (from_id=? OR to_id=?) ORDER BY created_at ASC, id ASC`, workspaceID, entityID, entityID)
	if err != nil {
		return nil, fmt.Errorf("list v2 links: %w", err)
	}
	defer rows.Close()
	var links []domain.Link
	for rows.Next() {
		var link domain.Link
		var created string
		if err := rows.Scan(&link.ID, &link.WorkspaceID, &link.FromID, &link.ToID, &link.Kind, &created, &link.Revision); err != nil {
			return nil, fmt.Errorf("scan v2 link: %w", err)
		}
		link.CreatedAt, err = parseTime(created)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *Store) QueueRun(ctx context.Context, run domain.Run, contextArtifact *domain.Artifact) (domain.Run, domain.Artifact, error) {
	run, err := s.prepareRun(run)
	if err != nil {
		return domain.Run{}, domain.Artifact{}, err
	}
	var artifact domain.Artifact
	if contextArtifact != nil {
		artifact = *contextArtifact
		if artifact.ID == "" {
			artifact.ID, err = domain.NewID()
			if err != nil {
				return domain.Run{}, domain.Artifact{}, err
			}
		}
		if artifact.WorkspaceID == "" {
			artifact.WorkspaceID = run.WorkspaceID
		}
		if artifact.RunID == "" {
			artifact.RunID = run.ID
		}
		if artifact.EntityID == "" {
			artifact.EntityID = run.TaskID
		}
		if artifact.CreatedAt.IsZero() {
			artifact.CreatedAt = run.CreatedAt
		}
		if artifact.Revision == 0 {
			artifact.Revision = 1
		}
		if artifact.WorkspaceID != run.WorkspaceID || artifact.RunID != run.ID || artifact.EntityID != run.TaskID {
			return domain.Run{}, domain.Artifact{}, errors.New("context artifact must belong to the queued run and task")
		}
		if err := artifact.Validate(); err != nil {
			return domain.Run{}, domain.Artifact{}, err
		}
		run.ContextArtifactID = artifact.ID
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Run{}, domain.Artifact{}, fmt.Errorf("begin v2 queue: %w", err)
	}
	defer tx.Rollback()
	var taskKind, taskWorkspace, taskStatus string
	var deletedAt sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT kind, workspace_id, status, deleted_at FROM entities WHERE id=?", run.TaskID).Scan(&taskKind, &taskWorkspace, &taskStatus, &deletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Run{}, domain.Artifact{}, fmt.Errorf("run task %s does not exist", run.TaskID)
		}
		return domain.Run{}, domain.Artifact{}, fmt.Errorf("check run task: %w", err)
	}
	if taskWorkspace != run.WorkspaceID {
		return domain.Run{}, domain.Artifact{}, fmt.Errorf("run task %s belongs to workspace %s, not %s", run.TaskID, taskWorkspace, run.WorkspaceID)
	}
	if taskKind != string(domain.KindTask) {
		return domain.Run{}, domain.Artifact{}, errors.New("only tasks can be queued for a run")
	}
	if deletedAt.Valid && deletedAt.String != "" {
		return domain.Run{}, domain.Artifact{}, errors.New("deleted tasks cannot be queued")
	}
	if taskStatus != string(domain.StatusDraft) && taskStatus != string(domain.StatusReady) {
		return domain.Run{}, domain.Artifact{}, fmt.Errorf("task %s is %s and cannot be queued", domain.DisplayID(run.TaskID), taskStatus)
	}
	if err := insertRun(ctx, tx, run); err != nil {
		return domain.Run{}, domain.Artifact{}, err
	}
	if contextArtifact != nil {
		if err := insertArtifact(ctx, tx, artifact); err != nil {
			return domain.Run{}, domain.Artifact{}, err
		}
	}
	updatedAt := s.now()
	result, err := tx.ExecContext(ctx, `UPDATE entities SET status=?, updated_at=?, finished_at=NULL, revision=revision+1 WHERE id=? AND workspace_id=? AND status IN (?, ?)`,
		domain.StatusQueued, formatTime(updatedAt), run.TaskID, run.WorkspaceID, domain.StatusDraft, domain.StatusReady)
	if err != nil {
		return domain.Run{}, domain.Artifact{}, fmt.Errorf("mark task queued: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return domain.Run{}, domain.Artifact{}, fmt.Errorf("read queued task result: %w", err)
		}
		return domain.Run{}, domain.Artifact{}, fmt.Errorf("task %s changed while queueing", domain.DisplayID(run.TaskID))
	}
	if err := insertRunEvent(ctx, tx, domain.RunEvent{RunID: run.ID, Sequence: 1, Kind: "status", Payload: string(run.Status)}, run.CreatedAt); err != nil {
		return domain.Run{}, domain.Artifact{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Run{}, domain.Artifact{}, fmt.Errorf("commit v2 queue: %w", err)
	}
	return run, artifact, nil
}

func (s *Store) GetRun(ctx context.Context, prefix, workspaceID string) (domain.Run, error) {
	runs, err := s.ListRuns(ctx, domain.RunListOptions{WorkspaceID: workspaceID})
	if err != nil {
		return domain.Run{}, err
	}
	prefix = strings.TrimSpace(prefix)
	var matches []domain.Run
	for _, run := range runs {
		if prefix == "" || strings.HasPrefix(run.ID, prefix) {
			matches = append(matches, run)
		}
	}
	switch len(matches) {
	case 0:
		return domain.Run{}, fmt.Errorf("no v2 run matches id %q", prefix)
	case 1:
		return matches[0], nil
	default:
		return domain.Run{}, &AmbiguousRunIDError{Prefix: prefix, Matches: matches}
	}
}

type AmbiguousRunIDError struct {
	Prefix  string
	Matches []domain.Run
}

func (e *AmbiguousRunIDError) Error() string {
	ids := make([]string, 0, len(e.Matches))
	for _, run := range e.Matches {
		ids = append(ids, fmt.Sprintf("%s (%s)", domain.DisplayID(run.ID), run.Status))
	}
	return fmt.Sprintf("run id prefix %q is ambiguous: %s", e.Prefix, strings.Join(ids, ", "))
}

func (s *Store) ListRuns(ctx context.Context, options domain.RunListOptions) ([]domain.Run, error) {
	query := `SELECT id, workspace_id, task_id, provider, model, status, permission_policy,
		context_snapshot, context_artifact_id, summary, error_text, created_at, started_at, finished_at, revision
		FROM runs WHERE 1=1`
	args := make([]any, 0, 3)
	if strings.TrimSpace(options.WorkspaceID) != "" {
		query += " AND workspace_id=?"
		args = append(args, options.WorkspaceID)
	}
	if strings.TrimSpace(options.TaskID) != "" {
		query += " AND task_id=?"
		args = append(args, options.TaskID)
	}
	if options.Status != "" {
		query += " AND status=?"
		args = append(args, options.Status)
	}
	query += " ORDER BY created_at DESC, id ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list v2 runs: %w", err)
	}
	defer rows.Close()
	var runs []domain.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read v2 runs: %w", err)
	}
	return runs, nil
}

func (s *Store) SetRunStatus(ctx context.Context, prefix, workspaceID string, status domain.RunStatus, summary, runError string) (domain.Run, error) {
	current, err := s.GetRun(ctx, prefix, workspaceID)
	if err != nil {
		return domain.Run{}, err
	}
	if !domain.CanTransitionRun(current.Status, status) {
		return domain.Run{}, fmt.Errorf("v2 run %s cannot transition from %s to %s", domain.DisplayID(current.ID), current.Status, status)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Run{}, fmt.Errorf("begin v2 run transition: %w", err)
	}
	defer tx.Rollback()
	var taskStatus string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM entities WHERE id=? AND workspace_id=?", current.TaskID, workspaceID).Scan(&taskStatus); err != nil {
		return domain.Run{}, fmt.Errorf("read run task status: %w", err)
	}
	if taskStatus != string(current.Status) {
		return domain.Run{}, fmt.Errorf("task %s status %s is out of sync with run %s", domain.DisplayID(current.TaskID), taskStatus, current.Status)
	}
	now := s.now()
	updated := current
	updated.Status = status
	if summary != "" {
		updated.Summary = summary
	}
	if runError != "" {
		updated.Error = runError
	}
	if status == domain.RunStatusRunning {
		updated.Error = ""
	}
	updated.Revision++
	if status == domain.RunStatusRunning && updated.StartedAt == nil {
		updated.StartedAt = &now
	}
	if status == domain.RunStatusCompleted || status == domain.RunStatusFailed || status == domain.RunStatusCanceled {
		updated.FinishedAt = &now
	}
	result, err := tx.ExecContext(ctx, `UPDATE runs SET status=?, summary=?, error_text=?, started_at=?, finished_at=?, revision=? WHERE id=? AND workspace_id=? AND revision=?`,
		updated.Status, updated.Summary, updated.Error, formatOptionalTime(updated.StartedAt), formatOptionalTime(updated.FinishedAt), updated.Revision,
		updated.ID, workspaceID, current.Revision)
	if err != nil {
		return domain.Run{}, fmt.Errorf("update v2 run status: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return domain.Run{}, fmt.Errorf("read v2 run status result: %w", err)
		}
		return domain.Run{}, fmt.Errorf("v2 run %s changed concurrently", domain.DisplayID(current.ID))
	}
	entityFinished := any(nil)
	if status == domain.RunStatusCompleted || status == domain.RunStatusFailed || status == domain.RunStatusCanceled {
		entityFinished = formatTime(now)
	}
	result, err = tx.ExecContext(ctx, "UPDATE entities SET status=?, updated_at=?, finished_at=?, revision=revision+1 WHERE id=? AND workspace_id=? AND status=?", status, formatTime(now), entityFinished, current.TaskID, workspaceID, current.Status)
	if err != nil {
		return domain.Run{}, fmt.Errorf("update v2 task status: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return domain.Run{}, fmt.Errorf("read v2 task status result: %w", err)
		}
		return domain.Run{}, fmt.Errorf("task %s changed concurrently", domain.DisplayID(current.TaskID))
	}
	payload := string(status)
	if runError != "" {
		payload = runError
	} else if summary != "" {
		payload = summary
	}
	if _, err := appendRunEvent(ctx, tx, current.ID, "status", payload, now); err != nil {
		return domain.Run{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Run{}, fmt.Errorf("commit v2 run transition: %w", err)
	}
	return updated, nil
}

func (s *Store) AppendRunEvent(ctx context.Context, runID, workspaceID, kind, payload string) (domain.RunEvent, error) {
	run, err := s.GetRun(ctx, runID, workspaceID)
	if err != nil {
		return domain.RunEvent{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("begin v2 run event: %w", err)
	}
	defer tx.Rollback()
	event, err := appendRunEvent(ctx, tx, run.ID, kind, payload, s.now())
	if err != nil {
		return domain.RunEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.RunEvent{}, fmt.Errorf("commit v2 run event: %w", err)
	}
	return event, nil
}

func (s *Store) ListRunEvents(ctx context.Context, runID, workspaceID string) ([]domain.RunEvent, error) {
	run, err := s.GetRun(ctx, runID, workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id, run_id, sequence, kind, payload, created_at FROM run_events WHERE run_id=? ORDER BY sequence ASC", run.ID)
	if err != nil {
		return nil, fmt.Errorf("list v2 run events: %w", err)
	}
	defer rows.Close()
	var events []domain.RunEvent
	for rows.Next() {
		event, err := scanRunEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) CreateArtifact(ctx context.Context, artifact domain.Artifact) (domain.Artifact, error) {
	if artifact.ID == "" {
		var err error
		artifact.ID, err = domain.NewID()
		if err != nil {
			return domain.Artifact{}, err
		}
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = s.now()
	}
	if artifact.Revision == 0 {
		artifact.Revision = 1
	}
	if err := artifact.Validate(); err != nil {
		return domain.Artifact{}, err
	}
	if err := s.validateArtifactReferences(ctx, artifact); err != nil {
		return domain.Artifact{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("begin v2 artifact: %w", err)
	}
	defer tx.Rollback()
	if err := insertArtifact(ctx, tx, artifact); err != nil {
		return domain.Artifact{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Artifact{}, fmt.Errorf("commit v2 artifact: %w", err)
	}
	return artifact, nil
}

type AmbiguousArtifactIDError struct {
	Prefix  string
	Matches []domain.Artifact
}

func (e *AmbiguousArtifactIDError) Error() string {
	ids := make([]string, 0, len(e.Matches))
	for _, artifact := range e.Matches {
		ids = append(ids, fmt.Sprintf("%s (%s)", domain.DisplayID(artifact.ID), artifact.Name))
	}
	return fmt.Sprintf("artifact id prefix %q is ambiguous: %s", e.Prefix, strings.Join(ids, ", "))
}

func (s *Store) GetArtifact(ctx context.Context, prefix, workspaceID string) (domain.Artifact, error) {
	artifacts, err := s.ListArtifacts(ctx, workspaceID, "")
	if err != nil {
		return domain.Artifact{}, err
	}
	prefix = strings.TrimSpace(prefix)
	var matches []domain.Artifact
	for _, artifact := range artifacts {
		if prefix == "" || strings.HasPrefix(artifact.ID, prefix) {
			matches = append(matches, artifact)
		}
	}
	switch len(matches) {
	case 0:
		return domain.Artifact{}, fmt.Errorf("no v2 artifact matches id %q", prefix)
	case 1:
		return matches[0], nil
	default:
		return domain.Artifact{}, &AmbiguousArtifactIDError{Prefix: prefix, Matches: matches}
	}
}

func (s *Store) ListArtifacts(ctx context.Context, workspaceID, runID string) ([]domain.Artifact, error) {
	query := `SELECT id, workspace_id, run_id, entity_id, kind, name, media_type, size_bytes,
		sha256, storage_key, retention, secret_state, created_at, revision FROM artifacts WHERE workspace_id=?`
	args := []any{workspaceID}
	if strings.TrimSpace(runID) != "" {
		query += " AND run_id=?"
		args = append(args, runID)
	}
	query += " ORDER BY created_at ASC, id ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list v2 artifacts: %w", err)
	}
	defer rows.Close()
	var artifacts []domain.Artifact
	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (s *Store) validateArtifactReferences(ctx context.Context, artifact domain.Artifact) error {
	if artifact.RunID != "" {
		var workspace, taskID string
		if err := s.db.QueryRowContext(ctx, "SELECT workspace_id, task_id FROM runs WHERE id=?", artifact.RunID).Scan(&workspace, &taskID); err != nil {
			return fmt.Errorf("check artifact run: %w", err)
		}
		if workspace != artifact.WorkspaceID {
			return errors.New("artifact run belongs to another workspace")
		}
		if artifact.EntityID != "" && artifact.EntityID != taskID {
			return errors.New("artifact entity does not match the run task")
		}
	}
	if artifact.EntityID != "" {
		var workspace string
		if err := s.db.QueryRowContext(ctx, "SELECT workspace_id FROM entities WHERE id=?", artifact.EntityID).Scan(&workspace); err != nil {
			return fmt.Errorf("check artifact entity: %w", err)
		}
		if workspace != artifact.WorkspaceID {
			return errors.New("artifact entity belongs to another workspace")
		}
	}
	return nil
}

func (s *Store) Import(ctx context.Context, workspaceID string, bundle markdown.Bundle) (ImportReport, error) {
	if strings.TrimSpace(workspaceID) == "" {
		workspaceID = defaultWorkspaceID
	}
	report := ImportReport{SourcePath: bundle.SourcePath, Warnings: append([]string(nil), bundle.Warnings...)}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return report, fmt.Errorf("begin v2 import: %w", err)
	}
	defer tx.Rollback()
	legacyToID := make(map[string]string, len(bundle.Items))
	existingKeys := make(map[string]bool, len(bundle.Items))
	for _, item := range bundle.Items {
		key := legacyKey(bundle.SourcePath, item.Entity.LegacyID)
		var existingID string
		err := tx.QueryRowContext(ctx, "SELECT id FROM entities WHERE workspace_id=? AND legacy_source=? AND legacy_id=?", workspaceID, bundle.SourcePath, item.Entity.LegacyID).Scan(&existingID)
		switch {
		case err == nil:
			legacyToID[key] = existingID
			existingKeys[key] = true
		case errors.Is(err, sql.ErrNoRows):
			legacyToID[key] = item.Entity.ID
		default:
			return report, fmt.Errorf("check imported legacy item %q: %w", item.Entity.LegacyID, err)
		}
	}
	for _, item := range bundle.Items {
		key := legacyKey(bundle.SourcePath, item.Entity.LegacyID)
		if existingKeys[key] {
			report.Skipped++
			continue
		}
		entity := item.Entity
		entity.WorkspaceID = workspaceID
		if item.ParentLegacyID != "" {
			entity.ParentID = legacyToID[legacyKey(bundle.SourcePath, item.ParentLegacyID)]
		}
		tags, properties, err := encodeMetadata(entity)
		if err != nil {
			return report, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO entities(
			id, kind, workspace_id, project, title, body, heading, tags_json,
			properties_json, status, priority, parent_id, source_note_id,
			legacy_id, legacy_source, created_at, updated_at, finished_at,
			revision, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			entity.ID, entity.Kind, entity.WorkspaceID, entity.Project, entity.Title, entity.Body,
			entity.Heading, tags, properties, entity.Status, entity.Priority, nullString(entity.ParentID),
			nullString(entity.SourceNoteID), entity.LegacyID, entity.LegacySource,
			formatTime(entity.CreatedAt), formatTime(entity.UpdatedAt), formatOptionalTime(entity.FinishedAt),
			entity.Revision, formatOptionalTime(entity.DeletedAt))
		if err != nil {
			return report, fmt.Errorf("import legacy item %q: %w", entity.LegacyID, err)
		}
		report.Created++
	}
	if err := tx.Commit(); err != nil {
		return report, fmt.Errorf("commit v2 import: %w", err)
	}
	return report, nil
}

func (s *Store) prepareEntity(entity domain.Entity) (domain.Entity, error) {
	if strings.TrimSpace(entity.ID) == "" {
		id, err := domain.NewID()
		if err != nil {
			return domain.Entity{}, err
		}
		entity.ID = id
	}
	if strings.TrimSpace(entity.WorkspaceID) == "" {
		entity.WorkspaceID = defaultWorkspaceID
	}
	if entity.Kind == domain.KindTask && entity.Status == "" {
		entity.Status = domain.StatusDraft
	}
	now := s.now()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	if entity.UpdatedAt.IsZero() {
		entity.UpdatedAt = entity.CreatedAt
	}
	if entity.Revision == 0 {
		entity.Revision = 1
	}
	entity.Tags = domain.NormalizeTags(entity.Tags)
	if entity.Properties == nil {
		entity.Properties = map[string]string{}
	}
	if err := entity.Validate(); err != nil {
		return domain.Entity{}, err
	}
	return entity, nil
}

func encodeMetadata(entity domain.Entity) (string, string, error) {
	tags, err := json.Marshal(domain.NormalizeTags(entity.Tags))
	if err != nil {
		return "", "", fmt.Errorf("encode entity tags: %w", err)
	}
	properties := entity.Properties
	if properties == nil {
		properties = map[string]string{}
	}
	encodedProperties, err := json.Marshal(properties)
	if err != nil {
		return "", "", fmt.Errorf("encode entity properties: %w", err)
	}
	return string(tags), string(encodedProperties), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEntity(row rowScanner) (domain.Entity, error) {
	var entity domain.Entity
	var tagsJSON, propertiesJSON, created, updated string
	var parentID, sourceNoteID, legacyID, legacySource sql.NullString
	var finishedAt, deletedAt sql.NullString
	if err := row.Scan(&entity.ID, &entity.Kind, &entity.WorkspaceID, &entity.Project, &entity.Title, &entity.Body,
		&entity.Heading, &tagsJSON, &propertiesJSON, &entity.Status, &entity.Priority, &parentID,
		&sourceNoteID, &legacyID, &legacySource, &created, &updated, &finishedAt, &entity.Revision, &deletedAt); err != nil {
		return domain.Entity{}, fmt.Errorf("scan v2 entity: %w", err)
	}
	entity.ParentID = parentID.String
	entity.SourceNoteID = sourceNoteID.String
	entity.LegacyID = legacyID.String
	entity.LegacySource = legacySource.String
	var err error
	entity.CreatedAt, err = parseTime(created)
	if err != nil {
		return domain.Entity{}, err
	}
	entity.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return domain.Entity{}, err
	}
	if finishedAt.Valid && finishedAt.String != "" {
		value, err := parseTime(finishedAt.String)
		if err != nil {
			return domain.Entity{}, err
		}
		entity.FinishedAt = &value
	}
	if deletedAt.Valid && deletedAt.String != "" {
		value, err := parseTime(deletedAt.String)
		if err != nil {
			return domain.Entity{}, err
		}
		entity.DeletedAt = &value
	}
	if err := json.Unmarshal([]byte(tagsJSON), &entity.Tags); err != nil {
		return domain.Entity{}, fmt.Errorf("decode entity tags: %w", err)
	}
	if err := json.Unmarshal([]byte(propertiesJSON), &entity.Properties); err != nil {
		return domain.Entity{}, fmt.Errorf("decode entity properties: %w", err)
	}
	return entity, nil
}

func (s *Store) prepareRun(run domain.Run) (domain.Run, error) {
	if run.ID == "" {
		var err error
		run.ID, err = domain.NewID()
		if err != nil {
			return domain.Run{}, err
		}
	}
	if run.WorkspaceID == "" {
		run.WorkspaceID = defaultWorkspaceID
	}
	if run.Status == "" {
		run.Status = domain.RunStatusQueued
	}
	if run.PermissionPolicy == "" {
		run.PermissionPolicy = domain.PermissionReadOnly
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = s.now()
	}
	if run.Revision == 0 {
		run.Revision = 1
	}
	if err := run.Validate(); err != nil {
		return domain.Run{}, err
	}
	if run.Status != domain.RunStatusQueued {
		return domain.Run{}, errors.New("new runs must start queued")
	}
	return run, nil
}

func insertRun(ctx context.Context, tx *sql.Tx, run domain.Run) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO runs(
		id, workspace_id, task_id, provider, model, status, permission_policy,
		context_snapshot, context_artifact_id, summary, error_text, created_at,
		started_at, finished_at, revision
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.WorkspaceID, run.TaskID, run.Provider, run.Model, run.Status, run.PermissionPolicy,
		run.ContextSnapshot, run.ContextArtifactID, run.Summary, run.Error, formatTime(run.CreatedAt),
		formatOptionalTime(run.StartedAt), formatOptionalTime(run.FinishedAt), run.Revision)
	if err != nil {
		return fmt.Errorf("insert v2 run: %w", err)
	}
	return nil
}

func insertArtifact(ctx context.Context, tx *sql.Tx, artifact domain.Artifact) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO artifacts(
		id, workspace_id, run_id, entity_id, kind, name, media_type, size_bytes,
		sha256, storage_key, retention, secret_state, created_at, revision
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifact.ID, artifact.WorkspaceID, nullString(artifact.RunID), nullString(artifact.EntityID), artifact.Kind,
		artifact.Name, artifact.MediaType, artifact.SizeBytes, artifact.SHA256, artifact.StorageKey,
		artifact.Retention, artifact.SecretState, formatTime(artifact.CreatedAt), artifact.Revision)
	if err != nil {
		return fmt.Errorf("insert v2 artifact: %w", err)
	}
	return nil
}

func insertRunEvent(ctx context.Context, tx *sql.Tx, event domain.RunEvent, createdAt time.Time) error {
	if event.ID == "" {
		var err error
		event.ID, err = domain.NewID()
		if err != nil {
			return err
		}
	}
	if event.Sequence == 0 {
		return errors.New("run event sequence is required")
	}
	if event.Kind == "" {
		return errors.New("run event kind is required")
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO run_events(id, run_id, sequence, kind, payload, created_at) VALUES (?, ?, ?, ?, ?, ?)", event.ID, event.RunID, event.Sequence, event.Kind, event.Payload, formatTime(createdAt))
	if err != nil {
		return fmt.Errorf("insert v2 run event: %w", err)
	}
	return nil
}

func appendRunEvent(ctx context.Context, tx *sql.Tx, runID, kind, payload string, createdAt time.Time) (domain.RunEvent, error) {
	var sequence int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence), 0) + 1 FROM run_events WHERE run_id=?", runID).Scan(&sequence); err != nil {
		return domain.RunEvent{}, fmt.Errorf("next v2 run event sequence: %w", err)
	}
	event := domain.RunEvent{RunID: runID, Sequence: sequence, Kind: kind, Payload: payload, CreatedAt: createdAt}
	eventID, err := domain.NewID()
	if err != nil {
		return domain.RunEvent{}, err
	}
	event.ID = eventID
	if err := event.Validate(); err != nil {
		return domain.RunEvent{}, err
	}
	if err := insertRunEvent(ctx, tx, event, createdAt); err != nil {
		return domain.RunEvent{}, err
	}
	return event, nil
}

func scanRun(row rowScanner) (domain.Run, error) {
	var run domain.Run
	var created string
	var startedAt, finishedAt sql.NullString
	if err := row.Scan(&run.ID, &run.WorkspaceID, &run.TaskID, &run.Provider, &run.Model, &run.Status,
		&run.PermissionPolicy, &run.ContextSnapshot, &run.ContextArtifactID, &run.Summary, &run.Error,
		&created, &startedAt, &finishedAt, &run.Revision); err != nil {
		return domain.Run{}, fmt.Errorf("scan v2 run: %w", err)
	}
	var err error
	run.CreatedAt, err = parseTime(created)
	if err != nil {
		return domain.Run{}, err
	}
	if startedAt.Valid && startedAt.String != "" {
		value, err := parseTime(startedAt.String)
		if err != nil {
			return domain.Run{}, err
		}
		run.StartedAt = &value
	}
	if finishedAt.Valid && finishedAt.String != "" {
		value, err := parseTime(finishedAt.String)
		if err != nil {
			return domain.Run{}, err
		}
		run.FinishedAt = &value
	}
	return run, nil
}

func scanRunEvent(row rowScanner) (domain.RunEvent, error) {
	var event domain.RunEvent
	var created string
	if err := row.Scan(&event.ID, &event.RunID, &event.Sequence, &event.Kind, &event.Payload, &created); err != nil {
		return domain.RunEvent{}, fmt.Errorf("scan v2 run event: %w", err)
	}
	var err error
	event.CreatedAt, err = parseTime(created)
	if err != nil {
		return domain.RunEvent{}, err
	}
	return event, nil
}

func scanArtifact(row rowScanner) (domain.Artifact, error) {
	var artifact domain.Artifact
	var runID, entityID sql.NullString
	var created string
	if err := row.Scan(&artifact.ID, &artifact.WorkspaceID, &runID, &entityID, &artifact.Kind, &artifact.Name,
		&artifact.MediaType, &artifact.SizeBytes, &artifact.SHA256, &artifact.StorageKey, &artifact.Retention,
		&artifact.SecretState, &created, &artifact.Revision); err != nil {
		return domain.Artifact{}, fmt.Errorf("scan v2 artifact: %w", err)
	}
	artifact.RunID = runID.String
	artifact.EntityID = entityID.String
	var err error
	artifact.CreatedAt, err = parseTime(created)
	if err != nil {
		return domain.Artifact{}, err
	}
	return artifact, nil
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse v2 timestamp %q: %w", value, err)
	}
	return parsed, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return formatTime(*value)
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func legacyKey(source, legacyID string) string {
	return source + "\x00" + legacyID
}
