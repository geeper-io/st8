package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/geeper-io/st8/internal/engine"
	"github.com/geeper-io/st8/internal/model"
)

type Scope struct {
	Namespace string `json:"namespace"`
	Branch    string `json:"branch"`
}

type Service struct {
	engine engine.Engine
	mu     sync.Mutex
}

func New(eng engine.Engine) *Service {
	return &Service{engine: eng}
}

func (s *Service) Ready(ctx context.Context) error {
	return s.engine.Ping(ctx)
}

type ApplyInput struct {
	Scope     Scope
	Documents []Document
	Message   string
}

type ApplyResult struct {
	Revision int64          `json:"revision"`
	Changes  []model.Change `json:"changes"`
	Noop     bool           `json:"noop"`
}

type GetResult struct {
	Revision int64             `json:"revision"`
	Objects  map[string]string `json:"objects"`
}

type CheckpointResult struct {
	Name       string `json:"name"`
	RevisionID int64  `json:"revision_id"`
}

type BranchResult struct {
	Name         string `json:"name"`
	BaseRevision int64  `json:"base_revision"`
	HeadRevision int64  `json:"head_revision"`
}

type DiffResult struct {
	FromRevision int64          `json:"from_revision"`
	ToRevision   int64          `json:"to_revision"`
	Changes      []model.Change `json:"changes"`
}

type LogEntry struct {
	ID        int64          `json:"id"`
	ParentID  int64          `json:"parent_id"`
	Message   string         `json:"message"`
	CreatedAt time.Time      `json:"created_at"`
	Branch    string         `json:"branch"`
	Changes   []model.Change `json:"changes"`
}

type RestoreInput struct {
	Scope          Scope
	FromRevision   int64
	FromCheckpoint string
	FromBranch     string
	Message        string
}

type BranchListEntry struct {
	Name         string `json:"name"`
	BaseRevision int64  `json:"base_revision"`
	HeadRevision int64  `json:"head_revision"`
	Current      bool   `json:"current"`
}

type Document struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

type GCResult struct {
	Pruned int `json:"pruned"`
}

func (s *Service) Apply(ctx context.Context, input ApplyInput) (*ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyDocuments(ctx, input.Scope, input.Documents, input.Message)
}

func (s *Service) GC(ctx context.Context, keep int) (*GCResult, error) {
	if keep <= 0 {
		keep = 10
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	db, err := s.engine.Load(ctx)
	if err != nil {
		return nil, err
	}

	protected := map[int64]struct{}{}
	for _, ns := range db.Namespaces {
		for _, cp := range ns.Checkpoints {
			protected[cp.RevisionID] = struct{}{}
		}
		for _, branch := range ns.Branches {
			if branch.BaseRevision > 0 {
				protected[branch.BaseRevision] = struct{}{}
			}
			id := branch.HeadRevision
			for i := 0; i < keep && id > 0; i++ {
				protected[id] = struct{}{}
				rev := db.Revisions[id]
				if rev == nil {
					break
				}
				id = rev.ParentID
			}
		}
	}

	pruned := 0
	for id := range db.Revisions {
		if _, ok := protected[id]; !ok {
			delete(db.Revisions, id)
			pruned++
		}
	}

	if pruned == 0 {
		return &GCResult{}, nil
	}
	if err := s.engine.Save(ctx, db); err != nil {
		return nil, err
	}
	return &GCResult{Pruned: pruned}, nil
}

func (s *Service) Get(ctx context.Context, scope Scope, revisionID int64, checkpointName string) (*GetResult, error) {
	db, err := s.engine.Load(ctx)
	if err != nil {
		return nil, err
	}
	scope = normalizeScope(scope)
	rev, err := resolveRevision(db, scope, revisionID, checkpointName, "")
	if err != nil {
		return nil, err
	}
	return &GetResult{Revision: rev.ID, Objects: cloneObjects(rev.Objects)}, nil
}

func (s *Service) Diff(ctx context.Context, scope Scope, revisionID int64, checkpointName string, files []string) (*DiffResult, error) {
	return s.DiffDocuments(ctx, scope, revisionID, checkpointName, nil)
}

func (s *Service) DiffDocuments(ctx context.Context, scope Scope, revisionID int64, checkpointName string, documents []Document) (*DiffResult, error) {
	db, err := s.engine.Load(ctx)
	if err != nil {
		return nil, err
	}
	scope = normalizeScope(scope)
	current, err := resolveRevision(db, scope, 0, "", "")
	if err != nil {
		return nil, err
	}
	if len(documents) > 0 {
		next := cloneObjects(current.Objects)
		for _, doc := range documents {
			next[doc.Key] = doc.Content
		}
		return &DiffResult{
			FromRevision: current.ID,
			ToRevision:   current.ID,
			Changes:      diffObjects(current.Objects, next),
		}, nil
	}
	target, err := resolveRevision(db, scope, revisionID, checkpointName, "")
	if err != nil {
		return nil, err
	}
	return &DiffResult{
		FromRevision: current.ID,
		ToRevision:   target.ID,
		Changes:      diffObjects(current.Objects, target.Objects),
	}, nil
}

func (s *Service) Checkpoint(ctx context.Context, scope Scope, name, description string) (*CheckpointResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("checkpoint name is required")
	}
	db, err := s.engine.Load(ctx)
	if err != nil {
		return nil, err
	}
	scope = normalizeScope(scope)
	ns := ensureNamespace(db, scope)
	branch := ensureBranch(ns, scope.Branch)
	ns.Checkpoints[name] = &model.Checkpoint{
		Name:        name,
		Namespace:   scope.Namespace,
		Branch:      scope.Branch,
		RevisionID:  branch.HeadRevision,
		CreatedAt:   time.Now().UTC(),
		Description: description,
	}
	if err := s.engine.Save(ctx, db); err != nil {
		return nil, err
	}
	return &CheckpointResult{Name: name, RevisionID: branch.HeadRevision}, nil
}

func (s *Service) Rollback(ctx context.Context, scope Scope, revisionID int64, checkpointName, message string) (*ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restore(ctx, RestoreInput{
		Scope:          scope,
		FromRevision:   revisionID,
		FromCheckpoint: checkpointName,
		Message:        chooseMessage(message, "rollback"),
	})
}

func (s *Service) Restore(ctx context.Context, input RestoreInput) (*ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restore(ctx, input)
}

func (s *Service) restore(ctx context.Context, input RestoreInput) (*ApplyResult, error) {
	db, err := s.engine.Load(ctx)
	if err != nil {
		return nil, err
	}
	scope := normalizeScope(input.Scope)
	ns := ensureNamespace(db, scope)
	branch := ensureBranch(ns, scope.Branch)
	current := snapshotForRevision(db, branch.HeadRevision)
	target, err := resolveRevision(db, scope, input.FromRevision, input.FromCheckpoint, input.FromBranch)
	if err != nil {
		return nil, err
	}
	changes := diffObjects(current, target.Objects)
	if len(changes) == 0 {
		return &ApplyResult{Revision: branch.HeadRevision, Noop: true}, nil
	}
	revision := appendRevision(db, scope, branch.HeadRevision, chooseMessage(input.Message, "restore"), cloneObjects(target.Objects), changes)
	branch.HeadRevision = revision.ID
	if err := s.engine.Save(ctx, db); err != nil {
		return nil, err
	}
	return &ApplyResult{Revision: revision.ID, Changes: changes}, nil
}

func (s *Service) Log(ctx context.Context, scope Scope, limit int) ([]LogEntry, error) {
	db, err := s.engine.Load(ctx)
	if err != nil {
		return nil, err
	}
	scope = normalizeScope(scope)
	ns := ensureNamespace(db, scope)
	branch := ensureBranch(ns, scope.Branch)
	if limit <= 0 {
		limit = 20
	}
	var out []LogEntry
	for revID := branch.HeadRevision; revID > 0 && len(out) < limit; {
		rev := db.Revisions[revID]
		if rev == nil {
			break
		}
		out = append(out, LogEntry{
			ID:        rev.ID,
			ParentID:  rev.ParentID,
			Message:   rev.Message,
			CreatedAt: rev.CreatedAt,
			Branch:    rev.Branch,
			Changes:   rev.Changes,
		})
		revID = rev.ParentID
	}
	return out, nil
}

func (s *Service) CreateBranch(ctx context.Context, scope Scope, name string, fromRevision int64, fromCheckpoint string) (*BranchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("branch name is required")
	}
	db, err := s.engine.Load(ctx)
	if err != nil {
		return nil, err
	}
	scope = normalizeScope(scope)
	ns := ensureNamespace(db, scope)
	if _, exists := ns.Branches[name]; exists {
		return nil, fmt.Errorf("branch %q already exists", name)
	}
	base, err := resolveRevision(db, scope, fromRevision, fromCheckpoint, "")
	if err != nil {
		return nil, err
	}
	ns.Branches[name] = &model.BranchState{
		Name:         name,
		BaseRevision: base.ID,
		HeadRevision: base.ID,
	}
	if err := s.engine.Save(ctx, db); err != nil {
		return nil, err
	}
	return &BranchResult{Name: name, BaseRevision: base.ID, HeadRevision: base.ID}, nil
}

func (s *Service) ListBranches(ctx context.Context, scope Scope) ([]BranchListEntry, error) {
	db, err := s.engine.Load(ctx)
	if err != nil {
		return nil, err
	}
	scope = normalizeScope(scope)
	ns := ensureNamespace(db, scope)
	var names []string
	for name := range ns.Branches {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]BranchListEntry, 0, len(names))
	for _, name := range names {
		branch := ns.Branches[name]
		out = append(out, BranchListEntry{
			Name:         branch.Name,
			BaseRevision: branch.BaseRevision,
			HeadRevision: branch.HeadRevision,
			Current:      name == scope.Branch,
		})
	}
	return out, nil
}

func normalizeScope(scope Scope) Scope {
	if strings.TrimSpace(scope.Namespace) == "" {
		scope.Namespace = "default"
	}
	if strings.TrimSpace(scope.Branch) == "" {
		scope.Branch = "main"
	}
	return scope
}

func ensureNamespace(db *model.Database, scope Scope) *model.NamespaceState {
	ns := db.Namespaces[scope.Namespace]
	if ns == nil {
		ns = &model.NamespaceState{
			ActiveBranch: "main",
			Branches:     map[string]*model.BranchState{},
			Checkpoints:  map[string]*model.Checkpoint{},
		}
		db.Namespaces[scope.Namespace] = ns
	}
	if ns.Branches == nil {
		ns.Branches = map[string]*model.BranchState{}
	}
	if ns.Checkpoints == nil {
		ns.Checkpoints = map[string]*model.Checkpoint{}
	}
	return ns
}

func ensureBranch(ns *model.NamespaceState, name string) *model.BranchState {
	if strings.TrimSpace(name) == "" {
		name = ns.ActiveBranch
	}
	if name == "" {
		name = "main"
	}
	branch := ns.Branches[name]
	if branch == nil {
		branch = &model.BranchState{Name: name}
		ns.Branches[name] = branch
	}
	if ns.ActiveBranch == "" {
		ns.ActiveBranch = name
	}
	return branch
}

func appendRevision(db *model.Database, scope Scope, parentID int64, message string, objects map[string]string, changes []model.Change) *model.Revision {
	id := db.NextRevision
	db.NextRevision++
	rev := &model.Revision{
		ID:        id,
		ParentID:  parentID,
		Namespace: scope.Namespace,
		Branch:    scope.Branch,
		Message:   message,
		CreatedAt: time.Now().UTC(),
		Objects:   cloneObjects(objects),
		Changes:   changes,
	}
	db.Revisions[id] = rev
	return rev
}

func snapshotForRevision(db *model.Database, revisionID int64) map[string]string {
	if revisionID == 0 {
		return map[string]string{}
	}
	if rev := db.Revisions[revisionID]; rev != nil {
		return cloneObjects(rev.Objects)
	}
	return map[string]string{}
}

func resolveRevision(db *model.Database, scope Scope, revisionID int64, checkpointName, fromBranch string) (*model.Revision, error) {
	ns := ensureNamespace(db, scope)
	switch {
	case revisionID > 0:
		rev := db.Revisions[revisionID]
		if rev == nil {
			return nil, fmt.Errorf("revision %d not found", revisionID)
		}
		return rev, nil
	case checkpointName != "":
		cp := ns.Checkpoints[checkpointName]
		if cp == nil {
			return nil, fmt.Errorf("checkpoint %q not found", checkpointName)
		}
		rev := db.Revisions[cp.RevisionID]
		if rev == nil {
			return nil, fmt.Errorf("checkpoint %q points to missing revision %d", checkpointName, cp.RevisionID)
		}
		return rev, nil
	case fromBranch != "":
		branch := ns.Branches[fromBranch]
		if branch == nil {
			return nil, fmt.Errorf("branch %q not found", fromBranch)
		}
		if branch.HeadRevision == 0 {
			return &model.Revision{Objects: map[string]string{}}, nil
		}
		rev := db.Revisions[branch.HeadRevision]
		if rev == nil {
			return nil, fmt.Errorf("branch %q points to missing revision %d", fromBranch, branch.HeadRevision)
		}
		return rev, nil
	default:
		branch := ensureBranch(ns, scope.Branch)
		if branch.HeadRevision == 0 {
			return &model.Revision{Objects: map[string]string{}}, nil
		}
		rev := db.Revisions[branch.HeadRevision]
		if rev == nil {
			return nil, fmt.Errorf("branch %q points to missing revision %d", scope.Branch, branch.HeadRevision)
		}
		return rev, nil
	}
}

func cloneObjects(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func diffObjects(before, after map[string]string) []model.Change {
	keys := map[string]struct{}{}
	for key := range before {
		keys[key] = struct{}{}
	}
	for key := range after {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	slices.Sort(ordered)
	changes := make([]model.Change, 0, len(ordered))
	for _, key := range ordered {
		prev, prevOK := before[key]
		next, nextOK := after[key]
		switch {
		case !prevOK && nextOK:
			changes = append(changes, model.Change{Key: key, Type: "create", After: next})
		case prevOK && !nextOK:
			changes = append(changes, model.Change{Key: key, Type: "delete", Before: prev})
		case prev != next:
			changes = append(changes, model.Change{Key: key, Type: "update", Before: prev, After: next})
		}
	}
	return changes
}

func chooseMessage(message, fallback string) string {
	if strings.TrimSpace(message) == "" {
		return fallback
	}
	return strings.TrimSpace(message)
}

func (s *Service) applyDocuments(ctx context.Context, scope Scope, documents []Document, message string) (*ApplyResult, error) {
	if len(documents) == 0 {
		return nil, errors.New("apply requires at least one file")
	}
	db, err := s.engine.Load(ctx)
	if err != nil {
		return nil, err
	}
	scope = normalizeScope(scope)
	ns := ensureNamespace(db, scope)
	branch := ensureBranch(ns, scope.Branch)
	current := snapshotForRevision(db, branch.HeadRevision)
	next := cloneObjects(current)

	for _, doc := range documents {
		next[doc.Key] = doc.Content
	}

	changes := diffObjects(current, next)
	if len(changes) == 0 {
		return &ApplyResult{Revision: branch.HeadRevision, Noop: true}, nil
	}

	revision := appendRevision(db, scope, branch.HeadRevision, chooseMessage(message, "apply"), next, changes)
	branch.HeadRevision = revision.ID
	if ns.ActiveBranch == "" {
		ns.ActiveBranch = branch.Name
	}
	if err := s.engine.Save(ctx, db); err != nil {
		return nil, err
	}
	return &ApplyResult{Revision: revision.ID, Changes: changes}, nil
}
