//! Firecracker microVM lifecycle: spawn the VMM, configure it over its
//! Unix-socket HTTP API, start the guest, and stop it.
//!
//! The actual VMM boot is Linux/KVM-only, but this module compiles on any
//! unix host: the API client is hand-rolled HTTP/1.1 over a `UnixStream` (no
//! extra deps), so the request-building logic stays unit-testable on macOS.
//! The real boot is exercised by the `#[ignore]`d e2e test and the
//! `fc-driver vm-boot` subcommand on a KVM host.

use std::path::{Path, PathBuf};
use std::process::{ExitStatus, Stdio};
use std::time::Duration;

use anyhow::{bail, Context, Result};
use serde::Serialize;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::UnixStream;
use tokio::process::{Child, Command};
use tokio::time::{sleep, timeout};
use tracing::{info, warn};

/// Everything needed to boot one microVM.
#[derive(Debug, Clone)]
pub struct VmConfig {
    pub kernel_image_path: PathBuf,
    pub rootfs_path: PathBuf,
    pub vcpu_count: u32,
    pub mem_size_mib: u32,
    pub boot_args: String,
    pub rootfs_read_only: bool,
    /// If set, the VMM's stdout/stderr (which carries the guest serial console)
    /// is redirected to this file. `None` inherits the parent's stdio.
    pub console_log: Option<PathBuf>,
    /// Optional guest NIC backed by a host TAP device. The guest IP itself is
    /// set via the kernel `ip=` boot arg by the caller.
    pub net: Option<NetConfig>,
}

/// A guest network interface backed by a host TAP device.
#[derive(Debug, Clone)]
pub struct NetConfig {
    /// Host TAP device name (must already exist + be configured by the host).
    pub tap_dev: String,
    /// Guest-side MAC address, e.g. "06:00:AC:10:00:02".
    pub guest_mac: String,
}

impl VmConfig {
    /// Sensible defaults: 1 vCPU, 256 MiB, serial console on ttyS0.
    pub fn new(kernel: impl Into<PathBuf>, rootfs: impl Into<PathBuf>) -> Self {
        Self {
            kernel_image_path: kernel.into(),
            rootfs_path: rootfs.into(),
            vcpu_count: 1,
            mem_size_mib: 256,
            boot_args: "console=ttyS0 reboot=k panic=1 pci=off".into(),
            rootfs_read_only: false,
            console_log: None,
            net: None,
        }
    }
}

#[derive(Serialize)]
struct BootSource<'a> {
    kernel_image_path: &'a str,
    boot_args: &'a str,
}

#[derive(Serialize)]
struct Drive<'a> {
    drive_id: &'a str,
    path_on_host: &'a str,
    is_root_device: bool,
    is_read_only: bool,
}

#[derive(Serialize)]
struct MachineConfig {
    vcpu_count: u32,
    mem_size_mib: u32,
}

#[derive(Serialize)]
struct NetIface<'a> {
    iface_id: &'a str,
    host_dev_name: &'a str,
    guest_mac: &'a str,
}

#[derive(Serialize)]
struct Action<'a> {
    action_type: &'a str,
}

/// A spawned Firecracker VMM and its guest.
pub struct MicroVm {
    child: Child,
    api_sock: PathBuf,
}

impl MicroVm {
    /// Spawn Firecracker, configure the guest from `cfg`, and start it.
    pub async fn boot(fc_bin: &Path, api_sock: PathBuf, cfg: &VmConfig) -> Result<MicroVm> {
        let _ = std::fs::remove_file(&api_sock);

        let mut cmd = Command::new(fc_bin);
        cmd.arg("--api-sock").arg(&api_sock).kill_on_drop(true);
        if let Some(ref log) = cfg.console_log {
            let f = std::fs::File::create(log)
                .with_context(|| format!("create console log {}", log.display()))?;
            let f2 = f.try_clone().context("clone console log handle")?;
            cmd.stdout(Stdio::from(f)).stderr(Stdio::from(f2));
        }
        let child = cmd
            .spawn()
            .with_context(|| format!("spawn firecracker ({})", fc_bin.display()))?;

        let vm = MicroVm { child, api_sock };
        vm.wait_for_socket(Duration::from_secs(5)).await?;
        vm.configure(cfg).await?;
        vm.put("/actions", &Action { action_type: "InstanceStart" }).await?;
        info!(sock = %vm.api_sock.display(), pid = ?vm.child.id(), "microVM started");
        Ok(vm)
    }

    async fn wait_for_socket(&self, dur: Duration) -> Result<()> {
        timeout(dur, async {
            while !self.api_sock.exists() {
                sleep(Duration::from_millis(50)).await;
            }
        })
        .await
        .context("firecracker API socket did not appear")
    }

    async fn configure(&self, cfg: &VmConfig) -> Result<()> {
        let kernel = cfg
            .kernel_image_path
            .to_str()
            .context("non-utf8 kernel path")?;
        let rootfs = cfg.rootfs_path.to_str().context("non-utf8 rootfs path")?;

        self.put(
            "/boot-source",
            &BootSource {
                kernel_image_path: kernel,
                boot_args: &cfg.boot_args,
            },
        )
        .await?;
        self.put(
            "/drives/rootfs",
            &Drive {
                drive_id: "rootfs",
                path_on_host: rootfs,
                is_root_device: true,
                is_read_only: cfg.rootfs_read_only,
            },
        )
        .await?;
        self.put(
            "/machine-config",
            &MachineConfig {
                vcpu_count: cfg.vcpu_count,
                mem_size_mib: cfg.mem_size_mib,
            },
        )
        .await?;

        if let Some(net) = &cfg.net {
            self.put(
                "/network-interfaces/eth0",
                &NetIface {
                    iface_id: "eth0",
                    host_dev_name: &net.tap_dev,
                    guest_mac: &net.guest_mac,
                },
            )
            .await?;
        }
        Ok(())
    }

    /// Issue a PUT to the Firecracker API over its unix socket.
    async fn put<T: Serialize>(&self, route: &str, body: &T) -> Result<()> {
        let json = serde_json::to_vec(body)?;
        let req = build_put_request(route, &json);

        let mut stream = UnixStream::connect(&self.api_sock)
            .await
            .with_context(|| format!("connect API socket for PUT {route}"))?;
        stream.write_all(&req).await?;
        stream.flush().await?;

        let mut buf = [0u8; 1024];
        let n = stream.read(&mut buf).await.unwrap_or(0);
        let status = parse_status_code(&buf[..n]).unwrap_or(0);
        if !(200..300).contains(&status) {
            bail!(
                "firecracker PUT {route} -> status {status}: {}",
                String::from_utf8_lossy(&buf[..n])
            );
        }
        Ok(())
    }

    /// The VMM process id, if still running.
    pub fn pid(&self) -> Option<u32> {
        self.child.id()
    }

    /// Wait up to `dur` for the VMM to exit (e.g. the guest powered off).
    pub async fn wait_for_exit(&mut self, dur: Duration) -> Result<ExitStatus> {
        timeout(dur, self.child.wait())
            .await
            .context("timed out waiting for VMM to exit")?
            .context("waiting on VMM process")
    }

    /// Block until the VMM exits (e.g. the process is killed). Used to keep the
    /// microVM running for as long as the driver process lives.
    pub async fn wait(&mut self) -> Result<ExitStatus> {
        self.child.wait().await.context("waiting on VMM process")
    }

    /// Force-stop the VMM and clean up its socket.
    pub async fn stop(mut self) -> Result<()> {
        if let Err(e) = self.child.start_kill() {
            warn!(error = %e, "failed to signal VMM");
        }
        let _ = self.child.wait().await;
        let _ = std::fs::remove_file(&self.api_sock);
        Ok(())
    }
}

/// Build a raw HTTP/1.1 PUT request for the Firecracker API.
fn build_put_request(route: &str, body: &[u8]) -> Vec<u8> {
    let mut req = format!(
        "PUT {route} HTTP/1.1\r\nHost: localhost\r\nAccept: application/json\r\n\
         Content-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()
    )
    .into_bytes();
    req.extend_from_slice(body);
    req
}

/// Parse the numeric status code from an HTTP response's status line.
fn parse_status_code(resp: &[u8]) -> Option<u16> {
    let text = std::str::from_utf8(resp).ok()?;
    let line = text.lines().next()?; // "HTTP/1.1 204 No Content"
    line.split_whitespace().nth(1)?.parse().ok()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn put_request_has_headers_and_body() {
        let req = build_put_request("/machine-config", br#"{"vcpu_count":1}"#);
        let s = String::from_utf8(req).unwrap();
        assert!(s.starts_with("PUT /machine-config HTTP/1.1\r\n"), "{s}");
        assert!(s.contains("Content-Length: 16\r\n"), "{s}");
        assert!(s.contains("Content-Type: application/json"));
        assert!(s.ends_with(r#"{"vcpu_count":1}"#));
    }

    #[test]
    fn parse_status_code_works() {
        assert_eq!(parse_status_code(b"HTTP/1.1 204 No Content\r\n\r\n"), Some(204));
        assert_eq!(parse_status_code(b"HTTP/1.1 400 Bad Request\r\n"), Some(400));
        assert_eq!(parse_status_code(b"garbage"), None);
        assert_eq!(parse_status_code(b""), None);
    }

    #[test]
    fn vmconfig_defaults() {
        let c = VmConfig::new("/k", "/r");
        assert_eq!(c.vcpu_count, 1);
        assert_eq!(c.mem_size_mib, 256);
        assert!(!c.rootfs_read_only);
        assert!(c.boot_args.contains("console=ttyS0"));
    }

    // Real boot — Linux/KVM only. Run on the validation host with:
    //   FC_BIN=~/.local/bin/firecracker \
    //   FC_KERNEL=~/fc-assets/vmlinux-5.10.223 \
    //   FC_ROOTFS=~/fc-assets/alpine.ext4 \
    //   cargo test -p fc-driver -- --ignored --nocapture boots_microvm_to_userspace
    // The rootfs must carry an /init that prints ANTARIKSH_GUEST_USERSPACE_OK.
    #[tokio::test]
    #[ignore]
    async fn boots_microvm_to_userspace() {
        let bin = std::env::var("FC_BIN").expect("FC_BIN");
        let kernel = std::env::var("FC_KERNEL").expect("FC_KERNEL");
        let rootfs = std::env::var("FC_ROOTFS").expect("FC_ROOTFS");
        let console = std::env::temp_dir().join("fc-e2e-console.log");

        let cfg = VmConfig {
            boot_args: "console=ttyS0 reboot=k panic=1 pci=off init=/init".into(),
            console_log: Some(console.clone()),
            ..VmConfig::new(kernel, rootfs)
        };
        let vm = MicroVm::boot(Path::new(&bin), PathBuf::from("/tmp/fc-e2e.sock"), &cfg)
            .await
            .expect("microVM should boot");

        // Give the guest time to boot and run /init, then verify via the console.
        sleep(Duration::from_secs(6)).await;
        let log = std::fs::read_to_string(&console).unwrap_or_default();
        vm.stop().await.expect("stop VMM");

        assert!(
            log.contains("ANTARIKSH_GUEST_USERSPACE_OK"),
            "guest console did not show the userspace marker:\n{log}"
        );
    }
}
