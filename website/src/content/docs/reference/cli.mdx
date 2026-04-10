---
title: CLI Reference (st8ctl)
description: Complete reference for the st8ctl command-line tool.
---

import { Aside } from '@astrojs/starlight/components';

`st8ctl` is the command-line interface for st8. It communicates with `st8d` over HTTP.

## Global flags

These flags are available on every command.

| Flag | Default | Description |
|---|---|---|
| `--config` | `~/.config/st8ctl/config.yaml` | Path to config file |
| `--server` | from config | st8d server URL |
| `--token` | from config | Bearer token for authentication |
| `--local` | false | Start an embedded st8d instance (for development) |
| `--state-dir` | `~/.st8` | State directory for `--local` mode |
| `--namespace` | `default` | Namespace to operate on |
| `--branch` | `main` | Branch to operate on |
| `--timeout` | 0 (none) | Request timeout (e.g. `5s`, `30s`) |

## apply

Apply one or more files or inline values to the current namespace/branch.

```
st8ctl apply [-f file...] [-v key=value...] [flags]
```

| Flag | Default | Description |
|---|---|---|
| `-f`, `--file` | | File to apply (repeatable) |
| `-v`, `--value` | | Inline `key=value` pair to apply (repeatable) |
| `--message` | | Commit message describing the change |
| `--namespace` | | Target namespace (overrides global) |
| `--branch` | | Target branch (overrides global) |

**Examples**

```bash
# Apply a single file
st8ctl apply -f feature_flags.json --message "enable dark mode"

# Apply multiple files
st8ctl apply -f feature_flags.json -f rate_limits.json --message "update config"

# Apply an inline value
st8ctl apply -v feature_flags='{"dark_mode":true}' --message "enable dark mode"

# Apply to a specific namespace and branch
st8ctl apply -f rate_limits.json \
  --namespace payments/prod \
  --branch canary \
  --message "canary: new rate limits"
```

## get

Fetch the current state of a namespace/branch, or a specific document by key.

```
st8ctl get [key] [flags]
```

| Flag | Description |
|---|---|
| `--revision` | Fetch a specific revision ID |
| `--checkpoint` | Fetch the revision named by a checkpoint |

**Examples**

```bash
# Get current state (all documents)
st8ctl get

# Get a specific document by key
st8ctl get feature_flags.json

# Get a specific revision
st8ctl get --revision 42

# Get a checkpoint
st8ctl get --checkpoint stable
```

## log

Show the revision history.

```
st8ctl log [flags]
```

| Flag | Description |
|---|---|
| `--limit` | Maximum number of entries to show (default 20) |

**Example**

```bash
st8ctl log --limit 50
```

## diff

Show what would change if the given files or values were applied, without actually applying them.

```
st8ctl diff [-f file...] [-v key=value...] [flags]
```

| Flag | Description |
|---|---|
| `-f`, `--file` | File to diff (repeatable) |
| `-v`, `--value` | Inline `key=value` pair to diff (repeatable) |
| `--revision` | Compare against a specific revision |
| `--checkpoint` | Compare against a checkpoint |

**Example**

```bash
st8ctl diff -f feature_flags.json
```

## checkpoint

Create a named checkpoint pointing to the current revision.

```
st8ctl checkpoint <name> [flags]
```

| Flag | Description |
|---|---|
| `--description` | Human-readable description |

**Example**

```bash
st8ctl checkpoint stable --description "Pre-launch state, verified by QA"
```

### checkpoint delete

Delete a named checkpoint.

```
st8ctl checkpoint delete <name>
```

**Example**

```bash
st8ctl checkpoint delete stable
```

## rollback

Roll back to a previous revision or checkpoint.

```
st8ctl rollback [flags]
```

| Flag | Description |
|---|---|
| `--revision` | Revision ID to roll back to |
| `--checkpoint` | Checkpoint name to roll back to |
| `--message` | Message describing why the rollback happened |

**Example**

```bash
st8ctl rollback --checkpoint stable --message "Reverting: latency regression in new checkout"
```

## branch

Manage branches.

### branch create

```bash
st8ctl branch create <name> [flags]
```

| Flag | Description |
|---|---|
| `--revision` | Base the new branch on a specific revision |
| `--checkpoint` | Base the new branch on a checkpoint |

**Example**

```bash
st8ctl branch create experiment/new-pricing --checkpoint stable
```

To list branches, run `st8ctl branch` with no arguments.

## restore

Restore the current namespace/branch from another branch (useful for promoting a tested branch to production).

```
st8ctl restore [flags]
```

| Flag | Description |
|---|---|
| `--from-branch` | Branch to restore from |
| `--revision` | Revision to restore from |
| `--checkpoint` | Checkpoint to restore from |
| `--message` | Message describing the restore |

**Example**

```bash
st8ctl restore --from-branch canary --message "Promote canary to main"
```

## gc

Run garbage collection to prune old revisions.

```
st8ctl gc [flags]
```

| Flag | Description |
|---|---|
| `--keep` | Number of recent revisions to keep per namespace/branch |

**Example**

```bash
st8ctl gc --keep 100
```

<Aside type="caution">
GC permanently deletes old revisions. Checkpoints that point to pruned revisions will also be removed. Always create a checkpoint for states you want to keep indefinitely.
</Aside>

## Configuration file

`~/.config/st8ctl/config.yaml`:

```yaml
server:
  url: https://st8.internal
  token: your-secret-token

defaults:
  namespace: myapp/prod
  branch: main
```

For local development:

```yaml
server:
  url: local
  dir: ~/.st8   # optional, this is the default

defaults:
  namespace: default
  branch: main
```
