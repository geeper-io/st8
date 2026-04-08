# st8

[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Backend](https://img.shields.io/badge/backend-t4-0F766E)](https://github.com/t4db/t4)
[![Mode](https://img.shields.io/badge/mode-embedded%20or%20client%2Fserver-1D4ED8)](#modes)
[![Status](https://img.shields.io/badge/status-prototype-F59E0B)](#current-notes)

`st8` (like "state") safely applies, tracks, diffs, checkpoints, rolls back, and branches config state.

It is built around a simple idea: state changes should feel more like Git and less like `scp` plus hope.

`st8` gives config and operational state a proper lifecycle:

- review what will change
- apply it as an immutable revision
- mark known-good checkpoints
- branch for experiments
- restore or roll back without rewriting history

Under the hood, `st8` uses [`t4`](https://github.com/t4db/t4) as its embedded persistence engine.

## What It Does

With `st8`, you can:

- apply config changes as immutable revisions
- inspect current and historical state
- diff pending changes before applying them
- create named checkpoints
- roll back instantly to a revision or checkpoint
- branch state for experiments
- restore from another branch

It is designed to work both as:

- a local embedded CLI for demos and single-user workflows
- a client talking to a shared `st8d` service for team and environment workflows

## Modes

`st8` supports three ways to run:

1. Direct embedded mode

   `st8` talks directly to an embedded `t4` engine in the local state directory.

2. Local client/server demo mode

   `st8 --local` starts a temporary local `st8d` server for the lifetime of the command, then talks to it over HTTP.

3. Remote client/server mode

   `st8 --server http://host:8748` talks to a separately running `st8d` daemon.

The same revision, checkpoint, rollback, and branching model is used in all three modes.

## Quick Start

The fastest way to get the feel of `st8` is:

Create a config file:

```bash
cat > config.json <<'EOF'
{
  "enabled": true,
  "timeout": 30
}
EOF
```

Apply it in direct embedded mode:

```bash
st8 --workspace payments --env prod apply config.json
```

See history:

```bash
st8 --workspace payments --env prod log
```

Create a checkpoint:

```bash
st8 --workspace payments --env prod checkpoint before-change
```

Read current state:

```bash
st8 --workspace payments --env prod get
st8 --workspace payments --env prod get config.json
```

At this point you already have:

- one applied immutable revision
- a named recovery point
- a way to inspect the stored state

## Demo Client/Server Mode

If you want to exercise the client/server path without manually running a daemon, use `--local`:

```bash
st8 --local --workspace payments --env prod apply config.json
st8 --local --workspace payments --env prod log
st8 --local --workspace payments --env prod checkpoint before-change
```

This starts a temporary local `st8d` instance in the background for each command and talks to it over HTTP.

This is useful when you want to demo the real client/server flow without separately managing a long-running process.

## Running st8d

Start the daemon:

```bash
st8d --listen :8748 --state-dir .st8d
```

Then point the CLI at it:

```bash
st8 --server http://127.0.0.1:8748 --workspace payments --env prod apply config.json
st8 --server http://127.0.0.1:8748 --workspace payments --env prod log
st8 --server http://127.0.0.1:8748 --workspace payments --env prod checkpoint before-change
```

This is the shape you would use for a shared deployment or CI-driven workflow.

## Common Workflows

### Diff Before Apply

```bash
st8 --workspace payments --env prod diff config.json
st8 --workspace payments --env prod apply config.json
```

### Roll Back To A Checkpoint

```bash
st8 --workspace payments --env prod rollback --checkpoint before-change
```

### Roll Back To A Revision

```bash
st8 --workspace payments --env prod rollback --revision 1
```

### How Rollback Works

`rollback` restores state from a specific target revision or checkpoint.

It does not move history backward destructively. Instead, it creates a new revision whose contents match the target state.

Example:

1. revision 1: good config
2. revision 2: bad config change
3. `st8 rollback --revision 1`
4. revision 3 is created, with the same contents as revision 1

So rollback behaves like "restore this known-good state as a new revision", not "reset branch history in place".

If you want to roll back to the immediately previous state, first inspect `st8 log`, then pass that previous revision id with `--revision`.

### Create A Branch

```bash
st8 --workspace payments --env prod branch create migration-test --checkpoint before-change
st8 --workspace payments --env prod --branch migration-test apply config.json
st8 --workspace payments --env prod --branch migration-test log
```

### Restore From Another Branch

```bash
st8 --workspace payments --env prod restore --from-branch migration-test
```

## Why It Exists

Most config workflows have weak ergonomics:

- edits happen out of band
- applying changes is easy, auditing them is harder
- rollback is often a manual scramble
- experimentation in real state is risky

`st8` aims to make state management explicit, inspectable, and reversible.

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
st8 --state-dir /tmp/st8-demo --workspace payments --env prod apply config.json
st8d --state-dir /tmp/st8d-demo
```

## Current Notes

- `apply` currently stores each input file under its normalized path as the state key.
- `--local` is per-command demo mode, not a persistent auto-reused daemon.
- `st8d` currently exposes a small HTTP JSON API that the CLI uses in remote mode.

