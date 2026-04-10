---
title: HTTP API Reference
description: REST API endpoints exposed by st8d.
---

All endpoints are served by `st8d`. The base URL is configurable (default `:8748`).

Unless authentication is disabled, all endpoints except `/healthz` and `/readyz` require:

```
Authorization: Bearer <token>
```

## Health

### GET /healthz

Liveness check. Returns `200 OK` with body `ok` if the server is running.

### GET /readyz

Readiness check. Returns `200 OK` if the storage backend is available, `503 Service Unavailable` otherwise.

## Metrics

### GET /metrics

Prometheus metrics in text exposition format. Includes:

- `st8_http_requests_total` — request count by route, method, status
- `st8_http_request_duration_seconds` — request latency histogram
- `st8_operation_duration_seconds` — operation latency by type and result
- `st8_changed_documents_total` — documents changed per operation type

## State

### POST /v1/apply

Apply documents to a namespace/branch. Creates a new revision if any document changed.

**Request body:**

```json
{
  "scope": {"namespace": "myapp/prod", "branch": "main"},
  "documents": [
    {"key": "feature_flags", "value": "{\"dark_mode\":true}"}
  ],
  "message": "enable dark mode"
}
```

**Response:**

```json
{
  "revision_id": 42,
  "noop": false,
  "changes": [
    {"key": "feature_flags", "old": "{\"dark_mode\":false}", "new": "{\"dark_mode\":true}"}
  ]
}
```

### GET /v1/state

Fetch the state of a namespace/branch.

**Query parameters:**

| Parameter | Description |
|---|---|
| `namespace` | Namespace (required) |
| `branch` | Branch (required) |
| `revision` | Specific revision ID (optional) |
| `checkpoint` | Checkpoint name (optional) |

**Response:**

```json
{
  "revision_id": 42,
  "objects": {
    "feature_flags": "{\"dark_mode\":true}",
    "rate_limits": "{\"api\":1000}"
  }
}
```

### POST /v1/diff

Check what would change without applying.

**Request body:**

```json
{
  "scope": {"namespace": "myapp/prod", "branch": "main"},
  "revision_id": 0,
  "checkpoint_name": "",
  "documents": [
    {"key": "feature_flags", "value": "{\"dark_mode\":false}"}
  ]
}
```

**Response:**

```json
{
  "revision_id": 42,
  "changes": [
    {"key": "feature_flags", "old": "{\"dark_mode\":true}", "new": "{\"dark_mode\":false}"}
  ]
}
```

## History

### GET /v1/log

Revision history for a namespace/branch.

**Query parameters:**

| Parameter | Description |
|---|---|
| `namespace` | Namespace |
| `branch` | Branch |
| `limit` | Maximum entries (default 20) |
| `after` | Return entries after this revision ID (for pagination) |

**Response:**

```json
{
  "entries": [
    {
      "revision_id": 42,
      "message": "enable dark mode",
      "created_at": "2024-01-15T10:30:00Z",
      "changes": [{"key": "feature_flags", "old": "...", "new": "..."}]
    }
  ]
}
```

### POST /v1/checkpoints

Create a checkpoint.

**Request body:**

```json
{
  "scope": {"namespace": "myapp/prod", "branch": "main"},
  "name": "stable",
  "description": "Pre-launch verified state"
}
```

**Response:**

```json
{
  "name": "stable",
  "revision_id": 42,
  "created_at": "2024-01-15T10:30:00Z"
}
```

### POST /v1/rollback

Roll back to a previous revision or checkpoint.

**Request body:**

```json
{
  "scope": {"namespace": "myapp/prod", "branch": "main"},
  "revision_id": 0,
  "checkpoint_name": "stable",
  "message": "Revert: latency regression"
}
```

**Response:** Same as `/v1/apply`.

## Branches

### GET /v1/branches

List branches in a namespace.

**Query parameters:** `namespace`, `branch` (for scope context), `limit`, `after`

**Response:**

```json
{
  "branches": [
    {"name": "main", "revision_id": 42, "created_at": "2024-01-01T00:00:00Z"},
    {"name": "canary", "revision_id": 5, "created_at": "2024-01-10T00:00:00Z"}
  ]
}
```

### POST /v1/branches

Create a branch.

**Request body:**

```json
{
  "scope": {"namespace": "myapp/prod", "branch": "main"},
  "name": "canary",
  "from_revision": 0,
  "from_checkpoint": "stable"
}
```

**Response:**

```json
{
  "name": "canary",
  "revision_id": 42
}
```

### POST /v1/restore

Restore a namespace/branch from another branch, revision, or checkpoint.

**Request body:**

```json
{
  "scope": {"namespace": "myapp/prod", "branch": "main"},
  "from_branch": "canary",
  "from_revision": 0,
  "from_checkpoint": "",
  "message": "Promote canary to main"
}
```

**Response:** Same as `/v1/apply`.

## Maintenance

### POST /v1/gc

Run garbage collection.

**Request body:**

```json
{
  "keep": 100
}
```

**Response:**

```json
{
  "pruned": 47
}
```
