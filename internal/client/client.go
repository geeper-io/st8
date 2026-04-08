package client

import (
	"context"

	"github.com/geeper-io/st8/internal/service"
)

type Client interface {
	Apply(context.Context, service.ApplyInput) (*service.ApplyResult, error)
	Get(context.Context, service.Scope, int64, string) (*service.GetResult, error)
	DiffDocuments(context.Context, service.Scope, int64, string, []service.Document) (*service.DiffResult, error)
	Checkpoint(context.Context, service.Scope, string, string) (*service.CheckpointResult, error)
	Rollback(context.Context, service.Scope, int64, string, string) (*service.ApplyResult, error)
	Log(context.Context, service.Scope, int) ([]service.LogEntry, error)
	CreateBranch(context.Context, service.Scope, string, int64, string) (*service.BranchResult, error)
	ListBranches(context.Context, service.Scope) ([]service.BranchListEntry, error)
	Restore(context.Context, service.RestoreInput) (*service.ApplyResult, error)
}
