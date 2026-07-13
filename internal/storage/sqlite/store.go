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
	if version >= 1 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema migration: %w", err)
	}
	defer tx.Rollback()
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
				return fmt.Errorf("apply schema migration %d: %w", index+1, err)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply schema migration %d: %w", index+1, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema migration: %w", err)
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
