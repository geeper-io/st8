# Production Readiness

## Critical

- [x] **t4kv: keep node open** — currently opens and closes the pebble node on every `Load`/`Save`; must hold it open for the process lifetime
- [x] **Server timeouts** — set `ReadTimeout`, `WriteTimeout`, `IdleTimeout` on `http.Server`
- [ ] **`/readyz` endpoint** — `/healthz` always returns 200; need a readiness check that verifies the storage engine is reachable
- [ ] **Revision GC** — revisions accumulate forever; need a pruning strategy (keep last N, TTL, or manual trigger)

## Important

- [ ] **Structured access log** — log method, path, status, latency, namespace on every request
- [ ] **Client timeout** — `client.HTTP` has no timeout; expose via `WithTimeout` option
- [ ] **Pagination** — `log` and `list-branches` return unbounded results; add `limit`/`cursor`
- [ ] **TLS** — decide: native TLS (`--tls-cert`/`--tls-key`) or proxy-in-front; document the assumption either way

## Operational

- [ ] **st8d config file/env configuration** — currently flags only; add YAML config file support (same pattern as st8ctl)
- [ ] **Backup / restore** — no way to dump and reload the full database; needs API endpoint + CLI command
- [ ] **Schema migration** — model changes silently misread existing data; need a versioned migration story
- [ ] **Single-node caveat** — t4/pebble is embedded, no replication; document the HA limitation explicitly
