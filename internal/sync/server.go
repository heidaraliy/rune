package runesync

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

// Server is the single-workspace HTTP implementation of the sync target. It
// is intentionally small and self-hostable for dogfooding; multi-user auth,
// workspace membership, and deployment policy remain outside this slice.
type Server struct {
	Store       *sqlite.Store
	Artifacts   *artifacts.Store
	WorkspaceID string
	Token       string
}

func NewServer(store *sqlite.Store, artifactStore *artifacts.Store, workspaceID, token string) (*Server, error) {
	if store == nil {
		return nil, errors.New("sync server store is required")
	}
	if artifactStore == nil {
		return nil, errors.New("sync server artifact store is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		workspaceID = "local"
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("sync server token is required")
	}
	return &Server{Store: store, Artifacts: artifactStore, WorkspaceID: workspaceID, Token: token}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == healthPath {
		if r.Method != http.MethodGet {
			writeHTTPError(w, http.StatusMethodNotAllowed, "health endpoint only accepts GET")
			return
		}
		writeHTTPJSON(w, http.StatusOK, map[string]string{
			"protocol": ProtocolVersion,
			"status":   "ok",
		})
		return
	}
	if !s.authorized(r) {
		writeHTTPError(w, http.StatusUnauthorized, "sync authorization is required")
		return
	}
	switch r.URL.Path {
	case changesPath:
		s.handleChanges(w, r)
	case artifactPath:
		s.handleArtifact(w, r)
	default:
		writeHTTPError(w, http.StatusNotFound, "sync endpoint not found")
	}
}

func (s *Server) handleChanges(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		workspaceID := r.URL.Query().Get("workspace_id")
		if err := s.requireWorkspace(workspaceID); err != nil {
			writeHTTPError(w, http.StatusForbidden, err.Error())
			return
		}
		afterCursor, err := parseNonNegativeInt64(r.URL.Query().Get("after_cursor"), 0)
		if err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		limit, err := parseNonNegativeInt(r.URL.Query().Get("limit"), defaultChangeSize)
		if err != nil || limit <= 0 || limit > 1000 {
			writeHTTPError(w, http.StatusBadRequest, "sync change limit must be between 1 and 1000")
			return
		}
		changes, err := s.Store.ListChanges(r.Context(), workspaceID, afterCursor, limit)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if changes == nil {
			changes = []domain.Change{}
		}
		writeHTTPJSON(w, http.StatusOK, ChangesResponse{Protocol: ProtocolVersion, Changes: changes})
	case http.MethodPost:
		var request ChangeRequest
		if err := decodeHTTPJSON(w, r, &request); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requireProtocol(request.Protocol); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.requireWorkspace(request.Change.WorkspaceID); err != nil {
			writeHTTPError(w, http.StatusForbidden, err.Error())
			return
		}
		request.Change.Origin = domain.ChangeOriginRemote
		stored, conflict, err := s.Store.ApplyRemoteChange(r.Context(), request.Change)
		if err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeHTTPJSON(w, http.StatusOK, ChangeResponse{Protocol: ProtocolVersion, Change: stored, Conflict: conflict})
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		writeHTTPError(w, http.StatusMethodNotAllowed, "sync changes endpoint does not support this method")
	}
}

func (s *Server) handleArtifact(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		workspaceID := r.URL.Query().Get("workspace_id")
		if err := s.requireWorkspace(workspaceID); err != nil {
			writeHTTPError(w, http.StatusForbidden, err.Error())
			return
		}
		storageKey := strings.TrimSpace(r.URL.Query().Get("storage_key"))
		if storageKey == "" {
			writeHTTPError(w, http.StatusBadRequest, "sync artifact storage key is required")
			return
		}
		content, err := s.Artifacts.Read(r.Context(), storageKey)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, os.ErrNotExist) {
				status = http.StatusNotFound
			}
			writeHTTPError(w, status, err.Error())
			return
		}
		writeHTTPJSON(w, http.StatusOK, ArtifactResponse{Protocol: ProtocolVersion, Content: content})
	case http.MethodPut:
		var request ArtifactTransfer
		if err := decodeHTTPJSON(w, r, &request); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := requireProtocol(request.Protocol); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.requireWorkspace(request.Artifact.WorkspaceID); err != nil {
			writeHTTPError(w, http.StatusForbidden, err.Error())
			return
		}
		if request.Artifact.SecretState == "withheld" {
			writeHTTPError(w, http.StatusForbidden, "withheld artifacts cannot sync")
			return
		}
		if err := request.Artifact.Validate(); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		blob, err := s.Artifacts.Put(r.Context(), request.Content)
		if err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		if blob.SHA256 != request.Artifact.SHA256 || blob.SizeBytes != request.Artifact.SizeBytes || blob.StorageKey != request.Artifact.StorageKey {
			writeHTTPError(w, http.StatusBadRequest, fmt.Sprintf("artifact %s metadata does not match content", domain.DisplayID(request.Artifact.ID)))
			return
		}
		writeHTTPJSON(w, http.StatusOK, ArtifactResponse{Protocol: ProtocolVersion})
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPut)
		writeHTTPError(w, http.StatusMethodNotAllowed, "sync artifact endpoint does not support this method")
	}
}

func (s *Server) authorized(r *http.Request) bool {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[1]), []byte(s.Token)) == 1
}

func (s *Server) requireWorkspace(workspaceID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return errors.New("sync workspace id is required")
	}
	if workspaceID != s.WorkspaceID {
		return fmt.Errorf("sync workspace %q is not available", workspaceID)
	}
	return nil
}

func parseNonNegativeInt(value string, fallback int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid non-negative integer %q", value)
	}
	return parsed, nil
}

func parseNonNegativeInt64(value string, fallback int64) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid non-negative integer %q", value)
	}
	return parsed, nil
}

func decodeHTTPJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode sync request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("sync request contains multiple JSON values")
		}
		return fmt.Errorf("decode sync request: %w", err)
	}
	return nil
}

func writeHTTPJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeHTTPError(w http.ResponseWriter, status int, message string) {
	writeHTTPJSON(w, status, ErrorResponse{Protocol: ProtocolVersion, Error: message})
}

var _ http.Handler = (*Server)(nil)
