package engine

import (
	"context"
	"time"

	"github.com/geeper-io/st8/internal/model"
)

// CommitRequest describes an atomic apply commit: object changes + revision
// metadata are all written together in one transaction.
type CommitRequest struct {
	Namespace   string
	Branch      string
	ParentRevID int64
	BaseRevision int64 // non-zero only when creating a new branch via Commit
	Message     string
	CreatedAt   time.Time
	Changes     []model.Change
	Puts        map[string]string // key → new value
	Deletes     []string          // keys to remove
}

// Engine is the storage abstraction for st8.
//
// All mutating operations on a namespace are serialised by the caller
// (Service holds a sync.Mutex), so the Engine itself is not required to
// provide optimistic-concurrency retries.
type Engine interface {
	// GetObjects returns all current objects stored under the branch.
	// Returns an empty map if the namespace/branch has never been written to.
	GetObjects(ctx context.Context, ns, branch string) (map[string]string, error)

	// Commit atomically writes object changes, creates a revision record,
	// and advances the branch head. Returns the assigned revision ID.
	Commit(ctx context.Context, req CommitRequest) (int64, error)

	// GetRevision returns the revision record for a specific revision ID.
	GetRevision(ctx context.Context, ns, branch string, revID int64) (*model.Revision, error)

	// DeleteRevision removes a revision record (used by GC).
	DeleteRevision(ctx context.Context, ns, branch string, revID int64) error

	// GetNamespaceMeta returns namespace metadata.
	// Returns a zero-value NamespaceMeta (not an error) if it has not been
	// initialised yet.
	GetNamespaceMeta(ctx context.Context, ns string) (*model.NamespaceMeta, error)

	// SaveNamespaceMeta persists namespace metadata.
	SaveNamespaceMeta(ctx context.Context, ns string, m *model.NamespaceMeta) error

	// GetBranchMeta returns branch metadata, or nil if the branch does not exist.
	GetBranchMeta(ctx context.Context, ns, branch string) (*model.BranchMeta, error)

	// SaveBranchMeta persists branch metadata.
	SaveBranchMeta(ctx context.Context, ns, branch string, m *model.BranchMeta) error

	// GetCheckpoint returns a checkpoint by name, or nil if not found.
	GetCheckpoint(ctx context.Context, ns, name string) (*model.Checkpoint, error)

	// SaveCheckpoint persists a checkpoint record.
	SaveCheckpoint(ctx context.Context, cp *model.Checkpoint) error

	// DeleteCheckpoint removes a checkpoint.
	DeleteCheckpoint(ctx context.Context, ns, name string) error

	// ListCheckpoints returns all checkpoint names for a namespace.
	ListCheckpoints(ctx context.Context, ns string) ([]string, error)

	// ListBranches returns all branch names for a namespace.
	ListBranches(ctx context.Context, ns string) ([]string, error)

	// ListNamespaces returns all namespace names known to the engine.
	ListNamespaces(ctx context.Context) ([]string, error)

	// Ping checks engine availability.
	Ping(context.Context) error

	// Close shuts down the engine.
	Close() error
}
