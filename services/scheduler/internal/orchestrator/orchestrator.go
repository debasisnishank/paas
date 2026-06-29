// Package orchestrator ties the proven deploy→URL steps into one flow:
// allocate a guest IP/TAP, boot the microVM, register the route on edge-proxy,
// and return the resolvable URL. The VM boot is abstracted behind VMRunner so
// the logic is testable without KVM; the real runner shells out to fc-driver.
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
	VCPU     int
	MemMB    int
}

// VMRunner boots and stops microVMs. The real implementation sets up the TAP
// device and runs `fc-driver`; tests use a fake.
type VMRunner interface {
	Boot(ctx context.Context, spec BootSpec) error
	Stop(ctx context.Context, tapDev string) error
}

// Orchestrator drives deploys. `domain` is the URL suffix (e.g. "local" →
// "web.local"; in production a region-scoped apex like "in-mum-1.antariksh.app").
type Orchestrator struct {
	ipam   *ipam.Allocator
	proxy  *edgeproxy.Client
	runner VMRunner
	domain string
}

func New(alloc *ipam.Allocator, proxy *edgeproxy.Client, runner VMRunner, domain string) *Orchestrator {
	return &Orchestrator{ipam: alloc, proxy: proxy, runner: runner, domain: domain}
}

// Result is the outcome of a successful deploy.
type Result struct {
	Host    string `json:"host"`
	URL     string `json:"url"`
	GuestIP string `json:"guest_ip"`
	TapDev  string `json:"tap_dev"`
}

// Deploy boots a microVM for `service` running `image` and routes a public URL
// to it. On any failure after allocation, it unwinds (stop VM, release lease).
func (o *Orchestrator) Deploy(ctx context.Context, service, image string) (Result, error) {
	lease, err := o.ipam.Allocate()
	if err != nil {
		return Result{}, fmt.Errorf("allocate network: %w", err)
	}

	spec := BootSpec{
		GuestIP:  lease.GuestIP,
		HostIP:   o.ipam.HostIP(),
		TapDev:   lease.TapDev,
		GuestMAC: lease.GuestMAC,
		Image:    image,
		VCPU:     1,
		MemMB:    256,
	}
	if err := o.runner.Boot(ctx, spec); err != nil {
		o.ipam.Release(lease)
		return Result{}, fmt.Errorf("boot microVM: %w", err)
	}

	host := fmt.Sprintf("%s.%s", service, o.domain)
	replicas := []string{lease.GuestIP + ":80"}
	if err := o.proxy.RegisterRoute(ctx, host, replicas); err != nil {
		// best-effort unwind so we don't leak a VM with no route
		if serr := o.runner.Stop(ctx, lease.TapDev); serr != nil {
			slog.Error("unwind: stop microVM", "tap", lease.TapDev, "err", serr)
		}
		o.ipam.Release(lease)
		return Result{}, fmt.Errorf("register route: %w", err)
	}

	slog.Info("deploy live", "service", service, "host", host, "guest_ip", lease.GuestIP)
	return Result{
		Host:    host,
		URL:     "http://" + host,
		GuestIP: lease.GuestIP,
		TapDev:  lease.TapDev,
	}, nil
}
