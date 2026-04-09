package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geeper-io/st8/internal/engine"
	"github.com/geeper-io/st8/internal/model"
)

// store is the JSON-serialised state for the local engine.
type store struct {
	Namespaces  map[string]*model.NamespaceMeta      `json:"namespaces"`
	Branches    map[string]*model.BranchMeta         `json:"branches"`    // key: ns+"/"+branch
	Objects     map[string]map[string]string         `json:"objects"`     // key: ns+"/"+branch → map[key]value
	Revisions   map[string]*model.Revision           `json:"revisions"`   // key: ns+"/"+branch+"/"+revID
	Checkpoints map[string]*model.Checkpoint         `json:"checkpoints"` // key: ns+"/"+name
}

func newStore() *store {
	return &store{
		Namespaces:  map[string]*model.NamespaceMeta{},
		Branches:    map[string]*model.BranchMeta{},
		Objects:     map[string]map[string]string{},
		Revisions:   map[string]*model.Revision{},
		Checkpoints: map[string]*model.Checkpoint{},
	}
}

// Engine implements engine.Engine using a local JSON file (dev / testing only).
type Engine struct {
	path string
}

func New(stateDir string) *Engine {
	return &Engine{path: filepath.Join(stateDir, "store.json")}
}

func (e *Engine) Close() error  { return nil }
func (e *Engine) Ping(_ context.Context) error { return nil }

// load reads the store from disk. Returns an empty store if the file does not exist.
func (e *Engine) load() (*store, error) {
	data, err := os.ReadFile(e.path)
	if errors.Is(err, os.ErrNotExist) {
		return newStore(), nil
	}
	if err != nil {
		return nil, err
	}
	s := newStore()
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (e *Engine) save(s *store) error {
	if err := os.MkdirAll(filepath.Dir(e.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(e.path, append(data, '\n'), 0o644)
}

func branchKey(ns, branch string) string       { return ns + "/" + branch }
func revKey(ns, branch string, id int64) string { return fmt.Sprintf("%s/%s/%d", ns, branch, id) }
func cpKey(ns, name string) string              { return ns + "/" + name }

// GetObjects returns all current objects for the namespace/branch.
func (e *Engine) GetObjects(_ context.Context, ns, branch string) (map[string]string, error) {
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	objs := s.Objects[branchKey(ns, branch)]
	out := make(map[string]string, len(objs))
	for k, v := range objs {
		out[k] = v
	}
	return out, nil
}

// Commit applies the commit atomically (under the caller's mutex).
func (e *Engine) Commit(_ context.Context, req engine.CommitRequest) (int64, error) {
	s, err := e.load()
	if err != nil {
		return 0, err
	}

	// Namespace meta
	meta := s.Namespaces[req.Namespace]
	if meta == nil {
		meta = &model.NamespaceMeta{NextRevision: 1}
		s.Namespaces[req.Namespace] = meta
	}
	if meta.ActiveBranch == "" {
		meta.ActiveBranch = req.Branch
	}
	revID := meta.NextRevision
	meta.NextRevision++

	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	// Revision record (delta only)
	s.Revisions[revKey(req.Namespace, req.Branch, revID)] = &model.Revision{
		ID:        revID,
		ParentID:  req.ParentRevID,
		Namespace: req.Namespace,
		Branch:    req.Branch,
		Message:   req.Message,
		CreatedAt: createdAt,
		Changes:   req.Changes,
	}

	// Branch meta
	bk := branchKey(req.Namespace, req.Branch)
	bm := s.Branches[bk]
	if bm == nil {
		bm = &model.BranchMeta{Name: req.Branch, BaseRevision: req.BaseRevision}
		s.Branches[bk] = bm
	}
	bm.HeadRevision = revID

	// Object keys
	objs := s.Objects[bk]
	if objs == nil {
		objs = map[string]string{}
		s.Objects[bk] = objs
	}
	for k, v := range req.Puts {
		objs[k] = v
	}
	for _, k := range req.Deletes {
		delete(objs, k)
	}

	return revID, e.save(s)
}

// GetRevision returns a revision record.
func (e *Engine) GetRevision(_ context.Context, ns, branch string, revID int64) (*model.Revision, error) {
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	rev := s.Revisions[revKey(ns, branch, revID)]
	if rev == nil {
		return nil, nil
	}
	return rev, nil
}

// DeleteRevision removes a revision record.
func (e *Engine) DeleteRevision(_ context.Context, ns, branch string, revID int64) error {
	s, err := e.load()
	if err != nil {
		return err
	}
	delete(s.Revisions, revKey(ns, branch, revID))
	return e.save(s)
}

// GetNamespaceMeta returns namespace metadata.
func (e *Engine) GetNamespaceMeta(_ context.Context, ns string) (*model.NamespaceMeta, error) {
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	m := s.Namespaces[ns]
	if m == nil {
		return &model.NamespaceMeta{NextRevision: 1}, nil
	}
	cp := *m
	return &cp, nil
}

// SaveNamespaceMeta persists namespace metadata.
func (e *Engine) SaveNamespaceMeta(_ context.Context, ns string, m *model.NamespaceMeta) error {
	s, err := e.load()
	if err != nil {
		return err
	}
	cp := *m
	s.Namespaces[ns] = &cp
	return e.save(s)
}

// GetBranchMeta returns branch metadata or nil if absent.
func (e *Engine) GetBranchMeta(_ context.Context, ns, branch string) (*model.BranchMeta, error) {
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	m := s.Branches[branchKey(ns, branch)]
	if m == nil {
		return nil, nil
	}
	cp := *m
	return &cp, nil
}

// SaveBranchMeta persists branch metadata.
func (e *Engine) SaveBranchMeta(_ context.Context, ns, branch string, m *model.BranchMeta) error {
	s, err := e.load()
	if err != nil {
		return err
	}
	cp := *m
	s.Branches[branchKey(ns, branch)] = &cp
	return e.save(s)
}

// GetCheckpoint returns a checkpoint or nil if absent.
func (e *Engine) GetCheckpoint(_ context.Context, ns, name string) (*model.Checkpoint, error) {
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	cp := s.Checkpoints[cpKey(ns, name)]
	if cp == nil {
		return nil, nil
	}
	out := *cp
	return &out, nil
}

// SaveCheckpoint persists a checkpoint record.
func (e *Engine) SaveCheckpoint(_ context.Context, cp *model.Checkpoint) error {
	s, err := e.load()
	if err != nil {
		return err
	}
	out := *cp
	s.Checkpoints[cpKey(cp.Namespace, cp.Name)] = &out
	return e.save(s)
}

// DeleteCheckpoint removes a checkpoint.
func (e *Engine) DeleteCheckpoint(_ context.Context, ns, name string) error {
	s, err := e.load()
	if err != nil {
		return err
	}
	delete(s.Checkpoints, cpKey(ns, name))
	return e.save(s)
}

// ListCheckpoints returns all checkpoint names for a namespace.
func (e *Engine) ListCheckpoints(_ context.Context, ns string) ([]string, error) {
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	prefix := ns + "/"
	var names []string
	for k := range s.Checkpoints {
		if strings.HasPrefix(k, prefix) {
			names = append(names, strings.TrimPrefix(k, prefix))
		}
	}
	return names, nil
}

// ListBranches returns all branch names for a namespace.
func (e *Engine) ListBranches(_ context.Context, ns string) ([]string, error) {
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	prefix := ns + "/"
	var names []string
	for k := range s.Branches {
		if strings.HasPrefix(k, prefix) {
			names = append(names, strings.TrimPrefix(k, prefix))
		}
	}
	return names, nil
}

// ListNamespaces returns all namespace names.
func (e *Engine) ListNamespaces(_ context.Context) ([]string, error) {
	s, err := e.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(s.Namespaces))
	for ns := range s.Namespaces {
		names = append(names, ns)
	}
	return names, nil
}
