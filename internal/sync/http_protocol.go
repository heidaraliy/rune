package runesync

import "github.com/heidaraliy/rune/internal/domain"

const (
	healthPath        = "/healthz"
	changesPath       = "/v1/sync/changes"
	artifactPath      = "/v1/sync/artifact"
	maxJSONBodyBytes  = 32 << 20
	maxHTTPReadBytes  = maxJSONBodyBytes + 1
	defaultChangeSize = 100
)

// ChangeRequest is the versioned wire envelope for accepting one revisioned
// change into a sync server.
type ChangeRequest struct {
	Protocol string        `json:"protocol"`
	Change   domain.Change `json:"change"`
}

type ChangeResponse struct {
	Protocol string           `json:"protocol"`
	Change   domain.Change    `json:"change"`
	Conflict *domain.Conflict `json:"conflict,omitempty"`
}

type ChangesResponse struct {
	Protocol string          `json:"protocol"`
	Changes  []domain.Change `json:"changes"`
}

// ArtifactTransfer keeps the first network transport JSON-only. The local
// artifact store still enforces the 16 MiB content limit and verifies the
// content-addressed metadata before accepting the blob.
type ArtifactTransfer struct {
	Protocol string          `json:"protocol"`
	Artifact domain.Artifact `json:"artifact"`
	Content  []byte          `json:"content"`
}

type ArtifactResponse struct {
	Protocol string `json:"protocol"`
	Content  []byte `json:"content,omitempty"`
}

type ErrorResponse struct {
	Protocol string `json:"protocol"`
	Error    string `json:"error"`
}
