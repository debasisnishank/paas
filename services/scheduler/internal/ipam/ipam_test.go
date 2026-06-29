package ipam

import "testing"

func TestAllocateIncrements(t *testing.T) {
	a := New()
	if a.HostIP() != "172.16.0.1" {
		t.Fatalf("host ip = %s", a.HostIP())
	}
	l1, err := a.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	if l1.GuestIP != "172.16.0.2" || l1.TapDev != "fctap2" || l1.GuestMAC != "06:00:AC:10:00:02" {
		t.Fatalf("lease1 = %+v", l1)
	}
	l2, _ := a.Allocate()
	if l2.GuestIP != "172.16.0.3" {
		t.Fatalf("lease2 = %+v", l2)
	}
}

func TestReleaseReusesOctet(t *testing.T) {
	a := New()
	l1, _ := a.Allocate()
	_, _ = a.Allocate()
	a.Release(l1)
	l3, _ := a.Allocate()
	if l3.GuestIP != l1.GuestIP {
		t.Fatalf("expected reuse of %s, got %s", l1.GuestIP, l3.GuestIP)
	}
}
