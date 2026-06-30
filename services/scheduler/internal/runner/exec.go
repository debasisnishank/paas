// Package runner provides the real VMRunner used by the orchestrator: it sets
// up a TAP device and runs `fc-driver` to boot a Firecracker microVM, tracking
// the process so it can be torn down. Requires KVM + passwordless sudo for TAP
// setup (Linux in practice; the package compiles cross-platform).
package runner

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/threemates/antariksh/services/scheduler/internal/orchestrator"
)

type Exec struct {
	FcDriverBin    string
	FirecrackerBin string
	KernelPath     string
	RootfsPath     string
	TapUser        string

	mu    sync.Mutex
	procs map[string]*exec.Cmd
}

func NewExec(fcDriver, firecracker, kernel, rootfs, tapUser string) *Exec {
	return &Exec{
		FcDriverBin:    fcDriver,
		FirecrackerBin: firecracker,
		KernelPath:     kernel,
		RootfsPath:     rootfs,
		TapUser:        tapUser,
		procs:          make(map[string]*exec.Cmd),
	}
}

func sudo(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "sudo", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo %v: %w: %s", args, err, string(out))
	}
	return nil
}

// Boot sets up the TAP, launches the microVM via fc-driver (in its own process
// group), and waits for the guest HTTP server to answer.
func (e *Exec) Boot(ctx context.Context, spec orchestrator.BootSpec) error {
	_ = sudo(ctx, "ip", "link", "del", spec.TapDev) // ignore if it doesn't exist
	if err := sudo(ctx, "ip", "tuntap", "add", "dev", spec.TapDev, "mode", "tap", "user", e.TapUser); err != nil {
		return err
	}
	_ = sudo(ctx, "ip", "addr", "add", spec.HostIP+"/24", "dev", spec.TapDev) // best-effort (shared subnet)
	if err := sudo(ctx, "ip", "link", "set", spec.TapDev, "up"); err != nil {
		return err
	}

	// A freshly built rootfs (per-deploy) overrides the configured default.
	rootfs := e.RootfsPath
	if spec.RootfsPath != "" {
		rootfs = spec.RootfsPath
	}

	bootArgs := fmt.Sprintf(
		"console=ttyS0 reboot=k panic=1 pci=off ip=%s::%s:255.255.255.0::eth0:off init=/init",
		spec.GuestIP, spec.HostIP,
	)
	logf, _ := os.Create("/tmp/fcvm-" + spec.TapDev + ".log")
	cmd := exec.Command(e.FcDriverBin, "vm-boot",
		"--fc-bin", e.FirecrackerBin,
		"--kernel", e.KernelPath,
		"--rootfs", rootfs,
		"--tap", spec.TapDev,
		"--guest-mac", spec.GuestMAC,
		"--boot-args", bootArgs,
		"--console", "/tmp/fccon-"+spec.TapDev+".log",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if logf != nil {
		cmd.Stdout, cmd.Stderr = logf, logf
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start fc-driver: %w", err)
	}
	e.mu.Lock()
	e.procs[spec.TapDev] = cmd
	e.mu.Unlock()

	if err := waitHTTP(spec.GuestIP+":80", 30*time.Second); err != nil {
		_ = e.Stop(ctx, spec.TapDev)
		return fmt.Errorf("guest %s did not become reachable: %w", spec.GuestIP, err)
	}
	return nil
}

// Stop kills the microVM's process group (fc-driver + firecracker) and removes
// the TAP device.
func (e *Exec) Stop(ctx context.Context, tapDev string) error {
	e.mu.Lock()
	cmd := e.procs[tapDev]
	delete(e.procs, tapDev)
	e.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	_ = sudo(ctx, "ip", "link", "del", tapDev)
	return nil
}

func waitHTTP(addr string, dur time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(dur)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + addr + "/")
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for http://%s", addr)
}
