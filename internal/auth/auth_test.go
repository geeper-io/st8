package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/geeper-io/st8/internal/auth"
)

// ─── Config / YAML parsing ───────────────────────────────────────────────────

func TestLoadConfig(t *testing.T) {
	yaml := `
tokens:
  - token: "abc"
    name: "app"
    allow:
      namespaces: ["prod"]
      branches: ["main", "release/*"]
      key_prefix: "app/"
      verbs: ["read", "write"]
  - token: "xyz"
    name: "admin"
    allow:
      namespaces: ["*"]
      branches: ["*"]
      key_prefix: ""
      verbs: ["read", "write", "admin"]
`
	path := writeTempYAML(t, yaml)
	cfg, err := auth.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(cfg.Tokens))
	}
	app := cfg.Tokens[0]
	if app.Name != "app" {
		t.Errorf("token[0].Name = %q, want %q", app.Name, "app")
	}
	if app.Allow.KeyPrefix != "app/" {
		t.Errorf("token[0].Allow.KeyPrefix = %q, want %q", app.Allow.KeyPrefix, "app/")
	}
}

func TestLoadConfig_Missing(t *testing.T) {
	_, err := auth.Load("/no/such/file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestAdminConfig(t *testing.T) {
	cfg := auth.AdminConfig("secret")
	if len(cfg.Tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(cfg.Tokens))
	}
	p := &auth.Principal{Name: cfg.Tokens[0].Name, Allow: cfg.Tokens[0].Allow}
	for _, verb := range []string{auth.VerbRead, auth.VerbWrite, auth.VerbAdmin} {
		if !p.AllowsVerb(verb) {
			t.Errorf("admin should allow verb %q", verb)
		}
	}
	if !p.AllowsNamespace("anything") {
		t.Error("admin should allow any namespace")
	}
	if !p.AllowsBranch("anything") {
		t.Error("admin should allow any branch")
	}
	if !p.AllowsKey("any/key") {
		t.Error("admin should allow any key")
	}
}

// ─── Principal policy checks ─────────────────────────────────────────────────

func TestPrincipal_AllowsVerb(t *testing.T) {
	p := &auth.Principal{Allow: auth.Policy{Verbs: []string{auth.VerbRead, auth.VerbWrite}}}
	if !p.AllowsVerb(auth.VerbRead) {
		t.Error("should allow read")
	}
	if !p.AllowsVerb(auth.VerbWrite) {
		t.Error("should allow write")
	}
	if p.AllowsVerb(auth.VerbAdmin) {
		t.Error("should not allow admin")
	}
}

func TestPrincipal_AllowsNamespace_Glob(t *testing.T) {
	p := &auth.Principal{Allow: auth.Policy{Namespaces: []string{"prod", "staging-*"}}}
	cases := []struct {
		ns   string
		want bool
	}{
		{"prod", true},
		{"staging-eu", true},
		{"staging-us", true},
		{"dev", false},
		{"production", false},
	}
	for _, c := range cases {
		if got := p.AllowsNamespace(c.ns); got != c.want {
			t.Errorf("AllowsNamespace(%q) = %v, want %v", c.ns, got, c.want)
		}
	}
}

func TestPrincipal_AllowsBranch_Glob(t *testing.T) {
	p := &auth.Principal{Allow: auth.Policy{Branches: []string{"main", "release/*"}}}
	cases := []struct {
		branch string
		want   bool
	}{
		{"main", true},
		{"release/1.0", true},
		{"release/2.3.1", true},
		{"feature/foo", false},
		{"dev", false},
	}
	for _, c := range cases {
		if got := p.AllowsBranch(c.branch); got != c.want {
			t.Errorf("AllowsBranch(%q) = %v, want %v", c.branch, got, c.want)
		}
	}
}

func TestPrincipal_AllowsKey(t *testing.T) {
	p := &auth.Principal{Allow: auth.Policy{KeyPrefix: "app/"}}
	if !p.AllowsKey("app/config") {
		t.Error("should allow app/config")
	}
	if !p.AllowsKey("app/nested/key") {
		t.Error("should allow app/nested/key")
	}
	if p.AllowsKey("other/key") {
		t.Error("should not allow other/key")
	}
	if p.AllowsKey("APP/config") {
		t.Error("key prefix is case-sensitive")
	}
}

func TestPrincipal_AllowsKey_EmptyPrefix(t *testing.T) {
	p := &auth.Principal{Allow: auth.Policy{KeyPrefix: ""}}
	if !p.AllowsKey("anything") {
		t.Error("empty prefix should allow any key")
	}
}

func TestPrincipal_AllowsNamespace_Wildcard(t *testing.T) {
	p := &auth.Principal{Allow: auth.Policy{Namespaces: []string{"*"}}}
	if !p.AllowsNamespace("any-ns") {
		t.Error("wildcard should match any namespace")
	}
}

// ─── Context helpers ─────────────────────────────────────────────────────────

func TestContextWithPrincipal(t *testing.T) {
	p := &auth.Principal{Name: "test"}
	ctx := auth.ContextWithPrincipal(context.Background(), p)
	got, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("PrincipalFromContext: not ok")
	}
	if got.Name != "test" {
		t.Errorf("got name %q, want %q", got.Name, "test")
	}
}

func TestPrincipalFromContext_Missing(t *testing.T) {
	_, ok := auth.PrincipalFromContext(context.Background())
	if ok {
		t.Fatal("expected no principal in empty context")
	}
}

// ─── Middleware ───────────────────────────────────────────────────────────────

func newTestHandler(t *testing.T, cfg *auth.Config) (http.Handler, *captureHandler) {
	t.Helper()
	inner := &captureHandler{}
	return auth.Middleware(cfg, inner), inner
}

// captureHandler records the request principal (if any) and always returns 200.
type captureHandler struct {
	principal *auth.Principal
	called    bool
}

func (h *captureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	h.principal, _ = auth.PrincipalFromContext(r.Context())
	w.WriteHeader(http.StatusOK)
}

func TestMiddleware_NoToken(t *testing.T) {
	cfg := auth.AdminConfig("secret")
	h, _ := newTestHandler(t, cfg)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestMiddleware_WrongToken(t *testing.T) {
	cfg := auth.AdminConfig("secret")
	h, _ := newTestHandler(t, cfg)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestMiddleware_ValidToken_SetsContext(t *testing.T) {
	cfg := auth.AdminConfig("secret")
	h, inner := newTestHandler(t, cfg)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	if !inner.called {
		t.Fatal("inner handler was not called")
	}
	if inner.principal == nil {
		t.Fatal("principal not set in context")
	}
	if inner.principal.Name != "admin" {
		t.Errorf("principal name = %q, want %q", inner.principal.Name, "admin")
	}
}

func TestMiddleware_VerbForbidden(t *testing.T) {
	cfg := &auth.Config{
		Tokens: []auth.TokenEntry{{
			Token: "reader",
			Name:  "reader",
			Allow: auth.Policy{
				Namespaces: []string{"*"},
				Branches:   []string{"*"},
				Verbs:      []string{auth.VerbRead},
			},
		}},
	}
	h, _ := newTestHandler(t, cfg)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/apply", nil)
	req.Header.Set("Authorization", "Bearer reader")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

func TestMiddleware_UnauthenticatedPaths(t *testing.T) {
	cfg := auth.AdminConfig("secret")
	h, inner := newTestHandler(t, cfg)
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		inner.called = false
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("path %s: want 200, got %d", path, rec.Code)
		}
		if !inner.called {
			t.Errorf("path %s: inner handler not called", path)
		}
	}
}

func TestMiddleware_GCRequiresAdmin(t *testing.T) {
	cfg := &auth.Config{
		Tokens: []auth.TokenEntry{{
			Token: "writer",
			Name:  "writer",
			Allow: auth.Policy{
				Namespaces: []string{"*"},
				Branches:   []string{"*"},
				Verbs:      []string{auth.VerbRead, auth.VerbWrite},
			},
		}},
	}
	h, _ := newTestHandler(t, cfg)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/gc", nil)
	req.Header.Set("Authorization", "Bearer writer")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

func TestMiddleware_MultipleTokens(t *testing.T) {
	cfg := &auth.Config{
		Tokens: []auth.TokenEntry{
			{Token: "reader", Name: "reader", Allow: auth.Policy{Namespaces: []string{"*"}, Branches: []string{"*"}, Verbs: []string{auth.VerbRead}}},
			{Token: "writer", Name: "writer", Allow: auth.Policy{Namespaces: []string{"*"}, Branches: []string{"*"}, Verbs: []string{auth.VerbRead, auth.VerbWrite}}},
		},
	}
	h, inner := newTestHandler(t, cfg)

	// writer can POST /v1/apply
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/apply", nil)
	req.Header.Set("Authorization", "Bearer writer")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("writer on apply: want 200, got %d", rec.Code)
	}
	if inner.principal.Name != "writer" {
		t.Errorf("principal name = %q, want writer", inner.principal.Name)
	}

	// reader cannot POST /v1/apply
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/apply", nil)
	req.Header.Set("Authorization", "Bearer reader")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("reader on apply: want 403, got %d", rec.Code)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}
	return path
}
