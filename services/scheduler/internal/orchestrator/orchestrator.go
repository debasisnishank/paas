// Package orchestrator ties the proven deploy→URL steps into one flow:
// build a rootfs from source (optional), allocate a guest IP/TAP, boot the
// microVM, register the route on edge-proxy, and return the resolvable URL.
// Build and VM boot are abstracted behind Builder/VMRunner so the logic is
// testable without Docker or KVM; the real implementations shell out to the
// `builder` and `fc-driver` binaries.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/threemates/antariksh/services/scheduler/internal/edgeproxy"
	"github.com/threemates/antariksh/services/scheduler/internal/ipam"
)

// BootSpec is what a VMRunner needs to launch one microVM.
type BootSpec struct {
	GuestIP  string
	HostIP   string
	TapDev   string
	GuestMAC string
	Image    string
	// RootfsPath, when set, is the ext4 image to boot (a freshly built one).
	// Empty means the runner falls back to its configured default rootfs.
	RootfsPath string
	VCPU       int
	MemMB      int
}

// VMRunner boots and stops microVMs. The real implementation sets up the TAP
// device and runs `fc-driver`; tests use a fake.
type VMRunner interface {
	Boot(ctx context.Context, spec BootSpec) error
	Stop(ctx context.Context, tapDev string) error
}

// Builder turns a project source directory into a bootable ext4 rootfs, tagged
// `tag`, and returns its path. The real implementation shells out to the
// `builder` binary; tests use a fake. A nil Builder disables source builds.
type Builder interface {
	Build(ctx context.Context, srcDir, tag string) (rootfsPath string, err error)
}

// Orchestrator drives deploys. `domain` is the URL suffix (e.g. "local" →
// "web.local"; in production a region-scoped apex like "in-mum-1.antariksh.app").
type Orchestrator struct {
	ipam    *ipam.Allocator
	proxy   *edgeproxy.Client
	runner  VMRunner
	builder Builder
	domain  string
}

func New(alloc *ipam.Allocator, proxy *edgeproxy.Client, runner VMRunner, domain string) *Orchestrator {
	return &Orchestrator{ipam: alloc, proxy: proxy, runner: runner, domain: domain}
}

// WithBuilder attaches a Builder so deploys carrying a SourceDir are built into
// a fresh rootfs before boot. Returns the receiver for chaining.
func (o *Orchestrator) WithBuilder(b Builder) *Orchestrator {
	o.builder = b
	return o
}

// Request is one deploy: a service name, an optional informational image ref,
// and an optional source directory to build a rootfs from. When SourceDir is
// empty (or no Builder is configured) the runner boots its default rootfs.
type Request struct {
	Service   string
	Image     string
	SourceDir string
}

// Result is the outcome of a successful deploy.
type Result struct {
	Host    string `json:"host"`
	URL     string `json:"url"`
	GuestIP string `json:"guest_ip"`
	TapDev  string `json:"tap_dev"`
}

// Deploy optionally builds a rootfs from req.SourceDir, boots a microVM for the
// service, and routes a public URL to it. On any failure after allocation, it
// unwinds (stop VM, release lease).
func (o *Orchestrator) Deploy(ctx context.Context, req Request) (Result, error) {
	var rootfs string
	if req.SourceDir != "" && o.builder != nil {
		tag := fmt.Sprintf("antariksh/%s:latest", req.Service)
		rp, err := o.builder.Build(ctx, req.SourceDir, tag)
		if err != nil {
			return Result{}, fmt.Errorf("build rootfs: %w", err)
		}
		rootfs = rp
		slog.Info("built rootfs", "service", req.Service, "rootfs", rootfs)
	}

	lease, err := o.ipam.Allocate()
	if err != nil {
		return Result{}, fmt.Errorf("allocate network: %w", err)
	}

	spec := BootSpec{
		GuestIP:    lease.GuestIP,
		HostIP:     o.ipam.HostIP(),
		TapDev:     lease.TapDev,
		GuestMAC:   lease.GuestMAC,
		Image:      req.Image,
		RootfsPath: rootfs,
		VCPU:       1,
		MemMB:      256,
	}
	if err := o.runner.Boot(ctx, spec); err != nil {
		o.ipam.Release(lease)
		return Result{}, fmt.Errorf("boot microVM: %w", err)
	}

	host := fmt.Sprintf("%s.%s", req.Service, o.domain)
	replicas := []string{lease.GuestIP + ":80"}
	if err := o.proxy.RegisterRoute(ctx, host, replicas); err != nil {
		// best-effort unwind so we don't leak a VM with no route
		if serr := o.runner.Stop(ctx, lease.TapDev); serr != nil {
			slog.Error("unwind: stop microVM", "tap", lease.TapDev, "err", serr)
		}
		o.ipam.Release(lease)
		return Result{}, fmt.Errorf("register route: %w", err)
	}

	slog.Info("deploy live", "service", req.Service, "host", host, "guest_ip", lease.GuestIP)
	return Result{
		Host:    host,
		URL:     "http://" + host,
		GuestIP: lease.GuestIP,
		TapDev:  lease.TapDev,
	}, nil
}
