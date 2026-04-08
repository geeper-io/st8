# st8ctl

[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Backend](https://img.shields.io/badge/backend-t4-0F766E)](https://github.com/t4db/t4)
[![Mode](https://img.shields.io/badge/mode-embedded%20or%20client%2Fserver-1D4ED8)](#modes)
[![Status](https://img.shields.io/badge/status-prototype-F59E0B)](#current-notes)

`st8ctl` (like "state") safely applies, tracks, diffs, checkpoints, rolls back, and branches config state.

It is built around a simple idea: state changes should feel more like Git and less like `scp` plus hope.

`st8ctl` gives config and operational state a proper lifecycle:

- review what will change
- apply it as an immutable revision
- mark known-good checkpoints
- branch for experiments
- restore or roll back without rewriting history

Under the hood, `st8ctl` uses [`t4`](https://github.com/t4db/t4) as its embedded persistence engine.

## What It Does

With `st8ctl`, you can:

- apply config changes as immutable revisions
- inspect current and historical state
- diff pending changes before applying them
- create named checkpoints
- roll back instantly to a revision or checkpoint
- branch state for experiments
- restore from another branch

It is designed as a pure client for `st8d` — all state lives in the daemon, `st8ctl` only talks to it over HTTP.

## Modes

`st8ctl` supports two ways to connect to `st8d`:

1. Remote mode

   `st8ctl --server http://host:8748` (or `server.url` in config) talks to a running `st8d` daemon.

2. Local demo mode

   `st8ctl --local` starts a temporary `st8d` instance for the lifetime of the command, then talks to it over HTTP. Useful for demos and local testing without running a separate process.

## Quick Start

The fastest way to get the feel of `st8ctl` is:

Create a config file:

```bash
cat > config.json <<'EOF'
{
  "enabled": true,
  "timeout": 30
}
EOF
```

Start a temporary local `st8d` and apply the config:

```bash
st8ctl --local --workspace payments --env prod apply config.json
```

See history:

```bash
st8ctl --local --workspace payments --env prod log
```

Create a checkpoint:

```bash
st8ctl --local --workspace payments --env prod checkpoint before-change
```

Read current state:

```bash
st8ctl --local --workspace payments --env prod get
st8ctl --local --workspace payments --env prod get config.json
```

`--local` starts a temporary `st8d` for the duration of the command. For persistent state, run `st8d` yourself (see below) and point `st8ctl` at it.

## Running st8d

Start the daemon:

```bash
st8d --listen :8748 --state-dir .st8d
```

Then point the CLI at it:

```bash
st8ctl --server http://127.0.0.1:8748 --workspace payments --env prod apply config.json
st8ctl --server http://127.0.0.1:8748 --workspace payments --env prod log
st8ctl --server http://127.0.0.1:8748 --workspace payments --env prod checkpoint before-change
```

This is the shape you would use for a shared deployment or CI-driven workflow.

## Common Workflows

### Diff Before Apply

```bash
st8ctl --workspace payments --env prod diff config.json
st8ctl --workspace payments --env prod apply config.json
```

### Roll Back To A Checkpoint

```bash
st8ctl --workspace payments --env prod rollback --checkpoint before-change
```

### Roll Back To A Revision

```bash
st8ctl --workspace payments --env prod rollback --revision 1
```

### How Rollback Works

`rollback` restores state from a specific target revision or checkpoint.

It does not move history backward destructively. Instead, it creates a new revision whose contents match the target state.

Example:

1. revision 1: good config
2. revision 2: bad config change
3. `st8ctl rollback --revision 1`
4. revision 3 is created, with the same contents as revision 1

So rollback behaves like "restore this known-good state as a new revision", not "reset branch history in place".

If you want to roll back to the immediately previous state, first inspect `st8ctl log`, then pass that previous revision id with `--revision`.

### Create A Branch

```bash
st8ctl --workspace payments --env prod branch create migration-test --checkpoint before-change
st8ctl --workspace payments --env prod --branch migration-test apply config.json
st8ctl --workspace payments --env prod --branch migration-test log
```

### Restore From Another Branch

```bash
st8ctl --workspace payments --env prod restore --from-branch migration-test
```

## Why It Exists

Most config workflows have weak ergonomics:

- edits happen out of band
- applying changes is easy, auditing them is harder
- rollback is often a manual scramble
- experimentation in real state is risky

`st8ctl` aims to make state management explicit, inspectable, and reversible.

## Command Summary

```text
apply       Apply one or more files into state
get         Read current or historical state
diff        Compare current state to files or a prior revision
checkpoint  Create a named checkpoint
rollback    Roll back to a revision or checkpoint
log         Show revision history
branch      List branches or create a new branch
restore     Restore state from a revision, checkpoint, or branch
```

## State Directories

By default:

- direct embedded mode uses `.st8`
- `st8d` examples above use `.st8d`

You can override either with `--state-dir`.

Examples:

```bash
st8ctl --state-dir /tmp/st8-demo --workspace payments --env prod apply config.json
st8d --state-dir /tmp/st8d-demo
```

## Current Notes

- `apply` currently stores each input file under its normalized path as the state key.
- `--local` is per-command demo mode, not a persistent auto-reused daemon.
- `st8d` currently exposes a small HTTP JSON API that the CLI uses in remote mode.

