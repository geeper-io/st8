package client

import (
	"context"

	st8client "github.com/geeper-io/st8/client"
	"github.com/geeper-io/st8/internal/model"
	"github.com/geeper-io/st8/internal/service"
)

// Local is a Client backed by an in-process service.
type Local struct {
	svc *service.Service
}

func NewLocal(svc *service.Service) *Local {
	return &Local{svc: svc}
}

func (c *Local) Apply(ctx context.Context, input st8client.ApplyInput) (*st8client.ApplyResult, error) {
	res, err := c.svc.Apply(ctx, service.ApplyInput{
		Scope:     toServiceScope(input.Scope),
		Documents: toServiceDocs(input.Documents),
		Message:   input.Message,
	})
	if err != nil {
		return nil, err
	}
	return &st8client.ApplyResult{
		Revision: res.Revision,
		Changes:  toClientChanges(res.Changes),
		Noop:     res.Noop,
	}, nil
}

func (c *Local) Get(ctx context.Context, scope st8client.Scope, revision int64, checkpoint string) (*st8client.GetResult, error) {
	res, err := c.svc.Get(ctx, toServiceScope(scope), revision, checkpoint)
	if err != nil {
		return nil, err
	}
	return &st8client.GetResult{Revision: res.Revision, Objects: res.Objects}, nil
}

func (c *Local) DiffDocuments(ctx context.Context, scope st8client.Scope, revision int64, checkpoint string, docs []st8client.Document) (*st8client.DiffResult, error) {
	res, err := c.svc.DiffDocuments(ctx, toServiceScope(scope), revision, checkpoint, toServiceDocs(docs))
	if err != nil {
		return nil, err
	}
	return &st8client.DiffResult{
		FromRevision: res.FromRevision,
		ToRevision:   res.ToRevision,
		Changes:      toClientChanges(res.Changes),
	}, nil
}

func (c *Local) Checkpoint(ctx context.Context, scope st8client.Scope, name, description string) (*st8client.CheckpointResult, error) {
	res, err := c.svc.Checkpoint(ctx, toServiceScope(scope), name, description)
	if err != nil {
		return nil, err
	}
	return &st8client.CheckpointResult{Name: res.Name, RevisionID: res.RevisionID}, nil
}

func (c *Local) Rollback(ctx context.Context, scope st8client.Scope, revision int64, checkpoint, message string) (*st8client.ApplyResult, error) {
	res, err := c.svc.Rollback(ctx, toServiceScope(scope), revision, checkpoint, message)
	if err != nil {
		return nil, err
	}
	return &st8client.ApplyResult{
		Revision: res.Revision,
		Changes:  toClientChanges(res.Changes),
		Noop:     res.Noop,
	}, nil
}

func (c *Local) Log(ctx context.Context, scope st8client.Scope, limit int) ([]st8client.LogEntry, error) {
	entries, err := c.svc.Log(ctx, toServiceScope(scope), limit)
	if err != nil {
		return nil, err
	}
	out := make([]st8client.LogEntry, len(entries))
	for i, e := range entries {
		out[i] = st8client.LogEntry{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Message:   e.Message,
			CreatedAt: e.CreatedAt,
			Branch:    e.Branch,
			Changes:   toClientChanges(e.Changes),
		}
	}
	return out, nil
}

func (c *Local) CreateBranch(ctx context.Context, scope st8client.Scope, name string, revision int64, checkpoint string) (*st8client.BranchResult, error) {
	res, err := c.svc.CreateBranch(ctx, toServiceScope(scope), name, revision, checkpoint)
	if err != nil {
		return nil, err
	}
	return &st8client.BranchResult{Name: res.Name, BaseRevision: res.BaseRevision, HeadRevision: res.HeadRevision}, nil
}

func (c *Local) ListBranches(ctx context.Context, scope st8client.Scope) ([]st8client.BranchListEntry, error) {
	entries, err := c.svc.ListBranches(ctx, toServiceScope(scope))
	if err != nil {
		return nil, err
	}
	out := make([]st8client.BranchListEntry, len(entries))
	for i, e := range entries {
		out[i] = st8client.BranchListEntry{
			Name:         e.Name,
			BaseRevision: e.BaseRevision,
			HeadRevision: e.HeadRevision,
			Current:      e.Current,
		}
	}
	return out, nil
}

func (c *Local) Restore(ctx context.Context, input st8client.RestoreInput) (*st8client.ApplyResult, error) {
	res, err := c.svc.Restore(ctx, service.RestoreInput{
		Scope:          toServiceScope(input.Scope),
		FromRevision:   input.FromRevision,
		FromCheckpoint: input.FromCheckpoint,
		FromBranch:     input.FromBranch,
		Message:        input.Message,
	})
	if err != nil {
		return nil, err
	}
	return &st8client.ApplyResult{
		Revision: res.Revision,
		Changes:  toClientChanges(res.Changes),
		Noop:     res.Noop,
	}, nil
}

func toServiceScope(s st8client.Scope) service.Scope {
	return service.Scope{Workspace: s.Workspace, Environment: s.Environment, Branch: s.Branch}
}

func toServiceDocs(docs []st8client.Document) []service.Document {
	out := make([]service.Document, len(docs))
	for i, d := range docs {
		out[i] = service.Document{Key: d.Key, Content: d.Content}
	}
	return out
}

func toClientChanges(changes []model.Change) []st8client.Change {
	out := make([]st8client.Change, len(changes))
	for i, c := range changes {
		out[i] = st8client.Change{Key: c.Key, Type: c.Type, Before: c.Before, After: c.After}
	}
	return out
}
