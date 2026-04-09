package model

import "time"

// NamespaceMeta holds namespace-level metadata including the revision counter.
type NamespaceMeta struct {
	ActiveBranch string `json:"active_branch"`
	NextRevision int64  `json:"next_revision"` // next revision ID to assign
}

// BranchMeta holds branch-level metadata.
type BranchMeta struct {
	Name         string `json:"name"`
	HeadRevision int64  `json:"head_revision"`
	BaseRevision int64  `json:"base_revision"`
}

// Revision records a delta commit on a branch.
// It stores only the list of changes, not the full object snapshot.
type Revision struct {
	ID        int64     `json:"id"`
	ParentID  int64     `json:"parent_id"`
	Namespace string    `json:"namespace"`
	Branch    string    `json:"branch"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Changes   []Change  `json:"changes"`
}

// Change describes one key modification within a revision.
type Change struct {
	Key    string `json:"key"`
	Type   string `json:"type"` // "create", "update", "delete"
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// Checkpoint is a named snapshot that stores the full object set at a revision.
type Checkpoint struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Branch      string            `json:"branch"`
	RevisionID  int64             `json:"revision_id"`
	CreatedAt   time.Time         `json:"created_at"`
	Description string            `json:"description,omitempty"`
	Objects     map[string]string `json:"objects"` // full snapshot at RevisionID
}
