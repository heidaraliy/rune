package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Kind string

const (
	KindNote Kind = "note"
	KindTask Kind = "task"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusReady     Status = "ready"
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusBlocked   Status = "blocked"
	StatusReview    Status = "review"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

var validStatuses = map[Status]struct{}{
	StatusDraft: {}, StatusReady: {}, StatusQueued: {}, StatusRunning: {},
	StatusBlocked: {}, StatusReview: {}, StatusCompleted: {},
	StatusFailed: {}, StatusCanceled: {},
}

type Entity struct {
	ID           string            `json:"id"`
	Kind         Kind              `json:"kind"`
	WorkspaceID  string            `json:"workspace_id"`
	Project      string            `json:"project,omitempty"`
	Title        string            `json:"title"`
	Body         string            `json:"body,omitempty"`
	Heading      string            `json:"heading,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Properties   map[string]string `json:"properties,omitempty"`
	Status       Status            `json:"status,omitempty"`
	Priority     int               `json:"priority,omitempty"`
	ParentID     string            `json:"parent_id,omitempty"`
	SourceNoteID string            `json:"source_note_id,omitempty"`
	LegacyID     string            `json:"legacy_id,omitempty"`
	LegacySource string            `json:"legacy_source,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	Revision     int64             `json:"revision"`
	DeletedAt    *time.Time        `json:"deleted_at,omitempty"`
}

type Update struct {
	ExpectedRevision int64
	Title            *string
	Body             *string
	AppendBody       *string
	Heading          *string
	Tags             *[]string
	Status           *Status
	Priority         *int
}

type ListOptions struct {
	WorkspaceID    string
	Project        string
	Kind           Kind
	Status         Status
	Query          string
	IncludeDeleted bool
}

type Link struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	FromID      string    `json:"from_id"`
	ToID        string    `json:"to_id"`
	Kind        string    `json:"kind"`
	CreatedAt   time.Time `json:"created_at"`
	Revision    int64     `json:"revision"`
}

var validLinkKinds = map[string]struct{}{
	"references": {}, "contains": {}, "parent": {}, "depends_on": {},
	"blocks": {}, "generated_by": {}, "attached_to": {},
}

func NewID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate entity id: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

func DisplayID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func (e Entity) IsTask() bool {
	return e.Kind == KindTask
}

func (e Entity) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return errors.New("entity id is required")
	}
	if e.Kind != KindNote && e.Kind != KindTask {
		return fmt.Errorf("unsupported entity kind %q", e.Kind)
	}
	if strings.TrimSpace(e.WorkspaceID) == "" {
		return errors.New("workspace id is required")
	}
	if strings.TrimSpace(e.Title) == "" {
		return errors.New("entity title is required")
	}
	if e.IsTask() {
		if e.Status == "" {
			e.Status = StatusDraft
		}
		if _, ok := validStatuses[e.Status]; !ok {
			return fmt.Errorf("unsupported task status %q", e.Status)
		}
	} else if e.Status != "" {
		return errors.New("notes cannot have task status")
	}
	return nil
}

func NormalizeStatus(value string) (Status, error) {
	status := Status(strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "_", "-"))))
	if status == "" {
		return "", nil
	}
	if status == "cancelled" {
		status = StatusCanceled
	}
	if _, ok := validStatuses[status]; !ok {
		return "", fmt.Errorf("unknown task status %q", value)
	}
	return status, nil
}

func (l Link) Validate() error {
	if strings.TrimSpace(l.ID) == "" || strings.TrimSpace(l.WorkspaceID) == "" {
		return errors.New("link id and workspace id are required")
	}
	if strings.TrimSpace(l.FromID) == "" || strings.TrimSpace(l.ToID) == "" {
		return errors.New("link endpoints are required")
	}
	if l.FromID == l.ToID {
		return errors.New("self-links are not allowed")
	}
	if _, ok := validLinkKinds[l.Kind]; !ok {
		return fmt.Errorf("unsupported link kind %q", l.Kind)
	}
	return nil
}

func NormalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}
