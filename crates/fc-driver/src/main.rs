//! Firecracker Nomad Task Driver
//!
//! Implements the Nomad task driver gRPC plugin protocol.
//! Wraps the Firecracker VMM API + jailer for per-tenant kernel isolation.
//!
//! Lifecycle per task allocation:
//!  1. PreStart  — pull rootfs snapshot from shared cache (by OCI digest)
//!  2. Start     — jailer fork → firecracker process → configure vCPU/RAM/drives/net
//!  3. Run       — monitor VMM; expose health via Nomad heartbeat
//!  4. Suspend   — snapshot guest state to NVMe (scale-to-zero autostop)
//!  5. Resume    — restore snapshot on first inbound request (autostart)
//!  6. Stop/Kill — send VMM SIGTERM; clean up jailer chroot
//!
//! Networking per VM:
//!  - TAP device → Cilium eBPF → WireGuard 6PN overlay
//!  - Each VM gets a /128 IPv6 ULA from the tenant's IPAM range
//!  - Cilium network policy enforces east-west tenant isolation
//!
//! gVisor/Kata fallback:
//!  - If FC_FALLBACK_RUNTIME=gvisor or kata in task config, delegate to that runtime
//!  - Used for workloads that misbehave under Firecracker (e.g. nested virt)

use std::path::{Path, PathBuf};
use std::time::Duration;

use anyhow::{bail, Result};
use tracing::{info, warn};

use crate::firecracker::{MicroVm, VmConfig};

mod driver;
mod firecracker;
mod jailer;
mod network;
mod snapshot;
mod nomad_plugin;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .json()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "info".into()),
        )
        .init();

    let mut args = std::env::args().skip(1);
    match args.next().as_deref() {
        // `fc-driver vm-boot --kernel K --rootfs R [--fc-bin B --vcpus N --mem MiB
        //  --sock PATH --boot-args ARGS --wait SECS]` — boot one microVM directly.
        // This is the path the Nomad task driver will drive internally; exposing it
        // as a subcommand lets us exercise the boot flow without a Nomad cluster.
        Some("vm-boot") => vm_boot(args.collect()).await,
        _ => {
            info!("fc-driver starting — Firecracker Nomad task driver");
            // TODO: start gRPC server implementing Nomad TaskDriver service
            // TODO: register with Nomad plugin catalog via handshake
            // TODO: spawn snapshot GC loop (evict stale NVMe snapshots)
            Ok(())
        }
    }
}

async fn vm_boot(args: Vec<String>) -> Result<()> {
    let mut fc_bin = "firecracker".to_string();
    let mut sock = "/tmp/fc-driver.sock".to_string();
    let mut kernel = String::new();
    let mut rootfs = String::new();
    let mut vcpus = 1u32;
    let mut mem = 256u32;
    let mut boot_args: Option<String> = None;
    let mut console: Option<String> = None;
    let mut tap: Option<String> = None;
    let mut guest_mac = "06:00:AC:10:00:02".to_string();
    let mut wait_secs = 0u64;

    let mut it = args.into_iter();
    while let Some(flag) = it.next() {
        let mut val = || it.next().unwrap_or_default();
        match flag.as_str() {
            "--fc-bin" => fc_bin = val(),
            "--sock" => sock = val(),
            "--kernel" => kernel = val(),
            "--rootfs" => rootfs = val(),
            "--vcpus" => vcpus = val().parse().unwrap_or(1),
            "--mem" => mem = val().parse().unwrap_or(256),
            "--boot-args" => boot_args = Some(val()),
            "--console" => console = Some(val()),
            "--tap" => tap = Some(val()),
            "--guest-mac" => guest_mac = val(),
            "--wait" => wait_secs = val().parse().unwrap_or(0),
            other => warn!(arg = other, "ignoring unknown flag"),
        }
    }
    if kernel.is_empty() || rootfs.is_empty() {
        bail!("--kernel and --rootfs are required");
    }

    let mut cfg = VmConfig::new(kernel, rootfs);
    cfg.vcpu_count = vcpus;
    cfg.mem_size_mib = mem;
    cfg.console_log = console.map(PathBuf::from);
    cfg.net = tap.map(|tap_dev| firecracker::NetConfig { tap_dev, guest_mac });
    if let Some(ba) = boot_args {
        cfg.boot_args = ba;
    }

    let mut vm = MicroVm::boot(Path::new(&fc_bin), PathBuf::from(sock), &cfg).await?;
    info!(pid = ?vm.pid(), "microVM booted");

    if wait_secs > 0 {
        match vm.wait_for_exit(Duration::from_secs(wait_secs)).await {
            Ok(status) => info!(success = status.success(), code = ?status.code(), "VMM exited"),
            Err(e) => {
                warn!(error = %e, "VMM did not exit in time; stopping");
                vm.stop().await?;
            }
        }
    } else {
        // No --wait: hold the microVM running until this process is killed
        // (the orchestrator's runner manages our lifetime and kills our group).
        let status = vm.wait().await?;
        info!(success = status.success(), code = ?status.code(), "VMM exited");
    }
    Ok(())
}
