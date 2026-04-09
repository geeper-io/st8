package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/geeper-io/st8/internal/api"
	"github.com/geeper-io/st8/internal/engine/local"
	t4kv "github.com/geeper-io/st8/internal/engine/t4kv"
	"github.com/geeper-io/st8/internal/logging"
	"github.com/geeper-io/st8/internal/service"
	"github.com/geeper-io/st8/internal/st8metrics"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	svc := service.New(local.New(filepath.Join(t.TempDir(), ".st8")))
	return NewHTTP(svc, st8metrics.New(prometheus.NewRegistry()), nil, nil)
}

func newT4Handler(t *testing.T) http.Handler {
	t.Helper()
	eng, err := t4kv.New(filepath.Join(t.TempDir(), ".st8d"), t4kv.Config{Logger: logging.Logger()})
	if err != nil {
		t.Fatalf("open t4kv engine: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	svc := service.New(eng)
	return NewHTTP(svc, st8metrics.New(prometheus.NewRegistry()), nil, nil)
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func doGet(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func decodeBody(t *testing.T, resp *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func mustOK(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

// ─── probes ──────────────────────────────────────────────────────────────────

func TestHealthz(t *testing.T) {
	rec := doGet(t, newHandler(t), "/healthz")
	mustOK(t, rec)
}

func TestReadyz(t *testing.T) {
	rec := doGet(t, newHandler(t), "/readyz")
	mustOK(t, rec)
}

// ─── Apply + Get ──────────────────────────────────────────────────────────────

func TestHTTPApplyAndGet(t *testing.T) {
	h := newHandler(t)
	scope := service.Scope{Namespace: "payments/prod", Branch: "main"}

	rec := doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{
		Scope:     scope,
		Documents: []service.Document{{Key: "cfg", Content: "v1"}},
		Message:   "initial",
	})
	mustOK(t, rec)
	var applied service.ApplyResult
	decodeBody(t, rec, &applied)
	if applied.Revision != 1 {
		t.Fatalf("revision = %d, want 1", applied.Revision)
	}
	if applied.Noop {
		t.Fatal("should not be noop")
	}

	rec = doGet(t, h, "/v1/state?namespace=payments/prod&branch=main")
	mustOK(t, rec)
	var got service.GetResult
	decodeBody(t, rec, &got)
	if got.Objects["cfg"] != "v1" {
		t.Fatalf("cfg = %q, want v1", got.Objects["cfg"])
	}
}

func TestHTTPApplyNoop(t *testing.T) {
	h := newHandler(t)
	scope := service.Scope{Namespace: "ns", Branch: "main"}
	doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{Scope: scope, Documents: []service.Document{{Key: "k", Content: "v"}}})

	rec := doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{Scope: scope, Documents: []service.Document{{Key: "k", Content: "v"}}})
	mustOK(t, rec)
	var res service.ApplyResult
	decodeBody(t, rec, &res)
	if !res.Noop {
		t.Fatal("expected noop")
	}
}

// ─── Diff ─────────────────────────────────────────────────────────────────────

func TestHTTPDiff(t *testing.T) {
	h := newHandler(t)
	scope := service.Scope{Namespace: "ns", Branch: "main"}
	doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{Scope: scope, Documents: []service.Document{{Key: "k", Content: "old"}}})

	rec := doJSON(t, h, http.MethodPost, "/v1/diff", api.DiffRequest{
		Scope:     scope,
		Documents: []service.Document{{Key: "k", Content: "new"}},
	})
	mustOK(t, rec)
	var diff service.DiffResult
	decodeBody(t, rec, &diff)
	if len(diff.Changes) != 1 || diff.Changes[0].Before != "old" || diff.Changes[0].After != "new" {
		t.Fatalf("unexpected diff: %+v", diff.Changes)
	}
}

// ─── Checkpoint + Rollback ────────────────────────────────────────────────────

func TestHTTPCheckpointAndRollback(t *testing.T) {
	h := newHandler(t)
	scope := service.Scope{Namespace: "ns", Branch: "main"}

	doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{Scope: scope, Documents: []service.Document{{Key: "k", Content: "good"}}})
	rec := doJSON(t, h, http.MethodPost, "/v1/checkpoints", api.CheckpointRequest{Scope: scope, Name: "stable"})
	mustOK(t, rec)

	doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{Scope: scope, Documents: []service.Document{{Key: "k", Content: "bad"}}})

	rec = doJSON(t, h, http.MethodPost, "/v1/rollback", api.RollbackRequest{Scope: scope, CheckpointName: "stable"})
	mustOK(t, rec)
	var rolled service.ApplyResult
	decodeBody(t, rec, &rolled)
	if rolled.Noop {
		t.Fatal("rollback should not be noop")
	}

	rec = doGet(t, h, "/v1/state?namespace=ns&branch=main")
	mustOK(t, rec)
	var got service.GetResult
	decodeBody(t, rec, &got)
	if got.Objects["k"] != "good" {
		t.Fatalf("k = %q after rollback, want good", got.Objects["k"])
	}
}

// ─── Log ──────────────────────────────────────────────────────────────────────

func TestHTTPLog(t *testing.T) {
	h := newHandler(t)
	scope := service.Scope{Namespace: "ns", Branch: "main"}
	for _, v := range []string{"v1", "v2", "v3"} {
		doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{Scope: scope, Documents: []service.Document{{Key: "k", Content: v}}, Message: v})
	}

	rec := doGet(t, h, "/v1/log?namespace=ns&branch=main&limit=10")
	mustOK(t, rec)
	var lr api.LogResponse
	decodeBody(t, rec, &lr)
	if len(lr.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(lr.Entries))
	}
	if lr.Entries[0].Message != "v3" {
		t.Fatalf("entries[0].Message = %q, want v3", lr.Entries[0].Message)
	}
}

// ─── Branches ─────────────────────────────────────────────────────────────────

func TestHTTPBranches(t *testing.T) {
	h := newHandler(t)
	scope := service.Scope{Namespace: "ns", Branch: "main"}
	doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{Scope: scope, Documents: []service.Document{{Key: "k", Content: "v"}}})

	rec := doJSON(t, h, http.MethodPost, "/v1/branches", api.BranchCreateRequest{Scope: scope, Name: "feature"})
	mustOK(t, rec)

	rec = doGet(t, h, "/v1/branches?namespace=ns&branch=main")
	mustOK(t, rec)
	var bl api.BranchListResponse
	decodeBody(t, rec, &bl)
	names := map[string]bool{}
	for _, b := range bl.Branches {
		names[b.Name] = true
	}
	if !names["main"] || !names["feature"] {
		t.Fatalf("branches = %v, want main and feature", names)
	}
}

// ─── Restore ─────────────────────────────────────────────────────────────────

func TestHTTPRestore(t *testing.T) {
	h := newHandler(t)
	mainScope := service.Scope{Namespace: "ns", Branch: "main"}
	doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{Scope: mainScope, Documents: []service.Document{{Key: "k", Content: "main"}}})
	doJSON(t, h, http.MethodPost, "/v1/branches", api.BranchCreateRequest{Scope: mainScope, Name: "fork"})

	forkScope := service.Scope{Namespace: "ns", Branch: "fork"}
	doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{Scope: forkScope, Documents: []service.Document{{Key: "k", Content: "fork-value"}}})

	rec := doJSON(t, h, http.MethodPost, "/v1/restore", api.RestoreRequest{
		Scope:      mainScope,
		FromBranch: "fork",
		Message:    "promote",
	})
	mustOK(t, rec)

	rec = doGet(t, h, "/v1/state?namespace=ns&branch=main")
	mustOK(t, rec)
	var got service.GetResult
	decodeBody(t, rec, &got)
	if got.Objects["k"] != "fork-value" {
		t.Fatalf("k = %q, want fork-value", got.Objects["k"])
	}
}

// ─── GC ───────────────────────────────────────────────────────────────────────

func TestHTTPGC(t *testing.T) {
	h := newHandler(t)
	scope := service.Scope{Namespace: "ns", Branch: "main"}
	// Need distinct values to avoid noops.
	for i := 0; i < 10; i++ {
		doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{
			Scope:     scope,
			Documents: []service.Document{{Key: "k", Content: string(rune('a' + i))}},
		})
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/gc", api.GCRequest{Keep: 3})
	mustOK(t, rec)
	var gcRes service.GCResult
	decodeBody(t, rec, &gcRes)
	if gcRes.Pruned == 0 {
		t.Fatal("expected pruned > 0")
	}
}

// ─── Method enforcement ───────────────────────────────────────────────────────

func TestHTTPMethodNotAllowed(t *testing.T) {
	h := newHandler(t)
	cases := []struct{ method, path string }{
		{http.MethodGet, "/v1/apply"},
		{http.MethodPost, "/v1/state"},
		{http.MethodPost, "/v1/log"},
		{http.MethodGet, "/v1/diff"},
		{http.MethodGet, "/v1/checkpoints"},
		{http.MethodGet, "/v1/rollback"},
		{http.MethodGet, "/v1/gc"},
		{http.MethodGet, "/v1/restore"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status %d, want 405", c.method, c.path, rec.Code)
		}
	}
}

// ─── t4kv engine round-trip ───────────────────────────────────────────────────

func TestHTTPServerRoundTrip(t *testing.T) {
	h := newT4Handler(t)
	scope := service.Scope{Namespace: "payments/prod", Branch: "main"}

	rec := doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{
		Scope:     scope,
		Documents: []service.Document{{Key: "config/app.json", Content: "{\n  \"enabled\": true\n}\n"}},
		Message:   "initial",
	})
	mustOK(t, rec)
	var applied service.ApplyResult
	decodeBody(t, rec, &applied)
	if applied.Revision != 1 {
		t.Fatalf("apply revision = %d, want 1", applied.Revision)
	}

	rec = doGet(t, h, "/v1/state?namespace=payments/prod&branch=main")
	mustOK(t, rec)
	var got service.GetResult
	decodeBody(t, rec, &got)
	if got.Revision != 1 {
		t.Fatalf("get revision = %d, want 1", got.Revision)
	}
	if got.Objects["config/app.json"] != "{\n  \"enabled\": true\n}\n" {
		t.Fatalf("unexpected object content: %q", got.Objects["config/app.json"])
	}

	rec = doGet(t, h, "/v1/log?namespace=payments/prod&branch=main&limit=10")
	mustOK(t, rec)
	var entries api.LogResponse
	decodeBody(t, rec, &entries)
	if len(entries.Entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries.Entries))
	}
}
