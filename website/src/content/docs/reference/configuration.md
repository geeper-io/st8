---
title: Configuration
description: Configuration options for st8d and st8ctl.
---

## st8ctl configuration

st8ctl reads its configuration from `~/.config/st8ctl/config.yaml` by default. Override the path with `--config`.

### Full example

```yaml
server:
  url: https://st8.internal
  token: your-secret-token

defaults:
  namespace: myapp/prod
  branch: main
```

### Local development

Use `url: local` to start an embedded st8d instead of connecting to a remote server:

```yaml
server:
  url: local
  dir: ~/.st8   # where to store state (optional, defaults to ~/.st8)

defaults:
  namespace: default
  branch: main
```

### Fields

#### server.url

The URL of the st8d server. Special value: `local` starts an embedded server.

#### server.token

Bearer token sent with every request. Must match the token configured in st8d. Omit if authentication is disabled.

#### server.dir

Only used when `url: local`. Directory for the embedded server's state. Default: `~/.st8`.

#### defaults.namespace

Default namespace used when `--namespace` is not specified on the command line. Default: `default`.

#### defaults.branch

Default branch used when `--branch` is not specified on the command line. Default: `main`.

### Precedence

Configuration values are applied in this order (highest priority first):

1. CLI flags (`--server`, `--token`, etc.)
2. Config file
3. Built-in defaults

---

## st8d flags

`st8d` is configured entirely via command-line flags (or a process supervisor / init system).

```
st8d [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--listen` | `:8748` | Address to listen on |
| `--state-dir` | `.st8d` | Directory for persistent state (Pebble data and local WAL) |
| `--token` | `""` | Bearer token (authentication disabled if empty) |
| `--read-timeout` | `30s` | HTTP read timeout |
| `--write-timeout` | `60s` | HTTP write timeout |
| `--idle-timeout` | `120s` | HTTP idle connection timeout |

### S3 storage flags

When `--s3-bucket` is set, `st8d` archives WAL segments and checkpoints to S3, providing durable storage that survives local disk loss. Without it, durability is local-only.

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--s3-bucket` | `ST8D_S3_BUCKET` | `""` | S3 bucket name for WAL archive and checkpoints |
| `--s3-prefix` | `ST8D_S3_PREFIX` | `""` | Optional key prefix inside the bucket (e.g. `st8d/prod`) |
| `--s3-endpoint` | `ST8D_S3_ENDPOINT` | `""` | Custom endpoint URL for MinIO or other S3-compatible stores (e.g. `http://minio:9000`) |
| `--s3-region` | `ST8D_S3_REGION` | `""` | AWS region for the S3 client |
| `--s3-profile` | `ST8D_S3_PROFILE` | `""` | Named profile from `~/.aws/config` (ignored when static credentials are set) |
| `--s3-access-key-id` | `ST8D_S3_ACCESS_KEY_ID` | `""` | Static S3 access key ID (preferred for explicit configuration) |
| `--s3-secret-access-key` | `ST8D_S3_SECRET_ACCESS_KEY` | `""` | Static S3 secret access key (preferred for explicit configuration) |

All S3 flags can also be supplied via the corresponding environment variables. CLI flags take precedence.

For explicit credentials, prefer `ST8D_S3_ACCESS_KEY_ID` / `ST8D_S3_SECRET_ACCESS_KEY` or the matching `--s3-access-key-id` / `--s3-secret-access-key` flags. When both static credential fields are set, `st8d` uses them directly and bypasses profile and AWS SDK chain resolution. Otherwise, credentials can be resolved from a named `--s3-profile` or the standard AWS SDK chain.

#### Example: AWS S3

```bash
st8d --listen :8748 \
     --state-dir /var/lib/st8 \
     --s3-bucket my-st8-bucket \
     --s3-prefix st8d/prod
```

#### Example: MinIO

```bash
st8d --listen :8748 \
     --state-dir /var/lib/st8 \
     --s3-bucket st8 \
     --s3-endpoint http://minio:9000
```

Set `ST8D_S3_ACCESS_KEY_ID` and `ST8D_S3_SECRET_ACCESS_KEY` (or `--s3-access-key-id` / `--s3-secret-access-key`) to provide explicit credentials.

### Example systemd unit

```ini
[Unit]
Description=st8d state server
After=network.target

[Service]
ExecStart=/usr/local/bin/st8d \
  --listen :8748 \
  --state-dir /var/lib/st8 \
  --token ${ST8_TOKEN} \
  --read-timeout 30s \
  --write-timeout 60s
Restart=on-failure
User=st8

[Install]
WantedBy=multi-user.target
```

### Example Docker

```dockerfile
FROM golang:1.23 AS builder
WORKDIR /src
COPY . .
RUN go build -o /st8d ./cmd/st8d

FROM debian:bookworm-slim
COPY --from=builder /st8d /usr/local/bin/st8d
VOLUME /data
EXPOSE 8748
ENTRYPOINT ["st8d", "--listen", ":8748", "--state-dir", "/data"]
```

```bash
docker run -d \
  -p 8748:8748 \
  -v st8-data:/data \
  -e ST8_TOKEN=mysecret \
  myrepo/st8d \
  --token mysecret
```
