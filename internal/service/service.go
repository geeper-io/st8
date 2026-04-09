package service

import (
"context"
"errors"
"fmt"
"slices"
"strings"
"sync"
"time"

"github.com/geeper-io/st8/internal/auth"
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

// ─── Auth helpers ─────────────────────────────────────────────────────────────

// checkScope returns an error if the caller's principal does not allow the
// given namespace and branch. A missing principal means auth is disabled.
func checkScope(ctx context.Context, ns, branch string) error {
p, ok := auth.PrincipalFromContext(ctx)
if !ok {
return nil
}
if !p.AllowsNamespace(ns) {
return fmt.Errorf("forbidden: namespace %q not allowed for token %q", ns, p.Name)
}
if branch != "" && !p.AllowsBranch(branch) {
return fmt.Errorf("forbidden: branch %q not allowed for token %q", branch, p.Name)
}
return nil
}

// enforceKeyPrefix returns an error if any document key falls outside the
// principal's allowed key prefix.
func enforceKeyPrefix(ctx context.Context, documents []Document) error {
p, ok := auth.PrincipalFromContext(ctx)
if !ok || p.Allow.KeyPrefix == "" {
return nil
}
for _, doc := range documents {
if !p.AllowsKey(doc.Key) {
return fmt.Errorf("forbidden: key %q is outside allowed prefix %q for token %q", doc.Key, p.Allow.KeyPrefix, p.Name)
}
}
return nil
}

// filterKeysByPrefix removes keys from objs that fall outside the principal's
// allowed key prefix. No-op when auth is disabled or key prefix is empty.
func filterKeysByPrefix(ctx context.Context, objs map[string]string) {
p, ok := auth.PrincipalFromContext(ctx)
if !ok || p.Allow.KeyPrefix == "" {
return
}
for k := range objs {
if !p.AllowsKey(k) {
delete(objs, k)
}
}
}

// ─── Apply ────────────────────────────────────────────────────────────────────

func (s *Service) Apply(ctx context.Context, input ApplyInput) (*ApplyResult, error) {
if err := checkScope(ctx, normalizeScope(input.Scope).Namespace, normalizeScope(input.Scope).Branch); err != nil {
return nil, err
}
if err := enforceKeyPrefix(ctx, input.Documents); err != nil {
return nil, err
}
s.mu.Lock()
defer s.mu.Unlock()
return s.applyDocuments(ctx, input.Scope, input.Documents, input.Message)
}

func (s *Service) applyDocuments(ctx context.Context, scope Scope, documents []Document, message string) (*ApplyResult, error) {
if len(documents) == 0 {
return nil, errors.New("apply requires at least one file")
}
scope = normalizeScope(scope)

bm, err := s.ensureBranch(ctx, scope)
if err != nil {
return nil, err
}

current, err := s.engine.GetObjects(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}

next := cloneObjects(current)
for _, doc := range documents {
next[doc.Key] = doc.Content
}

changes := diffObjects(current, next)
if len(changes) == 0 {
return &ApplyResult{Revision: bm.HeadRevision, Noop: true}, nil
}

puts, deletes := splitChanges(changes, next)
revID, err := s.engine.Commit(ctx, engine.CommitRequest{
Namespace:   scope.Namespace,
Branch:      scope.Branch,
ParentRevID: bm.HeadRevision,
Message:     chooseMessage(message, "apply"),
CreatedAt:   time.Now().UTC(),
Changes:     changes,
Puts:        puts,
Deletes:     deletes,
})
if err != nil {
return nil, err
}
return &ApplyResult{Revision: revID, Changes: changes}, nil
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func (s *Service) Get(ctx context.Context, scope Scope, revisionID int64, checkpointName string) (*GetResult, error) {
scope = normalizeScope(scope)
if err := checkScope(ctx, scope.Namespace, scope.Branch); err != nil {
return nil, err
}

// Checkpoint: stored with full snapshot.
if checkpointName != "" {
cp, err := s.engine.GetCheckpoint(ctx, scope.Namespace, checkpointName)
if err != nil {
return nil, err
}
if cp == nil {
return nil, fmt.Errorf("checkpoint %q not found", checkpointName)
}
objs := cloneObjects(cp.Objects)
filterKeysByPrefix(ctx, objs)
return &GetResult{Revision: cp.RevisionID, Objects: objs}, nil
}

// Current HEAD.
if revisionID == 0 {
bm, err := s.engine.GetBranchMeta(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}
headRev := int64(0)
if bm != nil {
headRev = bm.HeadRevision
}
objs, err := s.engine.GetObjects(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}
filterKeysByPrefix(ctx, objs)
return &GetResult{Revision: headRev, Objects: objs}, nil
}

// Specific revision: reconstruct by reverse-applying deltas from HEAD.
objs, err := s.reconstructAtRevision(ctx, scope, revisionID)
if err != nil {
return nil, err
}
filterKeysByPrefix(ctx, objs)
return &GetResult{Revision: revisionID, Objects: objs}, nil
}

// reconstructAtRevision walks backwards from HEAD applying reverse changes
// until it reaches targetRevID.
func (s *Service) reconstructAtRevision(ctx context.Context, scope Scope, targetRevID int64) (map[string]string, error) {
bm, err := s.engine.GetBranchMeta(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}
if bm == nil || bm.HeadRevision == 0 {
return map[string]string{}, nil
}

objs, err := s.engine.GetObjects(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}

if bm.HeadRevision == targetRevID {
return objs, nil
}

curID := bm.HeadRevision
for curID != targetRevID {
rev, err := s.engine.GetRevision(ctx, scope.Namespace, scope.Branch, curID)
if err != nil {
return nil, err
}
if rev == nil {
return nil, fmt.Errorf("revision %d not found (branch %s/%s)", curID, scope.Namespace, scope.Branch)
}
// Reverse each change in this revision.
for _, c := range rev.Changes {
switch c.Type {
case "create":
delete(objs, c.Key)
case "delete":
objs[c.Key] = c.Before
case "update":
objs[c.Key] = c.Before
}
}
if rev.ParentID == 0 {
break
}
curID = rev.ParentID
}
return objs, nil
}

// ─── Diff ─────────────────────────────────────────────────────────────────────

func (s *Service) Diff(ctx context.Context, scope Scope, revisionID int64, checkpointName string, _ []string) (*DiffResult, error) {
return s.DiffDocuments(ctx, scope, revisionID, checkpointName, nil)
}

func (s *Service) DiffDocuments(ctx context.Context, scope Scope, revisionID int64, checkpointName string, documents []Document) (*DiffResult, error) {
scope = normalizeScope(scope)
if err := checkScope(ctx, scope.Namespace, scope.Branch); err != nil {
return nil, err
}

bm, err := s.engine.GetBranchMeta(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}
headRev := int64(0)
if bm != nil {
headRev = bm.HeadRevision
}

current, err := s.engine.GetObjects(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}

if len(documents) > 0 {
next := cloneObjects(current)
for _, doc := range documents {
next[doc.Key] = doc.Content
}
return &DiffResult{
FromRevision: headRev,
ToRevision:   headRev,
Changes:      diffObjects(current, next),
}, nil
}

target, targetRev, err := s.resolveObjects(ctx, scope, revisionID, checkpointName, "")
if err != nil {
return nil, err
}
return &DiffResult{
FromRevision: headRev,
ToRevision:   targetRev,
Changes:      diffObjects(current, target),
}, nil
}

// ─── Checkpoint ───────────────────────────────────────────────────────────────

func (s *Service) Checkpoint(ctx context.Context, scope Scope, name, description string) (*CheckpointResult, error) {
s.mu.Lock()
defer s.mu.Unlock()
if strings.TrimSpace(name) == "" {
return nil, errors.New("checkpoint name is required")
}
scope = normalizeScope(scope)
if err := checkScope(ctx, scope.Namespace, scope.Branch); err != nil {
return nil, err
}

bm, err := s.ensureBranch(ctx, scope)
if err != nil {
return nil, err
}
objs, err := s.engine.GetObjects(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}
cp := &model.Checkpoint{
Name:        name,
Namespace:   scope.Namespace,
Branch:      scope.Branch,
RevisionID:  bm.HeadRevision,
CreatedAt:   time.Now().UTC(),
Description: description,
Objects:     cloneObjects(objs),
}
if err := s.engine.SaveCheckpoint(ctx, cp); err != nil {
return nil, err
}
return &CheckpointResult{Name: name, RevisionID: bm.HeadRevision}, nil
}

// ─── Rollback / Restore ───────────────────────────────────────────────────────

func (s *Service) Rollback(ctx context.Context, scope Scope, revisionID int64, checkpointName, message string) (*ApplyResult, error) {
if err := checkScope(ctx, normalizeScope(scope).Namespace, normalizeScope(scope).Branch); err != nil {
return nil, err
}
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
if err := checkScope(ctx, normalizeScope(input.Scope).Namespace, normalizeScope(input.Scope).Branch); err != nil {
return nil, err
}
s.mu.Lock()
defer s.mu.Unlock()
return s.restore(ctx, input)
}

func (s *Service) restore(ctx context.Context, input RestoreInput) (*ApplyResult, error) {
scope := normalizeScope(input.Scope)

bm, err := s.ensureBranch(ctx, scope)
if err != nil {
return nil, err
}
current, err := s.engine.GetObjects(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}

target, _, err := s.resolveObjects(ctx, scope, input.FromRevision, input.FromCheckpoint, input.FromBranch)
if err != nil {
return nil, err
}

// When a key prefix is enforced, scope both sides of the diff so that keys
// outside the caller's prefix are neither modified nor deleted.
filterKeysByPrefix(ctx, current)
filterKeysByPrefix(ctx, target)

changes := diffObjects(current, target)
if len(changes) == 0 {
return &ApplyResult{Revision: bm.HeadRevision, Noop: true}, nil
}

puts, deletes := splitChanges(changes, target)
revID, err := s.engine.Commit(ctx, engine.CommitRequest{
Namespace:   scope.Namespace,
Branch:      scope.Branch,
ParentRevID: bm.HeadRevision,
Message:     chooseMessage(input.Message, "restore"),
CreatedAt:   time.Now().UTC(),
Changes:     changes,
Puts:        puts,
Deletes:     deletes,
})
if err != nil {
return nil, err
}
return &ApplyResult{Revision: revID, Changes: changes}, nil
}

// ─── Log ──────────────────────────────────────────────────────────────────────

func (s *Service) Log(ctx context.Context, scope Scope, limit int) ([]LogEntry, error) {
scope = normalizeScope(scope)
if err := checkScope(ctx, scope.Namespace, scope.Branch); err != nil {
return nil, err
}
bm, err := s.engine.GetBranchMeta(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}
if bm == nil {
return nil, nil
}
if limit <= 0 {
limit = 20
}
var out []LogEntry
for revID := bm.HeadRevision; revID > 0 && len(out) < limit; {
rev, err := s.engine.GetRevision(ctx, scope.Namespace, scope.Branch, revID)
if err != nil {
return nil, err
}
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

// ─── Branches ─────────────────────────────────────────────────────────────────

func (s *Service) CreateBranch(ctx context.Context, scope Scope, name string, fromRevision int64, fromCheckpoint string) (*BranchResult, error) {
if err := checkScope(ctx, normalizeScope(scope).Namespace, normalizeScope(scope).Branch); err != nil {
return nil, err
}
s.mu.Lock()
defer s.mu.Unlock()
if strings.TrimSpace(name) == "" {
return nil, errors.New("branch name is required")
}
scope = normalizeScope(scope)

existing, err := s.engine.GetBranchMeta(ctx, scope.Namespace, name)
if err != nil {
return nil, err
}
if existing != nil {
return nil, fmt.Errorf("branch %q already exists", name)
}

srcObjects, srcRevID, err := s.resolveObjects(ctx, scope, fromRevision, fromCheckpoint, "")
if err != nil {
return nil, err
}

puts := make(map[string]string, len(srcObjects))
var changes []model.Change
for k, v := range srcObjects {
puts[k] = v
changes = append(changes, model.Change{Key: k, Type: "create", After: v})
}

revID, err := s.engine.Commit(ctx, engine.CommitRequest{
Namespace:    scope.Namespace,
Branch:       name,
ParentRevID:  0,
BaseRevision: srcRevID,
Message:      fmt.Sprintf("branch from %s@%d", scope.Branch, srcRevID),
CreatedAt:    time.Now().UTC(),
Changes:      changes,
Puts:         puts,
})
if err != nil {
return nil, err
}
return &BranchResult{Name: name, BaseRevision: srcRevID, HeadRevision: revID}, nil
}

func (s *Service) ListBranches(ctx context.Context, scope Scope) ([]BranchListEntry, error) {
scope = normalizeScope(scope)
if err := checkScope(ctx, scope.Namespace, ""); err != nil {
return nil, err
}
names, err := s.engine.ListBranches(ctx, scope.Namespace)
if err != nil {
return nil, err
}
slices.Sort(names)
out := make([]BranchListEntry, 0, len(names))
for _, name := range names {
bm, err := s.engine.GetBranchMeta(ctx, scope.Namespace, name)
if err != nil {
return nil, err
}
if bm == nil {
continue
}
out = append(out, BranchListEntry{
Name:         bm.Name,
BaseRevision: bm.BaseRevision,
HeadRevision: bm.HeadRevision,
Current:      name == scope.Branch,
})
}
return out, nil
}

// ─── GC ───────────────────────────────────────────────────────────────────────

func (s *Service) GC(ctx context.Context, keep int) (*GCResult, error) {
if keep <= 0 {
keep = 10
}
s.mu.Lock()
defer s.mu.Unlock()

nsList, err := s.engine.ListNamespaces(ctx)
if err != nil {
return nil, err
}

pruned := 0
for _, ns := range nsList {
branches, err := s.engine.ListBranches(ctx, ns)
if err != nil {
return nil, err
}
cpNames, err := s.engine.ListCheckpoints(ctx, ns)
if err != nil {
return nil, err
}

protected := map[int64]struct{}{}
for _, cpName := range cpNames {
cp, err := s.engine.GetCheckpoint(ctx, ns, cpName)
if err != nil {
return nil, err
}
if cp != nil {
protected[cp.RevisionID] = struct{}{}
}
}

for _, branch := range branches {
bm, err := s.engine.GetBranchMeta(ctx, ns, branch)
if err != nil {
return nil, err
}
if bm == nil {
continue
}
if bm.BaseRevision > 0 {
protected[bm.BaseRevision] = struct{}{}
}
id := bm.HeadRevision
for i := 0; i < keep && id > 0; i++ {
protected[id] = struct{}{}
rev, err := s.engine.GetRevision(ctx, ns, branch, id)
if err != nil || rev == nil {
break
}
id = rev.ParentID
}

// Prune unprotected revisions in the chain.
id = bm.HeadRevision
for id > 0 {
rev, err := s.engine.GetRevision(ctx, ns, branch, id)
if err != nil || rev == nil {
break
}
parent := rev.ParentID
if _, ok := protected[id]; !ok {
if err := s.engine.DeleteRevision(ctx, ns, branch, id); err != nil {
return nil, err
}
pruned++
}
id = parent
}
}
}
return &GCResult{Pruned: pruned}, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// ensureBranch returns the BranchMeta for scope, creating metadata entries if absent.
func (s *Service) ensureBranch(ctx context.Context, scope Scope) (*model.BranchMeta, error) {
bm, err := s.engine.GetBranchMeta(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, err
}
if bm != nil {
return bm, nil
}
nm, err := s.engine.GetNamespaceMeta(ctx, scope.Namespace)
if err != nil {
return nil, err
}
if nm.ActiveBranch == "" {
nm.ActiveBranch = scope.Branch
if err := s.engine.SaveNamespaceMeta(ctx, scope.Namespace, nm); err != nil {
return nil, err
}
}
bm = &model.BranchMeta{Name: scope.Branch}
if err := s.engine.SaveBranchMeta(ctx, scope.Namespace, scope.Branch, bm); err != nil {
return nil, err
}
return bm, nil
}

// resolveObjects returns the object snapshot and revision ID for the given selector.
func (s *Service) resolveObjects(ctx context.Context, scope Scope, revisionID int64, checkpointName, fromBranch string) (map[string]string, int64, error) {
switch {
case checkpointName != "":
cp, err := s.engine.GetCheckpoint(ctx, scope.Namespace, checkpointName)
if err != nil {
return nil, 0, err
}
if cp == nil {
return nil, 0, fmt.Errorf("checkpoint %q not found", checkpointName)
}
return cloneObjects(cp.Objects), cp.RevisionID, nil

case fromBranch != "":
bm, err := s.engine.GetBranchMeta(ctx, scope.Namespace, fromBranch)
if err != nil {
return nil, 0, err
}
if bm == nil {
return nil, 0, fmt.Errorf("branch %q not found", fromBranch)
}
objs, err := s.engine.GetObjects(ctx, scope.Namespace, fromBranch)
if err != nil {
return nil, 0, err
}
return objs, bm.HeadRevision, nil

case revisionID > 0:
objs, err := s.reconstructAtRevision(ctx, scope, revisionID)
if err != nil {
return nil, 0, err
}
return objs, revisionID, nil

default:
objs, err := s.engine.GetObjects(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, 0, err
}
bm, err := s.engine.GetBranchMeta(ctx, scope.Namespace, scope.Branch)
if err != nil {
return nil, 0, err
}
headRev := int64(0)
if bm != nil {
headRev = bm.HeadRevision
}
return objs, headRev, nil
}
}

// splitChanges separates a diff into put and delete operations.
func splitChanges(changes []model.Change, after map[string]string) (puts map[string]string, deletes []string) {
puts = map[string]string{}
for _, c := range changes {
switch c.Type {
case "delete":
deletes = append(deletes, c.Key)
default:
puts[c.Key] = after[c.Key]
}
}
return puts, deletes
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
