package domain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Kind string

// RuneKind is the canonical product vocabulary. Kind remains as a source-
// compatible alias for the transitional structured store and existing clients.
type RuneKind = Kind

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

// RuneState is the canonical lifecycle for authored workspace objects. The
// legacy Status field remains as a task/run compatibility projection while
// clients migrate to State.
type RuneState string

const (
	StateDraft      RuneState = "draft"
	StateReady      RuneState = "ready"
	StateInProgress RuneState = "in_progress"
	StateComplete   RuneState = "complete"
	StateBlocked    RuneState = "blocked"
	StateReview     RuneState = "review"
	StateFailed     RuneState = "failed"
)

var validRuneStates = map[RuneState]struct{}{
	StateDraft: {}, StateReady: {}, StateInProgress: {}, StateComplete: {},
	StateBlocked: {}, StateReview: {}, StateFailed: {},
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
	State        RuneState         `json:"state,omitempty"`
	Priority     int               `json:"priority,omitempty"`
	ParentID     string            `json:"parent_id,omitempty"`
	SiblingOrder int               `json:"sibling_order,omitempty"`
	FacetSet     []RuneFacet       `json:"-"`
	SourceNoteID string            `json:"source_note_id,omitempty"`
	LegacyID     string            `json:"legacy_id,omitempty"`
	LegacySource string            `json:"legacy_source,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	Revision     int64             `json:"revision"`
	DeletedAt    *time.Time        `json:"deleted_at,omitempty"`
}

// Rune is the canonical name for the durable workspace object. Entity is kept
// as the compatibility name while the SQLite table and v2 callers migrate.
type Rune = Entity

type RuneFacet string

const (
	FacetDocument RuneFacet = "document"
	FacetTask     RuneFacet = "task"
)

func (e Entity) Facets() []RuneFacet {
	if len(e.FacetSet) > 0 {
		return append([]RuneFacet(nil), e.FacetSet...)
	}
	return DefaultRuneFacets(e.Kind)
}

func DefaultRuneFacets(kind Kind) []RuneFacet {
	if kind == KindTask {
		return []RuneFacet{FacetDocument, FacetTask}
	}
	return []RuneFacet{FacetDocument}
}

func (e Entity) HasFacet(facet RuneFacet) bool {
	for _, candidate := range e.Facets() {
		if candidate == facet {
			return true
		}
	}
	return false
}

func NormalizeRuneFacets(facets []RuneFacet, kind Kind) ([]RuneFacet, error) {
	if len(facets) == 0 {
		return DefaultRuneFacets(kind), nil
	}
	seen := make(map[RuneFacet]struct{}, len(facets)+1)
	normalized := make([]RuneFacet, 0, len(facets)+1)
	for _, facet := range facets {
		facet = RuneFacet(strings.ToLower(strings.TrimSpace(string(facet))))
		if facet == "" {
			continue
		}
		if facet != FacetDocument && facet != FacetTask {
			return nil, fmt.Errorf("unsupported Rune facet %q", facet)
		}
		if _, ok := seen[facet]; ok {
			continue
		}
		seen[facet] = struct{}{}
		normalized = append(normalized, facet)
	}
	if _, ok := seen[FacetDocument]; !ok {
		normalized = append([]RuneFacet{FacetDocument}, normalized...)
	}
	if kind == KindTask {
		if _, ok := seen[FacetTask]; !ok {
			normalized = append(normalized, FacetTask)
		}
	}
	return normalized, nil
}

func (e Entity) Reference() string {
	return RuneReference(e.ID)
}

func RuneReference(id string) string {
	return "rune://" + strings.TrimSpace(id)
}

// ResolveRuneID accepts both the existing short/full ID forms and a stable
// rune:// reference so clients can share identifiers without a format fork.
func ResolveRuneID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "rune://") {
		value = strings.TrimPrefix(value, "rune://")
		if value == "" || strings.ContainsAny(value, "/?#") {
			return "", fmt.Errorf("invalid Rune reference %q", value)
		}
		return value, nil
	}
	if strings.Contains(value, "://") {
		return "", fmt.Errorf("unsupported Rune reference %q", value)
	}
	return value, nil
}

// MarshalJSON keeps the transitional kind/status fields while exposing stable
// client facets and Rune lifecycle state.
func (e Entity) MarshalJSON() ([]byte, error) {
	type entityJSON Entity
	return json.Marshal(struct {
		entityJSON
		Ref    string      `json:"ref"`
		Facets []RuneFacet `json:"facets"`
	}{
		entityJSON: entityJSON(e),
		Ref:        e.Reference(),
		Facets:     e.Facets(),
	})
}

func (e *Entity) UnmarshalJSON(data []byte) error {
	type entityJSON Entity
	var decoded struct {
		entityJSON
		Ref    string      `json:"ref"`
		Facets []RuneFacet `json:"facets"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*e = Entity(decoded.entityJSON)
	e.FacetSet = decoded.Facets
	return nil
}

func NormalizeRuneState(value string) (RuneState, error) {
	state := RuneState(strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "-", "_"))))
	if state == "" {
		return "", nil
	}
	if state == "completed" {
		state = StateComplete
	}
	if _, ok := validRuneStates[state]; !ok {
		return "", fmt.Errorf("unknown Rune state %q", value)
	}
	return state, nil
}

func RuneStateFromStatus(status Status, _ Kind) RuneState {
	switch status {
	case StatusReady:
		return StateReady
	case StatusRunning:
		return StateInProgress
	case StatusBlocked:
		return StateBlocked
	case StatusReview:
		return StateReview
	case StatusCompleted:
		return StateComplete
	case StatusFailed:
		return StateFailed
	case StatusDraft:
		return StateDraft
	case StatusQueued, StatusCanceled:
		return StateReady
	default:
		return StateDraft
	}
}

func LegacyStatusFromRuneState(state RuneState) Status {
	switch state {
	case StateReady:
		return StatusReady
	case StateInProgress:
		return StatusRunning
	case StateBlocked:
		return StatusBlocked
	case StateReview:
		return StatusReview
	case StateComplete:
		return StatusCompleted
	case StateFailed:
		return StatusFailed
	default:
		return StatusDraft
	}
}

func NextRuneState(state RuneState) RuneState {
	switch state {
	case StateDraft:
		return StateReady
	case StateReady:
		return StateInProgress
	case StateInProgress:
		return StateComplete
	default:
		return StateDraft
	}
}

type Update struct {
	ExpectedRevision int64
	Title            *string
	Body             *string
	AppendBody       *string
	Heading          *string
	Tags             *[]string
	Status           *Status
	State            *RuneState
	Priority         *int
	ParentID         *string
	SiblingOrder     *int
}

type RuneUpdate = Update

type RuneSortField string

const (
	RuneSortUpdatedAt RuneSortField = "updated_at"
	RuneSortCreatedAt RuneSortField = "created_at"
	RuneSortTitle     RuneSortField = "title"
	RuneSortPriority  RuneSortField = "priority"
	RuneSortStatus    RuneSortField = "status"
	RuneSortOrder     RuneSortField = "sibling_order"
)

func NormalizeRuneSort(value string) (RuneSortField, error) {
	sort := RuneSortField(strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "-", "_"))))
	if sort == "" {
		return "", nil
	}
	switch sort {
	case RuneSortUpdatedAt, RuneSortCreatedAt, RuneSortTitle, RuneSortPriority, RuneSortStatus, RuneSortOrder:
		return sort, nil
	default:
		return "", fmt.Errorf("unknown Rune sort %q", value)
	}
}

type ListOptions struct {
	WorkspaceID    string
	Project        string
	Kind           Kind
	Status         Status
	State          RuneState
	Query          string
	ParentID       string
	SortBy         RuneSortField
	Reverse        bool
	Limit          int
	Offset         int
	IncludeDeleted bool
}

type RuneQuery = ListOptions

type Link struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	FromID      string    `json:"from_id"`
	ToID        string    `json:"to_id"`
	Kind        string    `json:"kind"`
	CreatedAt   time.Time `json:"created_at"`
	Revision    int64     `json:"revision"`
}

type Change struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Cursor      int64     `json:"cursor"`
	Origin      string    `json:"origin,omitempty"`
	OperationID string    `json:"operation_id"`
	ActorID     string    `json:"actor_id"`
	DeviceID    string    `json:"device_id"`
	Kind        string    `json:"kind"`
	EntityID    string    `json:"entity_id,omitempty"`
	Revision    int64     `json:"revision"`
	Payload     string    `json:"payload"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	ChangeOriginLocal  = "local"
	ChangeOriginRemote = "remote"
)

type Conflict struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	EntityID       string    `json:"entity_id"`
	Kind           string    `json:"kind"`
	LocalRevision  int64     `json:"local_revision"`
	RemoteRevision int64     `json:"remote_revision"`
	LocalPayload   string    `json:"local_payload"`
	RemotePayload  string    `json:"remote_payload"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type SyncStatus struct {
	WorkspaceID    string `json:"workspace_id"`
	RemoteState    string `json:"remote_state"`
	RemoteID       string `json:"remote_id,omitempty"`
	LocalCursor    int64  `json:"local_cursor"`
	PushedCursor   int64  `json:"pushed_cursor"`
	PulledCursor   int64  `json:"pulled_cursor"`
	PendingChanges int    `json:"pending_changes"`
	OpenConflicts  int    `json:"open_conflicts"`
}

type SyncState struct {
	WorkspaceID  string    `json:"workspace_id"`
	RemoteID     string    `json:"remote_id"`
	PushedCursor int64     `json:"pushed_cursor"`
	PulledCursor int64     `json:"pulled_cursor"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (c Change) Validate() error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.WorkspaceID) == "" {
		return errors.New("change id and workspace id are required")
	}
	if c.Cursor < 0 {
		return errors.New("change cursor cannot be negative")
	}
	if strings.TrimSpace(c.OperationID) == "" || strings.TrimSpace(c.ActorID) == "" || strings.TrimSpace(c.DeviceID) == "" {
		return errors.New("change operation, actor, and device are required")
	}
	if strings.TrimSpace(c.Kind) == "" || strings.TrimSpace(c.Payload) == "" {
		return errors.New("change kind and payload are required")
	}
	if c.Revision < 1 {
		return errors.New("change revision must be positive")
	}
	return nil
}

func (c Conflict) Validate() error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.WorkspaceID) == "" || strings.TrimSpace(c.EntityID) == "" {
		return errors.New("conflict id, workspace id, and entity id are required")
	}
	if strings.TrimSpace(c.Kind) == "" || strings.TrimSpace(c.LocalPayload) == "" || strings.TrimSpace(c.RemotePayload) == "" {
		return errors.New("conflict kind and both payloads are required")
	}
	if c.LocalRevision < 1 || c.RemoteRevision < 1 {
		return errors.New("conflict revisions must be positive")
	}
	if c.Status == "" {
		c.Status = "open"
	}
	if c.Status != "open" {
		return fmt.Errorf("unsupported conflict status %q", c.Status)
	}
	return nil
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
	return e.HasFacet(FacetTask)
}

func (e *Entity) Normalize() error {
	facets, err := NormalizeRuneFacets(e.FacetSet, e.Kind)
	if err != nil {
		return err
	}
	e.FacetSet = facets
	stateWasExplicit := e.State != ""
	if e.State == "" {
		e.State = RuneStateFromStatus(e.Status, e.Kind)
	}
	e.State, err = NormalizeRuneState(string(e.State))
	if err != nil {
		return err
	}
	if e.IsTask() && (e.Status == "" || (stateWasExplicit && !(e.State == StateReady && (e.Status == StatusQueued || e.Status == StatusCanceled)))) {
		e.Status = LegacyStatusFromRuneState(e.State)
	}
	return e.Validate()
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
	if e.SiblingOrder < 0 {
		return errors.New("Rune sibling order cannot be negative")
	}
	if e.ParentID == e.ID && e.ParentID != "" {
		return errors.New("Rune cannot be its own parent")
	}
	if len(e.FacetSet) > 0 {
		if _, err := NormalizeRuneFacets(e.FacetSet, e.Kind); err != nil {
			return err
		}
	}
	if e.State != "" {
		if _, err := NormalizeRuneState(string(e.State)); err != nil {
			return err
		}
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
