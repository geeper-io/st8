---
title: Introduction
description: What st8 is, why it exists, and when to use it.
---

st8 is a lightweight, versioned key-value store designed for **dynamic configuration** — values your application reads at runtime without a redeployment.

## The problem

Most applications have two kinds of configuration:

- **Build-time config** — baked into the binary or image. Changing it requires a redeploy.
- **Environment variables** — slightly more flexible, but still require a restart and leave no audit trail.

Neither approach is great for values that need to change frequently, safely, and with history: feature flags, A/B test parameters, rate limits, kill switches, algorithm weights, model routing rules.

## What st8 provides

| Feature | Description |
|---|---|
| **Revisions** | Every write creates an immutable, numbered revision. Nothing is ever overwritten. |
| **Rollbacks** | Revert any namespace/branch to a previous revision in one operation. |
| **Checkpoints** | Tag a revision with a human-readable name (e.g. `stable`, `pre-launch`) for fast reference. |
| **Branches** | Multiple independent state lines within one namespace. Switch your app between them at request time. |
| **Namespaces** | Flat, user-defined partitioning — one namespace per service, environment, tenant, or whatever makes sense. |
| **HTTP API** | st8d is a plain HTTP server. Any language can talk to it. |
| **Go client** | Typed, context-aware client with built-in retry-friendly design. |

## Architecture

```
┌─────────────┐   HTTP    ┌──────────┐   ┌──────────┐
│   your app  │ ────────▶ │  st8d    │──▶│  disk    │
└─────────────┘           └──────────┘   └──────────┘
      ▲
      │  reads config at request time
```

`st8d` is the server. It stores all state on disk using [pebble](https://github.com/cockroachdb/pebble), a fast embedded key-value store.

`st8ctl` is the CLI. Use it to push config, inspect history, create checkpoints, and roll back.

Your application uses the **Go client** (or plain HTTP) to fetch the config it needs.

## When to use st8

st8 is a good fit when you need:

- Changing config without redeploying
- A clear audit trail of who changed what and when
- Safe rollbacks with a single command
- A/B testing at the config layer, not the code layer
- Per-environment or per-tenant config isolation

st8 is **not** a secret store, a distributed cache, or a full database. For secrets, use Vault or your cloud provider's secret manager.
