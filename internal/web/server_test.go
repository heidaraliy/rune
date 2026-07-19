package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heidaraliy/rune/internal/application"
	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
	runesync "github.com/heidaraliy/rune/internal/sync"
)

func testServer(t *testing.T, target runesync.SyncTarget) (*Server, application.V2Service) {
	t.Helper()
	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "rune-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	blobs, err := artifacts.Open(filepath.Join(root, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewV2ExecutionService(store, "local", blobs)
	server, err := NewServer(service, "local", "rune", "secret", target)
	if err != nil {
		t.Fatal(err)
	}
	return server, service
}

func doRequest(t *testing.T, handler http.Handler, method, path string, body any, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		request.Header.Set("Authorization", "Bearer secret")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func responseJSON(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("response JSON %q: %v", recorder.Body.String(), err)
	}
}

func TestServerServesUIHealthAndProtectsAPI(t *testing.T) {
	server, _ := testServer(t, nil)
	ui := doRequest(t, server, http.MethodGet, "/", nil, false)
	if ui.Code != http.StatusOK || !strings.Contains(ui.Body.String(), "What should Rune remember?") || !strings.Contains(ui.Body.String(), "Your Runes") || !strings.Contains(ui.Body.String(), "Ideas") || !strings.Contains(ui.Body.String(), "References") || !strings.Contains(ui.Body.String(), "new-rune") || !strings.Contains(ui.Body.String(), "brand-ascii") || !strings.Contains(ui.Body.String(), "fonts.googleapis.com") {
		t.Fatalf("UI response code=%d body=%q", ui.Code, ui.Body.String())
	}
	if ui.Header().Get("X-Content-Type-Options") != "nosniff" || ui.Header().Get("X-Frame-Options") != "DENY" || ui.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("UI security headers=%v", ui.Header())
	}
	styles := doRequest(t, server, http.MethodGet, "/static/styles.css", nil, false)
	if styles.Code != http.StatusOK || !strings.Contains(styles.Body.String(), "--canvas:") || !strings.Contains(styles.Body.String(), "--indigo:") || !strings.Contains(styles.Body.String(), "Philosopher") || !strings.Contains(styles.Body.String(), ".collection-card") {
		t.Fatalf("visual stylesheet code=%d body=%q", styles.Code, styles.Body.String())
	}
	if csp := ui.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "fonts.googleapis.com") || !strings.Contains(csp, "fonts.gstatic.com") {
		t.Fatalf("font CSP=%q", csp)
	}
	health := doRequest(t, server, http.MethodGet, "/healthz", nil, false)
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), protocol) {
		t.Fatalf("health response code=%d body=%q", health.Code, health.Body.String())
	}
	unauthorized := doRequest(t, server, http.MethodGet, "/v1/runes", nil, false)
	if unauthorized.Code != http.StatusUnauthorized || !strings.Contains(unauthorized.Body.String(), "authorization") {
		t.Fatalf("unauthorized response code=%d body=%q", unauthorized.Code, unauthorized.Body.String())
	}
}

func TestServerCRUDIsRevisionCheckedAndTombstoneSafe(t *testing.T) {
	server, _ := testServer(t, nil)
	createdResponse := doRequest(t, server, http.MethodPost, "/v1/runes", map[string]any{
		"kind":  "task",
		"title": "browser capture",
		"body":  "shape this in the browser",
	}, true)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createdResponse.Code, createdResponse.Body.String())
	}
	var created struct {
		Rune domain.Rune `json:"rune"`
	}
	responseJSON(t, createdResponse, &created)
	if created.Rune.Title != "browser capture" || created.Rune.Revision != 1 || created.Rune.Project != "rune" {
		t.Fatalf("created Rune=%#v", created.Rune)
	}

	listResponse := doRequest(t, server, http.MethodGet, "/v1/runes?kind=task&query=browser", nil, true)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "browser capture") {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}

	patchPath := "/v1/runes/" + created.Rune.ID
	updatedResponse := doRequest(t, server, http.MethodPatch, patchPath, map[string]any{
		"expected_revision": 1,
		"title":             "edited in browser",
		"body":              "a more useful body",
		"state":             "ready",
	}, true)
	if updatedResponse.Code != http.StatusOK || !strings.Contains(updatedResponse.Body.String(), "edited in browser") {
		t.Fatalf("update status=%d body=%s", updatedResponse.Code, updatedResponse.Body.String())
	}
	var updated struct {
		Rune domain.Rune `json:"rune"`
	}
	responseJSON(t, updatedResponse, &updated)
	if updated.Rune.Revision != 2 || updated.Rune.State != domain.StateReady {
		t.Fatalf("updated Rune=%#v", updated.Rune)
	}

	staleResponse := doRequest(t, server, http.MethodPatch, patchPath, map[string]any{
		"expected_revision": 1,
		"title":             "stale write",
	}, true)
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale update status=%d body=%s", staleResponse.Code, staleResponse.Body.String())
	}

	deletedResponse := doRequest(t, server, http.MethodDelete, patchPath, map[string]any{"expected_revision": 2}, true)
	if deletedResponse.Code != http.StatusOK || !strings.Contains(deletedResponse.Body.String(), "deleted_at") {
		t.Fatalf("delete status=%d body=%s", deletedResponse.Code, deletedResponse.Body.String())
	}
	var deleted struct {
		Rune domain.Rune `json:"rune"`
	}
	responseJSON(t, deletedResponse, &deleted)
	if deleted.Rune.DeletedAt == nil || deleted.Rune.Revision != 3 {
		t.Fatalf("deleted Rune=%#v", deleted.Rune)
	}

	restoredResponse := doRequest(t, server, http.MethodPost, patchPath+"/restore", map[string]any{"expected_revision": 3}, true)
	if restoredResponse.Code != http.StatusOK {
		t.Fatalf("restore status=%d body=%s", restoredResponse.Code, restoredResponse.Body.String())
	}
	var restored struct {
		Rune domain.Rune `json:"rune"`
	}
	responseJSON(t, restoredResponse, &restored)
	if restored.Rune.DeletedAt != nil || restored.Rune.Revision != 4 || restored.Rune.Title != "edited in browser" {
		t.Fatalf("restored Rune=%#v", restored.Rune)
	}
}

func TestServerQueuesRunsAndReportsSyncState(t *testing.T) {
	server, service := testServer(t, nil)
	entity, err := service.Create(context.Background(), domain.Rune{Kind: domain.KindTask, Project: "rune", Title: "queue from browser"})
	if err != nil {
		t.Fatal(err)
	}
	queueResponse := doRequest(t, server, http.MethodPost, "/v1/runes/"+entity.ID+"/queue", map[string]any{}, true)
	if queueResponse.Code != http.StatusCreated {
		t.Fatalf("queue status=%d body=%s", queueResponse.Code, queueResponse.Body.String())
	}
	runsResponse := doRequest(t, server, http.MethodGet, "/v1/runs", nil, true)
	if runsResponse.Code != http.StatusOK || !strings.Contains(runsResponse.Body.String(), "task_id") || !strings.Contains(runsResponse.Body.String(), "queued") {
		t.Fatalf("runs status=%d body=%s", runsResponse.Code, runsResponse.Body.String())
	}
	statusResponse := doRequest(t, server, http.MethodGet, "/v1/status", nil, true)
	if statusResponse.Code != http.StatusOK || !strings.Contains(statusResponse.Body.String(), "not-configured") || !strings.Contains(statusResponse.Body.String(), `"workspace_id":"local"`) {
		t.Fatalf("status status=%d body=%s", statusResponse.Code, statusResponse.Body.String())
	}
	syncResponse := doRequest(t, server, http.MethodPost, "/v1/sync", map[string]any{}, true)
	if syncResponse.Code != http.StatusConflict || !strings.Contains(syncResponse.Body.String(), "not configured") {
		t.Fatalf("unconfigured sync status=%d body=%s", syncResponse.Code, syncResponse.Body.String())
	}
}

func TestServerSyncRouteUsesInjectedTarget(t *testing.T) {
	peer, err := runesync.OpenFilePeer(filepath.Join(t.TempDir(), "remote"))
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	server, service := testServer(t, peer)
	status := doRequest(t, server, http.MethodGet, "/v1/status", nil, true)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"remote_configured":true`) {
		t.Fatalf("configured status=%d body=%s", status.Code, status.Body.String())
	}
	if _, err := service.Create(context.Background(), domain.Rune{Kind: domain.KindNote, Project: "rune", Title: "sync through browser"}); err != nil {
		t.Fatal(err)
	}
	response := doRequest(t, server, http.MethodPost, "/v1/sync", map[string]any{}, true)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "sync.v1") || !strings.Contains(response.Body.String(), "pushed") {
		t.Fatalf("sync status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestServerBoundsAndValidatesJSONRequests(t *testing.T) {
	server, _ := testServer(t, nil)
	unknownField := doRequest(t, server, http.MethodPost, "/v1/runes", map[string]any{
		"title":      "reject this",
		"unexpected": true,
	}, true)
	if unknownField.Code != http.StatusBadRequest || !strings.Contains(unknownField.Body.String(), "unknown field") {
		t.Fatalf("unknown field status=%d body=%s", unknownField.Code, unknownField.Body.String())
	}

	largeBody := strings.Repeat("x", maxJSONBodyBytes+1)
	oversized := doRequest(t, server, http.MethodPost, "/v1/runes", map[string]any{
		"title": largeBody,
	}, true)
	if oversized.Code != http.StatusBadRequest || !strings.Contains(oversized.Body.String(), "request body too large") {
		t.Fatalf("oversized body status=%d body=%s", oversized.Code, oversized.Body.String())
	}
}
