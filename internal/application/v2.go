package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/runs"
	"github.com/heidaraliy/rune/internal/storage/artifacts"
	"github.com/heidaraliy/rune/internal/storage/markdown"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

type V2Service struct {
	Store         *sqlite.Store
	WorkspaceID   string
	ArtifactStore *artifacts.Store
}

func NewV2Service(store *sqlite.Store, workspaceID string) V2Service {
	if workspaceID == "" {
		workspaceID = "local"
	}
	return V2Service{Store: store, WorkspaceID: workspaceID}
}

func NewV2ExecutionService(store *sqlite.Store, workspaceID string, artifactStore *artifacts.Store) V2Service {
	service := NewV2Service(store, workspaceID)
	service.ArtifactStore = artifactStore
	return service
}

func (s V2Service) Create(ctx context.Context, entity domain.Entity) (domain.Entity, error) {
	entity.WorkspaceID = s.WorkspaceID
	return s.Store.Create(ctx, entity)
}

func (s V2Service) Get(ctx context.Context, prefix string) (domain.Entity, error) {
	return s.Store.Get(ctx, prefix, s.WorkspaceID)
}

func (s V2Service) List(ctx context.Context, options domain.ListOptions) ([]domain.Entity, error) {
	options.WorkspaceID = s.WorkspaceID
	return s.Store.List(ctx, options)
}

func (s V2Service) Update(ctx context.Context, prefix string, update domain.Update) (domain.Entity, error) {
	return s.Store.Update(ctx, prefix, s.WorkspaceID, update)
}

func (s V2Service) Link(ctx context.Context, from, to, kind string) (domain.Link, error) {
	return s.Store.CreateLink(ctx, domain.Link{
		WorkspaceID: s.WorkspaceID,
		FromID:      from,
		ToID:        to,
		Kind:        kind,
	})
}

func (s V2Service) Links(ctx context.Context, entityID string) ([]domain.Link, error) {
	return s.Store.ListLinks(ctx, s.WorkspaceID, entityID)
}

func (s V2Service) Import(ctx context.Context, bundle markdown.Bundle) (sqlite.ImportReport, error) {
	return s.Store.Import(ctx, s.WorkspaceID, bundle)
}

type contextSnapshot struct {
	Schema     string          `json:"schema"`
	Task       domain.Entity   `json:"task"`
	Links      []domain.Link   `json:"links,omitempty"`
	Related    []domain.Entity `json:"related,omitempty"`
	CapturedAt string          `json:"captured_at"`
}

func (s V2Service) QueueRun(ctx context.Context, taskPrefix, providerName, model string, policy domain.PermissionPolicy) (domain.Run, error) {
	if s.ArtifactStore == nil {
		return domain.Run{}, errors.New("v2 execution requires an artifact store")
	}
	task, err := s.Get(ctx, taskPrefix)
	if err != nil {
		return domain.Run{}, err
	}
	if !task.IsTask() {
		return domain.Run{}, errors.New("only tasks can be queued for a run")
	}
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	if providerName == "" {
		providerName = "fake"
	}
	if _, err := runs.ProviderFor(providerName); err != nil {
		return domain.Run{}, err
	}
	if model == "" {
		model = "local"
	}
	if policy == "" {
		policy = domain.PermissionReadOnly
	}
	if _, err := domain.NormalizePermissionPolicy(string(policy)); err != nil {
		return domain.Run{}, err
	}
	links, err := s.Links(ctx, task.ID)
	if err != nil {
		return domain.Run{}, err
	}
	related := make([]domain.Entity, 0, len(links))
	seen := make(map[string]struct{}, len(links))
	for _, link := range links {
		id := link.FromID
		if id == task.ID {
			id = link.ToID
		}
		if id == task.ID {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		relatedEntity, err := s.Store.Get(ctx, id, s.WorkspaceID)
		if err != nil {
			return domain.Run{}, err
		}
		seen[id] = struct{}{}
		related = append(related, relatedEntity)
	}
	snapshot, err := json.Marshal(contextSnapshot{
		Schema:     "rune.context.v1",
		Task:       task,
		Links:      links,
		Related:    related,
		CapturedAt: task.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	})
	if err != nil {
		return domain.Run{}, fmt.Errorf("encode run context: %w", err)
	}
	blob, err := s.ArtifactStore.Put(ctx, snapshot)
	if err != nil {
		return domain.Run{}, err
	}
	runID, err := domain.NewID()
	if err != nil {
		_ = s.ArtifactStore.RemoveIfCreated(blob)
		return domain.Run{}, err
	}
	now := task.UpdatedAt
	if now.IsZero() {
		now = task.CreatedAt
	}
	artifact := domain.Artifact{
		WorkspaceID: s.WorkspaceID,
		RunID:       runID,
		EntityID:    task.ID,
		Kind:        "context",
		Name:        "context.json",
		MediaType:   "application/json",
		SizeBytes:   blob.SizeBytes,
		SHA256:      blob.SHA256,
		StorageKey:  blob.StorageKey,
		Retention:   "permanent",
		SecretState: "clear",
		CreatedAt:   now,
	}
	run, _, err := s.Store.QueueRun(ctx, domain.Run{
		ID:               runID,
		WorkspaceID:      s.WorkspaceID,
		TaskID:           task.ID,
		Provider:         providerName,
		Model:            model,
		Status:           domain.RunStatusQueued,
		PermissionPolicy: policy,
		ContextSnapshot:  string(snapshot),
		CreatedAt:        now,
	}, &artifact)
	if err != nil {
		_ = s.ArtifactStore.RemoveIfCreated(blob)
		return domain.Run{}, err
	}
	return run, nil
}

func (s V2Service) ExecuteRun(ctx context.Context, runPrefix string) (domain.Run, error) {
	if s.ArtifactStore == nil {
		return domain.Run{}, errors.New("v2 execution requires an artifact store")
	}
	queued, err := s.Store.GetRun(ctx, runPrefix, s.WorkspaceID)
	if err != nil {
		return domain.Run{}, err
	}
	task, err := s.Get(ctx, queued.TaskID)
	if err != nil {
		return domain.Run{}, err
	}
	provider, err := runs.ProviderFor(queued.Provider)
	if err != nil {
		return domain.Run{}, err
	}
	running, err := s.Store.SetRunStatus(ctx, queued.ID, s.WorkspaceID, domain.RunStatusRunning, "", "")
	if err != nil {
		return domain.Run{}, err
	}
	result, err := provider.Execute(ctx, runs.Request{Run: running, Task: task, ContextSnapshot: running.ContextSnapshot})
	if err != nil {
		failed, transitionErr := s.Store.SetRunStatus(ctx, running.ID, s.WorkspaceID, domain.RunStatusFailed, "", err.Error())
		if transitionErr != nil {
			return domain.Run{}, fmt.Errorf("run failed: %v; record failure: %w", err, transitionErr)
		}
		return failed, fmt.Errorf("run failed: %w", err)
	}
	for _, input := range result.Artifacts {
		if strings.TrimSpace(input.Kind) == "" || strings.TrimSpace(input.Name) == "" {
			return s.failRun(ctx, running, errors.New("provider returned an artifact without kind or name"))
		}
		blob, err := s.ArtifactStore.Put(ctx, input.Content)
		if err != nil {
			return s.failRun(ctx, running, fmt.Errorf("store provider artifact %s: %w", input.Name, err))
		}
		retention := input.Retention
		if retention == "" {
			retention = "normal"
		}
		secretState := input.SecretState
		if secretState == "" {
			secretState = "clear"
		}
		_, err = s.Store.CreateArtifact(ctx, domain.Artifact{
			WorkspaceID: s.WorkspaceID,
			RunID:       running.ID,
			EntityID:    task.ID,
			Kind:        input.Kind,
			Name:        input.Name,
			MediaType:   input.MediaType,
			SizeBytes:   blob.SizeBytes,
			SHA256:      blob.SHA256,
			StorageKey:  blob.StorageKey,
			Retention:   retention,
			SecretState: secretState,
		})
		if err != nil {
			_ = s.ArtifactStore.RemoveIfCreated(blob)
			return s.failRun(ctx, running, fmt.Errorf("record provider artifact %s: %w", input.Name, err))
		}
	}
	if _, err := s.Store.AppendRunEvent(ctx, running.ID, s.WorkspaceID, "output", result.Summary); err != nil {
		return s.failRun(ctx, running, err)
	}
	if _, err := s.Store.SetRunStatus(ctx, running.ID, s.WorkspaceID, domain.RunStatusReview, result.Summary, ""); err != nil {
		return s.failRun(ctx, running, err)
	}
	completed, err := s.Store.SetRunStatus(ctx, running.ID, s.WorkspaceID, domain.RunStatusCompleted, result.Summary, "")
	if err != nil {
		return domain.Run{}, err
	}
	return completed, nil
}

func (s V2Service) CancelRun(ctx context.Context, runPrefix string) (domain.Run, error) {
	run, err := s.Store.GetRun(ctx, runPrefix, s.WorkspaceID)
	if err != nil {
		return domain.Run{}, err
	}
	return s.Store.SetRunStatus(ctx, run.ID, s.WorkspaceID, domain.RunStatusCanceled, "", "canceled by user")
}

func (s V2Service) failRun(ctx context.Context, running domain.Run, runErr error) (domain.Run, error) {
	failed, transitionErr := s.Store.SetRunStatus(ctx, running.ID, s.WorkspaceID, domain.RunStatusFailed, "", runErr.Error())
	if transitionErr != nil {
		return domain.Run{}, fmt.Errorf("%v; record failure: %w", runErr, transitionErr)
	}
	return failed, runErr
}

func (s V2Service) Runs(ctx context.Context, options domain.RunListOptions) ([]domain.Run, error) {
	options.WorkspaceID = s.WorkspaceID
	return s.Store.ListRuns(ctx, options)
}

func (s V2Service) RunEvents(ctx context.Context, runID string) ([]domain.RunEvent, error) {
	return s.Store.ListRunEvents(ctx, runID, s.WorkspaceID)
}

func (s V2Service) Artifacts(ctx context.Context, runID string) ([]domain.Artifact, error) {
	run, err := s.Store.GetRun(ctx, runID, s.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return s.Store.ListArtifacts(ctx, s.WorkspaceID, run.ID)
}

func (s V2Service) Artifact(ctx context.Context, artifactID string) (domain.Artifact, error) {
	return s.Store.GetArtifact(ctx, artifactID, s.WorkspaceID)
}

func (s V2Service) ReadArtifact(ctx context.Context, artifactID string) (domain.Artifact, []byte, error) {
	if s.ArtifactStore == nil {
		return domain.Artifact{}, nil, errors.New("v2 execution requires an artifact store")
	}
	artifact, err := s.Artifact(ctx, artifactID)
	if err != nil {
		return domain.Artifact{}, nil, err
	}
	if artifact.SecretState == "withheld" {
		return domain.Artifact{}, nil, errors.New("artifact content is withheld by secret policy")
	}
	content, err := s.ArtifactStore.Read(ctx, artifact.StorageKey)
	if err != nil {
		return domain.Artifact{}, nil, err
	}
	return artifact, content, nil
}
