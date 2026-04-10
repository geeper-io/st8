---
title: Quick Start
description: Get st8 running in five minutes.
---

import { Tabs, TabItem, Steps } from '@astrojs/starlight/components';

This guide gets you from zero to a working st8 setup in about five minutes.

## Install

<Tabs>
  <TabItem label="Go install">
    ```bash
    go install github.com/geeper-io/st8/cmd/st8d@latest
    go install github.com/geeper-io/st8/cmd/st8ctl@latest
    ```
  </TabItem>
  <TabItem label="Build from source">
    ```bash
    git clone https://github.com/geeper-io/st8
    cd st8
    go build -o st8d ./cmd/st8d
    go build -o st8ctl ./cmd/st8ctl
    ```
  </TabItem>
</Tabs>

## Option A: Local demo (no server needed)

Use `--local` to spin up an embedded st8d instance backed by a local directory. Perfect for development and testing.

```bash
# Write some config
cat > feature_flags.json <<'EOF'
{"dark_mode": true, "new_checkout": false}
EOF
st8ctl --local apply feature_flags.json \
  --message "initial config"

# Read it back
st8ctl --local get

# Change a value
cat > feature_flags.json <<'EOF'
{"dark_mode": true, "new_checkout": true}
EOF
st8ctl --local apply feature_flags.json \
  --message "enable new checkout"

# See the history
st8ctl --local log
```

## Option B: Remote st8d server

<Steps>

1. **Start st8d**

   ```bash
   st8d --listen :8748 --state-dir /var/lib/st8
   ```

   With token auth:
   ```bash
   st8d --listen :8748 --state-dir /var/lib/st8 --token mysecrettoken
   ```

2. **Configure st8ctl**

   Create `~/.config/st8ctl/config.yaml`:

   ```yaml
   server:
     url: http://localhost:8748
     token: mysecrettoken   # omit if no auth

   defaults:
     namespace: default
     branch: main
   ```

3. **Push config**

   ```bash
   cat > rate_limits.json <<'EOF'
   {"api": 1000, "search": 100}
   EOF
   st8ctl apply rate_limits.json \
     --message "initial config"
   ```

4. **Read config in your app**

   ```go
   package main

   import (
       "context"
       "fmt"
       "log"

       st8 "github.com/geeper-io/st8/client"
   )

   func main() {
       client := st8.NewHTTP("http://localhost:8748",
           st8.WithToken("mysecrettoken"),
       )

       result, err := client.Get(context.Background(), st8.Scope{
           Namespace: "default",
           Branch:    "main",
       }, 0, "")
       if err != nil {
           log.Fatal(err)
       }

       for key, value := range result.Objects {
           fmt.Printf("%s = %s\n", key, value)
       }
   }
   ```

</Steps>

## Checkpoint and rollback

```bash
# Tag the current state
st8ctl checkpoint stable --description "Verified good state"

# Later, if something goes wrong
st8ctl rollback --checkpoint stable --message "Revert to stable"
```

## Next steps

- [Concepts](/guides/concepts/) — understand namespaces, branches, and revisions
- [Dynamic Configuration](/guides/dynamic-config/) — patterns for runtime config
- [A/B Testing](/guides/ab-testing/) — run experiments at the config layer
