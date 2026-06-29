# Antariksh

> India-first developer cloud — **PaaS + Serverless Postgres**. Firecracker microVMs
> on Nomad, the Neon engine with our own control plane, built for INR/UPI/GST and
> architected to expand to MEA via config-driven jurisdiction profiles (never a code
> fork).

---

## ⚠️ Read this first

This repository is an early **Phase-0 scaffold**. Most of what the design docs
describe is **not built yet** — many files are stubs.

- **What is actually built today → [`docs/STATUS.md`](docs/STATUS.md)** (authoritative).
- **What we're aiming at (full architecture + roadmap) → [`CLAUDE.md`](CLAUDE.md)**.

When the two disagree: `STATUS.md` wins for "what exists," `CLAUDE.md` wins for
"what's intended." Do not assume a route/workflow/module is implemented just because
`CLAUDE.md` mentions it.

## Documentation map

| Doc | Purpose |
|---|---|
| [`docs/STATUS.md`](docs/STATUS.md) | Reality map — per-module STUB / PARTIAL / WORKING / MISSING. Start here. |
| [`CLAUDE.md`](CLAUDE.md) | Intended architecture, stack, phase roadmap, per-module checklists, conventions. |
| [`docs/adr/`](docs/adr/) | Architecture Decision Records — *why* the load-bearing choices were made. |
| [`docs/GLOSSARY.md`](docs/GLOSSARY.md) | Exact meanings of domain & infra terms (6PN, jailer, CoW branch, data-class, …). |
| [`.claude/skills/antariksh/SKILL.md`](.claude/skills/antariksh/SKILL.md) | Playbooks for *how* to add a service / workflow / CLI command / region in this repo. |

## The two bets everything hangs on

1. **Firecracker microVMs scheduled by Nomad** — per-tenant kernel isolation,
   ~125ms boot, scale-to-zero via NVMe snapshot. ([ADR-0001](docs/adr/0001-firecracker-microvms-on-nomad.md))
2. **Neon engine (Apache 2.0) + our own control plane** — O(1) CoW DB branching,
   storage/compute separation, scale-to-zero Postgres. ([ADR-0002](docs/adr/0002-neon-engine-own-control-plane.md))

Do not revisit these lightly — see the ADRs.

## Repository layout

```
services/   Go control plane — api, scheduler, billing, dns, storage-cp, builder
crates/     Rust hot path    — edge-proxy, fc-driver
cli/        Go               — antctl (deploy/logs/scale/secrets/db/ssh/regions/open)
ops/        Nomad jobs, Consul + Vault config
infra/      Terraform + Ansible   (placeholder — not yet populated)
dev/        docker-compose dev stack + seed scripts
storage/    Neon engine submodule (placeholder — not yet checked out)
build/      Custom Nixpacks builder extensions (placeholder)
docs/       STATUS, ADRs, glossary
```

(For the live state of each of these, see [`docs/STATUS.md`](docs/STATUS.md).)

## Language split

- **Go** — all control-plane services + the CLI (I/O-bound glue over Nomad, Temporal,
  Vault, NATS).
- **Rust** — the edge proxy and Firecracker driver (line-rate hot path + systems-level
  VM/jailer/netlink work).

Firm boundary — see [ADR-0003](docs/adr/0003-go-rust-language-split.md). Don't write a
Go edge proxy or a Rust billing service.

## Getting started (dev)

> Toolchain pins (last confirmed): Go 1.23.4 · Rust 1.96.0 · Nomad 2.0.3 ·
> Docker 29.5.2. See the `Makefile` for the full target list.

```bash
make dev          # docker compose up (Postgres, NATS, Vault, Temporal, Grafana, MinIO, zot)
make dev-seed     # load Vault secrets + Consul ACL tokens
make build-go     # build all Go services
make build-rust   # cargo build --release --workspace
make antctl       # build antctl → cli/dist/antctl
make test         # go test + cargo test
make lint         # golangci-lint + cargo clippy
```

## Working in this repo with an AI assistant

`CLAUDE.md` is auto-loaded into every Claude Code session and `@import`s
`docs/STATUS.md`; a `SessionStart` hook (`.claude/hooks/session-context.sh`) also
injects the reality map so the assistant is grounded in what's actually built. Keep
`docs/STATUS.md` current — it is the single source of truth for build state, and
everything else points at it.
