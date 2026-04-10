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
| `--state-dir` | `.st8d` | Directory for persistent state |
| `--token` | `""` | Bearer token (authentication disabled if empty) |
| `--read-timeout` | `30s` | HTTP read timeout |
| `--write-timeout` | `60s` | HTTP write timeout |
| `--idle-timeout` | `120s` | HTTP idle connection timeout |

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
