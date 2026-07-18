package runesync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

func TestHTTPPeerRoundTripsChangesAndArtifacts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	remoteStore, err := sqlite.Open(filepath.Join(root, "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer remoteStore.Close()
	remoteArtifacts, err := artifacts.Open(filepath.Join(root, "server-artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(remoteStore, remoteArtifacts, "local", "secret")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	peer, err := OpenHTTPPeer(httpServer.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}

	localStore, err := sqlite.Open(filepath.Join(root, "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer localStore.Close()
	localArtifacts, err := artifacts.Open(filepath.Join(root, "local-artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	entity, err := localStore.Create(ctx, domain.Entity{Kind: domain.KindNote, WorkspaceID: "local", Title: "over HTTP", Body: "from device A"})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("artifact over HTTP")
	blob, err := localArtifacts.Put(ctx, content)
	if err != nil {
		t.Fatal(err)
	}
	artifactID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localStore.CreateArtifact(ctx, domain.Artifact{
		ID:          artifactID,
		WorkspaceID: "local",
		EntityID:    entity.ID,
		Kind:        "note",
		Name:        "evidence.txt",
		MediaType:   "text/plain",
		SizeBytes:   blob.SizeBytes,
		SHA256:      blob.SHA256,
		StorageKey:  blob.StorageKey,
		Retention:   "normal",
		SecretState: "clear",
		CreatedAt:   time.Now().UTC(),
		Revision:    1,
	}); err != nil {
		t.Fatal(err)
	}

	report, err := (EmbeddedClient{Store: localStore, Artifacts: localArtifacts}).Sync(ctx, SyncRequest{
		WorkspaceID: "local",
		Target:      peer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Protocol != ProtocolVersion || report.Pushed != 2 || report.Pulled != 2 || report.Status.PendingChanges != 0 {
		t.Fatalf("device A report = %#v", report)
	}
	remoteEntity, err := remoteStore.Get(ctx, entity.ID, "local")
	if err != nil || remoteEntity.Title != entity.Title || remoteEntity.Body != entity.Body {
		t.Fatalf("remote entity = %#v, err=%v", remoteEntity, err)
	}

	deviceBStore, err := sqlite.Open(filepath.Join(root, "device-b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer deviceBStore.Close()
	deviceBArtifacts, err := artifacts.Open(filepath.Join(root, "device-b-artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	deviceBReport, err := (EmbeddedClient{Store: deviceBStore, Artifacts: deviceBArtifacts}).Sync(ctx, SyncRequest{
		WorkspaceID: "local",
		Target:      peer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if deviceBReport.Pushed != 0 || deviceBReport.Pulled != 2 {
		t.Fatalf("device B report = %#v", deviceBReport)
	}
	deviceBEntity, err := deviceBStore.Get(ctx, entity.ID, "local")
	if err != nil || deviceBEntity.Body != "from device A" {
		t.Fatalf("device B entity = %#v, err=%v", deviceBEntity, err)
	}
	remoteArtifact, err := deviceBStore.GetArtifact(ctx, artifactID, "local")
	if err != nil {
		t.Fatal(err)
	}
	gotContent, err := deviceBArtifacts.Read(ctx, remoteArtifact.StorageKey)
	if err != nil || string(gotContent) != string(content) {
		t.Fatalf("device B artifact = %q, err=%v", gotContent, err)
	}
}

func TestHTTPPeerRequiresTokenAndServerWorkspace(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	artifactStore, err := artifacts.Open(filepath.Join(t.TempDir(), "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(store, artifactStore, "local", "secret")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	wrongToken, err := OpenHTTPPeer(httpServer.URL, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrongToken.Changes(context.Background(), "local", 0, 10); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("wrong token error = %v", err)
	}
	peer, err := OpenHTTPPeer(httpServer.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Changes(context.Background(), "other", 0, 10); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("wrong workspace error = %v", err)
	}

	response, err := http.Get(httpServer.URL + healthPath)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", response.StatusCode)
	}
	var health map[string]string
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil || health["protocol"] != ProtocolVersion {
		t.Fatalf("health = %#v, err=%v", health, err)
	}
}

func TestOpenHTTPPeerValidatesURLAndToken(t *testing.T) {
	for _, test := range []struct {
		name  string
		url   string
		token string
	}{
		{name: "missing URL", token: "secret"},
		{name: "file URL", url: "file:///tmp/rune", token: "secret"},
		{name: "missing token", url: "https://rune.example", token: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := OpenHTTPPeer(test.url, test.token); err == nil {
				t.Fatal("invalid HTTP peer unexpectedly accepted")
			}
		})
	}
}
