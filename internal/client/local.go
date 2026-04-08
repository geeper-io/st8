package client

import (
	"context"

	"github.com/geeper-io/st8/internal/service"
)

type Local struct {
	service *service.Service
}

func NewLocal(svc *service.Service) *Local {
	return &Local{service: svc}
}

func (c *Local) Apply(ctx context.Context, input service.ApplyInput) (*service.ApplyResult, error) {
	return c.service.Apply(ctx, input)
}

func (c *Local) Get(ctx context.Context, scope service.Scope, revision int64, checkpoint string) (*service.GetResult, error) {
	return c.service.Get(ctx, scope, revision, checkpoint)
}

func (c *Local) DiffDocuments(ctx context.Context, scope service.Scope, revision int64, checkpoint string, docs []service.Document) (*service.DiffResult, error) {
	return c.service.DiffDocuments(ctx, scope, revision, checkpoint, docs)
}

func (c *Local) Checkpoint(ctx context.Context, scope service.Scope, name, description string) (*service.CheckpointResult, error) {
	return c.service.Checkpoint(ctx, scope, name, description)
}

func (c *Local) Rollback(ctx context.Context, scope service.Scope, revision int64, checkpoint, message string) (*service.ApplyResult, error) {
	return c.service.Rollback(ctx, scope, revision, checkpoint, message)
}

func (c *Local) Log(ctx context.Context, scope service.Scope, limit int) ([]service.LogEntry, error) {
	return c.service.Log(ctx, scope, limit)
}

func (c *Local) CreateBranch(ctx context.Context, scope service.Scope, name string, revision int64, checkpoint string) (*service.BranchResult, error) {
	return c.service.CreateBranch(ctx, scope, name, revision, checkpoint)
}

func (c *Local) ListBranches(ctx context.Context, scope service.Scope) ([]service.BranchListEntry, error) {
	return c.service.ListBranches(ctx, scope)
}

func (c *Local) Restore(ctx context.Context, input service.RestoreInput) (*service.ApplyResult, error) {
	return c.service.Restore(ctx, input)
}
