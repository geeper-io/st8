package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/geeper-io/st8/internal/api"
	"github.com/geeper-io/st8/internal/service"
)

type HTTP struct {
	baseURL string
	client  *http.Client
}

func NewHTTP(baseURL string) *HTTP {
	return &HTTP{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{},
	}
}

func (c *HTTP) Apply(ctx context.Context, input service.ApplyInput) (*service.ApplyResult, error) {
	var out service.ApplyResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/apply", api.ApplyRequest{
		Scope:     input.Scope,
		Documents: input.Documents,
		Message:   input.Message,
	}, &out)
	return &out, err
}

func (c *HTTP) Get(ctx context.Context, scope service.Scope, revision int64, checkpoint string) (*service.GetResult, error) {
	values := url.Values{}
	encodeScope(values, scope)
	if revision > 0 {
		values.Set("revision", strconv.FormatInt(revision, 10))
	}
	if checkpoint != "" {
		values.Set("checkpoint", checkpoint)
	}
	var out service.GetResult
	err := c.doJSON(ctx, http.MethodGet, "/v1/state?"+values.Encode(), nil, &out)
	return &out, err
}

func (c *HTTP) DiffDocuments(ctx context.Context, scope service.Scope, revision int64, checkpoint string, docs []service.Document) (*service.DiffResult, error) {
	var out service.DiffResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/diff", api.DiffRequest{
		Scope:          scope,
		RevisionID:     revision,
		CheckpointName: checkpoint,
		Documents:      docs,
	}, &out)
	return &out, err
}

func (c *HTTP) Checkpoint(ctx context.Context, scope service.Scope, name, description string) (*service.CheckpointResult, error) {
	var out service.CheckpointResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/checkpoints", api.CheckpointRequest{
		Scope:       scope,
		Name:        name,
		Description: description,
	}, &out)
	return &out, err
}

func (c *HTTP) Rollback(ctx context.Context, scope service.Scope, revision int64, checkpoint, message string) (*service.ApplyResult, error) {
	var out service.ApplyResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/rollback", api.RollbackRequest{
		Scope:          scope,
		RevisionID:     revision,
		CheckpointName: checkpoint,
		Message:        message,
	}, &out)
	return &out, err
}

func (c *HTTP) Log(ctx context.Context, scope service.Scope, limit int) ([]service.LogEntry, error) {
	values := url.Values{}
	encodeScope(values, scope)
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	var out api.LogResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/log?"+values.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

func (c *HTTP) CreateBranch(ctx context.Context, scope service.Scope, name string, revision int64, checkpoint string) (*service.BranchResult, error) {
	var out service.BranchResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/branches", api.BranchCreateRequest{
		Scope:          scope,
		Name:           name,
		FromRevision:   revision,
		FromCheckpoint: checkpoint,
	}, &out)
	return &out, err
}

func (c *HTTP) ListBranches(ctx context.Context, scope service.Scope) ([]service.BranchListEntry, error) {
	values := url.Values{}
	encodeScope(values, scope)
	var out api.BranchListResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/branches?"+values.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out.Branches, nil
}

func (c *HTTP) Restore(ctx context.Context, input service.RestoreInput) (*service.ApplyResult, error) {
	var out service.ApplyResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/restore", api.RestoreRequest{
		Scope:          input.Scope,
		FromRevision:   input.FromRevision,
		FromCheckpoint: input.FromCheckpoint,
		FromBranch:     input.FromBranch,
		Message:        input.Message,
	}, &out)
	return &out, err
}

func (c *HTTP) doJSON(ctx context.Context, method, path string, reqBody any, out any) error {
	var body io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(msg) == 0 {
			return fmt.Errorf("request failed with status %s", resp.Status)
		}
		return fmt.Errorf("request failed with status %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func encodeScope(values url.Values, scope service.Scope) {
	values.Set("workspace", scope.Workspace)
	values.Set("env", scope.Environment)
	values.Set("branch", scope.Branch)
}
