# Antariksh — Project Memory

> India-first developer cloud (PaaS + Serverless Postgres).
> Fly.io + Railway + Neon, built for INR/UPI/GST, architected to expand to MEA and beyond via config-driven jurisdiction profiles — never a code fork.

> ⚠️ **This file is the *intended* design. For *what is actually built today*, read [`docs/STATUS.md`](docs/STATUS.md) FIRST.** Most of what follows is roadmap, not present-tense fact — the repo is a Phase-0 scaffold. Do not assume a route/workflow/module is implemented because it appears below.
>
> The reality map below is auto-imported into every session, so it is always in context:

@docs/STATUS.md

---

## The Two Bets Everything Hangs On

1. **Firecracker microVMs scheduled by Nomad** — one guest kernel per tenant, ~125ms boot, scale-to-zero via NVMe snapshot. Reject this bet and the entire architecture changes shape.
2. **Neon engine (Apache 2.0) + own control plane** — O(1) CoW branching, storage/compute separation, scale-to-zero Postgres. Neon's control plane is closed; we own ours.

Do not revisit these decisions lightly.

---

## Stack at a Glance

| Layer | Component | Tech |
|---|---|---|
| Substrate | Nomad + Consul federation | HCL, Consul ACL |
| Compute runtime | Firecracker microVM + jailer | Rust (`crates/fc-driver`) |
| Mesh networking | WireGuard 6PN + Cilium/eBPF | netlink, rtnetlink |
| Edge proxy | TLS termination, autostart, load-shed | Rust (`crates/edge-proxy`) |
| CDN (Phase 1) | Bunny.net → own anycast cache | External → in-house |
| DNS | PowerDNS (anycast) | HTTP API from `services/dns` |
| Serverless PG | Neon engine (Pageserver + Safekeeper) | Go control plane (`services/storage-cp`) |
| Vector DB | pgvector (baked into PG image) | — |
| Block / Object | Ceph RBD / MinIO | S3-compatible |
| Build layer | Nixpacks → Buildpacks → Dockerfile + BuildKit | Go (`services/builder`) |
| Registry | zot + Trivy | OCI |
| Control plane API | REST + domain model | Go (`services/api`) |
| Scheduler | Nomad bridge + Temporal workflows | Go (`services/scheduler`) |
| Billing / Metering | Lago + Razorpay + GST e-invoice | Go (`services/billing`) |
| DNS management | PowerDNS HTTP API | Go (`services/dns`) |
| Observability | VictoriaMetrics + Loki + OTel + Tempo + Grafana | — |
| Runtime security | Falco / Tetragon (eBPF) | — |
| Secrets / Identity | Vault + SPIFFE/SPIRE | mTLS everywhere |
| Event bus | NATS JetStream | — |
| Orchestration | Temporal | Go SDK |
| CLI | antctl | Go (`cli/`) |

---

## Module Map

```
antariksh/
├── services/
│   ├── api/          Go — REST control plane, domain model, Temporal client
│   ├── scheduler/    Go — Nomad bridge, deploy/scale Temporal workflows
│   ├── billing/      Go — Lago metering, Razorpay, RBI e-mandate, GST e-invoice
│   ├── dns/          Go — PowerDNS HTTP API wrapper, ACME DNS-01
│   ├── storage-cp/   Go — Neon Pageserver/Safekeeper orchestration, branch API, PITR
│   └── builder/      Go — Nixpacks/Buildpacks/Dockerfile + BuildKit + Trivy gate
├── crates/
│   ├── edge-proxy/   Rust — hyper + rustls + tower, autostart-on-request, ACME
│   └── fc-driver/    Rust — Firecracker VMM API + jailer + Nomad gRPC plugin
├── cli/              Go — antctl (cobra): deploy, logs, scale, secrets, db, ssh, regions, open
├── ops/
│   ├── nomad/jobs/   HCL job specs per service
│   ├── consul/       Server + client config
│   └── vault/        Policies per service
├── infra/
│   ├── terraform/    Cloud + DC node provisioning
│   └── ansible/      Node agent bundle (Nomad + FC + WireGuard install)
├── dev/              docker-compose dev stack + seed scripts
├── storage/          Neon engine submodule (pageserver, safekeeper, compute-node)
└── build/nixpacks/   Custom Nixpacks builder extensions
```

---

## Domain Model

```
Org → Team → Project → Service → Deployment
                     → Environment (production | staging | pr-<n>)
                     → Database (serverless PG instance)
```

Key types live in `services/api/internal/domain/types.go`. Every region carries a `JurisdictionProfile` — `residency_rule`, `payment_gateway`, `tax_model`, `breach_authority`. Adding a country = adding a profile + two adapters, never a code branch.

**Data classes:** `personal | sensitive | payment | telemetry`
Placement engine intersects (data class × jurisdiction profile) at write time — never audited after.

---

## Phase Roadmap

### Phase 0 — Spike (4–6 wks) — *current*
- [ ] 3-node Nomad cluster on bare-metal in Mumbai colo
- [ ] Custom Firecracker task driver (`crates/fc-driver`) — gRPC plugin handshake with Nomad
- [~] Firecracker VMM boot: microVM boots to userspace via `fc-driver` (`firecracker.rs` MicroVm + `fc-driver vm-boot`, verified on KVM host). OCI-image → rootfs conversion and NVMe snapshot still TODO
- [x] Edge proxy HTTP listener (`crates/edge-proxy`) — plaintext, no TLS yet (hyper 1.x listener + reverse proxy, `/healthz`, host-based routing; verified e2e)
- [x] Deploy a container → get a public URL via the edge proxy — **driven by `antctl deploy`, now build-from-source** (box-verified, ANTCTL_BUILD_DEPLOY_OK): `antctl deploy` uploads the project → API → scheduler builds the source into an ext4 rootfs (`builder`) → `fc-driver` boots it → edge-proxy route → URL serves the BUILT app. Remaining polish: multi-VM bridge networking, route via a real Nomad job
- [ ] ACME (Let's Encrypt) DNS-01 TLS provisioning
- [~] Prove end-to-end: `antctl deploy` → … → URL — **DONE incl. build-from-source** (antctl uploads source → API → scheduler builds rootfs via `builder` → Firecracker microVM → edge-proxy URL, verified on KVM host, ANTCTL_BUILD_DEPLOY_OK). Still uses `fc-driver` directly (not via a Nomad job in this path); single-VM only

### Phase 1 — MVP PaaS (Q1)
- [ ] CLI (`antctl`) — full deploy/logs/scale/secrets/db/ssh/regions/open
- [~] `platform.toml` manifest parsing (done — `cli/internal/manifest`) + git-push-to-deploy (TODO)
- [ ] Nixpacks auto-detect build pipeline + BuildKit farm
- [ ] zot OCI registry + Trivy scan gate
- [ ] Horizontal autoscale (replica count)
- [ ] Scale-to-zero (NVMe snapshot on idle, wake on request)
- [ ] Custom domains + auto-TLS (ACME HTTP-01 + DNS-01)
- [ ] Log streaming (SSE from edge proxy → `antctl logs -f`)
- [ ] Secrets management (Vault-backed, per-env)
- [ ] NATS JetStream event bus wired across all services
- [ ] Temporal workflows: DeployWorkflow, ScaleWorkflow, DrainWorkflow
- [ ] Bunny.net CDN in front from day one
- [ ] Status page (public)
- [ ] Lago + Razorpay pay-as-you-go (prepaid credits, no e-mandate yet)

### Phase 2 — Serverless Postgres (Q2)
- [ ] Neon Pageserver + Safekeeper deployed on platform nodes
- [ ] `services/storage-cp` — tenant lifecycle, compute pool, branch API
- [ ] O(1) CoW branch per PR environment (Pageserver timeline fork)
- [ ] Scale-to-zero Postgres compute (autosuspend/resume)
- [ ] PITR — continuous WAL archiving to MinIO, point-in-time restore
- [ ] Read replicas (shared storage — cheap)
- [ ] pgvector enabled by default on all compute images
- [ ] DB billing: CU-hours → NATS → Lago

### Phase 3 — Multi-region + DR (Q3)
- [ ] Second India region (Hyderabad/Chennai)
- [ ] Anycast BGP — global PoPs lit (Bunny/CF origin-fronting)
- [ ] Cross-region failover (Nomad + Consul federation)
- [ ] Backup/DR: cross-region WAL replication, quarterly restore drills
- [ ] Overload controls: cgroup v2 + eBPF QoS + proxy concurrency limits
- [ ] Autoscale: CPU/RPS/queue-depth policy-driven
- [ ] SLA: published RTO/RPO per tier
- [ ] ClickHouse for audit + billing analytics

### Phase 4 — Compliance + Enterprise (Q4)
- [ ] ISO 27001 prep + DPDP readiness (72-hr breach notify to DPB)
- [ ] SOC 2 Type II in flight
- [ ] CERT-In: 6-hr incident reporting + 1-yr log retention
- [ ] DSAR + consent + erasure subsystem (2027 hard gate)
- [ ] Dedicated / isolated compute tiers
- [ ] PrivateLink (VPC peering for enterprise)
- [ ] RBAC, SSO/SAML/OIDC
- [ ] Immutable audit log (ClickHouse)
- [ ] RBI e-mandate recurring billing (24-hr pre-debit notify, AFA step-up, UPI Autopay + eNACH)
- [ ] GST e-invoicing (IRN/IRP) for B2B above threshold
- [ ] Direct-DC onboarding GA (node-agent installer)
- [ ] Jurisdiction-profile + data-class placement engine

### Phase 5 — AI + Own CDN + Multi-cloud (Y2 H1)
- [ ] AI-SRE copilot (reads L7 telemetry + L6 cost, proposes/executes via Temporal)
- [ ] NL → deploy / NL → manifest
- [ ] Anomaly detection + cost optimisation (idle-resource, right-sizing)
- [ ] Managed GPU inference + one-click model endpoints
- [ ] CDN in-house: cache module + image optimisation + edge KV + Wasm fns on own PoP fabric
- [ ] Cloud-region onboarding (AWS/GCP/Azure as just-another-region)
- [ ] Terraform provider

### Phase 6 — MEA + Global GA (Y2 H2)
- [ ] UAE hub region (Khazna/Equinix Dubai) — GCC CDN hub, soft residency
- [ ] KSA in-country region (Riyadh) — hard-residency profile, SAMA/NCA/ZATCA
- [ ] Nigeria PoP/region (NDPA 2023 + NIBSS/BVN localisation)
- [ ] South Africa region (POPIA)
- [ ] Per-jurisdiction payment adapter: PayTabs/Telr (GCC), Paystack/Flutterwave (Africa)
- [ ] ZATCA Fatoora e-invoicing (KSA 15% VAT)

---

## Per-Module Checklist

### `crates/fc-driver` (Rust — Phase 0 critical path)
- [ ] Nomad task driver gRPC plugin handshake (`TaskPlugin` service)
- [ ] `PreStart` — pull rootfs snapshot from shared NVMe cache by OCI digest
- [~] `Start` — spawn Firecracker process + configure vCPU/RAM/drives via API + InstanceStart (`firecracker.rs`, working). Jailer fork + TAP still TODO
- [ ] `Run` — VMM health monitor, Nomad heartbeat
- [ ] `Suspend` — guest snapshot to NVMe (autostop / scale-to-zero)
- [ ] `Resume` — restore snapshot on first inbound request (autostart)
- [~] `Stop` / `Kill` — `MicroVm::stop()` kills the VMM + cleans the socket; graceful SIGTERM + jailer chroot cleanup TODO
- [~] TAP networking — `MicroVm` attaches a host TAP NIC (`/network-interfaces`, guest IP via kernel `ip=`), host↔guest verified. Cilium eBPF + WireGuard 6PN + IPv6 ULA IPAM still TODO
- [ ] gVisor/Kata fallback runtime for FC-hostile workloads
- [ ] Snapshot GC loop (evict stale NVMe snapshots)

### `crates/edge-proxy` (Rust — Phase 0 critical path)
- [ ] TLS termination (rustls) — certs from Vault, hot-reload on SIGUSR1
- [~] SNI → tenant routing table lookup (Host-header routing done in `router.rs`; SNI-based lookup still TODO with TLS)
- [ ] Autostart-on-request: detect suspended VM, call fc-driver wake API, hold connection
- [x] Consistent-hash load-balance across healthy VM replicas (`Backend::select`, std hash; health-awareness TODO)
- [ ] Per-tenant concurrency limits (tower `LoadShed`)
- [ ] Retries + replay for idempotent requests
- [ ] ACME (Let's Encrypt DNS-01 for wildcards, HTTP-01 fallback)
- [ ] HTTP → HTTPS redirect listener
- [ ] Prometheus metrics on `:9090/metrics` (VictoriaMetrics scrape)
- [ ] NATS publish: `platform.metering.req` per request (for billing)

### `services/api` (Go)
- [~] Auth middleware — HS256 JWT bearer auth done (`internal/auth`, `POST /v1/auth/login`, protects `/v1`); SPIFFE mTLS for service-to-service still TODO
- [ ] Org / Project / Service / Environment CRUD handlers
- [~] Deploy handler — records a Deployment + `GET …/deployments`, and drives the scheduler to build-from-source + boot + route a URL (multipart source upload → `scheduler.Client.DeployWithSource`); box-verified ANTCTL_BUILD_DEPLOY_OK. NATS publish `platform.deploy.<orgID>` still TODO
- [ ] Log streaming endpoint (SSE, fan-out from Loki)
- [ ] Secrets API (proxy to Vault, never store in our DB)
- [x] Regions endpoint — `GET /v1/regions` + jurisdiction profiles (in-memory catalog in `internal/regions`; DB-backed `regions` table still TBD)
- [ ] Database API (proxy to `storage-cp`)
- [ ] Pagination, rate-limiting, request-id propagation

### `services/scheduler` (Go)
- [ ] NATS consumer on `platform.deploy.>` → trigger `DeployWorkflow` in Temporal
- [ ] `DeployWorkflow`: build → push → Nomad job submit → health check → emit `deploy.live`
- [ ] `ScaleWorkflow`: adjust Nomad job count, drain old allocs
- [ ] `DrainWorkflow`: graceful stop, snapshot, remove DNS
- [ ] Nomad event stream watcher → translate alloc events to NATS lifecycle events
- [x] Firecracker job HCL template generator from `platform.toml` service spec (`internal/nomadspec`, unit-tested; submission to a live Nomad is Linux-only)

### `services/billing` (Go)
- [ ] NATS consumer on `platform.metering.>` → Lago event ingest
- [ ] Razorpay mandate flow (AFA at registration, ₹15k no-AFA ceiling)
- [ ] 24-hr pre-debit notification + opt-out flow
- [ ] UPI Autopay + eNACH rails
- [ ] GST e-invoice generation (IRN via IRP sandbox → production)
- [ ] Per-tenant real-time spend tracker → budget alert publisher
- [ ] Jurisdiction adapter interface (swap gateway + tax profile per region)

### `services/dns` (Go)
- [ ] NATS consumer on `platform.deploy.live` → upsert A/CNAME in PowerDNS
- [ ] Custom domain attach flow (CNAME + TXT ownership proof)
- [ ] ACME DNS-01 challenge responder (TXT records via PowerDNS API)
- [ ] Wildcard cert provisioning (`*.app.antariksh.in`)
- [ ] Auto-cleanup on service delete

### `services/storage-cp` (Go)
- [ ] Pageserver tenant create/drop via HTTP management API
- [ ] Timeline (branch) fork — O(1) CoW for PR environments
- [ ] Compute node pool: spin up / autosuspend / resume Postgres microVMs
- [ ] WAL archiving watchdog → MinIO (continuous, per-tenant)
- [ ] PITR restore Temporal workflow
- [ ] Read replica provisioning
- [ ] pgvector extension enabled on all compute images
- [ ] CU-hour billing events → NATS `platform.metering.db`
- [ ] Quarterly automated restore drills (Temporal scheduled workflow)

### `services/builder` (Go)
- [ ] NATS consumer on `platform.build.>`
- [~] Builder detection: Nixpacks → Buildpacks → Dockerfile — Dockerfile path done (`internal/build`: `docker build` → export → `/init` inject → `mkfs.ext4 -d` → bootable rootfs; box-verified BUILT_ROOTFS_BOOT_OK). Nixpacks/Buildpacks auto-detect TODO
- [ ] BuildKit gRPC client with layer cache
- [ ] Ephemeral Firecracker build VM via Nomad (dogfooding)
- [ ] Trivy scan gate — block push on CRITICAL CVE
- [ ] Push to zot OCI registry, emit `platform.build.done` / `platform.build.failed`
- [ ] Build-arg + build-secret injection (Vault paths)

### `cli/` (Go — antctl)
- [~] Auth: `antctl login` (email + API key) → store token in `~/.antctl.yaml`, attach as bearer on all calls. OIDC/device flow still TODO
- [~] `deploy` — reads `platform.toml`, streams a gzip-tar of the project as the build context → API builds-from-source + boots → live URL (box-verified ANTCTL_BUILD_DEPLOY_OK); `--watch` log streaming still TODO
- [ ] `logs` — SSE stream with lipgloss level colouring
- [ ] `scale` — replica count or `--zero` for autostop
- [ ] `secrets set/get/rm/list` — proxy to API → Vault
- [ ] `db branch/connect/restore/list` — serverless PG branch API
- [ ] `ssh` — short-lived WireGuard peer creds → exec ssh via 6PN address
- [x] `regions` — tabular list with jurisdiction profile columns (calls `GET /v1/regions` via `cmd/client.go`)
- [ ] `open` — fetch URL from API, `open`/`xdg-open`
- [ ] `antctl init` — interactive scaffold `platform.toml` + link git remote

---

## Development Conventions

### Go
- `log/slog` with JSON handler everywhere — no `fmt.Println` in services
- `envOr(key, default)` pattern for all config — no panics on missing env
- Domain types in `services/api/internal/domain/types.go` — shared across services via workspace
- Temporal workflows in `internal/workflows/`, activities in `internal/activities/`
- NATS subjects: `platform.<domain>.<event>` (e.g. `platform.deploy.live`)

### Rust
- `tracing` crate with JSON subscriber — `RUST_LOG=info` default
- Config via `config` crate + `PROXY__` env prefix (double-underscore separator)
- All async with `tokio::main` — no `std::thread::spawn` in hot paths
- Error handling: `anyhow` for bin crates, `thiserror` for library crates

### Nomad jobs
- One `.nomad.hcl` file per logical service in `ops/nomad/jobs/`
- Phase 0: `docker` or `raw_exec` driver; Phase 1+: `firecracker` custom driver
- Always include `update {}` stanza with `auto_revert = true`
- Vault integration: `vault {}` stanza + named policy per job

### NATS subjects (canonical)
```
platform.deploy.<orgID>        deploy intent (API → scheduler)
platform.deploy.started        scheduler → all
platform.deploy.live           scheduler → all (triggers DNS upsert)
platform.deploy.failed         scheduler → all
platform.build.<buildID>       build request (scheduler → builder)
platform.build.done            builder → scheduler
platform.build.failed          builder → scheduler
platform.metering.req          edge-proxy → billing (per-request)
platform.metering.db           storage-cp → billing (CU-hours)
platform.metering.>            billing ingests all metering events
platform.domain.attach         API → dns (custom domain flow)
platform.db.>                  storage-cp lifecycle events
```

---

## Key Commands

```bash
# Dev stack
make dev              # docker compose up -d (Postgres, NATS, Vault, Temporal, Grafana, MinIO, zot)
make dev-down         # tear down + wipe volumes
make dev-seed         # load Vault secrets + Consul ACL tokens

# Build
make build-go         # build all Go services
make build-rust       # cargo build --release --workspace
make antctl           # build antctl → cli/dist/antctl

# Test + lint
make test             # go test + cargo test
make lint             # golangci-lint + cargo clippy

# Nomad
make nomad-plan       # dry-run all job specs
make nomad-run        # submit all jobs to Nomad cluster

# antctl (once built)
antctl deploy         # deploy from cwd platform.toml
antctl logs <svc> -f  # stream logs
antctl db branch <db> # O(1) CoW DB branch
antctl regions        # list regions + jurisdiction profiles
```

---

## Critical Architecture Warnings

1. **Edge proxy is the single most load-bearing binary.** Most platform incidents will originate here. Routing + TLS + autostart + load-shed must all be correct at line rate.
2. **Data-class placement must be enforced at write time.** A placement error = residency violation (SDAIA/SAMA/DPB fines). Tag every record; the placement engine decides, not humans after the fact.
3. **RBI e-mandate is a regulated system.** Mis-handling pre-debit notify / AFA / mandate caps = compliance failure + trust failure.
4. **Untested backups are theatre.** Automate quarterly restore drills from day one.
5. **Cache must be residency-aware.** Static/public assets cache globally; anything tagged `personal | sensitive | payment` is `no-store` at foreign PoPs.

---

## External Accounts / Keys Needed (not in repo)

- [ ] Razorpay API key (sandbox → production)
- [ ] IRP sandbox credentials (GST e-invoice)
- [ ] Let's Encrypt account key (ACME)
- [ ] Bunny.net API key + Pull Zone ID
- [ ] Mumbai colo SSH access + IPMI credentials
- [ ] NIXI peering letter of intent
- [ ] Consul gossip key (`consul keygen`)
- [ ] Vault unseal keys + root token (production — never commit)

---

*Last updated: Phase 0 scaffold complete. Stack confirmed: Go 1.23.4 / Rust 1.96.0 / Nomad 2.0.3 / Docker 29.5.2 / protoc 35.1 / golangci-lint 2.12.2*
