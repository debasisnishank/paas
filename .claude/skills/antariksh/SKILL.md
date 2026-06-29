---
name: antariksh
description: >
  Platform engineering skill for the Antariksh developer cloud project.
  Use this skill whenever working on the Antariksh codebase — adding a new Go
  service, wiring a NATS consumer, writing a Nomad job spec, adding a Firecracker
  driver module, creating a new domain entity, adding an antctl command, or
  implementing a Temporal workflow. Trigger on: "add a service", "new module",
  "wire NATS", "Nomad job", "domain type", "antctl command", "Temporal workflow",
  "billing adapter", "jurisdiction profile", or any task that touches files under
  services/, crates/, cli/, or ops/nomad/. Also trigger when the user asks how
  something should be structured or which pattern to follow in this project.
---

# Antariksh Platform Skill

## Project north star

India-first developer cloud (PaaS + Serverless Postgres).
Two bets everything hangs on — do not revisit without strong cause:
1. **Firecracker microVMs scheduled by Nomad** — tenant kernel isolation, ~125ms boot, scale-to-zero via NVMe snapshot.
2. **Neon engine (Apache 2.0) + own control plane** — O(1) CoW DB branching, storage/compute separation. Neon's control plane is closed; ours is `services/storage-cp`.

Read `CLAUDE.md` at the repo root for the full architecture, phase roadmap, and per-module checklists before starting any non-trivial task.

---

## Language split

| What | Lang | Why |
|---|---|---|
| All control-plane services, CLI | **Go** | Rich ecosystem for Nomad/Temporal/Vault/NATS; fast iteration |
| Edge proxy, Firecracker driver | **Rust** | Line-rate hot path + systems-level VM/jailer/netlink work |

Never flip this. Don't write a Go edge proxy or a Rust billing service.

---

## Playbook: Adding a new Go service

When the user asks to add a new backend service (e.g. `placement`, `certmgr`, `gateway`):

1. **Scaffold the module**
```bash
SVC=<name>
mkdir -p services/$SVC/{cmd/$SVC,internal/{workflows,activities,nats}}
cat > services/$SVC/go.mod <<EOF
module github.com/threemates/antariksh/services/$SVC

go 1.23

require (
    github.com/nats-io/nats.go v1.38.0
    go.uber.org/zap v1.27.0
)
EOF
```

2. **Write `cmd/<name>/main.go`** — use `log/slog` JSON handler, `envOr()` for config, `select {}` to park until signal. Include a comment block listing the service's responsibilities.

3. **Add to `go.work`** — append `./services/<name>` to the `use ()` block.

4. **Write `services/<name>/Dockerfile`** — copy the distroless pattern from any existing service Dockerfile (two-stage: `golang:1.23-alpine` builder → `gcr.io/distroless/static-debian12`).

5. **Add a Nomad job spec** at `ops/nomad/jobs/<name>.nomad.hcl` — include `update {}` with `auto_revert = true`, a Consul service check on `/healthz`, and a `vault {}` stanza referencing a named policy.

6. **Add Vault policy** at `ops/vault/policies/<name>.hcl` — grant read on `kv/data/antariksh/<name>/*` and `pki/issue/antariksh-services`.

7. **Add to docker-compose** at `dev/docker-compose.yml` — wire `NATS_URL`, `DATABASE_URL` if needed, and list its upstream `depends_on`.

---

## Playbook: Wiring a NATS consumer

NATS subjects follow the pattern `platform.<domain>.<event>`. The canonical subject registry is in `CLAUDE.md`.

```go
// In internal/nats/consumer.go
import "github.com/nats-io/nats.go"

func Subscribe(nc *nats.Conn, subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
    // Use QueueSubscribe for load-balanced consumers (multiple replicas)
    return nc.QueueSubscribe(subject, "workers", handler)
}
```

Key rules:
- **QueueSubscribe** for any consumer that runs with multiple replicas — Nomad will run N allocs and you don't want duplicate processing.
- **JetStream** (`nc.JetStream()`) for anything that must survive restarts — deploys, builds, metering events.
- Emit lifecycle events back onto NATS after state changes — don't assume callers poll.
- Subject for a new domain: add it to the canonical table in `CLAUDE.md` before writing code.

---

## Playbook: Adding a Temporal workflow

Workflows live in `internal/workflows/`, activities in `internal/activities/`.

```go
// internal/workflows/deploy.go
import (
    "go.temporal.io/sdk/workflow"
    "go.temporal.io/sdk/activity"
)

type DeployInput struct {
    ServiceID  string
    Image      string
    EnvID      string
}

func DeployWorkflow(ctx workflow.Context, input DeployInput) error {
    // Step 1: submit Nomad job
    err := workflow.ExecuteActivity(ctx,
        workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
            StartToCloseTimeout: 5 * time.Minute,
        }),
        SubmitNomadJobActivity, input,
    ).Get(ctx, nil)
    if err != nil {
        return err
    }
    // Step 2: wait for health check
    // Step 3: emit platform.deploy.live on NATS
    return nil
}
```

Rules:
- Never call external APIs directly from workflow functions — only from activities.
- Workflows must be deterministic. No `time.Now()`, no random, no goroutines.
- Use `workflow.ExecuteActivity` for every side effect.
- Register both workflow and activities in the Temporal worker in `cmd/<svc>/main.go`.

---

## Playbook: Adding a new domain type

All domain types live in `services/api/internal/domain/types.go`. When adding a new entity:

1. Define the ID type: `type FooID string`
2. Define the struct with JSON tags matching the DB column names.
3. If the entity has a status lifecycle, define a `FooStatus` string type with constants.
4. If it's tenant-scoped, add an `OrgID` field.
5. If it touches regulated data, add a `DataClass` field and document it.

Never add business logic to domain types — they are pure value objects.

---

## Playbook: Adding an antctl command

Commands live in `cli/cmd/<name>.go`. Follow the pattern exactly:

```go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var myCmd = &cobra.Command{
    Use:   "mycommand <arg>",
    Short: "One-line description",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // TODO: call API
        return errNotImplemented("mycommand")
    },
}

func init() {
    myCmd.Flags().StringVarP(&someFlag, "flag", "f", "", "description")
}
```

Then register it in `cli/cmd/root.go` inside `rootCmd.AddCommand(...)`.

`errNotImplemented` is defined once in `cli/cmd/open.go` — do not redefine it.

---

## Playbook: Adding a Rust module to edge-proxy or fc-driver

1. Create `crates/<crate>/src/<module>.rs` with a module-level doc comment explaining its role.
2. Declare it in `main.rs`: `mod <module>;`
3. Keep `main.rs` thin — it only initialises tracing, loads config, and starts the async runtime. All logic belongs in modules.
4. Config values: add fields to `src/config.rs` `Config` struct with serde defaults.
5. Errors: use `thiserror::Error` for domain errors, `anyhow::Result` at the call site.

---

## Playbook: Adding a jurisdiction profile

New country = new profile entry + two adapters. Never branch the platform code.

1. **Add a row to `regions` table** in `dev/init-db.sql` with the full `jurisdiction_profile` JSONB — residency rule, breach authority, payment gateway, currency, tax model, data class pinning.
2. **Add a payment adapter** in `services/billing/internal/gateways/<country>.go` implementing the `GatewayAdapter` interface (to be defined).
3. **Add a tax/invoice profile** in `services/billing/internal/tax/<country>.go`.
4. No new Nomad jobs, no new services, no new env vars — only the profile + two adapters.

Refer to `CLAUDE.md` for the MEA rollout sequence (UAE first → KSA → Nigeria/SA).

---

## Conventions (quick reference)

| Context | Rule |
|---|---|
| Go logging | `log/slog` JSON handler, `slog.SetDefault` in main |
| Go config | `envOr(key, default)` — no panics on missing env |
| Rust logging | `tracing` JSON subscriber, `RUST_LOG=info` default |
| Rust config | `config` crate, `PROXY__` env prefix, `__` separator |
| NATS multi-replica | `QueueSubscribe` always |
| NATS durable | `JetStream` for deploy/build/metering |
| Nomad jobs | `auto_revert = true` in every `update {}` stanza |
| Secrets | Vault paths only — never store secrets in our Postgres |
| Domain types | `services/api/internal/domain/types.go` — shared, no logic |
| Data residency | Tag every record at write time — placement engine decides, not humans |

---

## File locations cheat sheet

```
New Go service         services/<name>/
New Rust module        crates/<crate>/src/<module>.rs
New CLI command        cli/cmd/<name>.go  +  register in root.go
New domain type        services/api/internal/domain/types.go
New Nomad job          ops/nomad/jobs/<name>.nomad.hcl
New Vault policy       ops/vault/policies/<name>.hcl
New Temporal workflow  services/<svc>/internal/workflows/<name>.go
New NATS subject       Add to canonical table in CLAUDE.md first
New region/country     dev/init-db.sql + billing adapter + tax profile
Dev stack change       dev/docker-compose.yml
Schema change          dev/init-db.sql (dev) + migration file (prod)
```
