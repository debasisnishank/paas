// Package ipam hands out per-microVM guest IPs, TAP device names, and MACs
// from a single host /24. Phase 0: a simple in-memory sequential allocator with
// reuse of released octets; a real IPAM (per-tenant ULA ranges) replaces it later.
package ipam

import (
	"fmt"
	"sync"
)

// Lease is one microVM's network identity.
type Lease struct {
	GuestIP  string
	TapDev   string
	GuestMAC string
	octet    int
}

// Allocator dispenses leases from 172.16.0.0/24 (host is .1).
type Allocator struct {
	mu     sync.Mutex
	next   int // next host octet (2..254)
	freed  []int
	hostIP string
}

func New() *Allocator {
	return &Allocator{next: 2, hostIP: "172.16.0.1"}
}

// HostIP is the gateway address the guest routes through (the host TAP IP).
func (a *Allocator) HostIP() string { return a.hostIP }

// Allocate reserves the next free octet and returns its lease.
func (a *Allocator) Allocate() (Lease, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var o int
	if n := len(a.freed); n > 0 {
		o = a.freed[n-1]
		a.freed = a.freed[:n-1]
	} else {
		if a.next > 254 {
			return Lease{}, fmt.Errorf("ipam: address pool exhausted")
		}
		o = a.next
		a.next++
	}
	return Lease{
		GuestIP:  fmt.Sprintf("172.16.0.%d", o),
		TapDev:   fmt.Sprintf("fctap%d", o),
		GuestMAC: fmt.Sprintf("06:00:AC:10:00:%02X", o),
		octet:    o,
	}, nil
}

// Release returns a lease's octet to the pool for reuse.
func (a *Allocator) Release(l Lease) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if l.octet >= 2 {
		a.freed = append(a.freed, l.octet)
	}
}
