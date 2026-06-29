# STATUS — what is actually built

> **Read this before touching code.** `CLAUDE.md` describes the *intended* platform
> (the full architecture and roadmap). This file describes what *currently exists in
> the tree*. When the two disagree, **this file wins for "what's built"; CLAUDE.md
> wins for "what we're aiming at."**
>
> Phase: **0 — Spike**. The repo is a scaffold. Most logic is not written yet.
> If you are asked to change behavior, first confirm here whether that behavior
> exists. Do not assume a function/route/workflow is implemented because CLAUDE.md
> mentions it.

Legend:
- **STUB** — file/dir exists but has no real logic (boots and parks, returns `501`/`not implemented`, or is a 1-line placeholder).
- **PARTIAL** — real skeleton present (router, config, types) but core behavior unimplemented.
- **WORKING** — does its job end to end.
- **MISSING** — described in CLAUDE.md but not present in the tree.

_Last verified: 2026-06-29 by reading the tree. Update the date when you change status._

> ✅ **Build blocker RESOLVED (2026-06-29):** `services/scheduler/go.mod` previously
> pinned `github.com/hashicorp/nomad/api` to a fake placeholder pseudo-version that
> broke the whole-workspace module graph. The unused `nomad/api` require was removed
> (nothing imports it yet; it will be re-added with a pinned, Go 1.23-compatible
> version when the Nomad bridge is implemented — see the NOTE in that go.mod). Also
> fixed three latent Makefile bugs this had been masking: `build-go`/`lint-go`/`test-go`
> used root-level `./...` patterns that fail in a multi-module `go.work` layout — all
> three now iterate per-module. **`make build-go`, `make lint-go`, `make test-go`, and
> `make antctl` all pass workspace-wide.**

> 🐧 **Linux validation host (2026-06-29):** `nsk-desktop` — Linux Mint 22.3
> (Ubuntu 24.04 base, kernel 6.17), bare-metal Intel i5-2320, **VT-x enabled**,
> `/dev/kvm` present (user has rw via session ACL — Firecracker runs without sudo).
> Toolchains installed user-local: Go 1.23.4, Rust 1.94, Nomad v2.0.3, Consul v2.0.1,
> Firecracker/Jailer v1.16.0. **The full workspace builds + tests on Linux, including
> `fc-driver`** (netlink/nix deps compile under `cfg(target_os=linux)`).
> **Firecracker bet VALIDATED on hardware:** booted a microVM end to end under KVM —
> guest kernel 5.10.223 + Alpine 3.24.1 userspace via custom `/init`, clean halt
> (boot scripts in the session scratchpad; reproduce with kernel `vmlinux-5.10.223`
> + an `mkfs.ext4 -d` Alpine rootfs). Sudo on the box needs a password (installs/runs
> are user-local; cluster-run as root is the remaining gap). This is a single-node dev
> box, *not* the Mumbai colo. Note: i5-2320 is pre-Skylake — basic boot works;
> snapshot/resume (scale-to-zero) is unproven on this CPU.
>
> **Nomad → Firecracker proven (2026-06-29):** a 1-node dev Nomad (`nomad agent -dev`,
> non-root, `raw_exec` enabled) scheduled a batch job that ran `fc-driver vm-boot` →
> microVM booted Alpine to userspace, task exited 0 (`NOMAD_FIRECRACKER_E2E_OK`).
> This is the **raw_exec** path (per CLAUDE.md's Phase-0 Nomad convention); the custom
> `firecracker` task-driver gRPC plugin is still future work. fc-driver binary copied
> to `~/.local/bin/fc-driver` for raw_exec; the dev agent is not left running.
>
> **deploy→URL proven (2026-06-29):** `fc-driver` boots a microVM with a TAP NIC
> (host `tap0` 172.16.0.1, guest `eth0` 172.16.0.2 via kernel `ip=`) running a
> static Go HTTP server as `/init`; **edge-proxy routes `Host: app.local` →
> 172.16.0.2:80 → guest response** (`DEPLOY_TO_URL_OK`). Needs passwordless sudo
> (enabled) for the TAP; the tap is user-owned so Firecracker still runs rootless.
> **Full chain proven (2026-06-30, ANTCTL_DEPLOY_TO_URL_OK):** `antctl login` →
> `antctl deploy` → API (`SCHEDULER_URL`) → scheduler `POST /internal/deploy`
> (`internal/schedhttp`) → orchestrator boots the microVM via `fc-driver` →
> registers the route on edge-proxy → `http://web.local` resolves. All four
> services + antctl built from the tree and run on the box.

---

## Services (Go)

| Module | Status | What actually exists |
|---|---|---|
| `services/api` | **PARTIAL** | `cmd/api/main.go`: chi router, graceful shutdown, `/healthz`. **Auth WORKING** — HS256 JWT (`internal/auth`, stdlib, no external dep), `POST /v1/auth/login` (email + `API_KEY` → bearer token), `authMiddleware` protects the whole `/v1` group; `cmd/api/auth.go` + tests passing. **`GET /v1/regions` WORKING** (now behind auth) — `cmd/api/regions.go` serving an in-memory catalog (`internal/regions`: Mumbai available; Hyderabad/Dubai/Riyadh planned) with jurisdiction profiles; `regions_test.go` passing. **Deploy intent WORKING** — `POST …/services/{id}/deploy` records a pending Deployment and `GET …/deployments` lists them (`cmd/api/deploy.go`, in-memory `internal/deploystore`), both authed; tested. All *other* `/v1` routes still `stubHandler` → `501`. **Real:** `internal/domain/types.go`. Dev defaults `API_AUTH_SECRET`/`API_KEY` warn at boot. No DB, NATS, Vault, Temporal, or SPIFFE mTLS yet (deploy is not yet published to the scheduler). |
| `services/scheduler` | **PARTIAL** | `cmd/scheduler/main.go` still boots-and-parks. **New, all unit-tested (Mac):** `internal/nomadspec` (Firecracker Nomad job-spec HCL generator + `ParseMemoryMB`); `internal/ipam` (per-VM guest IP/TAP/MAC allocator from 172.16.0.0/24); `internal/edgeproxy` (admin-API client: register/remove routes); `internal/orchestrator` (`Deploy()` = allocate IP → boot VM via a `VMRunner` seam → register route → return URL, with unwind on failure). `internal/runner` (`Exec` VMRunner — real: sets up TAP via sudo, runs `fc-driver` in its own process group, waits for guest HTTP, kills the group on stop). `cmd/scheduler` has a **`scheduler deploy <service> [image]`** subcommand wiring it all. **Box-verified (AUTODEPLOY_OK):** one command allocated an IP, set up the TAP, booted a microVM, and registered `web.local` → guest on edge-proxy; the URL resolved. Caveats: single-VM (multi-VM needs a bridge, not per-tap host IPs); `antctl deploy`→API→scheduler wiring and NATS/Temporal still TODO. |
| `services/billing` | **STUB** | `cmd/billing/main.go` (~35 LOC): boots and parks. No Lago, Razorpay, GST, or metering consumer. |
| `services/dns` | **STUB** | `cmd/dns/main.go` (~31 LOC): boots and parks. No PowerDNS client, no ACME responder. |
| `services/storage-cp` | **STUB** | `cmd/storage-cp/main.go` (~38 LOC): boots and parks. No Pageserver/Safekeeper orchestration, no branch API, no PITR. |
| `services/builder` | **STUB** | `cmd/builder/main.go` (~39 LOC): boots and parks. No Nixpacks/BuildKit/Trivy. |

All services share: `log/slog` JSON handler, `envOr()` config, distroless `Dockerfile`, entry in `go.work`.
No service currently imports NATS, Temporal, or Vault. **The NATS event bus is not wired anywhere yet.**

## Crates (Rust)

| Module | Status | What actually exists |
|---|---|---|
| `crates/edge-proxy` | **PARTIAL** | **Plaintext HTTP listener + reverse proxy WORKING.** `proxy.rs` (hyper 1.x listener, `/healthz`, Host→backend forwarding) and `router.rs` (host-keyed table, consistent-hash replica `select`, `parse_routes` for `PROXY__STATIC_ROUTES`) are real and tested (7 unit/integration tests pass; verified e2e end-to-end). `main.rs`/`config.rs` real. **Dynamic routing admin API WORKING** — `Router` is now interior-mutable (`RwLock`); a separate admin listener (`PROXY__ADMIN_ADDR`, default `127.0.0.1:9901`) serves `GET/POST/DELETE /routes` so the control plane can register `host → VM-address` at runtime. Proven on the KVM host: boot microVM → `POST /routes` → proxy forwards `Host: app.local` → guest → `DELETE` → 404 (`DYNAMIC_DEPLOY_TO_URL_OK`). Still **1-line stubs**: `tls.rs`, `wakeup.rs`, `metrics.rs`, `shed.rs` — no TLS, autostart, metrics, or load-shed yet. Dropped the broken `fasthash` dep (x86-only C backend, won't build on arm64). |
| `crates/fc-driver` | **PARTIAL** | **`firecracker.rs` WORKING** — `MicroVm` spawns Firecracker, configures boot-source/rootfs/machine-config over the unix-socket API (hand-rolled HTTP, no extra deps), starts the guest, and stops it; `fc-driver vm-boot` subcommand drives it. **Verified on the KVM host: boots Alpine to userspace** (e2e test `boots_microvm_to_userspace`, console marker asserted). `main.rs` real. Still **1-line stubs**: `jailer.rs`, `network.rs`, `nomad_plugin.rs`, `snapshot.rs`, `driver.rs` — no jailer, TAP/netlink, Nomad gRPC plugin, or snapshot/restore yet. |

> ℹ️ **fc-driver: Linux-only deps are gated; the workspace now builds on macOS.**
> `rtnetlink` (→ `netlink-sys`, references the Linux-only `libc::sockaddr_nl`) and
> `nix` (jailer setuid/chroot/mount) live under
> `[target.'cfg(target_os = "linux")'.dependencies]` in `crates/fc-driver/Cargo.toml`,
> so macOS compiles fc-driver's cross-platform parts (tonic/prost/tokio) while
> skipping the Linux bits. **`make build-rust` and `make lint-rust` pass on macOS.**
> The *real* Firecracker/jailer/netlink implementation is still Linux-only and must
> be built/run on Linux — those modules (`network.rs`, `jailer.rs`, …) are still stubs.

## CLI (`cli/` — antctl, Go)

**PARTIAL.** Full cobra skeleton: `root.go` plus commands. **`login` WORKING** (`cli/cmd/login.go`) — `antctl login --email --api-key` → `POST /v1/auth/login`, stores `token`/`email` in `~/.antctl.yaml`; `client.go` attaches it as `Authorization: Bearer` on every request (`apiGet`/`apiPost`). **`regions` WORKING** — authed call rendering a tabwriter table; API base from `api_url`/`ANTARIKSH_API_URL`. **`deploy` WORKING** — parses `platform.toml` (`cli/internal/manifest`, toml→structs, validated + tested) and POSTs a deploy; `--watch` log streaming still TODO. All *other* commands still return `errNotImplemented` (in `cli/cmd/open.go`).

## Ops (`ops/`)

**PARTIAL — real config files, untested against a live cluster.**
- `ops/nomad/jobs/`: `vault`, `edge-proxy`, `nats`, `observability`, `control-plane-api` `.nomad.hcl` specs exist. Other services have **no** job spec yet.
- `ops/consul/config/`: `server.hcl`, `client.hcl`.
- `ops/vault/policies/`: `edge-proxy-tls.hcl`, `antariksh-api.hcl` only.

## Dev stack (`dev/`)

**PARTIAL — present, not verified working here.** `docker-compose.yml`, `init-db.sql`, `zot-config.json`, `install-toolchain.sh`, Grafana datasource provisioning, Temporal dynamic config.

---

## MISSING — described in CLAUDE.md but NOT in the tree

These directories/components are referenced as if they exist. They do **not**. Do not edit or cite files inside them as if real — they must be created first.

- `infra/terraform/`, `infra/ansible/` — **`infra/` is an empty directory.** No Terraform, no Ansible.
- `storage/` — **empty directory.** The Neon engine submodule (pageserver/safekeeper/compute-node) is **not** checked out.
- `build/nixpacks/` — **empty.** No custom Nixpacks builder extensions.
- `docs/` — only this file (and whatever else has been added since).
- No README at repo root.
- No CI: `.github/` has no workflows.
- Temporal workflows/activities — no `internal/workflows/` or `internal/activities/` anywhere yet.
- NATS subjects from the canonical table — none are published or consumed in code yet.

---

## How to keep this file honest

- When you implement something, flip its row STUB → PARTIAL → WORKING and update the date line.
- When you create a MISSING item, move it up into the real tables.
- Mark new placeholders in code with: `// STUB(phaseN): not implemented — see docs/STATUS.md`
  so they are greppable (`grep -rn "STUB(" .`).
- If CLAUDE.md and this file conflict, fix whichever is wrong in the same change.
