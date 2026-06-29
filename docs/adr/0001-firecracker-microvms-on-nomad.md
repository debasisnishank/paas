# ADR-0001: Firecracker microVMs scheduled by Nomad

- **Status:** Accepted
- **Date:** 2026-06-29
- **Deciders:** Founding team
- **Tags:** load-bearing — do not revisit without a superseding ADR

## Context

We need a compute substrate that gives every tenant strong isolation, boots fast
enough to support scale-to-zero (wake-on-request without users noticing), and
bin-packs cheaply on bare metal in a Mumbai colo. The realistic options:

- **Shared-kernel containers** (Docker/containerd, K8s) — cheap and fast, but a
  shared host kernel is a soft tenant boundary. A kernel escape is a multi-tenant
  breach, which is fatal for a platform that will hold `personal | sensitive |
  payment` data under DPDP/SAMA-style regimes.
- **Full VMs** (QEMU/KVM, Firecracker's heavyweight cousins) — strong isolation
  but multi-second boots and large memory overhead; scale-to-zero becomes painful.
- **Firecracker microVMs** — one guest kernel per tenant (hard isolation), ~125ms
  boot, minimal device model, snapshot/restore to NVMe for suspend/resume.

We also need an orchestrator. Kubernetes assumes containers and a shared kernel;
bending it to drive microVMs fights the grain. Nomad has a documented custom task
driver plugin interface, runs a single binary, and federates cleanly with Consul —
a better fit for a bare-metal-first, microVM-first design.

## Decision

Use **Firecracker microVMs as the unit of tenant compute**, **scheduled by Nomad**
via a **custom task driver** (`crates/fc-driver`, Rust) that speaks the Nomad task
driver gRPC plugin protocol. Scale-to-zero is implemented as guest snapshot to NVMe
on idle, restore on first inbound request (autostart, driven by `crates/edge-proxy`).

## Consequences

- **Easier:** hard per-tenant isolation; cheap idle (snapshotted VMs cost ~storage,
  not RAM); fast wake; a clean story for residency/compliance.
- **Harder:** we own a systems-level Rust driver (jailer, VMM lifecycle, TAP/eBPF
  networking, snapshot GC) — this is the Phase-0 critical path and a real cost.
- **Accepted:** some workloads are FC-hostile (need full device model / nested
  virt). We accept carrying a gVisor/Kata fallback runtime rather than abandoning
  microVMs as the default.
- This decision shapes the entire architecture. Rejecting it (e.g. moving to K8s +
  containers) is not a config change — it reshapes networking, the scheduler bridge,
  the edge proxy's wake path, and the isolation/compliance model.

## Alternatives considered

- **Kubernetes + Kata Containers** — gets microVM isolation but inherits K8s
  operational weight and a container-shaped mental model we'd constantly fight.
- **Plain containers + strong seccomp/AppArmor** — cheaper, but a shared kernel is
  not a defensible tenant boundary for regulated data.
- **gVisor as the default** — good isolation via a userspace kernel, but higher
  syscall overhead and worse compatibility than a real guest kernel. Kept as a
  fallback (see `crates/fc-driver` checklist), not the default.

## Related

- ADR-0003 (why the driver is Rust)
- `crates/fc-driver` per-module checklist in `CLAUDE.md`
- Reality: this is Phase-0 and **not yet built** — see `docs/STATUS.md`.
