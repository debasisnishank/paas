# Glossary

Exact meanings of terms used across Antariksh, so they aren't conflated or guessed.
Terms are grouped by area. When a term maps to a directory or type, the location is
given. For *what's built vs. planned*, always check [`STATUS.md`](STATUS.md).

## Compute & isolation

- **Firecracker** — a minimal VMM (virtual machine monitor) that runs lightweight
  **microVMs** with a stripped-down device model. Our unit of tenant compute. See
  [ADR-0001](adr/0001-firecracker-microvms-on-nomad.md).
- **microVM** — a full VM (its own guest kernel) but minimal/fast: ~125ms boot, low
  memory overhead. Stronger isolation than a container (no shared host kernel).
- **jailer** — Firecracker's companion that sandboxes each VMM process (chroot,
  cgroups, namespaces, dropped privileges) before the VM starts. One jailer per VM.
- **VMM** — Virtual Machine Monitor; the Firecracker process supervising one microVM.
- **fc-driver** — our Rust Nomad task driver (`crates/fc-driver`) that makes
  Firecracker a Nomad-schedulable workload via the task-driver gRPC plugin protocol.
- **rootfs snapshot** — an OCI image converted to a root filesystem image, cached on
  shared NVMe and addressed by OCI digest, used to boot a microVM.
- **snapshot / restore** — Firecracker can serialize a running guest's memory+state
  to NVMe (snapshot) and later resume it (restore). The mechanism behind
  scale-to-zero.
- **scale-to-zero** — suspend an idle workload (snapshot to NVMe, free RAM) and
  resume it on the next request, so idle cost ≈ storage only.
- **autostart / autostop** — autostop = snapshot an idle VM; autostart = the edge
  proxy detects a request for a suspended VM, wakes it (restore), and holds the
  connection until it's ready.
- **gVisor / Kata** — alternative sandbox runtimes kept as a **fallback** for
  workloads Firecracker can't host. Not the default (see ADR-0001).

## Scheduling & orchestration

- **Nomad** — HashiCorp's scheduler. Drives where workloads run; we extend it with
  the custom `fc-driver` task driver. Federates with Consul.
- **Consul** — service discovery + health checking + ACL/service mesh substrate;
  federated alongside Nomad.
- **Temporal** — durable workflow engine. Long-running, retryable orchestrations
  (DeployWorkflow, ScaleWorkflow, DrainWorkflow, PITR restore) live here.
- **workflow vs. activity** (Temporal) — a *workflow* is deterministic
  orchestration code (no `time.Now()`, no I/O, no goroutines); an *activity* is where
  all side effects / external calls happen. Workflows call activities.
- **task driver** — a Nomad plugin that knows how to run one kind of workload
  (docker, raw_exec, or our `firecracker`). Speaks a gRPC handshake to Nomad.
- **alloc** (allocation) — Nomad's term for one running instance of a task on a node.

## Networking & edge

- **edge-proxy** — the Rust data-plane binary (`crates/edge-proxy`): TLS
  termination, SNI→tenant routing, autostart-on-request, load-shedding, per-request
  metering. The single most load-bearing binary (`CLAUDE.md` warning #1).
- **6PN (IPv6 Private Network)** — per-tenant private IPv6 (ULA) mesh over WireGuard;
  each microVM gets an address from tenant IPAM. `antctl ssh` reaches a VM over 6PN.
- **WireGuard** — the encrypted tunnel layer carrying the 6PN mesh.
- **Cilium / eBPF** — kernel-level (eBPF) networking/policy; TAP devices from each VM
  are wired through Cilium into the 6PN.
- **TAP device** — a virtual L2 network interface the VMM uses for guest networking.
- **IPAM** — IP Address Management; allocates per-tenant 6PN addresses.
- **SNI routing** — using the TLS Server Name Indication hostname to pick which
  tenant/VM a connection routes to.
- **load-shed** — deliberately rejecting/queuing excess load (tower `LoadShed`,
  per-tenant concurrency limits) to protect the system under overload.
- **ACME / DNS-01 / HTTP-01** — the Let's Encrypt certificate-issuance protocol;
  DNS-01 proves domain control via a TXT record (needed for wildcards), HTTP-01 via
  an HTTP path. Provisioned by edge-proxy + `services/dns`.
- **PoP (Point of Presence)** — an edge/CDN location. Phase 1 uses Bunny.net; later
  our own anycast fabric.
- **anycast** — one IP advertised from many locations; clients reach the nearest PoP.

## Serverless Postgres (Neon)

- **Neon engine** — the Apache-2.0 storage engine we run (Pageserver + Safekeeper +
  compute-node). We build our own control plane around it. See
  [ADR-0002](adr/0002-neon-engine-own-control-plane.md).
- **Pageserver** — Neon's storage server: stores Postgres pages, serves them to
  compute nodes, manages timelines. Storage half of storage/compute separation.
- **Safekeeper** — Neon's WAL durability layer; safely persists the write-ahead log
  (typically a quorum) before it reaches the Pageserver.
- **compute-node** — a stateless Postgres process that gets its pages from the
  Pageserver. Can scale to zero and resume; the "compute" half.
- **storage/compute separation** — Postgres compute is stateless and disposable;
  durable state lives in Pageserver/Safekeeper. Enables cheap replicas + scale-to-0.
- **timeline** — Neon's term for a branch of database history (a WAL lineage).
- **CoW branch (O(1) branching)** — a copy-on-write fork of a timeline; creating a
  branch (e.g. per PR environment) is constant-time because pages are shared until
  written. Exposed as `antctl db branch`.
- **PITR (Point-In-Time Restore)** — restore the database to any moment, by replaying
  archived WAL. Requires continuous WAL archiving (to MinIO) — and **tested** restore
  drills (`CLAUDE.md` warning #4).
- **WAL (Write-Ahead Log)** — Postgres's durability log; the source of truth for
  replication, PITR, and Safekeeper durability.
- **CU-hour (Compute-Unit hour)** — the billing unit for serverless PG compute;
  metered and emitted to billing via NATS `platform.metering.db`.
- **pgvector** — Postgres extension for vector similarity search; baked into all
  compute images by default.
- **storage-cp** — our Go control plane (`services/storage-cp`) for the Neon engine:
  tenant lifecycle, compute pool, branch API, WAL archiving, PITR.

## Build & registry

- **Nixpacks / Buildpacks / Dockerfile** — the build-strategy detection order: try
  Nixpacks auto-detection first, then Cloud Native Buildpacks, then a raw Dockerfile.
- **BuildKit** — the build backend (with layer caching) that actually produces images.
- **zot** — the OCI image registry we run.
- **Trivy** — vulnerability scanner; a gate that blocks pushing an image on a
  CRITICAL CVE.
- **OCI digest** — the content-addressed `sha256:…` identifier of an image; how
  deployments and rootfs snapshots reference an exact image.

## Domain model (control plane)

- **Org → Team → Project → Service → Deployment** — the entity hierarchy. Types in
  `services/api/internal/domain/types.go`.
- **Environment** — a named deploy target within a project: `production`, `staging`,
  or `pr-<n>` (ephemeral preview, often with its own CoW DB branch).
- **Service** — a deployable unit (`app | worker | cron | database`).
- **Deployment** — an immutable record of a service version live in an environment.
- **PlanTier** — `shared` (bin-packed, burstable) / `dedicated` (pinned hosts) /
  `enterprise` (whole-node/rack, isolated DC).

## Jurisdiction & compliance

- **JurisdictionProfile** — per-region config (residency, breach authority, payment
  gateway, currency, tax model, data-class pinning) that lets a new country be added
  as data, not code. See [ADR-0004](adr/0004-config-driven-jurisdiction-profiles.md).
- **DataClass** — a record's sensitivity tag: `personal | sensitive | payment |
  telemetry`. Drives placement and cache rules.
- **ResidencyRule** — `none` (no restriction) / `soft` (local copy, SCC-style
  transfer allowed) / `hard` (must stay in-country, e.g. KSA, payment data).
- **placement engine** — decides at *write time* where a record may live, by
  intersecting `DataClass` × `JurisdictionProfile`. Never an after-the-fact audit
  (`CLAUDE.md` warning #2).
- **DPDP / DPB** — India's Digital Personal Data Protection Act; the Data Protection
  Board is its breach authority (72-hour notification).
- **CERT-In** — India's cyber-incident authority: 6-hour incident reporting, 1-year
  log retention.
- **RBI e-mandate** — Reserve Bank of India rules for recurring payments (pre-debit
  notification, AFA step-up, mandate caps); a regulated system (`CLAUDE.md` warning #3).
- **AFA (Additional Factor of Authentication)** — the extra auth step RBI requires
  at mandate registration / above thresholds.
- **GST / IRP / IRN** — India's Goods & Services Tax; e-invoices get an Invoice
  Reference Number (IRN) from the Invoice Registration Portal (IRP).
- **ZATCA Fatoora** — Saudi Arabia's e-invoicing system (15% VAT) for the KSA region.
- **SDAIA / SAMA / NCA** — Saudi data-protection, central-bank, and cybersecurity
  authorities (relevant to the KSA hard-residency profile).
- **POPIA / NDPA** — South Africa's and Nigeria's data-protection regimes.

## Platform plumbing

- **NATS / JetStream** — the event bus. Subjects follow `platform.<domain>.<event>`
  (canonical list in `CLAUDE.md`). JetStream = the durable/persistent variant for
  events that must survive restarts (deploy/build/metering).
- **Vault** — secrets + PKI (mTLS certs). Secrets live in Vault paths only — never in
  our Postgres.
- **SPIFFE / SPIRE** — workload identity; issues the mTLS identities services use to
  authenticate to each other.
- **Lago** — open-source metering/billing engine that ingests usage events.
- **Razorpay** — the India payment gateway (the IN `payment_gateway` adapter).
- **PowerDNS** — authoritative DNS we drive via its HTTP API (`services/dns`).
- **VictoriaMetrics / Loki / Tempo / OTel / Grafana** — metrics / logs / traces /
  instrumentation / dashboards (the observability stack).
- **Falco / Tetragon** — eBPF-based runtime security monitoring.
- **antctl** — the Go CLI (`cli/`); the primary user interface (`deploy`, `logs`,
  `scale`, `secrets`, `db`, `ssh`, `regions`, `open`, `init`).
- **platform.toml** — the per-project manifest (`platform.toml.example` at repo root)
  describing services, build config, and deploy settings.
