# st8ctl

[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Backend](https://img.shields.io/badge/backend-t4-0F766E)](https://github.com/t4db/t4)
[![Status](https://img.shields.io/badge/status-prototype-F59E0B)](#current-notes)

`st8ctl` (like "state") safely applies, tracks, diffs, checkpoints, rolls back, and branches config state.

It is built around a simple idea: state changes should feel more like Git and less like `scp` plus hope.

`st8ctl` gives config and operational state a proper lifecycle:

- review what will change
- apply it as an immutable revision
- mark known-good checkpoints
- branch for experiments
- restore or roll back without rewriting history

It is a pure client for `st8d` — all state lives in the daemon, `st8ctl` only talks to it over HTTP.

Under the hood, `st8d` uses [`t4`](https://github.com/t4db/t4) as its embedded persistence engine.

## Namespaces and Branches

State is organized into **namespaces** and **branches**.

A **namespace** is a flat string that scopes state to a logical owner — a service, team, or environment. You choose the naming convention: `payments/prod`, `platform-dev`, `acme` — whatever makes sense for your workflow. The default namespace is `default`.

A **branch** works like a Git branch: the same namespace can have multiple branches so you can draft changes, experiment, or promote state without touching the live revision history. The default branch is `main`.

## What It Does

With `st8ctl`, you can:

- apply config changes as immutable revisions
- inspect current and historical state
- diff pending changes before applying them
- create named checkpoints
- roll back instantly to any revision or checkpoint
- branch state for experiments
- restore from another branch

## Modes

`st8ctl` supports two ways to connect to `st8d`:

1. **Remote** — `st8ctl --server http://host:8748` (or `server.url` in config) talks to a running `st8d` daemon.

2. **Local demo** — `st8ctl --local` starts a temporary `st8d` instance for the lifetime of the command. Useful for demos and local testing without running a separate process.

## Configuration

Create `~/.config/st8ctl/config.yaml` to avoid repeating flags:

```yaml
server:
  url: http://st8d.internal:8748
  token: my-secret-token

defaults:
  namespace: payments/prod
  branch: main
```

For local demo mode:

```yaml
server:
  url: local
  dir: ~/.st8

defaults:
  namespace: payments-dev
```

## Quick Start

Create a config file:

```bash
cat > config.json <<'EOF'
{
  "enabled": true,
  "timeout": 30
}
EOF
```

Start a temporary local `st8d` and apply:

```bash
st8ctl --local --namespace payments/prod apply -f config.json
```

See history:

```bash
st8ctl --local --namespace payments/prod log
```

Create a checkpoint:

```bash
st8ctl --local --namespace payments/prod checkpoint before-change
```

Read current state:

```bash
st8ctl --local --namespace payments/prod get
st8ctl --local --namespace payments/prod get config.json
```

`--local` starts a temporary `st8d` for the duration of the command. For persistent state, run `st8d` yourself (see below) and point `st8ctl` at it.

## Running st8d

Start the daemon:

```bash
st8d --listen :8748 --state-dir .st8d
```

Protect the API with a bearer token:

```bash
st8d --listen :8748 --state-dir .st8d --token my-secret-token
```

Enable TLS by supplying a certificate and key:

```bash
st8d --listen :8748 --state-dir .st8d \
     --tls-cert /etc/st8d/server.crt \
     --tls-key  /etc/st8d/server.key
```

> **Proxy alternative:** if you already have an nginx / Caddy / Envoy reverse proxy in front, simply terminate TLS there and let `st8d` listen on plain HTTP on a local port.

Then point the CLI at it:

```bash
st8ctl --server http://127.0.0.1:8748 --namespace payments/prod apply -f config.json
st8ctl --server http://127.0.0.1:8748 --namespace payments/prod log
st8ctl --server http://127.0.0.1:8748 --namespace payments/prod checkpoint before-change
```

## Common Workflows

### Diff Before Apply

```bash
st8ctl --namespace payments/prod diff config.json
st8ctl --namespace payments/prod apply -f config.json
```

### Roll Back To A Checkpoint

```bash
st8ctl --namespace payments/prod rollback --checkpoint before-change
```

### Roll Back To A Revision

```bash
st8ctl --namespace payments/prod rollback --revision 1
```

### How Rollback Works

`rollback` does not rewrite history. It creates a new revision whose contents match the target state.

Example:

1. revision 1: good config
2. revision 2: bad config change
3. `st8ctl rollback --revision 1`
4. revision 3 is created with the same contents as revision 1

### Branch for Experiments

```bash
st8ctl --namespace payments/prod branch create migration-test --checkpoint before-change
st8ctl --namespace payments/prod --branch migration-test apply -f config.json
st8ctl --namespace payments/prod --branch migration-test log
```

### Promote a Branch

```bash
st8ctl --namespace payments/prod restore --from-branch migration-test
```

## Command Summary

```text
apply              Apply one or more files into state
get                Read current or historical state
diff               Compare current state to files or a prior revision
checkpoint         Create a named checkpoint
checkpoint delete  Delete a named checkpoint
rollback           Roll back to a revision or checkpoint
log                Show revision history
branch             List branches or create a new branch
restore            Restore state from a revision, checkpoint, or branch
gc                 Garbage-collect old revisions  (--keep N, default 10)
```

## Current Notes

- `apply` stores each input file under its normalized path as the state key.
- `--local` is per-command demo mode, not a persistent auto-reused daemon.
- `st8d` exposes an HTTP JSON API that `st8ctl` uses in all modes.
