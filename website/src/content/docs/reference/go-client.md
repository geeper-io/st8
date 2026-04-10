---
title: Go Client Reference
description: API reference for the github.com/geeper-io/st8/client package.
---

The `client` package provides a typed Go client for st8d.

## Installation

```bash
go get github.com/geeper-io/st8/client
```

## Creating a client

```go
import st8 "github.com/geeper-io/st8/client"

// Basic client
c := st8.NewHTTP("https://st8.internal")

// With authentication
c := st8.NewHTTP("https://st8.internal",
    st8.WithToken("your-token"),
)

// With request timeout
c := st8.NewHTTP("https://st8.internal",
    st8.WithToken("your-token"),
    st8.WithTimeout(5*time.Second),
)
```

## Types

### Scope

Every operation targets a scope: a namespace + branch combination.

```go
type Scope struct {
    Namespace string `json:"namespace"`
    Branch    string `json:"branch"`
}
```

### Document

A named blob of content. Value is an arbitrary string (typically JSON).

```go
type Document struct {
    Key   string `json:"key"`
    Value string `json:"value"`
}
```

### ApplyResult

```go
type ApplyResult struct {
    RevisionID int64    `json:"revision_id"`
    Noop       bool     `json:"noop"`     // true if nothing changed
    Changes    []Change `json:"changes"`
}

type Change struct {
    Key  string `json:"key"`
    Old  string `json:"old"`   // empty for new keys
    New  string `json:"new"`   // empty for deleted keys
}
```

### GetResult

```go
type GetResult struct {
    RevisionID int64             `json:"revision_id"`
    Objects    map[string]string `json:"objects"`
}
```

### LogEntry

```go
type LogEntry struct {
    RevisionID int64     `json:"revision_id"`
    Message    string    `json:"message"`
    CreatedAt  time.Time `json:"created_at"`
    Changes    []Change  `json:"changes"`
}
```

## Methods

### Apply

Write one or more documents. Creates a new revision if any document changed.

```go
func (c *HTTP) Apply(ctx context.Context, input ApplyInput) (*ApplyResult, error)

type ApplyInput struct {
    Scope     Scope      `json:"scope"`
    Documents []Document `json:"documents"`
    Message   string     `json:"message"`
}
```

**Example:**

```go
result, err := client.Apply(ctx, st8.ApplyInput{
    Scope:   st8.Scope{Namespace: "myapp/prod", Branch: "main"},
    Message: "enable dark mode",
    Documents: []st8.Document{
        {Key: "feature_flags", Value: `{"dark_mode":true}`},
    },
})
if err != nil {
    return err
}
if result.Noop {
    fmt.Println("nothing changed")
} else {
    fmt.Printf("created revision %d\n", result.RevisionID)
}
```

### Get

Fetch the current state of a namespace/branch.

```go
func (c *HTTP) Get(ctx context.Context, scope Scope, revision int64, checkpoint string) (*GetResult, error)
```

- Pass `revision = 0` and `checkpoint = ""` to get the latest revision.
- Pass a revision ID to get historical state.
- Pass a checkpoint name to get the revision named by that checkpoint.

**Example:**

```go
result, err := client.Get(ctx, st8.Scope{
    Namespace: "myapp/prod",
    Branch:    "main",
}, 0, "")
if err != nil {
    return err
}
raw := result.Objects["feature_flags"] // e.g. `{"dark_mode":true}`
```

### DiffDocuments

Check what would change if the given documents were applied, without applying them.

```go
func (c *HTTP) DiffDocuments(ctx context.Context, scope Scope, revision int64, checkpoint string, docs []Document) (*DiffResult, error)
```

### Checkpoint

Create a named checkpoint at the current revision.

```go
func (c *HTTP) Checkpoint(ctx context.Context, scope Scope, name, description string) (*CheckpointResult, error)
```

### Rollback

Roll back to a previous revision or checkpoint.

```go
func (c *HTTP) Rollback(ctx context.Context, scope Scope, revision int64, checkpoint, message string) (*ApplyResult, error)
```

### Log

Retrieve revision history.

```go
func (c *HTTP) Log(ctx context.Context, scope Scope, limit int) ([]LogEntry, error)
```

### CreateBranch

Create a new branch, optionally based on a specific revision or checkpoint.

```go
func (c *HTTP) CreateBranch(ctx context.Context, scope Scope, name string, revision int64, checkpoint string) (*BranchResult, error)
```

### ListBranches

List all branches in a namespace.

```go
func (c *HTTP) ListBranches(ctx context.Context, scope Scope) ([]BranchListEntry, error)
```

### Restore

Restore the current scope from another branch, revision, or checkpoint.

```go
func (c *HTTP) Restore(ctx context.Context, input RestoreInput) (*ApplyResult, error)

type RestoreInput struct {
    Scope          Scope  `json:"scope"`
    FromRevision   int64  `json:"from_revision"`
    FromCheckpoint string `json:"from_checkpoint"`
    FromBranch     string `json:"from_branch"`
    Message        string `json:"message"`
}
```

### GC

Run garbage collection. Pass `keep` to retain the most recent N revisions per namespace/branch.

```go
func (c *HTTP) GC(ctx context.Context, keep int) (*GCResult, error)

type GCResult struct {
    Pruned int `json:"pruned"`
}
```

## Error handling

All methods return a non-nil error on failure. HTTP errors (4xx, 5xx) are returned as `error` values with the server's message included.

```go
result, err := client.Get(ctx, scope, 0, "")
if err != nil {
    // err.Error() includes HTTP status and server message
    // e.g. "request failed with status 401 Unauthorized: unauthorized"
    log.Fatal(err)
}
```

Always use a context with a deadline or timeout in production:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()
result, err := client.Get(ctx, scope, 0, "")
```

Or set a default timeout on the client:

```go
c := st8.NewHTTP(url, st8.WithToken(token), st8.WithTimeout(2*time.Second))
```

## The Client interface

The `client` package exports a `Client` interface satisfied by `*HTTP`. Use it to simplify testing:

```go
type Client interface {
    Apply(ctx context.Context, input ApplyInput) (*ApplyResult, error)
    Get(ctx context.Context, scope Scope, revision int64, checkpoint string) (*GetResult, error)
    DiffDocuments(ctx context.Context, scope Scope, revision int64, checkpoint string, docs []Document) (*DiffResult, error)
    Checkpoint(ctx context.Context, scope Scope, name, description string) (*CheckpointResult, error)
    Rollback(ctx context.Context, scope Scope, revision int64, checkpoint, message string) (*ApplyResult, error)
    Log(ctx context.Context, scope Scope, limit int) ([]LogEntry, error)
    CreateBranch(ctx context.Context, scope Scope, name string, revision int64, checkpoint string) (*BranchResult, error)
    ListBranches(ctx context.Context, scope Scope) ([]BranchListEntry, error)
    Restore(ctx context.Context, input RestoreInput) (*ApplyResult, error)
    GC(ctx context.Context, keep int) (*GCResult, error)
}
```
