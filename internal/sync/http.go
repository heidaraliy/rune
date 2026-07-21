package runesync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/heidaraliy/rune/internal/domain"
)

// HTTPPeer adapts the versioned sync protocol to the SyncTarget contract used
// by the local engine. It intentionally carries no local store or auth state
// beyond the bearer token needed to reach the configured personal server.
type HTTPPeer struct {
	baseURL string
	token   string
	client  *http.Client
}

func OpenHTTPPeer(baseURL, token string) (*HTTPPeer, error) {
	baseURL, err := normalizeHTTPBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("sync HTTP token is required")
	}
	return &HTTPPeer{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		client:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (p *HTTPPeer) ID() string {
	if p == nil {
		return ""
	}
	return "http:" + p.baseURL
}

func (p *HTTPPeer) Close() error { return nil }

func (p *HTTPPeer) Accept(ctx context.Context, change domain.Change) (domain.Change, *domain.Conflict, error) {
	var response ChangeResponse
	err := p.requestJSON(ctx, http.MethodPost, changesPath, nil, ChangeRequest{
		Protocol: ProtocolVersion,
		Change:   change,
	}, &response)
	if err != nil {
		return domain.Change{}, nil, err
	}
	if err := requireProtocol(response.Protocol); err != nil {
		return domain.Change{}, nil, err
	}
	return response.Change, response.Conflict, nil
}

func (p *HTTPPeer) Changes(ctx context.Context, workspaceID string, afterCursor int64, limit int) ([]domain.Change, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("sync HTTP workspace id is required")
	}
	if afterCursor < 0 {
		return nil, errors.New("sync HTTP cursor cannot be negative")
	}
	if limit <= 0 {
		limit = defaultChangeSize
	}
	query := url.Values{}
	query.Set("workspace_id", workspaceID)
	query.Set("after_cursor", strconv.FormatInt(afterCursor, 10))
	query.Set("limit", strconv.Itoa(limit))
	var response ChangesResponse
	if err := p.requestJSON(ctx, http.MethodGet, changesPath, query, nil, &response); err != nil {
		return nil, err
	}
	if err := requireProtocol(response.Protocol); err != nil {
		return nil, err
	}
	return response.Changes, nil
}

func (p *HTTPPeer) PutArtifact(ctx context.Context, artifact domain.Artifact, content []byte) error {
	if artifact.SecretState == "withheld" {
		return fmt.Errorf("withheld artifact %s cannot sync", domain.DisplayID(artifact.ID))
	}
	if err := artifact.Validate(); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("workspace_id", artifact.WorkspaceID)
	var response ArtifactResponse
	if err := p.requestJSON(ctx, http.MethodPut, artifactPath, query, ArtifactTransfer{
		Protocol: ProtocolVersion,
		Artifact: artifact,
		Content:  content,
	}, &response); err != nil {
		return err
	}
	return requireProtocol(response.Protocol)
}

func (p *HTTPPeer) ReadArtifact(ctx context.Context, artifact domain.Artifact) ([]byte, error) {
	if strings.TrimSpace(artifact.WorkspaceID) == "" || strings.TrimSpace(artifact.StorageKey) == "" {
		return nil, errors.New("sync HTTP artifact workspace and storage key are required")
	}
	query := url.Values{}
	query.Set("workspace_id", artifact.WorkspaceID)
	query.Set("storage_key", artifact.StorageKey)
	var response ArtifactResponse
	if err := p.requestJSON(ctx, http.MethodGet, artifactPath, query, nil, &response); err != nil {
		return nil, err
	}
	if err := requireProtocol(response.Protocol); err != nil {
		return nil, err
	}
	return response.Content, nil
}

func (p *HTTPPeer) requestJSON(ctx context.Context, method, path string, query url.Values, input, output any) error {
	if p == nil || p.client == nil {
		return errors.New("sync HTTP peer is closed")
	}
	endpoint := p.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var body io.Reader
	if input != nil {
		var encoded bytes.Buffer
		if err := json.NewEncoder(&encoded).Encode(input); err != nil {
			return fmt.Errorf("encode sync HTTP request: %w", err)
		}
		body = &encoded
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("create sync HTTP request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.token)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("sync HTTP request: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxHTTPReadBytes))
	if err != nil {
		return fmt.Errorf("read sync HTTP response: %w", err)
	}
	if int64(len(data)) > maxJSONBodyBytes {
		return errors.New("sync HTTP response exceeds size limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var failure ErrorResponse
		if json.Unmarshal(data, &failure) == nil && strings.TrimSpace(failure.Error) != "" {
			return fmt.Errorf("sync HTTP %s: %s", response.Status, failure.Error)
		}
		return fmt.Errorf("sync HTTP %s", response.Status)
	}
	if output == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode sync HTTP response: %w", err)
	}
	return nil
}

func normalizeHTTPBaseURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("sync HTTP URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid sync HTTP URL %q", value)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("sync HTTP URL must use http or https, got %q", parsed.Scheme)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("sync HTTP URL cannot include credentials, query, or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func requireProtocol(protocol string) error {
	if strings.TrimSpace(protocol) != ProtocolVersion {
		return fmt.Errorf("unsupported sync protocol %q", protocol)
	}
	return nil
}

var _ SyncTarget = (*HTTPPeer)(nil)
