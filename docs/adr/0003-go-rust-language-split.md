# ADR-0003: Go for the control plane, Rust for the hot path

- **Status:** Accepted
- **Date:** 2026-06-29
- **Deciders:** Founding team

## Context

We are building two very different kinds of software:

1. **Control-plane services** — REST API, scheduler/Temporal workflows, billing,
   DNS, storage control plane, builder, CLI. These are I/O-bound glue over Nomad,
   Temporal, Vault, NATS, PowerDNS, Lago/Razorpay. Iteration speed and ecosystem
   breadth matter most; raw latency does not.
2. **The data-plane hot path** — the edge proxy (TLS termination + routing +
   autostart + load-shed at line rate) and the Firecracker driver (jailer, VMM
   lifecycle, netlink/rtnetlink, snapshotting). These are latency-critical and/or
   systems-level, with no GC pauses tolerable on the request path.

Using one language for both would compromise one side: Go's GC and weaker
systems-level story hurt the hot path; Rust's slower iteration and thinner
ops-ecosystem (Nomad/Temporal/Vault SDKs) would tax the control plane.

## Decision

- **Go** for all control-plane services and the `antctl` CLI (`services/*`, `cli/`).
  Rich SDKs for Nomad/Temporal/Vault/NATS, fast iteration, easy hiring.
- **Rust** for the edge proxy (`crates/edge-proxy`) and the Firecracker task driver
  (`crates/fc-driver`). Line-rate performance, no GC, first-class systems access
  (jailer, netlink, Firecracker VMM API), memory safety on a security-critical path.

**This split is firm:** do not write a Go edge proxy or a Rust billing service.

## Consequences

- **Easier:** each side uses the language that fits; the most load-bearing and most
  security-sensitive binaries get memory safety + predictable latency.
- **Harder:** two toolchains, two CI lanes, a smaller pool of engineers fluent in
  both. Shared types cross the boundary via APIs/protobuf, not a shared library.
- **Accepted:** some duplication of small helpers across the boundary. We prefer
  that to a lowest-common-denominator monolang.

## Conventions that follow from this

- **Go:** `log/slog` JSON handler; `envOr(key, default)` config; domain types in
  `services/api/internal/domain/types.go`; Temporal workflows in
  `internal/workflows/`.
- **Rust:** `tracing` JSON subscriber; `config` crate with `PROXY__` env prefix;
  `tokio` async (no `std::thread::spawn` in hot paths); `thiserror` for libraries,
  `anyhow` for binaries.

## Alternatives considered

- **All Go** — simplest org, but GC pauses and a weaker systems story on the proxy/
  driver hot path. Rejected for the data plane.
- **All Rust** — uniform and fast, but materially slower iteration on the broad,
  I/O-bound control plane and thinner orchestration SDKs. Rejected for the control
  plane.

## Related

- ADR-0001 (the driver is a systems-level Rust component)
- See `docs/STATUS.md` for which of these binaries actually exist yet (most Rust
  modules are still 1-line stubs).
