package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/geeper-io/st8/internal/auth"
	"github.com/geeper-io/st8/internal/document"
	"github.com/geeper-io/st8/internal/engine/local"
)

func newSvc(t *testing.T) *Service {
	t.Helper()
	return New(local.New(filepath.Join(t.TempDir(), ".st8")))
}

func applyDocs(t *testing.T, svc *Service, scope Scope, msg string, kvs ...string) *ApplyResult {
	t.Helper()
	docs := make([]Document, 0, len(kvs)/2)
	for i := 0; i < len(kvs)-1; i += 2 {
		docs = append(docs, Document{Key: kvs[i], Content: kvs[i+1]})
	}
	res, err := svc.Apply(context.Background(), ApplyInput{Scope: scope, Documents: docs, Message: msg})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return res
}

// ─── Basic apply ─────────────────────────────────────────────────────────────

func TestApplyAssignsSequentialRevisions(t *testing.T) {
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}
	r1 := applyDocs(t, svc, scope, "first", "k1", "v1")
	if r1.Revision != 1 {
		t.Fatalf("rev1 = %d, want 1", r1.Revision)
	}
	r2 := applyDocs(t, svc, scope, "second", "k2", "v2")
	if r2.Revision != 2 {
		t.Fatalf("rev2 = %d, want 2", r2.Revision)
	}
}

func TestApplyNoop(t *testing.T) {
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}
	r1 := applyDocs(t, svc, scope, "first", "k", "v")
	r2 := applyDocs(t, svc, scope, "same", "k", "v")
	if !r2.Noop {
		t.Fatal("expected noop on identical content")
	}
	if r2.Revision != r1.Revision {
		t.Fatalf("noop revision = %d, want %d", r2.Revision, r1.Revision)
	}
}

func TestApplyDelete(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}
	applyDocs(t, svc, scope, "add", "k", "v")

	// Apply without the key — results in a delete change.
	// We need a different key to avoid noop.
	applyDocs(t, svc, scope, "other", "k2", "v2")

	// Verify k is still there, then apply only k2 to effectively remove k via restore.
	got, err := svc.Get(ctx, scope, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Objects["k"] != "v" {
		t.Fatalf("expected k=v, got %q", got.Objects["k"])
	}
}

// ─── Get at revision ─────────────────────────────────────────────────────────

func TestGetAtRevision(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}

	r1 := applyDocs(t, svc, scope, "set v1", "cfg", "v1")
	r2 := applyDocs(t, svc, scope, "set v2", "cfg", "v2")
	applyDocs(t, svc, scope, "set v3", "cfg", "v3")

	// Current HEAD should be v3.
	cur, _ := svc.Get(ctx, scope, 0, "")
	if cur.Objects["cfg"] != "v3" {
		t.Fatalf("current = %q, want v3", cur.Objects["cfg"])
	}

	// At rev1 should be v1.
	at1, err := svc.Get(ctx, scope, r1.Revision, "")
	if err != nil {
		t.Fatalf("get@r1: %v", err)
	}
	if at1.Objects["cfg"] != "v1" {
		t.Fatalf("@r1 = %q, want v1", at1.Objects["cfg"])
	}

	// At rev2 should be v2.
	at2, err := svc.Get(ctx, scope, r2.Revision, "")
	if err != nil {
		t.Fatalf("get@r2: %v", err)
	}
	if at2.Objects["cfg"] != "v2" {
		t.Fatalf("@r2 = %q, want v2", at2.Objects["cfg"])
	}
}

// ─── Checkpoint ───────────────────────────────────────────────────────────────

func TestCheckpointRestoresSnapshot(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}

	applyDocs(t, svc, scope, "base", "cfg", "stable")
	if _, err := svc.Checkpoint(ctx, scope, "stable", "known good"); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	applyDocs(t, svc, scope, "change", "cfg", "broken")

	// Get via checkpoint should return the snapshot value.
	got, err := svc.Get(ctx, scope, 0, "stable")
	if err != nil {
		t.Fatalf("get@checkpoint: %v", err)
	}
	if got.Objects["cfg"] != "stable" {
		t.Fatalf("checkpoint get = %q, want stable", got.Objects["cfg"])
	}
}

func TestCheckpointNameRequired(t *testing.T) {
	svc := newSvc(t)
	_, err := svc.Checkpoint(context.Background(), Scope{Namespace: "ns"}, "", "")
	if err == nil {
		t.Fatal("expected error for empty checkpoint name")
	}
}

// ─── Rollback ─────────────────────────────────────────────────────────────────

func TestRollbackCreatesNewRevision(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "platform-dev", Branch: "main"}

	first := applyDocs(t, svc, scope, "first", "k", "v1")
	second := applyDocs(t, svc, scope, "second", "k", "v2")

	rolled, err := svc.Rollback(ctx, scope, first.Revision, "", "")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolled.Revision <= second.Revision {
		t.Fatalf("rollback should create new revision, got %d after %d", rolled.Revision, second.Revision)
	}

	got, _ := svc.Get(ctx, scope, 0, "")
	if got.Objects["k"] != "v1" {
		t.Fatalf("after rollback k = %q, want v1", got.Objects["k"])
	}
}

func TestRollbackToCheckpoint(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}

	applyDocs(t, svc, scope, "good", "k", "good")
	if _, err := svc.Checkpoint(ctx, scope, "good", ""); err != nil {
		t.Fatal(err)
	}
	applyDocs(t, svc, scope, "bad", "k", "bad")

	rolled, err := svc.Rollback(ctx, scope, 0, "good", "back to good")
	if err != nil {
		t.Fatalf("rollback to checkpoint: %v", err)
	}
	got, _ := svc.Get(ctx, scope, 0, "")
	if got.Objects["k"] != "good" {
		t.Fatalf("after rollback k = %q, want good", got.Objects["k"])
	}
	if rolled.Noop {
		t.Fatal("rollback should not be noop")
	}
}

// ─── Log ──────────────────────────────────────────────────────────────────────

func TestLog(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}

	applyDocs(t, svc, scope, "first", "k", "1")
	applyDocs(t, svc, scope, "second", "k", "2")
	applyDocs(t, svc, scope, "third", "k", "3")

	entries, err := svc.Log(ctx, scope, 10)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("log len = %d, want 3", len(entries))
	}
	// Log is newest-first.
	if entries[0].Message != "third" {
		t.Fatalf("log[0].Message = %q, want third", entries[0].Message)
	}
	if entries[2].Message != "first" {
		t.Fatalf("log[2].Message = %q, want first", entries[2].Message)
	}
}

func TestLogLimit(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}

	for i := 0; i < 5; i++ {
		applyDocs(t, svc, scope, fmt.Sprintf("r%d", i), "k", fmt.Sprintf("%d", i))
	}
	entries, err := svc.Log(ctx, scope, 2)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("log len = %d, want 2", len(entries))
	}
}

// ─── Diff ─────────────────────────────────────────────────────────────────────

func TestDiffDocuments(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}
	applyDocs(t, svc, scope, "base", "k", "old")

	diff, err := svc.DiffDocuments(ctx, scope, 0, "", []Document{{Key: "k", Content: "new"}})
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diff.Changes) != 1 {
		t.Fatalf("diff changes = %d, want 1", len(diff.Changes))
	}
	if diff.Changes[0].Type != "update" || diff.Changes[0].Before != "old" || diff.Changes[0].After != "new" {
		t.Fatalf("unexpected change: %+v", diff.Changes[0])
	}
}

func TestDiffNoop(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}
	applyDocs(t, svc, scope, "base", "k", "v")

	diff, err := svc.DiffDocuments(ctx, scope, 0, "", []Document{{Key: "k", Content: "v"}})
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diff.Changes) != 0 {
		t.Fatalf("diff changes = %d, want 0", len(diff.Changes))
	}
}

// ─── Branches ─────────────────────────────────────────────────────────────────

func TestCreateAndListBranches(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}
	applyDocs(t, svc, scope, "base", "k", "v")

	if _, err := svc.CreateBranch(ctx, scope, "feature", 0, ""); err != nil {
		t.Fatalf("create branch: %v", err)
	}

	branches, err := svc.ListBranches(ctx, scope)
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	names := map[string]bool{}
	for _, b := range branches {
		names[b.Name] = true
	}
	if !names["main"] || !names["feature"] {
		t.Fatalf("branches = %v, want main and feature", names)
	}
}

func TestCreateBranchInheritsObjects(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}
	applyDocs(t, svc, scope, "base", "k", "inherited")

	if _, err := svc.CreateBranch(ctx, scope, "fork", 0, ""); err != nil {
		t.Fatalf("create branch: %v", err)
	}

	got, err := svc.Get(ctx, Scope{Namespace: "ns", Branch: "fork"}, 0, "")
	if err != nil {
		t.Fatalf("get from fork: %v", err)
	}
	if got.Objects["k"] != "inherited" {
		t.Fatalf("fork k = %q, want inherited", got.Objects["k"])
	}
}

func TestCreateBranchDuplicate(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}
	applyDocs(t, svc, scope, "base", "k", "v")

	if _, err := svc.CreateBranch(ctx, scope, "dup", 0, ""); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := svc.CreateBranch(ctx, scope, "dup", 0, ""); err == nil {
		t.Fatal("expected error creating duplicate branch")
	}
}

func TestBranchIsolation(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	mainScope := Scope{Namespace: "ns", Branch: "main"}
	forkScope := Scope{Namespace: "ns", Branch: "fork"}

	applyDocs(t, svc, mainScope, "base", "k", "main-value")
	if _, err := svc.CreateBranch(ctx, mainScope, "fork", 0, ""); err != nil {
		t.Fatal(err)
	}

	// Change main; fork should be unaffected.
	applyDocs(t, svc, mainScope, "update main", "k", "main-updated")

	forkState, _ := svc.Get(ctx, forkScope, 0, "")
	if forkState.Objects["k"] != "main-value" {
		t.Fatalf("fork k = %q after main update, want main-value", forkState.Objects["k"])
	}
}

// ─── GC ───────────────────────────────────────────────────────────────────────

func TestGCPrunesOldRevisions(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}

	for i := 0; i < 15; i++ {
		applyDocs(t, svc, scope, fmt.Sprintf("r%d", i), "k", fmt.Sprintf("%d", i))
	}

	res, err := svc.GC(ctx, 5)
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if res.Pruned == 0 {
		t.Fatal("expected some revisions to be pruned")
	}

	// HEAD should still be readable after GC.
	got, err := svc.Get(ctx, scope, 0, "")
	if err != nil {
		t.Fatalf("get after gc: %v", err)
	}
	if got.Objects["k"] != "14" {
		t.Fatalf("after gc k = %q, want 14", got.Objects["k"])
	}
}

func TestGCProtectsCheckpoints(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	scope := Scope{Namespace: "ns", Branch: "main"}

	applyDocs(t, svc, scope, "base", "k", "checkpoint-value")
	if _, err := svc.Checkpoint(ctx, scope, "snap", ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		applyDocs(t, svc, scope, fmt.Sprintf("r%d", i), "k", fmt.Sprintf("%d", i))
	}

	if _, err := svc.GC(ctx, 2); err != nil {
		t.Fatalf("gc: %v", err)
	}

	// Checkpoint snapshot should still be readable.
	got, err := svc.Get(ctx, scope, 0, "snap")
	if err != nil {
		t.Fatalf("get checkpoint after gc: %v", err)
	}
	if got.Objects["k"] != "checkpoint-value" {
		t.Fatalf("checkpoint k = %q, want checkpoint-value", got.Objects["k"])
	}
}

// ─── Restore from branch ─────────────────────────────────────────────────────

func TestApplyCheckpointBranchRestoreFlow(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc := New(local.New(filepath.Join(dir, ".st8")))

	appFile := filepath.Join(dir, "config.json")
	if err := os.WriteFile(appFile, []byte(`{"timeout":30,"enabled":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	scope := Scope{Namespace: "payments/prod", Branch: "main"}
	applied, err := svc.Apply(ctx, ApplyInput{Scope: scope, Documents: []Document{{Key: document.NormalizeKey(appFile), Content: "{\n  \"enabled\": true,\n  \"timeout\": 30\n}\n"}}, Message: "initial apply"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.Revision != 1 {
		t.Fatalf("apply revision: got %d want 1", applied.Revision)
	}

	if _, err := svc.Checkpoint(ctx, scope, "prod-stable", "known good"); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	if _, err := svc.CreateBranch(ctx, scope, "experiment", 0, "prod-stable"); err != nil {
		t.Fatalf("create branch: %v", err)
	}

	expScope := Scope{Namespace: "payments/prod", Branch: "experiment"}
	// Rev 2 is consumed by CreateBranch (copies source snapshot to new branch).
	expApplied, err := svc.Apply(ctx, ApplyInput{Scope: expScope, Documents: []Document{{Key: document.NormalizeKey(appFile), Content: "{\n  \"enabled\": true,\n  \"timeout\": 45\n}\n"}}, Message: "branch tweak"})
	if err != nil {
		t.Fatalf("branch apply: %v", err)
	}
	if expApplied.Revision != 3 {
		t.Fatalf("branch revision: got %d want 3", expApplied.Revision)
	}

	restored, err := svc.Restore(ctx, RestoreInput{
		Scope:      scope,
		FromBranch: "experiment",
		Message:    "promote experiment",
	})
	if err != nil {
		t.Fatalf("restore from branch: %v", err)
	}
	if restored.Revision != 4 {
		t.Fatalf("restore revision: got %d want 4", restored.Revision)
	}

	got, err := svc.Get(ctx, scope, 0, "")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Objects["../tmp/nope"] != "" {
		t.Fatalf("unexpected object lookup")
	}
	if got.Objects[document.NormalizeKey(appFile)] == "" {
		t.Fatalf("expected config object after restore")
	}
}

// ─── Concurrency ──────────────────────────────────────────────────────────────

func TestConcurrentApplyDifferentNamespaces(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	const goroutines = 10

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			scope := Scope{Namespace: fmt.Sprintf("ns%d", i), Branch: "main"}
			_, err := svc.Apply(ctx, ApplyInput{
				Scope:     scope,
				Documents: []Document{{Key: "k", Content: fmt.Sprintf("v%d", i)}},
				Message:   "concurrent",
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent apply error: %v", err)
		}
	}
}

// ─── Auth / RBAC tests ────────────────────────────────────────────────────────

func newSvc(t *testing.T) *Service {
t.Helper()
return New(local.New(t.TempDir()))
}

func principalCtx(p *auth.Principal) context.Context {
return auth.ContextWithPrincipal(context.Background(), p)
}

func TestAuthScope_NamespaceDenied(t *testing.T) {
svc := newSvc(t)
p := &auth.Principal{Name: "app", Allow: auth.Policy{
Namespaces: []string{"prod"},
Branches:   []string{"*"},
Verbs:      []string{auth.VerbRead, auth.VerbWrite},
}}
ctx := principalCtx(p)
_, err := svc.Apply(ctx, ApplyInput{
Scope:     Scope{Namespace: "dev", Branch: "main"},
Documents: []Document{{Key: "x", Content: "v"}},
})
if err == nil || !strings.Contains(err.Error(), "forbidden") {
t.Errorf("expected forbidden error, got %v", err)
}
}

func TestAuthScope_BranchDenied(t *testing.T) {
svc := newSvc(t)
p := &auth.Principal{Name: "app", Allow: auth.Policy{
Namespaces: []string{"*"},
Branches:   []string{"main"},
Verbs:      []string{auth.VerbRead, auth.VerbWrite},
}}
ctx := principalCtx(p)
_, err := svc.Apply(ctx, ApplyInput{
Scope:     Scope{Namespace: "default", Branch: "feature"},
Documents: []Document{{Key: "x", Content: "v"}},
})
if err == nil || !strings.Contains(err.Error(), "forbidden") {
t.Errorf("expected forbidden error, got %v", err)
}
}

func TestAuthKeyPrefix_WriteRejected(t *testing.T) {
svc := newSvc(t)
p := &auth.Principal{Name: "app", Allow: auth.Policy{
Namespaces: []string{"*"},
Branches:   []string{"*"},
KeyPrefix:  "app/",
Verbs:      []string{auth.VerbRead, auth.VerbWrite},
}}
ctx := principalCtx(p)
// key outside prefix → should be rejected
_, err := svc.Apply(ctx, ApplyInput{
Scope:     Scope{Namespace: "default", Branch: "main"},
Documents: []Document{{Key: "other/key", Content: "v"}},
})
if err == nil || !strings.Contains(err.Error(), "forbidden") {
t.Errorf("expected forbidden error for out-of-prefix key, got %v", err)
}
}

func TestAuthKeyPrefix_WriteAllowed(t *testing.T) {
svc := newSvc(t)
p := &auth.Principal{Name: "app", Allow: auth.Policy{
Namespaces: []string{"*"},
Branches:   []string{"*"},
KeyPrefix:  "app/",
Verbs:      []string{auth.VerbRead, auth.VerbWrite},
}}
ctx := principalCtx(p)
res, err := svc.Apply(ctx, ApplyInput{
Scope:     Scope{Namespace: "default", Branch: "main"},
Documents: []Document{{Key: "app/config", Content: "v1"}},
})
if err != nil {
t.Fatalf("apply within prefix: %v", err)
}
if res.Noop {
t.Error("expected non-noop apply")
}
}

func TestAuthKeyPrefix_ReadFiltered(t *testing.T) {
svc := newSvc(t)
adminCtx := context.Background()
scope := Scope{Namespace: "default", Branch: "main"}

// Admin writes two keys under different prefixes
if _, err := svc.Apply(adminCtx, ApplyInput{
Scope: scope,
Documents: []Document{
{Key: "app/config", Content: "app-val"},
{Key: "infra/config", Content: "infra-val"},
},
}); err != nil {
t.Fatalf("admin apply: %v", err)
}

// Reader with prefix "app/" should only see app/config
p := &auth.Principal{Name: "app", Allow: auth.Policy{
Namespaces: []string{"*"},
Branches:   []string{"*"},
KeyPrefix:  "app/",
Verbs:      []string{auth.VerbRead},
}}
ctx := principalCtx(p)
got, err := svc.Get(ctx, scope, 0, "")
if err != nil {
t.Fatalf("get: %v", err)
}
if _, ok := got.Objects["infra/config"]; ok {
t.Error("infra/config should not be visible to app token")
}
if got.Objects["app/config"] != "app-val" {
t.Errorf("app/config = %q, want %q", got.Objects["app/config"], "app-val")
}
}

func TestAuthKeyPrefix_RestoreScoped(t *testing.T) {
svc := newSvc(t)
adminCtx := context.Background()
scope := Scope{Namespace: "default", Branch: "main"}

// Setup: two keys, two revisions
r1, err := svc.Apply(adminCtx, ApplyInput{
Scope:     scope,
Documents: []Document{{Key: "app/config", Content: "v1"}, {Key: "infra/net", Content: "net1"}},
})
if err != nil {
t.Fatalf("apply r1: %v", err)
}
if _, err := svc.Apply(adminCtx, ApplyInput{
Scope:     scope,
Documents: []Document{{Key: "app/config", Content: "v2"}, {Key: "infra/net", Content: "net2"}},
}); err != nil {
t.Fatalf("apply r2: %v", err)
}

// App token rolls back to r1 — should only affect app/ keys
p := &auth.Principal{Name: "app", Allow: auth.Policy{
Namespaces: []string{"*"},
Branches:   []string{"*"},
KeyPrefix:  "app/",
Verbs:      []string{auth.VerbRead, auth.VerbWrite},
}}
ctx := principalCtx(p)
if _, err := svc.Rollback(ctx, scope, r1.Revision, "", ""); err != nil {
t.Fatalf("rollback: %v", err)
}

// infra/net should remain at v2 (untouched by app token)
got, err := svc.Get(adminCtx, scope, 0, "")
if err != nil {
t.Fatalf("get: %v", err)
}
if got.Objects["infra/net"] != "net2" {
t.Errorf("infra/net = %q after scoped rollback, want net2", got.Objects["infra/net"])
}
if got.Objects["app/config"] != "v1" {
t.Errorf("app/config = %q after scoped rollback, want v1", got.Objects["app/config"])
}
}

func TestAuthScope_AllowedAccess(t *testing.T) {
svc := newSvc(t)
p := &auth.Principal{Name: "app", Allow: auth.Policy{
Namespaces: []string{"prod", "staging"},
Branches:   []string{"main", "release/*"},
Verbs:      []string{auth.VerbRead, auth.VerbWrite},
}}
ctx := principalCtx(p)
// Allowed namespace + branch
if _, err := svc.Apply(ctx, ApplyInput{
Scope:     Scope{Namespace: "prod", Branch: "main"},
Documents: []Document{{Key: "k", Content: "v"}},
}); err != nil {
t.Errorf("apply to allowed scope: %v", err)
}
// Allowed branch glob
if _, err := svc.Apply(ctx, ApplyInput{
Scope:     Scope{Namespace: "staging", Branch: "release/1.0"},
Documents: []Document{{Key: "k", Content: "v"}},
}); err != nil {
t.Errorf("apply to allowed glob branch: %v", err)
}
}
