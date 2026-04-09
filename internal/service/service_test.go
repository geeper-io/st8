package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geeper-io/st8/internal/document"
	"github.com/geeper-io/st8/internal/engine/local"
)

func TestApplyCheckpointBranchRestoreFlow(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc := New(local.New(filepath.Join(dir, ".st8")))

	appFile := filepath.Join(dir, "config.json")
	if err := os.WriteFile(appFile, []byte("{\"timeout\":30,\"enabled\":true}"), 0o644); err != nil {
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

	expFile := filepath.Join(dir, "config.json")
	if err := os.WriteFile(expFile, []byte("{\"timeout\":45,\"enabled\":true}"), 0o644); err != nil {
		t.Fatal(err)
	}

	expScope := Scope{Namespace: "payments/prod", Branch: "experiment"}
	expApplied, err := svc.Apply(ctx, ApplyInput{Scope: expScope, Documents: []Document{{Key: document.NormalizeKey(expFile), Content: "{\n  \"enabled\": true,\n  \"timeout\": 45\n}\n"}}, Message: "branch tweak"})
	if err != nil {
		t.Fatalf("branch apply: %v", err)
	}
	// Rev 2 is consumed by CreateBranch (copies source snapshot to new branch).
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
	content := got.Objects["../tmp/nope"]
	if content != "" {
		t.Fatalf("unexpected object lookup")
	}
	if got.Objects[document.NormalizeKey(appFile)] == "" {
		t.Fatalf("expected config object after restore")
	}
}

func TestRollbackCreatesNewRevision(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc := New(local.New(filepath.Join(dir, ".st8")))
	scope := Scope{Namespace: "platform-dev", Branch: "main"}

	file := filepath.Join(dir, "limits.json")
	if err := os.WriteFile(file, []byte("{\"qps\":100}"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Apply(ctx, ApplyInput{Scope: scope, Documents: []Document{{Key: document.NormalizeKey(file), Content: "{\n  \"qps\": 100\n}\n"}}})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}

	if err := os.WriteFile(file, []byte("{\"qps\":250}"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := svc.Apply(ctx, ApplyInput{Scope: scope, Documents: []Document{{Key: document.NormalizeKey(file), Content: "{\n  \"qps\": 250\n}\n"}}})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}

	rolled, err := svc.Rollback(ctx, scope, first.Revision, "", "")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolled.Revision <= second.Revision {
		t.Fatalf("rollback should create new revision, got %d after %d", rolled.Revision, second.Revision)
	}

	got, err := svc.Get(ctx, scope, 0, "")
	if err != nil {
		t.Fatalf("get after rollback: %v", err)
	}
	if got.Objects[document.NormalizeKey(file)] != "{\n  \"qps\": 100\n}\n" {
		t.Fatalf("rollback did not restore prior content: %q", got.Objects[document.NormalizeKey(file)])
	}
}
