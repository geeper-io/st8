package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/geeper-io/st8/internal/api"
	"github.com/geeper-io/st8/internal/service"
	"github.com/geeper-io/st8/internal/st8metrics"
)

func NewHTTP(svc *service.Service, metrics *st8metrics.Collector, gatherer prometheus.Gatherer, logger *logrus.Logger) http.Handler {
	mux := http.NewServeMux()
	handle := func(route string, fn http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			start := time.Now()
			fn(recorder, r)
			latency := time.Since(start)
			metrics.ObserveHTTPRequest(route, r.Method, recorder.statusCode, latency)
			if logger != nil {
				logger.WithFields(logrus.Fields{
					"method":     r.Method,
					"path":       r.URL.Path,
					"status":     recorder.statusCode,
					"latency_ms": latency.Milliseconds(),
					"namespace":  r.URL.Query().Get("namespace"),
				}).Info("access")
			}
		}
	}

	mux.HandleFunc("/healthz", handle("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}))
	mux.HandleFunc("/readyz", handle("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Ready(r.Context()); err != nil {
			http.Error(w, "storage unavailable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}))
	if gatherer != nil {
		mux.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	}
	mux.HandleFunc("/v1/apply", handle("/v1/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.ApplyRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		start := time.Now()
		res, err := svc.Apply(r.Context(), service.ApplyInput{
			Scope:     req.Scope,
			Documents: req.Documents,
			Message:   req.Message,
		})
		result := "success"
		if err != nil {
			result = "error"
		} else if res.Noop {
			result = "noop"
		} else {
			metrics.AddChangedDocuments("apply", len(res.Changes))
		}
		metrics.ObserveOperation("apply", result, time.Since(start))
		writeResult(w, res, err)
	}))
	mux.HandleFunc("/v1/state", handle("/v1/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		scope := readScope(r)
		revision, _ := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		checkpoint := r.URL.Query().Get("checkpoint")
		start := time.Now()
		res, err := svc.Get(r.Context(), scope, revision, checkpoint)
		result := "success"
		if err != nil {
			result = "error"
		}
		metrics.ObserveOperation("get", result, time.Since(start))
		writeResult(w, res, err)
	}))
	mux.HandleFunc("/v1/diff", handle("/v1/diff", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.DiffRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		start := time.Now()
		res, err := svc.DiffDocuments(r.Context(), req.Scope, req.RevisionID, req.CheckpointName, req.Documents)
		result := "success"
		if err != nil {
			result = "error"
		} else {
			metrics.AddChangedDocuments("diff", len(res.Changes))
		}
		metrics.ObserveOperation("diff", result, time.Since(start))
		writeResult(w, res, err)
	}))
	mux.HandleFunc("/v1/checkpoints", handle("/v1/checkpoints", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req api.CheckpointRequest
			if !decodeJSON(w, r, &req) {
				return
			}
			start := time.Now()
			res, err := svc.Checkpoint(r.Context(), req.Scope, req.Name, req.Description)
			result := "success"
			if err != nil {
				result = "error"
			}
			metrics.ObserveOperation("checkpoint", result, time.Since(start))
			writeResult(w, res, err)
		case http.MethodDelete:
			var req api.DeleteCheckpointRequest
			if !decodeJSON(w, r, &req) {
				return
			}
			start := time.Now()
			err := svc.DeleteCheckpoint(r.Context(), req.Namespace, req.Name)
			result := "success"
			if err != nil {
				result = "error"
			}
			metrics.ObserveOperation("delete_checkpoint", result, time.Since(start))
			writeResult(w, struct{}{}, err)
		default:
			methodNotAllowed(w)
		}
	}))
	mux.HandleFunc("/v1/rollback", handle("/v1/rollback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.RollbackRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		start := time.Now()
		res, err := svc.Rollback(r.Context(), req.Scope, req.RevisionID, req.CheckpointName, req.Message)
		result := "success"
		if err != nil {
			result = "error"
		} else if res.Noop {
			result = "noop"
		} else {
			metrics.AddChangedDocuments("rollback", len(res.Changes))
		}
		metrics.ObserveOperation("rollback", result, time.Since(start))
		writeResult(w, res, err)
	}))
	mux.HandleFunc("/v1/log", handle("/v1/log", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		scope := readScope(r)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		start := time.Now()
		res, err := svc.Log(r.Context(), scope, limit)
		result := "success"
		if err != nil {
			result = "error"
		}
		metrics.ObserveOperation("log", result, time.Since(start))
		writeResult(w, api.LogResponse{Entries: res}, err)
	}))
	mux.HandleFunc("/v1/branches", handle("/v1/branches", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			start := time.Now()
			res, err := svc.ListBranches(r.Context(), readScope(r))
			result := "success"
			if err != nil {
				result = "error"
			}
			metrics.ObserveOperation("list_branches", result, time.Since(start))
			writeResult(w, api.BranchListResponse{Branches: res}, err)
		case http.MethodPost:
			var req api.BranchCreateRequest
			if !decodeJSON(w, r, &req) {
				return
			}
			start := time.Now()
			res, err := svc.CreateBranch(r.Context(), req.Scope, req.Name, req.FromRevision, req.FromCheckpoint)
			result := "success"
			if err != nil {
				result = "error"
			}
			metrics.ObserveOperation("create_branch", result, time.Since(start))
			writeResult(w, res, err)
		default:
			methodNotAllowed(w)
		}
	}))
	mux.HandleFunc("/v1/restore", handle("/v1/restore", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.RestoreRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		start := time.Now()
		res, err := svc.Restore(r.Context(), service.RestoreInput{
			Scope:          req.Scope,
			FromRevision:   req.FromRevision,
			FromCheckpoint: req.FromCheckpoint,
			FromBranch:     req.FromBranch,
			Message:        req.Message,
		})
		result := "success"
		if err != nil {
			result = "error"
		} else if res.Noop {
			result = "noop"
		} else {
			metrics.AddChangedDocuments("restore", len(res.Changes))
		}
		metrics.ObserveOperation("restore", result, time.Since(start))
		writeResult(w, res, err)
	}))
	mux.HandleFunc("/v1/gc", handle("/v1/gc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.GCRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		start := time.Now()
		res, err := svc.GC(r.Context(), req.Keep)
		result := "success"
		if err != nil {
			result = "error"
		}
		metrics.ObserveOperation("gc", result, time.Since(start))
		writeResult(w, res, err)
	}))
	return mux
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	defer r.Body.Close() //nolint:errcheck
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 20<<20)).Decode(out); err != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", err), http.StatusBadRequest)
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter, value any, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func readScope(r *http.Request) service.Scope {
	return service.Scope{
		Namespace: r.URL.Query().Get("namespace"),
		Branch:    r.URL.Query().Get("branch"),
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
