package api

import "github.com/geeper-io/st8/internal/service"

type Scope = service.Scope
type Document = service.Document

type ApplyRequest struct {
	Scope     Scope      `json:"scope"`
	Documents []Document `json:"documents"`
	Message   string     `json:"message"`
}

type GetResponse = service.GetResult

type DiffRequest struct {
	Scope          Scope      `json:"scope"`
	RevisionID     int64      `json:"revision_id"`
	CheckpointName string     `json:"checkpoint_name"`
	Documents      []Document `json:"documents"`
}

type CheckpointRequest struct {
	Scope       Scope  `json:"scope"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type RollbackRequest struct {
	Scope          Scope  `json:"scope"`
	RevisionID     int64  `json:"revision_id"`
	CheckpointName string `json:"checkpoint_name"`
	Message        string `json:"message"`
}

type LogResponse struct {
	Entries []service.LogEntry `json:"entries"`
}

type BranchCreateRequest struct {
	Scope          Scope  `json:"scope"`
	Name           string `json:"name"`
	FromRevision   int64  `json:"from_revision"`
	FromCheckpoint string `json:"from_checkpoint"`
}

type BranchListResponse struct {
	Branches []service.BranchListEntry `json:"branches"`
}

type RestoreRequest struct {
	Scope          Scope  `json:"scope"`
	FromRevision   int64  `json:"from_revision"`
	FromCheckpoint string `json:"from_checkpoint"`
	FromBranch     string `json:"from_branch"`
	Message        string `json:"message"`
}
