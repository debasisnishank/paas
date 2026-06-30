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

_Last verified: 2026-06-30 by reading the tree + box e2e. Update the date when you change status._

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
>
> **Build-from-source deploy proven (2026-06-30, ANTCTL_BUILD_DEPLOY_OK):** the
> spine now deploys *the user's actual code*, not a prebuilt rootfs. `antctl
> deploy dev/sample-app` streams a gzip-tar of the project as multipart → API
> forwards the source to the scheduler (`/internal/deploy` multipart) → scheduler
> shells out to `builder` (Docker build → ext4) → boots **that** rootfs → route
> registered → `curl -H 'Host: web.local'` returns the BUILT app. Two fixes
> landed during e2e: per-VM fc-driver `--sock` (the fixed default collided with a
> stale root-owned socket), and a 20m timeout on just the deploy route (the
> global 30s request/WriteTimeout had aborted the boot). Box config: edge-proxy
> `:8088`, scheduler `:7070` (`BUILDER_BIN`/`BUILD_OUT_DIR`), API `:8080`.

---

## Services (Go)

| Module | Status | What actually exists |
|---|---|---|
| `services/api` | **PARTIAL** | `cmd/api/main.go`: chi router, graceful shutdown, `/healthz`. **Auth WORKING** — HS256 JWT (`internal/auth`, stdlib, no external dep), `POST /v1/auth/login` (email + `API_KEY` → bearer token), `authMiddleware` protects the whole `/v1` group; `cmd/api/auth.go` + tests passing. **`GET /v1/regions` WORKING** (now behind auth) — `cmd/api/regions.go` serving an in-memory catalog (`internal/regions`: Mumbai available; Hyderabad/Dubai/Riyadh planned) with jurisdiction profiles; `regions_test.go` passing. **Deploy WORKING (build-from-source)** — `POST …/services/{id}/deploy` records a Deployment and, when `SCHEDULER_URL` is set, drives the scheduler to boot a microVM + route a URL (`internal/scheduler` client). A **multipart upload** carrying a `source` tarball triggers a build-from-source deploy (`DeployWithSource` streams the tarball to the scheduler); a JSON/empty body boots the default rootfs. `GET …/deployments` lists them (`internal/deploystore`). The deploy route carries a 20m timeout (build+boot); other routes have none-needed instant handlers. All *other* `/v1` routes still `stubHandler` → `501`. **Real:** `internal/domain/types.go`. Dev defaults `API_AUTH_SECRET`/`API_KEY` warn at boot. No DB, NATS, Vault, Temporal, or SPIFFE mTLS yet. |
| `services/scheduler` | **PARTIAL** | `cmd/scheduler/main.go` runs an HTTP orchestration API (`/internal/deploy`, default `:7070`) and has a `scheduler deploy <svc> [image-or-sourceDir]` subcommand. **Unit-tested (Mac):** `internal/nomadspec` (Firecracker Nomad job-spec HCL generator); `internal/ipam` (per-VM guest IP/TAP/MAC from 172.16.0.0/24); `internal/edgeproxy` (admin-API client); `internal/orchestrator` (`Deploy(Request{Service,Image,SourceDir})` = optional build → allocate IP → boot VM via `VMRunner` seam → register route → URL, with unwind; `Builder` seam via `WithBuilder`); `internal/buildrunner` (shells to the `builder` binary to make a per-deploy ext4); `internal/runner` (`Exec` VMRunner — sets up TAP via sudo, runs `fc-driver` with a **per-VM `--sock`**, waits for guest HTTP, kills the group + removes sock on stop); `internal/schedhttp` (JSON deploy **and** multipart source-tarball deploy: extract → build → boot). **Box-verified build-from-source (ANTCTL_BUILD_DEPLOY_OK):** source upload → `built rootfs` → `deploy live` → routed URL serves the built app. Caveats: single-VM (multi-VM needs a **bridge**, not per-tap host IPs); NATS/Temporal still TODO; build+deploy are collapsed onto one host for Phase 0 (NATS will separate builder/scheduler). |
| `services/billing` | **STUB** | `cmd/billing/main.go` (~35 LOC): boots and parks. No Lago, Razorpay, GST, or metering consumer. |
| `services/dns` | **STUB** | `cmd/dns/main.go` (~31 LOC): boots and parks. No PowerDNS client, no ACME responder. |
| `services/storage-cp` | **STUB** | `cmd/storage-cp/main.go` (~38 LOC): boots and parks. No Pageserver/Safekeeper orchestration, no branch API, no PITR. |
| `services/builder` | **PARTIAL** | `cmd/builder/main.go` daemon still boots-and-parks (no NATS consumer/Nixpacks/Trivy yet). **`builder build <dir> <out.ext4> [tag]` WORKING** — `internal/build` does `docker build` → container export → inject `/init` → `mkfs.ext4 -d`, producing a bootable microVM rootfs (`build.go`, unit-tested on Mac). `dev/sample-app` (multi-stage Go HTTP server on :80) is the fixture. **Box-verified e2e (BUILT_ROOTFS_BOOT_OK, 2026-06-30):** `builder build dev/sample-app /tmp/app.ext4` → `fc-driver vm-boot` (TAP, guest 172.16.0.2) → `/init` runs the built binary → `curl http://172.16.0.2/` returns `Hello from a BUILT Antariksh app`. This replaces the prebuilt-Alpine rootfs the deploy spine had used. Box notes: pass `--fc-bin` to fc-driver under sudo (firecracker not on root PATH); boot a long-running server **without** `--wait`. Still no BuildKit/Trivy/registry-push. |

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

**PARTIAL.** Full cobra skeleton: `root.go` plus commands. **`login` WORKING** (`cli/cmd/login.go`) — `antctl login --email --api-key` → `POST /v1/auth/login`, stores `token`/`email` in `~/.antctl.yaml`; `client.go` attaches it as `Authorization: Bearer` on every request (`apiGet`/`apiPost`). **`regions` WORKING** — authed call rendering a tabwriter table; API base from `api_url`/`ANTARIKSH_API_URL`. **`deploy` WORKING (build-from-source)** — parses `platform.toml` (`cli/internal/manifest`), then streams a gzip-tar of the project dir (skipping `.git/node_modules/target/vendor`) as multipart/form-data to the deploy endpoint (`tarGzDir` + `apiPostMultipart`, both streamed via `io.Pipe`, 15m timeout). Box-verified: `antctl deploy dev/sample-app` → live URL serving the built app. `--watch` log streaming still TODO. All *other* commands still return `errNotImplemented` (in `cli/cmd/open.go`).

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
