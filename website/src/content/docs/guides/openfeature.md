---
title: OpenFeature Integration (OFREP)
description: Use st8 as a flag management system with any OpenFeature SDK via the Remote Evaluation Protocol.
---

`st8d` implements the [OpenFeature Remote Evaluation Protocol (OFREP)](https://github.com/open-feature/protocol), which means any OpenFeature SDK can read st8 state as feature flags without a custom provider.

Each document key stored in st8 becomes a flag key. The document's JSON content determines the flag's value and type.

## How it works

```
┌──────────────────┐  OFREP   ┌──────────┐   ┌──────────┐
│ your app         │ ───────▶ │  st8d    │──▶│  disk    │
│ (OpenFeature SDK)│          └──────────┘   └──────────┘
└──────────────────┘
```

1. Your application uses any OpenFeature SDK with an OFREP provider.
2. The provider calls st8d's OFREP endpoints.
3. st8d returns the current document values as typed flag evaluations.

## Endpoints

### POST /ofrep/v1/evaluate/flags/{key}

Evaluate a single flag. Used by **server-side** (dynamic context) OFREP providers, which call this endpoint on every evaluation.

**Query parameters:**

| Parameter | Default | Description |
|---|---|---|
| `namespace` | `default` | The st8 namespace to read from |
| `branch` | `main` | The st8 branch to read from |
| `prefix` | _(none)_ | Key prefix to prepend when looking up the document (see [Key prefix](#key-prefix)) |

**Request body:**

```json
{
  "context": {
    "targetingKey": "user-123"
  }
}
```

**Response (200):**

```json
{
  "key": "feature-x",
  "value": true,
  "reason": "STATIC",
  "variant": "rev-4"
}
```

**Response (404) — flag not found:**

```json
{
  "key": "feature-x",
  "errorCode": "FLAG_NOT_FOUND",
  "errorDetails": "flag \"feature-x\" not found"
}
```

---

### POST /ofrep/v1/evaluate/flags

Evaluate all flags at once. Used by **client-side** (static context) OFREP providers, which cache all values locally after a single call.

Accepts the same `?namespace=`, `?branch=`, and `?prefix=` query parameters.

**Request body:**

```json
{
  "context": {
    "targetingKey": "user-123"
  }
}
```

**Response (200):**

```json
{
  "flags": [
    {"key": "feature-x",  "value": true,   "reason": "STATIC", "variant": "rev-4"},
    {"key": "timeout",    "value": 30,     "reason": "STATIC", "variant": "rev-4"},
    {"key": "theme",      "value": "dark", "reason": "STATIC", "variant": "rev-4"}
  ],
  "metadata": {"revision": 4}
}
```

Responds with **304 Not Modified** if the client sends `If-None-Match` with the ETag from a previous response and the revision hasn't changed.

## Flag types

The value type is inferred from the document content by JSON-parsing it:

| Document content | OFREP type | Example |
|---|---|---|
| `true` or `false` | boolean | `true` |
| Integer JSON number | integer | `42` |
| Fractional JSON number | float | `3.14` |
| JSON string | string | `"dark"` |
| JSON object or array | object | `{"rate": 100}` |
| Non-JSON text | string | `some-plain-text` |

The evaluation `reason` is always `STATIC` — st8 stores values directly without targeting rules. The `variant` is the current revision ID (e.g. `rev-4`).

## Authentication

OFREP endpoints follow the same token-based auth as the rest of st8d. Provide a bearer token:

```
Authorization: Bearer <token>
```

Tokens need at least `read` permission for the target namespace and branch.

## Example: curl

```bash
# Single flag
curl -s -X POST \
  "http://localhost:8748/ofrep/v1/evaluate/flags/feature-x?namespace=myapp/prod&branch=main" \
  -H "Authorization: Bearer my-token" \
  -H "Content-Type: application/json" \
  -d '{"context": {"targetingKey": "user-123"}}'

# All flags
curl -s -X POST \
  "http://localhost:8748/ofrep/v1/evaluate/flags?namespace=myapp/prod&branch=main" \
  -H "Authorization: Bearer my-token" \
  -H "Content-Type: application/json" \
  -d '{"context": {"targetingKey": "user-123"}}'
```

## Using with an OpenFeature SDK

Point any OFREP provider at your st8d instance. The base URL should include the namespace and branch as query parameters — most providers let you set a custom base URL or path.

Find OFREP providers for your language at [openfeature.dev/ecosystem](https://openfeature.dev/ecosystem/?instant_search%5BrefinementList%5D%5Bvendor%5D%5B0%5D=OFREP).

**Example (Go):**

```go
import (
    ofrep "github.com/open-feature/go-sdk-contrib/providers/ofrep/pkg"
    "github.com/open-feature/go-sdk/openfeature"
)

provider := ofrep.NewProvider(
    "http://localhost:8748",
    ofrep.WithBaseURLSuffix("?namespace=myapp/prod&branch=main"),
    ofrep.WithBearerToken("my-token"),
)
openfeature.SetProvider(provider)
```

Once set up, evaluate flags the standard OpenFeature way:

```go
client := openfeature.NewClient("my-service")

enabled, _ := client.BooleanValue(ctx, "feature-x", false, openfeature.NewEvaluationContext("user-123", nil))
timeout, _ := client.IntValue(ctx, "timeout", 30, openfeature.NewEvaluationContext("user-123", nil))
```

## Targeting and dynamic evaluation

st8 does not have built-in targeting rules — all flags evaluate to `STATIC`. The evaluation context passed by the OpenFeature SDK is accepted by st8d but not used to vary the result.

For per-user targeting, use st8 **branches** to maintain separate flag sets and route your application to the correct branch based on the request context before calling the OpenFeature SDK.
