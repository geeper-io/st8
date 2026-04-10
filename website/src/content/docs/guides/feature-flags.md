---
title: Feature Flags
description: Use st8 to implement feature flags without a dedicated feature-flag service.
---

import { Aside } from '@astrojs/starlight/components';

Feature flags let you ship code that's disabled by default and enable it for specific users or globally — without a redeploy. st8 makes this simple with a single document that your app reads at request time.

## Define your flags

Create a `feature_flags` document in your namespace:

```bash
st8ctl apply \
  --namespace myapp/prod \
  --message "initial feature flags" \
  --doc feature_flags='{
    "new_checkout": false,
    "dark_mode": true,
    "ai_search": false,
    "max_upload_mb": 10
  }'
```

## Read flags in Go

```go
package flags

import (
    "context"
    "encoding/json"
    "sync/atomic"
    "time"
    "unsafe"

    st8 "github.com/geeper-io/st8/client"
)

// Flags holds the current feature flag values.
// Add new fields here as you add new flags.
type Flags struct {
    NewCheckout bool `json:"new_checkout"`
    DarkMode    bool `json:"dark_mode"`
    AISearch    bool `json:"ai_search"`
    MaxUploadMB int  `json:"max_upload_mb"`
}

// Client fetches and caches feature flags from st8.
type Client struct {
    st8     st8.Client
    scope   st8.Scope
    current unsafe.Pointer // *Flags
}

func NewClient(serverURL, token, namespace string) *Client {
    c := &Client{
        st8:   st8.NewHTTP(serverURL, st8.WithToken(token), st8.WithTimeout(2*time.Second)),
        scope: st8.Scope{Namespace: namespace, Branch: "main"},
    }
    // Initialize with safe defaults
    empty := &Flags{MaxUploadMB: 10}
    atomic.StorePointer(&c.current, unsafe.Pointer(empty))
    return c
}

// Current returns the cached flags (no network call).
func (c *Client) Current() *Flags {
    return (*Flags)(atomic.LoadPointer(&c.current))
}

// Refresh fetches the latest flags from st8.
func (c *Client) Refresh(ctx context.Context) error {
    result, err := c.st8.Get(ctx, c.scope, 0, "")
    if err != nil {
        return err
    }
    raw, ok := result.Objects["feature_flags"]
    if !ok {
        return nil
    }
    var f Flags
    if err := json.Unmarshal([]byte(raw), &f); err != nil {
        return err
    }
    atomic.StorePointer(&c.current, unsafe.Pointer(&f))
    return nil
}

// Poll refreshes flags every interval until ctx is cancelled.
func (c *Client) Poll(ctx context.Context, interval time.Duration) {
    go func() {
        t := time.NewTicker(interval)
        defer t.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-t.C:
                _ = c.Refresh(ctx)
            }
        }
    }()
}
```

## Use flags in your handlers

```go
type Server struct {
    flags *flags.Client
    // ...
}

func (s *Server) Upload(w http.ResponseWriter, r *http.Request) {
    f := s.flags.Current()

    maxBytes := int64(f.MaxUploadMB) << 20
    r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

    // handle upload...
}

func (s *Server) Search(w http.ResponseWriter, r *http.Request) {
    f := s.flags.Current()

    if f.AISearch {
        s.aiSearch(w, r)
    } else {
        s.legacySearch(w, r)
    }
}
```

## Enable a flag without deploying

```bash
# Enable AI search for everyone
st8ctl apply \
  --namespace myapp/prod \
  --message "enable ai search" \
  --doc feature_flags='{
    "new_checkout": false,
    "dark_mode": true,
    "ai_search": true,
    "max_upload_mb": 10
  }'
```

Your app picks up the change within seconds (based on the poll interval). No deploy, no restart.

## Kill switch

The same mechanism works as a kill switch — set `enabled: false` to immediately disable a misbehaving feature:

```bash
st8ctl apply \
  --namespace myapp/prod \
  --message "kill switch: disable ai search (latency spike)" \
  --doc feature_flags='{
    "new_checkout": false,
    "dark_mode": true,
    "ai_search": false,
    "max_upload_mb": 10
  }'
```

Or use rollback if you have a checkpoint:

```bash
st8ctl rollback --checkpoint stable --message "rollback: ai search causing latency"
```

## Per-environment flags

Use namespaces to maintain separate flags per environment:

```bash
# Staging gets the flag first
st8ctl apply --namespace myapp/staging \
  --message "enable ai search in staging" \
  --doc feature_flags='{"ai_search": true, ...}'

# After validation, enable in production
st8ctl apply --namespace myapp/prod \
  --message "enable ai search in production" \
  --doc feature_flags='{"ai_search": true, ...}'
```

<Aside type="tip">
For gradual rollouts, combine feature flags with [A/B testing](/guides/ab-testing/): use a flag to define the rollout percentage, then use consistent hashing to decide which users see the new behavior.
</Aside>

## Startup

```go
func main() {
    flagClient := flags.NewClient("https://st8.internal", os.Getenv("ST8_TOKEN"), "myapp/prod")

    // Block on initial load; fall back to defaults if st8 is unreachable
    ctx := context.Background()
    if err := flagClient.Refresh(ctx); err != nil {
        log.Printf("warning: could not load feature flags: %v", err)
    }

    // Poll every 5 seconds in background
    flagClient.Poll(ctx, 5*time.Second)

    // ... start server
}
```
