---
title: Concepts
description: Namespaces, branches, revisions, checkpoints — the core primitives of st8.
---

## Revisions

Every write to st8 creates a **revision** — an immutable snapshot of all documents in a namespace/branch at that point in time. Revisions are numbered sequentially starting from 1.

```
rev 1: { feature_flags: {...} }
rev 2: { feature_flags: {...}, rate_limits: {...} }   ← applied new doc
rev 3: { feature_flags: {...}, rate_limits: {...} }   ← updated feature_flags
```

Nothing is ever deleted or overwritten. You can always read any historical revision.

## Documents

Within a revision, state is organized as **documents** — named blobs of arbitrary content (usually JSON, but st8 doesn't care). Each apply operation sets one or more documents.

```bash
cat > feature_flags.json <<'EOF'
{"dark_mode": true}
EOF
cat > rate_limits.json <<'EOF'
{"api": 1000}
EOF
st8ctl apply -f feature_flags.json -f rate_limits.json
```

Documents not included in an apply are carried forward unchanged. An apply with no actual changes is a no-op (no new revision is created).

## Namespaces

A **namespace** is the top-level partition. It's a free-form string — use whatever naming convention makes sense:

| Convention | Example |
|---|---|
| Service | `payments`, `search`, `auth` |
| Service + environment | `payments/prod`, `payments/staging` |
| Team | `platform-team`, `infra` |
| Tenant | `acme-corp`, `tenant-42` |

The default namespace is `default`. Namespaces are created implicitly on first write.

## Branches

Within a namespace, **branches** are independent state lines. The default branch is `main`.

Branches let you maintain multiple versions of config simultaneously:

- `main` — production config
- `experiment/new-pricing` — A/B test variant
- `canary` — config for the 5% canary deployment

Branches are also created implicitly on first write.

### Branch operations

```bash
# Write to a branch
cat > config.json <<'EOF'
{"timeout": 5}
EOF
st8ctl apply -f config.json --branch canary

# Create a branch from a specific revision or checkpoint
st8ctl branch create canary --checkpoint stable

# List branches
st8ctl branch

# Restore a namespace/branch from another branch
st8ctl restore --from-branch canary --message "promote canary to main"
```

## Checkpoints

A **checkpoint** is a named pointer to a specific revision. Think of it like a git tag.

```bash
st8ctl checkpoint stable --description "Pre-launch verified state"
```

Checkpoints are useful for:
- Fast rollbacks: `st8ctl rollback --checkpoint stable`
- Reading a known-good state: `st8ctl get --checkpoint stable`
- CI/CD gates: checkpoint after a successful smoke test

## The apply → checkpoint → rollback lifecycle

```
apply "initial config"   →  rev 1
apply "enable feature"   →  rev 2
checkpoint "stable"      →  (names rev 2)
apply "risky change"     →  rev 3
apply "another change"   →  rev 4

# Something went wrong — roll back
rollback --checkpoint stable  →  rev 5 (content of rev 2)
```

Notice that rollback creates a **new** revision rather than deleting the recent ones. History is always preserved.

## Scope

Every operation in st8 targets a **scope**: a namespace + branch pair.

```go
scope := st8.Scope{
    Namespace: "payments/prod",
    Branch:    "main",
}
```

If not specified, st8ctl uses the defaults from your config file (`default` / `main`).
