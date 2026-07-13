package application

import (
	"context"

	"github.com/heidaraliy/rune/internal/domain"
	"github.com/heidaraliy/rune/internal/storage/markdown"
	"github.com/heidaraliy/rune/internal/storage/sqlite"
)

type V2Service struct {
	Store       *sqlite.Store
	WorkspaceID string
}

func NewV2Service(store *sqlite.Store, workspaceID string) V2Service {
	if workspaceID == "" {
		workspaceID = "local"
	}
	return V2Service{Store: store, WorkspaceID: workspaceID}
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
