package model

import "time"

type Database struct {
	NextRevision int64                      `json:"next_revision"`
	Revisions    map[int64]*Revision        `json:"revisions"`
	Workspaces   map[string]*WorkspaceState `json:"workspaces"`
}

type WorkspaceState struct {
	Environments map[string]*EnvironmentState `json:"environments"`
}

type EnvironmentState struct {
	ActiveBranch string                  `json:"active_branch"`
	Branches     map[string]*BranchState `json:"branches"`
	Checkpoints  map[string]*Checkpoint  `json:"checkpoints"`
}

type BranchState struct {
	Name         string `json:"name"`
	HeadRevision int64  `json:"head_revision"`
	BaseRevision int64  `json:"base_revision"`
}

type Revision struct {
	ID          int64             `json:"id"`
	ParentID    int64             `json:"parent_id"`
	Workspace   string            `json:"workspace"`
	Environment string            `json:"environment"`
	Branch      string            `json:"branch"`
	Message     string            `json:"message"`
	CreatedAt   time.Time         `json:"created_at"`
	Objects     map[string]string `json:"objects"`
	Changes     []Change          `json:"changes"`
}

type Change struct {
	Key    string `json:"key"`
	Type   string `json:"type"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

type Checkpoint struct {
	Name        string    `json:"name"`
	Workspace   string    `json:"workspace"`
	Environment string    `json:"environment"`
	Branch      string    `json:"branch"`
	RevisionID  int64     `json:"revision_id"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description,omitempty"`
}

func NewDatabase() *Database {
	return &Database{
		NextRevision: 1,
		Revisions:    map[int64]*Revision{},
		Workspaces:   map[string]*WorkspaceState{},
	}
}
