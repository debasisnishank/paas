# ADR-0002: Neon engine + our own control plane

- **Status:** Accepted
- **Date:** 2026-06-29
- **Deciders:** Founding team
- **Tags:** load-bearing — do not revisit without a superseding ADR

## Context

A core product promise is **serverless Postgres**: scale-to-zero compute, O(1)
copy-on-write branches (one per PR environment), point-in-time restore, and cheap
read replicas over shared storage. Building this storage engine from scratch is a
multi-year effort. The options:

- **Build our own storage/compute-separated Postgres** — years of work, high risk.
- **Use a managed provider** (Neon cloud, Supabase, RDS) — abdicates the margin and
  the data-residency control we need for India/MEA jurisdictions; their control
  plane and placement are not ours to govern.
- **Run the Neon *engine*** (Pageserver + Safekeeper + compute-node) — Apache-2.0
  licensed, gives us storage/compute separation, WAL-based durability, and timeline
  forking (the primitive behind O(1) branches). Neon's *control plane* (tenant
  lifecycle, compute pool, billing, branch API) is closed-source.

## Decision

Adopt the **Neon engine (Apache 2.0)** as the storage substrate, and **build our
own control plane** in `services/storage-cp` (Go): tenant create/drop, compute pool
spin-up/autosuspend/resume, branch (timeline fork) API, WAL archiving to MinIO,
PITR, and read-replica provisioning. The engine lives as a submodule under
`storage/`.

## Consequences

- **Easier:** we get a battle-tested storage engine and its branching primitive for
  free; we keep full control of placement, residency, and billing (CU-hours).
- **Harder:** we own the orchestration of a complex stateful system (Pageserver/
  Safekeeper failure modes, WAL archiving correctness, restore drills). Operating
  someone else's engine means tracking upstream.
- **Accepted:** we are coupled to the Neon engine's on-disk formats and APIs. A hard
  upstream breaking change is a real risk we accept in exchange for not building a
  storage engine ourselves.
- **Non-negotiable discipline:** PITR and cross-region WAL replication are worthless
  untested. Automated quarterly restore drills are required from day one (see
  `CLAUDE.md` warning #4).

## Alternatives considered

- **Neon cloud (managed)** — fastest to ship, but cedes margin, residency control,
  and control-plane ownership. Defeats the India-first thesis.
- **Patroni/Stolon + vanilla Postgres** — proven HA, but no storage/compute split,
  no O(1) branching, no cheap scale-to-zero. Wrong primitive for the product.
- **Vitess / CockroachDB / Yugabyte** — different consistency and operational model;
  not Postgres-wire-faithful in the way our users expect, and no CoW branch story.

## Related

- ADR-0001 (compute microVMs — DB compute nodes run as microVMs too)
- `services/storage-cp` per-module checklist in `CLAUDE.md`
- Reality: Phase-2 work, **not yet built**; `storage/` is an empty placeholder —
  see `docs/STATUS.md`.
