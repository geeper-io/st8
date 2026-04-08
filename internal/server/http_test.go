package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/geeper-io/st8/internal/api"
	"github.com/geeper-io/st8/internal/engine/t4kv"
	"github.com/geeper-io/st8/internal/logging"
	"github.com/geeper-io/st8/internal/service"
	"github.com/geeper-io/st8/internal/st8metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestHTTPServerRoundTrip(t *testing.T) {
	svc := service.New(t4kv.New(filepath.Join(t.TempDir(), ".st8d"), t4kv.Config{
		Logger: logging.Logger(),
	}))
	handler := NewHTTP(svc, st8metrics.New(prometheus.NewRegistry()))
	scope := service.Scope{Workspace: "payments", Environment: "prod", Branch: "main"}

	applyResp := performJSON(t, handler, http.MethodPost, "/v1/apply", api.ApplyRequest{
		Scope: scope,
		Documents: []service.Document{{
			Key:     "config/app.json",
			Content: "{\n  \"enabled\": true\n}\n",
		}},
		Message: "initial",
	})
	var applied service.ApplyResult
	decodeBody(t, applyResp, &applied)
	if applied.Revision != 1 {
		t.Fatalf("apply revision = %d, want 1", applied.Revision)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/state?workspace=payments&env=prod&branch=main", nil)
	getResp := httptest.NewRecorder()
	handler.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200: %s", getResp.Code, getResp.Body.String())
	}
	var got service.GetResult
	decodeBody(t, getResp, &got)
	if got.Revision != 1 {
		t.Fatalf("get revision = %d, want 1", got.Revision)
	}
	if got.Objects["config/app.json"] != "{\n  \"enabled\": true\n}\n" {
		t.Fatalf("unexpected object content: %q", got.Objects["config/app.json"])
	}

	logReq := httptest.NewRequest(http.MethodGet, "/v1/log?workspace=payments&env=prod&branch=main&limit=10", nil)
	logResp := httptest.NewRecorder()
	handler.ServeHTTP(logResp, logReq)
	if logResp.Code != http.StatusOK {
		t.Fatalf("log status = %d, want 200: %s", logResp.Code, logResp.Body.String())
	}
	var entries api.LogResponse
	decodeBody(t, logResp, &entries)
	if len(entries.Entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries.Entries))
	}
}

func performJSON(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d, want 200: %s", method, path, resp.Code, resp.Body.String())
	}
	return resp
}

func decodeBody(t *testing.T, resp *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
