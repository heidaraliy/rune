package application

import (
	"context"

	"github.com/heidaraliy/rune/internal/domain"
)

// RuneClient is the shared application boundary for Rune surfaces. The method
// names intentionally retain the transitional v2 verbs so the CLI and TUI can
// move onto one contract without a storage or command cutover in this slice.
// V2Service is the current local implementation; a hosted sync client can
// implement the same contract later.
type RuneClient interface {
	Create(context.Context, domain.Rune) (domain.Rune, error)
	Get(context.Context, string) (domain.Rune, error)
	GetIncludingDeleted(context.Context, string) (domain.Rune, error)
	Update(context.Context, string, domain.RuneUpdate) (domain.Rune, error)
	SetTaskStatus(context.Context, string, domain.Status, int64) (domain.Rune, error)
	Delete(context.Context, string, int64) (domain.Rune, error)
	Restore(context.Context, string, int64) (domain.Rune, error)
	List(context.Context, domain.RuneQuery) ([]domain.Rune, error)
	Links(context.Context, string) ([]domain.Link, error)
	QueueRun(context.Context, string, string, string, domain.PermissionPolicy) (domain.Run, error)
	ExecuteRun(context.Context, string) (domain.Run, error)
	CancelRun(context.Context, string) (domain.Run, error)
	Runs(context.Context, domain.RunListOptions) ([]domain.Run, error)
	RunEvents(context.Context, string) ([]domain.RunEvent, error)
	Artifacts(context.Context, string) ([]domain.Artifact, error)
	AllArtifacts(context.Context) ([]domain.Artifact, error)
	ReadArtifact(context.Context, string) (domain.Artifact, []byte, error)
	SyncStatus(context.Context) (domain.SyncStatus, error)
	Conflicts(context.Context) ([]domain.Conflict, error)
}

var _ RuneClient = V2Service{}
