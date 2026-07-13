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

type RunStatus string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusReview    RunStatus = "review"
	RunStatusCompleted RunStatus = "completed"
	RunStatusBlocked   RunStatus = "blocked"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

var validRunStatuses = map[RunStatus]struct{}{
	RunStatusQueued: {}, RunStatusRunning: {}, RunStatusReview: {},
	RunStatusCompleted: {}, RunStatusBlocked: {}, RunStatusFailed: {},
	RunStatusCanceled: {},
}

type PermissionPolicy string

const (
	PermissionReadOnly       PermissionPolicy = "read-only"
	PermissionWorkspaceWrite PermissionPolicy = "workspace-write"
	PermissionFull           PermissionPolicy = "full"
)

var validPermissionPolicies = map[PermissionPolicy]struct{}{
	PermissionReadOnly: {}, PermissionWorkspaceWrite: {}, PermissionFull: {},
}

type Run struct {
	ID                string           `json:"id"`
	WorkspaceID       string           `json:"workspace_id"`
	TaskID            string           `json:"task_id"`
	Provider          string           `json:"provider"`
	Model             string           `json:"model,omitempty"`
	Status            RunStatus        `json:"status"`
	PermissionPolicy  PermissionPolicy `json:"permission_policy"`
	ContextSnapshot   string           `json:"context_snapshot,omitempty"`
	ContextArtifactID string           `json:"context_artifact_id,omitempty"`
	Summary           string           `json:"summary,omitempty"`
	Error             string           `json:"error,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	StartedAt         *time.Time       `json:"started_at,omitempty"`
	FinishedAt        *time.Time       `json:"finished_at,omitempty"`
	Revision          int64            `json:"revision"`
}

type RunEvent struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id"`
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	Payload   string    `json:"payload,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Artifact struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	RunID       string    `json:"run_id,omitempty"`
	EntityID    string    `json:"entity_id,omitempty"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	MediaType   string    `json:"media_type"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	StorageKey  string    `json:"storage_key"`
	Retention   string    `json:"retention"`
	SecretState string    `json:"secret_state"`
	CreatedAt   time.Time `json:"created_at"`
	Revision    int64     `json:"revision"`
}

type RunListOptions struct {
	WorkspaceID string
	TaskID      string
	Status      RunStatus
}

func NormalizeRunStatus(value string) (RunStatus, error) {
	status := RunStatus(strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "_", "-"))))
	if status == "cancelled" {
		status = RunStatusCanceled
	}
	if _, ok := validRunStatuses[status]; !ok {
		return "", fmt.Errorf("unknown run status %q", value)
	}
	return status, nil
}

func NormalizePermissionPolicy(value string) (PermissionPolicy, error) {
	policy := PermissionPolicy(strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "_", "-"))))
	if policy == "" {
		policy = PermissionReadOnly
	}
	if _, ok := validPermissionPolicies[policy]; !ok {
		return "", fmt.Errorf("unknown permission policy %q", value)
	}
	return policy, nil
}

func (r Run) Validate() error {
	if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.WorkspaceID) == "" {
		return errors.New("run id and workspace id are required")
	}
	if strings.TrimSpace(r.TaskID) == "" {
		return errors.New("run task id is required")
	}
	if strings.TrimSpace(r.Provider) == "" {
		return errors.New("run provider is required")
	}
	if _, ok := validRunStatuses[r.Status]; !ok {
		return fmt.Errorf("unsupported run status %q", r.Status)
	}
	if _, ok := validPermissionPolicies[r.PermissionPolicy]; !ok {
		return fmt.Errorf("unsupported permission policy %q", r.PermissionPolicy)
	}
	if r.Revision < 1 {
		return errors.New("run revision must be positive")
	}
	return nil
}

func CanTransitionRun(from, to RunStatus) bool {
	switch from {
	case RunStatusQueued:
		return to == RunStatusRunning || to == RunStatusCanceled || to == RunStatusFailed
	case RunStatusRunning:
		return to == RunStatusReview || to == RunStatusBlocked || to == RunStatusFailed || to == RunStatusCanceled
	case RunStatusReview:
		return to == RunStatusCompleted || to == RunStatusBlocked || to == RunStatusFailed || to == RunStatusCanceled
	default:
		return false
	}
}

func (e RunEvent) Validate() error {
	if strings.TrimSpace(e.ID) == "" || strings.TrimSpace(e.RunID) == "" {
		return errors.New("run event id and run id are required")
	}
	if strings.TrimSpace(e.Kind) == "" {
		return errors.New("run event kind is required")
	}
	return nil
}

func (a Artifact) Validate() error {
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.WorkspaceID) == "" {
		return errors.New("artifact id and workspace id are required")
	}
	if strings.TrimSpace(a.Kind) == "" || strings.TrimSpace(a.Name) == "" {
		return errors.New("artifact kind and name are required")
	}
	if strings.TrimSpace(a.MediaType) == "" {
		return errors.New("artifact media type is required")
	}
	if a.SizeBytes < 0 {
		return errors.New("artifact size cannot be negative")
	}
	if len(strings.TrimSpace(a.SHA256)) != 64 {
		return errors.New("artifact sha256 must be a 64-character hex digest")
	}
	if _, err := hex.DecodeString(a.SHA256); err != nil {
		return fmt.Errorf("artifact sha256 is not hexadecimal: %w", err)
	}
	if strings.TrimSpace(a.StorageKey) == "" {
		return errors.New("artifact storage key is required")
	}
	if a.Retention != "ephemeral" && a.Retention != "normal" && a.Retention != "permanent" {
		return fmt.Errorf("unsupported artifact retention %q", a.Retention)
	}
	if a.SecretState != "clear" && a.SecretState != "redacted" && a.SecretState != "withheld" {
		return fmt.Errorf("unsupported artifact secret state %q", a.SecretState)
	}
	return nil
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
