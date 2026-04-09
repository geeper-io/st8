package local

import (
"context"
"testing"
"time"

"github.com/geeper-io/st8/internal/engine"
"github.com/geeper-io/st8/internal/model"
)

func newEngine(t *testing.T) *Engine {
t.Helper()
return New(t.TempDir())
}

func ctx(t *testing.T) context.Context {
t.Helper()
return context.Background()
}

func commit(t *testing.T, e *Engine, ns, branch string, parent int64, puts map[string]string, deletes []string) int64 {
t.Helper()
var changes []model.Change
for k, v := range puts {
changes = append(changes, model.Change{Key: k, Type: "create", After: v})
}
revID, err := e.Commit(ctx(t), engine.CommitRequest{
Namespace:   ns,
Branch:      branch,
ParentRevID: parent,
Message:     "test",
CreatedAt:   time.Now().UTC(),
Changes:     changes,
Puts:        puts,
Deletes:     deletes,
})
if err != nil {
t.Fatalf("Commit: %v", err)
}
return revID
}

func TestPing(t *testing.T) {
e := newEngine(t)
if err := e.Ping(ctx(t)); err != nil {
t.Fatalf("Ping: %v", err)
}
}

// ─── GetObjects / Commit ──────────────────────────────────────────────────────

func TestCommitAndGetObjects(t *testing.T) {
e := newEngine(t)
revID := commit(t, e, "ns", "main", 0, map[string]string{"k": "v"}, nil)
if revID != 1 {
t.Fatalf("revID = %d, want 1", revID)
}
objs, err := e.GetObjects(ctx(t), "ns", "main")
if err != nil {
t.Fatalf("GetObjects: %v", err)
}
if objs["k"] != "v" {
t.Fatalf("k = %q, want v", objs["k"])
}
}

func TestCommitDelete(t *testing.T) {
e := newEngine(t)
commit(t, e, "ns", "main", 0, map[string]string{"k": "v", "k2": "v2"}, nil)
commit(t, e, "ns", "main", 1, map[string]string{}, []string{"k"})

objs, _ := e.GetObjects(ctx(t), "ns", "main")
if _, ok := objs["k"]; ok {
t.Fatal("expected k to be deleted")
}
if objs["k2"] != "v2" {
t.Fatalf("k2 = %q, want v2", objs["k2"])
}
}

func TestCommitIncrementsRevision(t *testing.T) {
e := newEngine(t)
r1 := commit(t, e, "ns", "main", 0, map[string]string{"a": "1"}, nil)
r2 := commit(t, e, "ns", "main", r1, map[string]string{"a": "2"}, nil)
if r2 != r1+1 {
t.Fatalf("r2 = %d, want %d", r2, r1+1)
}
}

func TestGetObjectsEmptyBranch(t *testing.T) {
e := newEngine(t)
objs, err := e.GetObjects(ctx(t), "ns", "main")
if err != nil {
t.Fatalf("GetObjects: %v", err)
}
if len(objs) != 0 {
t.Fatalf("expected empty objects, got %v", objs)
}
}

// ─── GetRevision / DeleteRevision ─────────────────────────────────────────────

func TestGetRevision(t *testing.T) {
e := newEngine(t)
revID := commit(t, e, "ns", "main", 0, map[string]string{"k": "v"}, nil)

rev, err := e.GetRevision(ctx(t), "ns", "main", revID)
if err != nil {
t.Fatalf("GetRevision: %v", err)
}
if rev == nil {
t.Fatal("expected revision, got nil")
}
if rev.ID != revID {
t.Fatalf("rev.ID = %d, want %d", rev.ID, revID)
}
if rev.Message != "test" {
t.Fatalf("rev.Message = %q, want test", rev.Message)
}
}

func TestGetRevisionMissing(t *testing.T) {
e := newEngine(t)
rev, err := e.GetRevision(ctx(t), "ns", "main", 999)
if err != nil {
t.Fatalf("GetRevision: %v", err)
}
if rev != nil {
t.Fatal("expected nil for missing revision")
}
}

func TestDeleteRevision(t *testing.T) {
e := newEngine(t)
revID := commit(t, e, "ns", "main", 0, map[string]string{"k": "v"}, nil)
if err := e.DeleteRevision(ctx(t), "ns", "main", revID); err != nil {
t.Fatalf("DeleteRevision: %v", err)
}
rev, _ := e.GetRevision(ctx(t), "ns", "main", revID)
if rev != nil {
t.Fatal("expected nil after delete")
}
}

// ─── NamespaceMeta ────────────────────────────────────────────────────────────

func TestNamespaceMeta(t *testing.T) {
e := newEngine(t)
m, err := e.GetNamespaceMeta(ctx(t), "ns")
if err != nil {
t.Fatalf("GetNamespaceMeta: %v", err)
}
if m.NextRevision != 1 {
t.Fatalf("NextRevision = %d, want 1", m.NextRevision)
}

m.ActiveBranch = "main"
m.NextRevision = 42
if err := e.SaveNamespaceMeta(ctx(t), "ns", m); err != nil {
t.Fatalf("SaveNamespaceMeta: %v", err)
}

m2, _ := e.GetNamespaceMeta(ctx(t), "ns")
if m2.NextRevision != 42 || m2.ActiveBranch != "main" {
t.Fatalf("meta = %+v, want NextRevision=42 ActiveBranch=main", m2)
}
}

// ─── BranchMeta ───────────────────────────────────────────────────────────────

func TestBranchMeta(t *testing.T) {
e := newEngine(t)
m, err := e.GetBranchMeta(ctx(t), "ns", "main")
if err != nil {
t.Fatalf("GetBranchMeta: %v", err)
}
if m != nil {
t.Fatalf("expected nil for new branch, got %+v", m)
}

bm := &model.BranchMeta{Name: "main", HeadRevision: 5, BaseRevision: 1}
if err := e.SaveBranchMeta(ctx(t), "ns", "main", bm); err != nil {
t.Fatalf("SaveBranchMeta: %v", err)
}

got, _ := e.GetBranchMeta(ctx(t), "ns", "main")
if got.HeadRevision != 5 {
t.Fatalf("HeadRevision = %d, want 5", got.HeadRevision)
}
}

// ─── Checkpoint ───────────────────────────────────────────────────────────────

func TestCheckpoint(t *testing.T) {
e := newEngine(t)
cp := &model.Checkpoint{
Name:       "snap",
Namespace:  "ns",
Branch:     "main",
RevisionID: 3,
CreatedAt:  time.Now().UTC(),
Objects:    map[string]string{"k": "v"},
}
if err := e.SaveCheckpoint(ctx(t), cp); err != nil {
t.Fatalf("SaveCheckpoint: %v", err)
}

got, err := e.GetCheckpoint(ctx(t), "ns", "snap")
if err != nil {
t.Fatalf("GetCheckpoint: %v", err)
}
if got.Objects["k"] != "v" {
t.Fatalf("k = %q, want v", got.Objects["k"])
}
}

func TestCheckpointMissing(t *testing.T) {
e := newEngine(t)
got, err := e.GetCheckpoint(ctx(t), "ns", "nope")
if err != nil {
t.Fatalf("GetCheckpoint: %v", err)
}
if got != nil {
t.Fatal("expected nil for missing checkpoint")
}
}

func TestDeleteCheckpoint(t *testing.T) {
e := newEngine(t)
cp := &model.Checkpoint{Name: "snap", Namespace: "ns", Branch: "main", Objects: map[string]string{}}
_ = e.SaveCheckpoint(ctx(t), cp)
if err := e.DeleteCheckpoint(ctx(t), "ns", "snap"); err != nil {
t.Fatalf("DeleteCheckpoint: %v", err)
}
got, _ := e.GetCheckpoint(ctx(t), "ns", "snap")
if got != nil {
t.Fatal("expected nil after delete")
}
}

// ─── List methods ─────────────────────────────────────────────────────────────

func TestListCheckpoints(t *testing.T) {
e := newEngine(t)
for _, name := range []string{"snap1", "snap2", "snap3"} {
_ = e.SaveCheckpoint(ctx(t), &model.Checkpoint{Name: name, Namespace: "ns", Branch: "main", Objects: map[string]string{}})
}
names, err := e.ListCheckpoints(ctx(t), "ns")
if err != nil {
t.Fatalf("ListCheckpoints: %v", err)
}
if len(names) != 3 {
t.Fatalf("len = %d, want 3", len(names))
}
}

func TestListBranches(t *testing.T) {
e := newEngine(t)
for _, b := range []string{"main", "feature", "hotfix"} {
commit(t, e, "ns", b, 0, map[string]string{"k": "v"}, nil)
}
names, err := e.ListBranches(ctx(t), "ns")
if err != nil {
t.Fatalf("ListBranches: %v", err)
}
if len(names) != 3 {
t.Fatalf("len = %d, want 3", len(names))
}
}

func TestListNamespaces(t *testing.T) {
e := newEngine(t)
for _, ns := range []string{"a", "b", "c"} {
commit(t, e, ns, "main", 0, map[string]string{"k": "v"}, nil)
}
names, err := e.ListNamespaces(ctx(t))
if err != nil {
t.Fatalf("ListNamespaces: %v", err)
}
if len(names) != 3 {
t.Fatalf("len = %d, want 3", len(names))
}
}

// ─── Persistence across reloads ───────────────────────────────────────────────

func TestPersistence(t *testing.T) {
dir := t.TempDir()
e1 := New(dir)
commit(t, e1, "ns", "main", 0, map[string]string{"k": "persistent"}, nil)

e2 := New(dir)
objs, err := e2.GetObjects(ctx(t), "ns", "main")
if err != nil {
t.Fatalf("GetObjects after reload: %v", err)
}
if objs["k"] != "persistent" {
t.Fatalf("k = %q after reload, want persistent", objs["k"])
}
}
