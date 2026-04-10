---
title: A/B Testing
description: Run experiments at the config layer using st8 branches and namespaces.
---

import { Aside, Tabs, TabItem } from '@astrojs/starlight/components';

st8 is well-suited for A/B testing and experimentation. There are two main approaches: **a ratio key** (single namespace, probabilistic split) and **branches** (deterministic split by cohort).

## Approach 1: Ratio key

Store the experiment config in a single document. Your application reads the ratio and uses it to split traffic probabilistically.

### Push the experiment config

```bash
cat > experiments.json <<'EOF'
{
  "new_pricing": {
    "enabled": true,
    "ratio": 0.1,
    "variant": {
      "price_display": "monthly",
      "cta_text": "Start free trial"
    },
    "control": {
      "price_display": "annual",
      "cta_text": "Get started"
    }
  }
}
EOF
st8ctl apply experiments.json \
  --namespace myapp/prod \
  --message "launch pricing experiment"
```

### Read and split in your app

```go
package experiment

import (
    "context"
    "encoding/json"
    "hash/fnv"
    "math/rand"

    st8 "github.com/geeper-io/st8/client"
)

type Variant map[string]any

type Experiment struct {
    Enabled bool    `json:"enabled"`
    Ratio   float64 `json:"ratio"`
    Variant Variant `json:"variant"`
    Control Variant `json:"control"`
}

type Experiments map[string]Experiment

// Assignment returns the variant for a given user ID.
// Uses consistent hashing so a user always gets the same variant
// for the duration of the experiment.
func (e Experiment) Assignment(userID string) Variant {
    if !e.Enabled {
        return e.Control
    }
    h := fnv.New32a()
    _, _ = h.Write([]byte(userID))
    bucket := float64(h.Sum32()) / float64(1<<32)
    if bucket < e.Ratio {
        return e.Variant
    }
    return e.Control
}

type Client struct {
    st8   st8.Client
    scope st8.Scope
    cache *cachedConfig // see dynamic-config guide
}

func (c *Client) Experiments(ctx context.Context) (Experiments, error) {
    result, err := c.st8.Get(ctx, c.scope, 0, "")
    if err != nil {
        return nil, err
    }
    var exps Experiments
    raw, ok := result.Objects["experiments"]
    if !ok {
        return Experiments{}, nil
    }
    if err := json.Unmarshal([]byte(raw), &exps); err != nil {
        return nil, err
    }
    return exps, nil
}
```

```go
// In your handler
func (h *Handler) PricingPage(w http.ResponseWriter, r *http.Request) {
    userID := auth.UserID(r)
    exps, _ := h.experiments.Experiments(r.Context())

    exp := exps["new_pricing"]
    variant := exp.Assignment(userID)

    renderPricing(w, variant)
}
```

### Adjust the ratio without a deploy

```bash
# Roll out to 50%
cat > experiments.json <<'EOF'
{"new_pricing": {"enabled": true, "ratio": 0.5}}
EOF
st8ctl apply experiments.json \
  --namespace myapp/prod \
  --message "increase pricing experiment to 50%"

# Kill the experiment instantly if something goes wrong
st8ctl rollback --checkpoint pre-experiment --message "abort pricing experiment"
```

## Approach 2: Branches (deterministic cohorts)

Use a branch per variant when you want deterministic, explicit assignments — no hashing, no probability. Each branch holds the complete config for that variant.

This is ideal for:
- Internal beta testers (always get variant `beta`)
- Enterprise customers with custom config
- Canary deployments to a specific fleet

### Create the experiment branches

```bash
# Start from the stable state
st8ctl checkpoint pre-experiment

# Set up control branch (copy of main)
st8ctl branch create control --checkpoint pre-experiment

# Set up variant branch
st8ctl branch create variant-a --checkpoint pre-experiment

# Write variant-specific config
cat > recommendations.json <<'EOF'
{"algorithm": "collaborative", "max_items": 12}
EOF
st8ctl apply recommendations.json \
  --branch variant-a \
  --message "variant-a: new recommendation algorithm"
```

### Your app reads the right branch per request

```go
func branchForUser(userID string, groups map[string]string) string {
    if branch, ok := groups[userID]; ok {
        return branch // explicit cohort assignment
    }
    return "main" // everyone else gets production config
}

func (h *Handler) Recommendations(w http.ResponseWriter, r *http.Request) {
    userID := auth.UserID(r)
    branch := branchForUser(userID, h.betaGroups)

    result, err := h.st8.Get(r.Context(), st8.Scope{
        Namespace: "myapp/prod",
        Branch:    branch,
    }, 0, "")
    // ...
}
```

### Promote the winner

```bash
# variant-a won — restore main from variant-a
st8ctl restore \
  --from-branch variant-a \
  --message "promote variant-a: new recommendation algorithm"

# Or just apply the winning config directly to main
cat > recommendations.json <<'EOF'
{"algorithm": "collaborative", "max_items": 12}
EOF
st8ctl apply recommendations.json \
  --branch main \
  --message "promote: new recommendation algorithm"
```

## Approach 3: Namespaces for tenant-level experiments

When you need completely isolated config per tenant or customer:

```bash
# Tenant A gets default config
cat > config.json <<'EOF'
{"theme": "light", "plan": "enterprise"}
EOF
st8ctl apply config.json --namespace acme-corp

# Tenant B is in a beta program
cat > config.json <<'EOF'
{"theme": "dark", "plan": "enterprise", "beta": true}
EOF
st8ctl apply config.json --namespace globex-corp
```

```go
func (h *Handler) ServeRequest(w http.ResponseWriter, r *http.Request) {
    tenantID := auth.TenantID(r)

    result, err := h.st8.Get(r.Context(), st8.Scope{
        Namespace: tenantID, // e.g. "acme-corp"
        Branch:    "main",
    }, 0, "")
    // ...
}
```

<Aside type="tip">
For large-scale multi-tenant deployments, consider a naming convention like `tenant/<id>` so your config namespace is predictable and filterable.
</Aside>

## Combining ratio + branches

You can combine both approaches: use a **ratio key** to decide _which_ users are in the experiment, and **branches** to isolate their config:

```go
func (h *Handler) branchForUser(ctx context.Context, userID string) string {
    exps, _ := h.experiments.Experiments(ctx)
    exp := exps["new_ui"]
    variant := exp.Assignment(userID)
    if variant["group"] == "treatment" {
        return "experiment/new-ui"
    }
    return "main"
}
```
