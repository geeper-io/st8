package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/geeper-io/st8/internal/api"
	"github.com/geeper-io/st8/internal/service"
)

// ─── OFREP helpers ───────────────────────────────────────────────────────────

func applyOFREP(t *testing.T, h http.Handler, ns, branch string, docs []service.Document) {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/apply", api.ApplyRequest{
		Scope:     service.Scope{Namespace: ns, Branch: branch},
		Documents: docs,
	})
	mustOK(t, rec)
}

func ofrepBulkURL(ns, branch string) string {
	return "/ofrep/v1/evaluate/flags?namespace=" + ns + "&branch=" + branch
}

func ofrepSingleURL(key, ns, branch string) string {
	return "/ofrep/v1/evaluate/flags/" + key + "?namespace=" + ns + "&branch=" + branch
}

func newJSONRequest(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func doRequestRaw(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ─── Bulk evaluation ─────────────────────────────────────────────────────────

func TestOFREPBulkEmpty(t *testing.T) {
	h := newHandler(t)
	rec := doJSON(t, h, http.MethodPost, ofrepBulkURL("ns", "main"),
		ofrepEvalRequest{Context: map[string]any{"targetingKey": "u1"}})
	mustOK(t, rec)

	var resp ofrepBulkSuccess
	decodeBody(t, rec, &resp)
	if resp.Flags == nil {
		t.Fatal("flags must not be nil")
	}
	if len(resp.Flags) != 0 {
		t.Fatalf("expected 0 flags, got %d", len(resp.Flags))
	}
}

func TestOFREPBulkTypes(t *testing.T) {
	h := newHandler(t)
	applyOFREP(t, h, "ns", "main", []service.Document{
		{Key: "bool-flag", Content: "true"},
		{Key: "int-flag", Content: "42"},
		{Key: "float-flag", Content: "3.14"},
		{Key: "str-flag", Content: `"hello"`},
		{Key: "obj-flag", Content: `{"x":1}`},
		{Key: "raw-flag", Content: "not-json"},
	})

	rec := doJSON(t, h, http.MethodPost, ofrepBulkURL("ns", "main"),
		ofrepEvalRequest{Context: map[string]any{"targetingKey": "u1"}})
	mustOK(t, rec)

	if rec.Header().Get("ETag") == "" {
		t.Fatal("ETag header must be set")
	}

	var bulk ofrepBulkSuccess
	decodeBody(t, rec, &bulk)
	if len(bulk.Flags) != 6 {
		t.Fatalf("expected 6 flags, got %d", len(bulk.Flags))
	}

	byKey := map[string]map[string]any{}
	for _, raw := range bulk.Flags {
		b, _ := json.Marshal(raw)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		byKey[m["key"].(string)] = m
	}

	if v := byKey["bool-flag"]["value"]; v != true {
		t.Errorf("bool-flag: got %v (%T), want true", v, v)
	}
	if v := byKey["int-flag"]["value"]; v != float64(42) { // JSON numbers are float64 after decode
		t.Errorf("int-flag: got %v (%T), want 42", v, v)
	}
	if v := byKey["float-flag"]["value"]; v != 3.14 {
		t.Errorf("float-flag: got %v, want 3.14", v)
	}
	if v := byKey["str-flag"]["value"]; v != "hello" {
		t.Errorf("str-flag: got %v, want hello", v)
	}
	if _, ok := byKey["obj-flag"]["value"].(map[string]any); !ok {
		t.Errorf("obj-flag: expected object value")
	}
	if v := byKey["raw-flag"]["value"]; v != "not-json" {
		t.Errorf("raw-flag: got %v, want not-json", v)
	}

	for key, flag := range byKey {
		if flag["reason"] != "STATIC" {
			t.Errorf("%s: reason = %v, want STATIC", key, flag["reason"])
		}
		if flag["variant"] == "" {
			t.Errorf("%s: variant must not be empty", key)
		}
	}
}

func TestOFREPBulkETagNotModified(t *testing.T) {
	h := newHandler(t)
	applyOFREP(t, h, "ns", "main", []service.Document{{Key: "k", Content: `"v"`}})

	// first request to get ETag
	rec := doJSON(t, h, http.MethodPost, ofrepBulkURL("ns", "main"),
		ofrepEvalRequest{Context: map[string]any{"targetingKey": "u1"}})
	mustOK(t, rec)
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on first response")
	}

	// second request with matching ETag → 304
	data, _ := json.Marshal(ofrepEvalRequest{Context: map[string]any{"targetingKey": "u1"}})
	req := newJSONRequest(t, http.MethodPost, ofrepBulkURL("ns", "main"), data)
	req.Header.Set("If-None-Match", etag)
	rec2 := doRequestRaw(h, req)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", rec2.Code)
	}
}

func TestOFREPBulkMethodNotAllowed(t *testing.T) {
	h := newHandler(t)
	rec := doGet(t, h, ofrepBulkURL("ns", "main"))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// ─── Single flag evaluation ───────────────────────────────────────────────────

func TestOFREPSingleFlag(t *testing.T) {
	h := newHandler(t)
	applyOFREP(t, h, "ns", "main", []service.Document{
		{Key: "feature-x", Content: "true"},
	})

	rec := doJSON(t, h, http.MethodPost,
		ofrepSingleURL("feature-x", "ns", "main"),
		ofrepEvalRequest{Context: map[string]any{"targetingKey": "u1"}})
	mustOK(t, rec)

	var resp ofrepEvalSuccess
	decodeBody(t, rec, &resp)
	if resp.Key != "feature-x" {
		t.Errorf("key = %q, want feature-x", resp.Key)
	}
	if resp.Value != true {
		t.Errorf("value = %v, want true", resp.Value)
	}
	if resp.Reason != "STATIC" {
		t.Errorf("reason = %q, want STATIC", resp.Reason)
	}
	if resp.Variant == "" {
		t.Error("variant must not be empty")
	}
}

func TestOFREPSingleFlagNotFound(t *testing.T) {
	h := newHandler(t)
	rec := doJSON(t, h, http.MethodPost,
		ofrepSingleURL("missing", "ns", "main"),
		ofrepEvalRequest{Context: map[string]any{"targetingKey": "u1"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var resp ofrepEvalFailure
	decodeBody(t, rec, &resp)
	if resp.ErrorCode != "FLAG_NOT_FOUND" {
		t.Errorf("errorCode = %q, want FLAG_NOT_FOUND", resp.ErrorCode)
	}
}

func TestOFREPSingleFlagPathWithSlashes(t *testing.T) {
	h := newHandler(t)
	applyOFREP(t, h, "ns", "main", []service.Document{
		{Key: "payments/config.json", Content: `{"timeout":30}`},
	})

	rec := doJSON(t, h, http.MethodPost,
		ofrepSingleURL("payments/config.json", "ns", "main"),
		ofrepEvalRequest{Context: map[string]any{"targetingKey": "u1"}})
	mustOK(t, rec)

	var resp ofrepEvalSuccess
	decodeBody(t, rec, &resp)
	if resp.Key != "payments/config.json" {
		t.Errorf("key = %q, want payments/config.json", resp.Key)
	}
}

func TestOFREPSingleFlagMethodNotAllowed(t *testing.T) {
	h := newHandler(t)
	rec := doGet(t, h, ofrepSingleURL("k", "ns", "main"))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// ─── parseDocumentValue unit tests ───────────────────────────────────────────

func TestParseDocumentValue(t *testing.T) {
	tests := []struct {
		content string
		want    any
	}{
		{"true", true},
		{"false", false},
		{"42", int64(42)},
		{"-7", int64(-7)},
		{"0", int64(0)},
		{"3.14", 3.14},
		{`"hello"`, "hello"},
		{`{"a":1}`, map[string]any{"a": float64(1)}},
		{"not-json", "not-json"},
		{"", ""},
	}
	for _, tc := range tests {
		got := parseDocumentValue(tc.content)
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(tc.want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("parseDocumentValue(%q) = %v (%T), want %v (%T)",
				tc.content, got, got, tc.want, tc.want)
		}
	}
}
