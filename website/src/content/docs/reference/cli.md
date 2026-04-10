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

Apply one or more documents to the current namespace/branch.

```
st8ctl apply [flags]
```

| Flag | Description |
|---|---|
| `--doc <key>=<value>` | Document to apply (repeatable). Value is the raw content (usually JSON). |
| `--message` | Commit message describing the change |
| `--namespace` | Target namespace (overrides global) |
| `--branch` | Target branch (overrides global) |

**Examples**

```bash
# Apply a single document
st8ctl apply --message "enable dark mode" \
  --doc feature_flags='{"dark_mode":true}'

# Apply multiple documents
st8ctl apply --message "update config" \
  --doc feature_flags='{"dark_mode":true}' \
  --doc rate_limits='{"api":1000,"search":50}'

# Apply to a specific namespace and branch
st8ctl apply \
  --namespace payments/prod \
  --branch canary \
  --message "canary: new rate limits" \
  --doc rate_limits='{"api":500}'
```

## get

Fetch the current state of a namespace/branch.

```
st8ctl get [flags]
```

| Flag | Description |
|---|---|
| `--revision` | Fetch a specific revision ID |
| `--checkpoint` | Fetch the revision named by a checkpoint |
| `--format` | Output format: `table` (default) or `json` |

**Examples**

```bash
# Get current state
st8ctl get

# Get as JSON
st8ctl get --format json

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

Show what would change if the given documents were applied, without actually applying them.

```
st8ctl diff [flags]
```

| Flag | Description |
|---|---|
| `--doc <key>=<value>` | Document to diff (repeatable) |
| `--revision` | Compare against a specific revision |
| `--checkpoint` | Compare against a checkpoint |

**Example**

```bash
st8ctl diff \
  --doc feature_flags='{"dark_mode":false,"new_checkout":true}'
```

## checkpoint

Create a named checkpoint pointing to the current revision.

```
st8ctl checkpoint [flags]
```

| Flag | Description |
|---|---|
| `--name` | Checkpoint name (required) |
| `--description` | Human-readable description |

**Example**

```bash
st8ctl checkpoint --name stable --description "Pre-launch state, verified by QA"
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
| `--from-revision` | Base the new branch on a specific revision |
| `--from-checkpoint` | Base the new branch on a checkpoint |

**Example**

```bash
st8ctl branch create experiment/new-pricing --from-checkpoint stable
```

### branch list

```bash
st8ctl branch list
```

## restore

Restore the current namespace/branch from another branch (useful for promoting a tested branch to production).

```
st8ctl restore [flags]
```

| Flag | Description |
|---|---|
| `--from-branch` | Branch to restore from |
| `--from-revision` | Revision to restore from |
| `--from-checkpoint` | Checkpoint to restore from |
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
