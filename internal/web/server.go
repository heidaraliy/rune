package web

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/heidaraliy/rune/internal/application"
	"github.com/heidaraliy/rune/internal/domain"
	runesync "github.com/heidaraliy/rune/internal/sync"
)

const (
	protocol         = "rune.web.v1"
	apiPrefix        = "/v1/"
	maxJSONBodyBytes = 2 << 20
	webSyncTimeout   = 35 * time.Second
)

//go:embed static/*
var staticAssets embed.FS

// Server is the first browser-facing Rune client boundary. It serves a small
// same-origin UI and JSON routes over the shared application contract; it does
// not open a store or construct a second domain model.
type Server struct {
	Client      application.RuneClient
	SyncClient  application.SyncClient
	SyncTarget  runesync.SyncTarget
	WorkspaceID string
	Project     string
	Token       string
}

func NewServer(client application.RuneClient, workspaceID, project, token string, target runesync.SyncTarget) (*Server, error) {
	if client == nil {
		return nil, errors.New("web Rune client is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		workspaceID = "local"
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("web server token is required")
	}
	syncClient, _ := client.(application.SyncClient)
	if target != nil && syncClient == nil {
		return nil, errors.New("web Rune client does not support sync")
	}
	return &Server{
		Client:      client,
		SyncClient:  syncClient,
		SyncTarget:  target,
		WorkspaceID: workspaceID,
		Project:     strings.TrimSpace(project),
		Token:       token,
	}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)
	if r.URL.Path == "/healthz" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "health endpoint only accepts GET")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"protocol": protocol, "status": "ok"})
		return
	}
	if r.URL.Path == "/" || r.URL.Path == "/index.html" || strings.HasPrefix(r.URL.Path, "/static/") {
		s.serveAsset(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, apiPrefix) {
		writeError(w, http.StatusNotFound, "web route not found")
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "web authorization is required")
		return
	}
	s.handleAPI(w, r)
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'")
}

func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request) {
	name := "static/index.html"
	if strings.HasPrefix(r.URL.Path, "/static/") {
		name = path.Join("static", strings.TrimPrefix(r.URL.Path, "/static/"))
	}
	if name != "static/index.html" && !strings.HasPrefix(name, "static/") {
		writeError(w, http.StatusNotFound, "web asset not found")
		return
	}
	data, err := staticAssets.ReadFile(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "web asset not found")
		return
	}
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) authorized(r *http.Request) bool {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(value, "Bearer ") {
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
	return subtle.ConstantTimeCompare([]byte(provided), []byte(s.Token)) == 1
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/v1/runes":
		s.handleRunes(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1/runes/"):
		s.handleRune(w, r)
	case r.URL.Path == "/v1/runs":
		s.handleRuns(w, r)
	case r.URL.Path == "/v1/status":
		s.handleStatus(w, r)
	case r.URL.Path == "/v1/sync":
		s.handleSync(w, r)
	default:
		writeError(w, http.StatusNotFound, "web API route not found")
	}
}

func (s *Server) handleRunes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		options, err := listOptions(r, s.Project)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		runes, err := s.Client.List(r.Context(), options)
		if err != nil {
			writeError(w, statusForError(err), err.Error())
			return
		}
		if runes == nil {
			runes = []domain.Rune{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"protocol": protocol,
			"runes":    runes,
			"count":    len(runes),
		})
	case http.MethodPost:
		var request createRuneRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		kind := request.Kind
		if kind == "" {
			kind = domain.KindTask
		}
		entity := domain.Rune{
			Kind:            kind,
			WorkspaceID:     s.WorkspaceID,
			Project:         request.Project,
			Title:           request.Title,
			Body:            request.Body,
			State:           request.State,
			Status:          request.Status,
			Priority:        request.Priority,
			ParentID:        request.ParentID,
			SiblingOrder:    request.SiblingOrder,
			FacetSet:        request.Facets,
			Properties:      request.Properties,
			FacetProperties: request.FacetProperties,
		}
		if entity.Project == "" {
			entity.Project = s.Project
		}
		created, err := s.Client.Create(r.Context(), entity)
		if err != nil {
			writeError(w, statusForError(err), err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"protocol": protocol, "rune": created})
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "runes endpoint only accepts GET and POST")
	}
}

func (s *Server) handleRune(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/runes/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "Rune id is required")
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		s.handleRuneResource(w, r, id)
		return
	}
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "web Rune route not found")
		return
	}
	switch parts[1] {
	case "restore":
		s.handleRuneRestore(w, r, id)
	case "queue":
		s.handleRuneQueue(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "web Rune action not found")
	}
}

func (s *Server) handleRuneResource(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		entity, err := s.Client.GetIncludingDeleted(r.Context(), id)
		if err != nil {
			writeError(w, statusForError(err), err.Error())
			return
		}
		links, err := s.Client.Links(r.Context(), entity.ID)
		if err != nil {
			writeError(w, statusForError(err), err.Error())
			return
		}
		if links == nil {
			links = []domain.Link{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"protocol": protocol, "rune": entity, "links": links})
	case http.MethodPatch:
		var request updateRuneRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if request.ExpectedRevision < 1 {
			writeError(w, http.StatusBadRequest, "expected_revision must be positive")
			return
		}
		updated, err := s.Client.Update(r.Context(), id, request.Update())
		if err != nil {
			writeError(w, statusForError(err), err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"protocol": protocol, "rune": updated})
	case http.MethodDelete:
		var request revisionRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if request.ExpectedRevision < 1 {
			writeError(w, http.StatusBadRequest, "expected_revision must be positive")
			return
		}
		deleted, err := s.Client.Delete(r.Context(), id, request.ExpectedRevision)
		if err != nil {
			writeError(w, statusForError(err), err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"protocol": protocol, "rune": deleted})
	default:
		w.Header().Set("Allow", "GET, PATCH, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "Rune endpoint only accepts GET, PATCH, and DELETE")
	}
}

func (s *Server) handleRuneRestore(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, "restore only accepts POST")
		return
	}
	var request revisionRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.ExpectedRevision < 1 {
		writeError(w, http.StatusBadRequest, "expected_revision must be positive")
		return
	}
	restored, err := s.Client.Restore(r.Context(), id, request.ExpectedRevision)
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"protocol": protocol, "rune": restored})
}

func (s *Server) handleRuneQueue(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, "queue only accepts POST")
		return
	}
	var request queueRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	provider := strings.TrimSpace(request.Provider)
	if provider == "" {
		provider = "fake"
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = "local"
	}
	policy := request.Permission
	if policy == "" {
		policy = domain.PermissionReadOnly
	}
	queued, err := s.Client.QueueRun(r.Context(), id, provider, model, policy)
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"protocol": protocol, "run": queued})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, "runs endpoint only accepts GET")
		return
	}
	statusValue := strings.TrimSpace(r.URL.Query().Get("status"))
	var status domain.RunStatus
	var err error
	if statusValue != "" {
		status, err = domain.NormalizeRunStatus(statusValue)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	runs, err := s.Client.Runs(r.Context(), domain.RunListOptions{TaskID: strings.TrimSpace(r.URL.Query().Get("task_id")), Status: status})
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	if runs == nil {
		runs = []domain.Run{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"protocol": protocol, "runs": runs, "count": len(runs)})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, "status endpoint only accepts GET")
		return
	}
	status, err := s.Client.SyncStatus(r.Context())
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	conflicts, err := s.Client.Conflicts(r.Context())
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	if conflicts == nil {
		conflicts = []domain.Conflict{}
	}
	remoteID := status.RemoteID
	if s.SyncTarget != nil {
		remoteID = s.SyncTarget.ID()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"protocol":     protocol,
		"workspace_id": s.WorkspaceID,
		"project":      s.Project,
		"sync": syncStatusResponse{
			Status:           status,
			RemoteConfigured: s.SyncTarget != nil,
			RemoteID:         remoteID,
		},
		"conflicts": conflicts,
	})
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, "sync endpoint only accepts POST")
		return
	}
	if s.SyncClient == nil || s.SyncTarget == nil {
		writeError(w, http.StatusConflict, "sync is not configured for this Rune web server")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), webSyncTimeout)
	defer cancel()
	report, err := s.SyncClient.Sync(ctx, s.SyncTarget)
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"protocol": protocol, "report": report})
}

type createRuneRequest struct {
	Kind            domain.Kind                            `json:"kind"`
	Project         string                                 `json:"project,omitempty"`
	Title           string                                 `json:"title"`
	Body            string                                 `json:"body,omitempty"`
	State           domain.RuneState                       `json:"state,omitempty"`
	Status          domain.Status                          `json:"status,omitempty"`
	Priority        int                                    `json:"priority,omitempty"`
	ParentID        string                                 `json:"parent_id,omitempty"`
	SiblingOrder    int                                    `json:"sibling_order,omitempty"`
	Facets          []domain.RuneFacet                     `json:"facets,omitempty"`
	Properties      map[string]string                      `json:"properties,omitempty"`
	FacetProperties map[domain.RuneFacet]map[string]string `json:"facet_properties,omitempty"`
}

type updateRuneRequest struct {
	ExpectedRevision int64                       `json:"expected_revision"`
	Title            *string                     `json:"title,omitempty"`
	Body             *string                     `json:"body,omitempty"`
	AppendBody       *string                     `json:"append_body,omitempty"`
	State            *domain.RuneState           `json:"state,omitempty"`
	Status           *domain.Status              `json:"status,omitempty"`
	Priority         *int                        `json:"priority,omitempty"`
	ParentID         *string                     `json:"parent_id,omitempty"`
	SiblingOrder     *int                        `json:"sibling_order,omitempty"`
	Facets           *[]domain.RuneFacet         `json:"facets,omitempty"`
	PropertyChanges  []domain.RunePropertyChange `json:"property_changes,omitempty"`
}

func (r updateRuneRequest) Update() domain.RuneUpdate {
	return domain.RuneUpdate{
		ExpectedRevision: r.ExpectedRevision,
		Title:            r.Title,
		Body:             r.Body,
		AppendBody:       r.AppendBody,
		State:            r.State,
		Status:           r.Status,
		Priority:         r.Priority,
		ParentID:         r.ParentID,
		SiblingOrder:     r.SiblingOrder,
		Facets:           r.Facets,
		PropertyChanges:  r.PropertyChanges,
	}
}

type revisionRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type queueRequest struct {
	Provider   string                  `json:"provider,omitempty"`
	Model      string                  `json:"model,omitempty"`
	Permission domain.PermissionPolicy `json:"permission,omitempty"`
}

type syncStatusResponse struct {
	Status           domain.SyncStatus `json:"status"`
	RemoteConfigured bool              `json:"remote_configured"`
	RemoteID         string            `json:"remote_id,omitempty"`
}

func listOptions(r *http.Request, defaultProject string) (domain.RuneQuery, error) {
	options := domain.RuneQuery{
		Project:        strings.TrimSpace(r.URL.Query().Get("project")),
		Query:          strings.TrimSpace(r.URL.Query().Get("query")),
		ParentID:       strings.TrimSpace(r.URL.Query().Get("parent_id")),
		IncludeDeleted: true,
	}
	if options.Project == "" {
		options.Project = defaultProject
	}
	if value := strings.TrimSpace(r.URL.Query().Get("kind")); value != "" {
		kind := domain.Kind(strings.ToLower(value))
		if kind != domain.KindNote && kind != domain.KindTask {
			return domain.RuneQuery{}, fmt.Errorf("unknown Rune kind %q", value)
		}
		options.Kind = kind
	}
	if value := strings.TrimSpace(r.URL.Query().Get("state")); value != "" {
		state, err := domain.NormalizeRuneState(value)
		if err != nil {
			return domain.RuneQuery{}, err
		}
		options.State = state
	}
	if value := strings.TrimSpace(r.URL.Query().Get("status")); value != "" {
		status, err := domain.NormalizeStatus(value)
		if err != nil {
			return domain.RuneQuery{}, err
		}
		options.Status = status
	}
	if value := strings.TrimSpace(r.URL.Query().Get("sort")); value != "" {
		sortBy, err := domain.NormalizeRuneSort(value)
		if err != nil {
			return domain.RuneQuery{}, err
		}
		options.SortBy = sortBy
	}
	if value := strings.TrimSpace(r.URL.Query().Get("reverse")); value != "" {
		reverse, err := strconv.ParseBool(value)
		if err != nil {
			return domain.RuneQuery{}, fmt.Errorf("invalid reverse flag %q", value)
		}
		options.Reverse = reverse
	}
	if value := strings.TrimSpace(r.URL.Query().Get("include_deleted")); value != "" {
		includeDeleted, err := strconv.ParseBool(value)
		if err != nil {
			return domain.RuneQuery{}, fmt.Errorf("invalid include_deleted flag %q", value)
		}
		options.IncludeDeleted = includeDeleted
	}
	return options, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request contains multiple JSON values")
		}
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return nil
}

func statusForError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "not found"), strings.Contains(message, "no v2 "):
		return http.StatusNotFound
	case strings.Contains(message, "revision conflict"), strings.Contains(message, "ambiguous"):
		return http.StatusConflict
	case strings.Contains(message, "required"), strings.Contains(message, "unsupported"), strings.Contains(message, "invalid"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"protocol": protocol, "error": message})
}
