// Package client provides an st8d client and shared types for interacting
// with an st8d server over HTTP or directly via the embedded service.
package client

import (
	"context"
	"time"
)

// Scope identifies which namespace and branch to operate on.
type Scope struct {
	Namespace string `json:"namespace"`
	Branch    string `json:"branch"`
}

// Document is a key/content pair applied to state.
type Document struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

// Change describes a single key modification in a revision.
type Change struct {
	Key    string `json:"key"`
	Type   string `json:"type"` // create, update, delete
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// ApplyInput is the input to Apply.
type ApplyInput struct {
	Scope     Scope
	Documents []Document
	Message   string
}

// RestoreInput is the input to Restore.
type RestoreInput struct {
	Scope          Scope
	FromRevision   int64
	FromCheckpoint string
	FromBranch     string
	Message        string
}

// ApplyResult is returned by Apply, Rollback, and Restore.
type ApplyResult struct {
	Revision int64    `json:"revision"`
	Changes  []Change `json:"changes"`
	Noop     bool     `json:"noop"`
}

// GetResult is returned by Get.
type GetResult struct {
	Revision int64             `json:"revision"`
	Objects  map[string]string `json:"objects"`
}

// DiffResult is returned by DiffDocuments.
type DiffResult struct {
	FromRevision int64    `json:"from_revision"`
	ToRevision   int64    `json:"to_revision"`
	Changes      []Change `json:"changes"`
}

// CheckpointResult is returned by Checkpoint.
type CheckpointResult struct {
	Name       string `json:"name"`
	RevisionID int64  `json:"revision_id"`
}

// BranchResult is returned by CreateBranch.
type BranchResult struct {
	Name         string `json:"name"`
	BaseRevision int64  `json:"base_revision"`
	HeadRevision int64  `json:"head_revision"`
}

// LogEntry is a single revision in the log.
type LogEntry struct {
	ID        int64     `json:"id"`
	ParentID  int64     `json:"parent_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Branch    string    `json:"branch"`
	Changes   []Change  `json:"changes"`
}

// BranchListEntry describes a branch returned by ListBranches.
type BranchListEntry struct {
	Name         string `json:"name"`
	BaseRevision int64  `json:"base_revision"`
	HeadRevision int64  `json:"head_revision"`
	Current      bool   `json:"current"`
}

// GCResult is returned by GC.
type GCResult struct {
	Pruned int `json:"pruned"`
}

type Client interface {
	Apply(ctx context.Context, input ApplyInput) (*ApplyResult, error)
	Get(ctx context.Context, scope Scope, revision int64, checkpoint string) (*GetResult, error)
	DiffDocuments(ctx context.Context, scope Scope, revision int64, checkpoint string, docs []Document) (*DiffResult, error)
	Checkpoint(ctx context.Context, scope Scope, name, description string) (*CheckpointResult, error)
	DeleteCheckpoint(ctx context.Context, namespace, name string) error
	Rollback(ctx context.Context, scope Scope, revision int64, checkpoint, message string) (*ApplyResult, error)
	Log(ctx context.Context, scope Scope, limit int) ([]LogEntry, error)
	CreateBranch(ctx context.Context, scope Scope, name string, revision int64, checkpoint string) (*BranchResult, error)
	ListBranches(ctx context.Context, scope Scope) ([]BranchListEntry, error)
	Restore(ctx context.Context, input RestoreInput) (*ApplyResult, error)
	GC(ctx context.Context, keep int) (*GCResult, error)
}
