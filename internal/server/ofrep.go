package server

// OFREP (OpenFeature Remote Evaluation Protocol) handler for st8d.
//
// st8 acts as a flag management system: each document key is a flag key and
// its JSON content determines the typed flag value. The namespace and branch
// are selected via the usual ?namespace=&branch= query parameters, so any
// OpenFeature OFREP provider can target a specific scope.
//
// Spec: https://github.com/open-feature/protocol

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/geeper-io/st8/internal/service"
)

// ─── OFREP wire types ────────────────────────────────────────────────────────

type ofrepEvalRequest struct {
	Context map[string]any `json:"context"`
}

type ofrepEvalSuccess struct {
	Key      string         `json:"key"`
	Value    any            `json:"value"`
	Reason   string         `json:"reason"`
	Variant  string         `json:"variant,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ofrepEvalFailure struct {
	Key          string `json:"key"`
	ErrorCode    string `json:"errorCode"`
	ErrorDetails string `json:"errorDetails,omitempty"`
}

type ofrepBulkSuccess struct {
	Flags    []any          `json:"flags"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ofrepBulkFailure struct {
	ErrorCode    string `json:"errorCode"`
	ErrorDetails string `json:"errorDetails,omitempty"`
}

type ofrepGeneralError struct {
	ErrorDetails string `json:"errorDetails"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// ofrepSingleFlagHandler handles POST /ofrep/v1/evaluate/flags/{key...}.
//
// Server-side (dynamic context) evaluation: evaluates one flag per request.
// The flag key is the st8 document key. Namespace and branch are read from
// query parameters (?namespace=&branch=) and default to "default"/"main".
func ofrepSingleFlagHandler(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}

		key := r.PathValue("key")
		if key == "" {
			writeOFREPError(w, http.StatusBadRequest, ofrepEvalFailure{
				Key:          "",
				ErrorCode:    "PARSE_ERROR",
				ErrorDetails: "flag key is required",
			})
			return
		}

		var req ofrepEvalRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		scope := readScope(r)
		result, err := svc.Get(r.Context(), scope, 0, "")
		if err != nil {
			writeOFREPGeneral(w, http.StatusInternalServerError, err.Error())
			return
		}

		content, ok := result.Objects[key]
		if !ok {
			writeOFREPError(w, http.StatusNotFound, ofrepEvalFailure{
				Key:          key,
				ErrorCode:    "FLAG_NOT_FOUND",
				ErrorDetails: fmt.Sprintf("flag %q not found", key),
			})
			return
		}

		resp := ofrepEvalSuccess{
			Key:     key,
			Value:   parseDocumentValue(content),
			Reason:  "STATIC",
			Variant: revVariant(result.Revision),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// ofrepBulkFlagsHandler handles POST /ofrep/v1/evaluate/flags.
//
// Client-side (static context) evaluation: evaluates all flags at once and
// returns them with an ETag header. If the client sends If-None-Match with a
// matching ETag, 304 Not Modified is returned, indicating nothing changed.
func ofrepBulkFlagsHandler(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}

		var req ofrepEvalRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		scope := readScope(r)
		result, err := svc.Get(r.Context(), scope, 0, "")
		if err != nil {
			writeOFREPGeneral(w, http.StatusInternalServerError, err.Error())
			return
		}

		etag := fmt.Sprintf(`"rev-%d"`, result.Revision)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		variant := revVariant(result.Revision)
		flags := make([]any, 0, len(result.Objects))
		for key, content := range result.Objects {
			flags = append(flags, ofrepEvalSuccess{
				Key:     key,
				Value:   parseDocumentValue(content),
				Reason:  "STATIC",
				Variant: variant,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", etag)
		_ = json.NewEncoder(w).Encode(ofrepBulkSuccess{
			Flags:    flags,
			Metadata: map[string]any{"revision": result.Revision},
		})
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// parseDocumentValue JSON-parses a st8 document's raw string content and
// returns a Go value with an appropriate type for OFREP flag evaluation:
//   - JSON boolean  → bool
//   - JSON integer  → int64
//   - JSON float    → float64
//   - JSON string   → string
//   - JSON object/array → map[string]any / []any
//
// If the content is not valid JSON it is returned as a plain string.
func parseDocumentValue(content string) any {
	var v any
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return content
	}
	if f, ok := v.(float64); ok {
		if f == math.Trunc(f) && !math.IsInf(f, 0) {
			return int64(f)
		}
	}
	return v
}

func revVariant(revision int64) string {
	return fmt.Sprintf("rev-%d", revision)
}

func writeOFREPError(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeOFREPGeneral(w http.ResponseWriter, status int, details string) {
	writeOFREPError(w, status, ofrepGeneralError{ErrorDetails: details})
}
