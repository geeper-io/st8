---
title: Dynamic Configuration
description: Read config from st8 at request time without redeploying.
---

import { Aside } from '@astrojs/starlight/components';

Dynamic configuration means your application reads settings from st8 at runtime — not once at startup, but on each request (or on a short polling interval). This lets you change behavior without redeploying.

## Basic pattern: fetch on each request

The simplest approach: fetch the relevant config document on every request.

```go
package config

import (
    "context"
    "encoding/json"
    "time"

    st8 "github.com/geeper-io/st8/client"
)

type FeatureFlags struct {
    DarkMode    bool    `json:"dark_mode"`
    NewCheckout bool    `json:"new_checkout"`
    MaxPageSize int     `json:"max_page_size"`
}

type Client struct {
    st8    st8.Client
    scope  st8.Scope
}

func NewClient(serverURL, token string) *Client {
    return &Client{
        st8: st8.NewHTTP(serverURL, st8.WithToken(token), st8.WithTimeout(2*time.Second)),
        scope: st8.Scope{Namespace: "myapp/prod", Branch: "main"},
    }
}

func (c *Client) FeatureFlags(ctx context.Context) (FeatureFlags, error) {
    result, err := c.st8.Get(ctx, c.scope, 0, "")
    if err != nil {
        return FeatureFlags{}, err
    }
    var flags FeatureFlags
    if raw, ok := result.Objects["feature_flags"]; ok {
        _ = json.Unmarshal([]byte(raw), &flags)
    }
    return flags, nil
}
```

```go
// In your HTTP handler
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
    flags, err := h.config.FeatureFlags(r.Context())
    if err != nil {
        // fall back to safe defaults — never block the request
        flags = FeatureFlags{MaxPageSize: 100}
    }

    if !flags.NewCheckout {
        h.legacyCheckout(w, r)
        return
    }
    h.newCheckout(w, r)
}
```

<Aside type="tip">
Always fall back to safe defaults when st8 is unavailable. Configuration failures should degrade gracefully, not take down your service.
</Aside>

## Cached client: poll for changes

Fetching on every request adds latency. A smarter approach: poll st8 in the background and serve from an in-memory cache.

```go
package config

import (
    "context"
    "encoding/json"
    "sync"
    "sync/atomic"
    "time"
    "unsafe"

    st8 "github.com/geeper-io/st8/client"
)

type Config struct {
    FeatureFlags FeatureFlags `json:"feature_flags"`
    RateLimits   RateLimits   `json:"rate_limits"`
}

type cachedConfig struct {
    value     unsafe.Pointer // *Config
    lastRev   int64
    mu        sync.Mutex
    st8       st8.Client
    scope     st8.Scope
}

// Get returns the current config from cache (zero allocation on hot path).
func (c *cachedConfig) Get() *Config {
    p := atomic.LoadPointer(&c.value)
    if p == nil {
        return &Config{}
    }
    return (*Config)(p)
}

// Refresh fetches the latest revision from st8 if it has changed.
func (c *cachedConfig) Refresh(ctx context.Context) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    result, err := c.st8.Get(ctx, c.scope, 0, "")
    if err != nil {
        return err
    }
    if result.RevisionID == c.lastRev {
        return nil // nothing changed
    }

    cfg := &Config{}
    for key, raw := range result.Objects {
        switch key {
        case "feature_flags":
            _ = json.Unmarshal([]byte(raw), &cfg.FeatureFlags)
        case "rate_limits":
            _ = json.Unmarshal([]byte(raw), &cfg.RateLimits)
        }
    }

    atomic.StorePointer(&c.value, unsafe.Pointer(cfg))
    c.lastRev = result.RevisionID
    return nil
}

// StartPolling polls st8 every interval and refreshes the cache.
func (c *cachedConfig) StartPolling(ctx context.Context, interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                _ = c.Refresh(ctx)
            }
        }
    }()
}
```

```go
// Startup
cfg := &cachedConfig{
    st8:   st8.NewHTTP("https://st8.internal", st8.WithToken(token)),
    scope: st8.Scope{Namespace: "myapp/prod", Branch: "main"},
}
if err := cfg.Refresh(ctx); err != nil {
    log.Printf("warning: could not load initial config: %v", err)
}
cfg.StartPolling(ctx, 5*time.Second)

// In handler — zero network calls
flags := cfg.Get().FeatureFlags
```

## Diff-based updates

If you have many documents, use the `Diff` method to check which ones actually changed before unmarshalling them all:

```go
func (c *cachedConfig) RefreshWithDiff(ctx context.Context, docs []st8.Document) error {
    result, err := c.st8.DiffDocuments(ctx, c.scope, c.lastRev, "", docs)
    if err != nil {
        return err
    }
    // result.Changes only contains documents that differ from lastRev
    for _, change := range result.Changes {
        c.applyChange(change)
    }
    c.lastRev = result.RevisionID
    return nil
}
```

## Pushing config updates

```bash
# Update a single document
cat > feature_flags.json <<'EOF'
{"dark_mode": true, "new_checkout": true, "max_page_size": 200}
EOF
st8ctl apply -f feature_flags.json \
  --namespace myapp/prod \
  --message "increase page size limit"

# Verify in production within seconds (no deploy)
```
