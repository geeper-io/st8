package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/geeper-io/st8/internal/api"
	"github.com/geeper-io/st8/internal/service"
)

func NewHTTP(svc *service.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/v1/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.ApplyRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		res, err := svc.Apply(r.Context(), service.ApplyInput{
			Scope:     req.Scope,
			Documents: req.Documents,
			Message:   req.Message,
		})
		writeResult(w, res, err)
	})
	mux.HandleFunc("/v1/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		scope := readScope(r)
		revision, _ := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		checkpoint := r.URL.Query().Get("checkpoint")
		res, err := svc.Get(r.Context(), scope, revision, checkpoint)
		writeResult(w, res, err)
	})
	mux.HandleFunc("/v1/diff", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.DiffRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		res, err := svc.DiffDocuments(r.Context(), req.Scope, req.RevisionID, req.CheckpointName, req.Documents)
		writeResult(w, res, err)
	})
	mux.HandleFunc("/v1/checkpoints", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.CheckpointRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		res, err := svc.Checkpoint(r.Context(), req.Scope, req.Name, req.Description)
		writeResult(w, res, err)
	})
	mux.HandleFunc("/v1/rollback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.RollbackRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		res, err := svc.Rollback(r.Context(), req.Scope, req.RevisionID, req.CheckpointName, req.Message)
		writeResult(w, res, err)
	})
	mux.HandleFunc("/v1/log", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		scope := readScope(r)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		res, err := svc.Log(r.Context(), scope, limit)
		writeResult(w, api.LogResponse{Entries: res}, err)
	})
	mux.HandleFunc("/v1/branches", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			res, err := svc.ListBranches(r.Context(), readScope(r))
			writeResult(w, api.BranchListResponse{Branches: res}, err)
		case http.MethodPost:
			var req api.BranchCreateRequest
			if !decodeJSON(w, r, &req) {
				return
			}
			res, err := svc.CreateBranch(r.Context(), req.Scope, req.Name, req.FromRevision, req.FromCheckpoint)
			writeResult(w, res, err)
		default:
			methodNotAllowed(w)
		}
	})
	mux.HandleFunc("/v1/restore", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req api.RestoreRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		res, err := svc.Restore(r.Context(), service.RestoreInput{
			Scope:          req.Scope,
			FromRevision:   req.FromRevision,
			FromCheckpoint: req.FromCheckpoint,
			FromBranch:     req.FromBranch,
			Message:        req.Message,
		})
		writeResult(w, res, err)
	})
	return mux
}

func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	defer r.Body.Close()
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
		Workspace:   r.URL.Query().Get("workspace"),
		Environment: r.URL.Query().Get("env"),
		Branch:      r.URL.Query().Get("branch"),
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
