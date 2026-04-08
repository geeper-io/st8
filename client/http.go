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
	"time"
)

// HTTP is a Client that communicates with an st8d server over HTTP.
type HTTP struct {
	baseURL string
	token   string
	http    *http.Client
}

// Option configures an HTTP client.
type Option func(*HTTP)

// WithToken sets a bearer token sent as "Authorization: Bearer <token>" on
// every request. The server must be configured with the same token.
func WithToken(token string) Option {
	return func(h *HTTP) { h.token = token }
}

// WithTimeout sets a timeout on the underlying HTTP client. Requests that
// exceed the timeout are cancelled and return an error.
func WithTimeout(d time.Duration) Option {
	return func(h *HTTP) { h.http.Timeout = d }
}

// NewHTTP creates an HTTP client for the given st8d base URL.
func NewHTTP(baseURL string, opts ...Option) *HTTP {
	h := &HTTP{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Wire types for request bodies — private, match the server's JSON format.
type applyRequest struct {
	Scope     Scope      `json:"scope"`
	Documents []Document `json:"documents"`
	Message   string     `json:"message"`
}

type diffRequest struct {
	Scope          Scope      `json:"scope"`
	RevisionID     int64      `json:"revision_id"`
	CheckpointName string     `json:"checkpoint_name"`
	Documents      []Document `json:"documents"`
}

type checkpointRequest struct {
	Scope       Scope  `json:"scope"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type rollbackRequest struct {
	Scope          Scope  `json:"scope"`
	RevisionID     int64  `json:"revision_id"`
	CheckpointName string `json:"checkpoint_name"`
	Message        string `json:"message"`
}

type branchCreateRequest struct {
	Scope          Scope  `json:"scope"`
	Name           string `json:"name"`
	FromRevision   int64  `json:"from_revision"`
	FromCheckpoint string `json:"from_checkpoint"`
}

type restoreRequest struct {
	Scope          Scope  `json:"scope"`
	FromRevision   int64  `json:"from_revision"`
	FromCheckpoint string `json:"from_checkpoint"`
	FromBranch     string `json:"from_branch"`
	Message        string `json:"message"`
}

type gcRequest struct {
	Keep int `json:"keep"`
}

type logResponse struct {
	Entries []LogEntry `json:"entries"`
}

type branchListResponse struct {
	Branches []BranchListEntry `json:"branches"`
}

func (c *HTTP) Apply(ctx context.Context, input ApplyInput) (*ApplyResult, error) {
	var out ApplyResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/apply", applyRequest{
		Scope:     input.Scope,
		Documents: input.Documents,
		Message:   input.Message,
	}, &out)
	return &out, err
}

func (c *HTTP) Get(ctx context.Context, scope Scope, revision int64, checkpoint string) (*GetResult, error) {
	q := c.scopeQuery(scope)
	if revision > 0 {
		q.Set("revision", strconv.FormatInt(revision, 10))
	}
	if checkpoint != "" {
		q.Set("checkpoint", checkpoint)
	}
	var out GetResult
	err := c.doJSON(ctx, http.MethodGet, "/v1/state?"+q.Encode(), nil, &out)
	return &out, err
}

func (c *HTTP) DiffDocuments(ctx context.Context, scope Scope, revision int64, checkpoint string, docs []Document) (*DiffResult, error) {
	var out DiffResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/diff", diffRequest{
		Scope:          scope,
		RevisionID:     revision,
		CheckpointName: checkpoint,
		Documents:      docs,
	}, &out)
	return &out, err
}

func (c *HTTP) Checkpoint(ctx context.Context, scope Scope, name, description string) (*CheckpointResult, error) {
	var out CheckpointResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/checkpoints", checkpointRequest{
		Scope:       scope,
		Name:        name,
		Description: description,
	}, &out)
	return &out, err
}

func (c *HTTP) Rollback(ctx context.Context, scope Scope, revision int64, checkpoint, message string) (*ApplyResult, error) {
	var out ApplyResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/rollback", rollbackRequest{
		Scope:          scope,
		RevisionID:     revision,
		CheckpointName: checkpoint,
		Message:        message,
	}, &out)
	return &out, err
}

func (c *HTTP) Log(ctx context.Context, scope Scope, limit int) ([]LogEntry, error) {
	q := c.scopeQuery(scope)
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out logResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/log?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

func (c *HTTP) CreateBranch(ctx context.Context, scope Scope, name string, revision int64, checkpoint string) (*BranchResult, error) {
	var out BranchResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/branches", branchCreateRequest{
		Scope:          scope,
		Name:           name,
		FromRevision:   revision,
		FromCheckpoint: checkpoint,
	}, &out)
	return &out, err
}

func (c *HTTP) ListBranches(ctx context.Context, scope Scope) ([]BranchListEntry, error) {
	q := c.scopeQuery(scope)
	var out branchListResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/branches?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out.Branches, nil
}

func (c *HTTP) Restore(ctx context.Context, input RestoreInput) (*ApplyResult, error) {
	var out ApplyResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/restore", restoreRequest{
		Scope:          input.Scope,
		FromRevision:   input.FromRevision,
		FromCheckpoint: input.FromCheckpoint,
		FromBranch:     input.FromBranch,
		Message:        input.Message,
	}, &out)
	return &out, err
}

func (c *HTTP) GC(ctx context.Context, keep int) (*GCResult, error) {
	var out GCResult
	err := c.doJSON(ctx, http.MethodPost, "/v1/gc", gcRequest{Keep: keep}, &out)
	return &out, err
}

func (c *HTTP) scopeQuery(scope Scope) url.Values {
	q := url.Values{}
	q.Set("namespace", scope.Namespace)
	q.Set("branch", scope.Branch)
	return q
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
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
