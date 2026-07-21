package runesync

import (
	"context"
	"fmt"
	"strings"

	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

// ProtocolVersion identifies the first versioned embedded sync contract. It
// describes the request/report shape, not a hosted endpoint or transport.
const ProtocolVersion = "sync.v1"

// SyncRequest is the runtime request passed to a sync client. Target is an
// adapter supplied by the embedding application and is intentionally omitted
// from JSON because transports own their wire representation.
type SyncRequest struct {
	Protocol    string     `json:"protocol,omitempty"`
	WorkspaceID string     `json:"workspace_id"`
	Target      SyncTarget `json:"-"`
}

// Client is the shared sync boundary used by the embedded Rune application.
// Hosted clients can implement the same request/report shape later without
// exposing the local SQLite store or sync engine to callers.
type Client interface {
	Sync(context.Context, SyncRequest) (Report, error)
}

// EmbeddedClient runs the transport-neutral sync engine against a local store.
// A peer remains an adapter, so the file-backed peer is only one development
// implementation of the target side of this contract.
type EmbeddedClient struct {
	Store     *sqlite.Store
	Artifacts *artifacts.Store
}

func (c EmbeddedClient) Sync(ctx context.Context, request SyncRequest) (Report, error) {
	protocol := strings.TrimSpace(request.Protocol)
	if protocol == "" {
		protocol = ProtocolVersion
	}
	if protocol != ProtocolVersion {
		return Report{}, fmt.Errorf("unsupported sync protocol %q", protocol)
	}
	if strings.TrimSpace(request.WorkspaceID) == "" {
		request.WorkspaceID = "local"
	}
	report, err := (Engine{
		Store:       c.Store,
		Artifacts:   c.Artifacts,
		WorkspaceID: request.WorkspaceID,
		Peer:        request.Target,
	}).Sync(ctx)
	if err != nil {
		return Report{}, err
	}
	report.Protocol = protocol
	return report, nil
}

var _ Client = EmbeddedClient{}
