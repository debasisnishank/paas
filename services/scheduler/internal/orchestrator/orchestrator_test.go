package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/threemates/antariksh/services/scheduler/internal/edgeproxy"
	"github.com/threemates/antariksh/services/scheduler/internal/ipam"
)

type fakeRunner struct {
	mu      sync.Mutex
	booted  []BootSpec
	stopped []string
	bootErr error
}

func (f *fakeRunner) Boot(_ context.Context, s BootSpec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.bootErr != nil {
		return f.bootErr
	}
	f.booted = append(f.booted, s)
	return nil
}

func (f *fakeRunner) Stop(_ context.Context, tap string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, tap)
	return nil
}

type fakeBuilder struct {
	mu       sync.Mutex
	built    []string // tags built
	rootfs   string
	buildErr error
}

func (f *fakeBuilder) Build(_ context.Context, _, tag string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.buildErr != nil {
		return "", f.buildErr
	}
	f.built = append(f.built, tag)
	return f.rootfs, nil
}

func TestDeployBootsRegistersAndReturnsURL(t *testing.T) {
	routePosted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/routes" {
			routePosted = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	runner := &fakeRunner{}
	o := New(ipam.New(), edgeproxy.New(srv.URL), runner, "local")

	res, err := o.Deploy(context.Background(), Request{Service: "web", Image: "registry/web@sha256:abc"})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if res.Host != "web.local" || res.URL != "http://web.local" {
		t.Errorf("result = %+v", res)
	}
	if res.GuestIP != "172.16.0.2" {
		t.Errorf("guest ip = %s", res.GuestIP)
	}
	if len(runner.booted) != 1 {
		t.Fatalf("booted = %d, want 1", len(runner.booted))
	}
	if runner.booted[0].GuestIP != "172.16.0.2" || runner.booted[0].Image != "registry/web@sha256:abc" {
		t.Errorf("boot spec = %+v", runner.booted[0])
	}
	if !routePosted {
		t.Error("expected a route to be registered")
	}
}

func TestDeployBootFailureReleasesLease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	runner := &fakeRunner{bootErr: errors.New("kvm exploded")}
	alloc := ipam.New()
	o := New(alloc, edgeproxy.New(srv.URL), runner, "local")

	if _, err := o.Deploy(context.Background(), Request{Service: "web", Image: "img"}); err == nil {
		t.Fatal("expected deploy error")
	}
	// lease should have been released → next deploy reuses .2
	runner.bootErr = nil
	res, err := o.Deploy(context.Background(), Request{Service: "web2", Image: "img"})
	if err != nil {
		t.Fatal(err)
	}
	if res.GuestIP != "172.16.0.2" {
		t.Errorf("expected lease reuse (172.16.0.2), got %s", res.GuestIP)
	}
}

func TestDeployRouteFailureStopsVM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // route registration fails
	}))
	defer srv.Close()

	runner := &fakeRunner{}
	o := New(ipam.New(), edgeproxy.New(srv.URL), runner, "local")

	if _, err := o.Deploy(context.Background(), Request{Service: "web", Image: "img"}); err == nil {
		t.Fatal("expected deploy error from route registration")
	}
	if len(runner.stopped) != 1 {
		t.Fatalf("expected VM to be stopped on unwind, stopped=%v", runner.stopped)
	}
}

func TestDeployBuildsRootfsFromSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	runner := &fakeRunner{}
	builder := &fakeBuilder{rootfs: "/tmp/web.ext4"}
	o := New(ipam.New(), edgeproxy.New(srv.URL), runner, "local").WithBuilder(builder)

	res, err := o.Deploy(context.Background(), Request{Service: "web", SourceDir: "/src/web"})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if res.URL != "http://web.local" {
		t.Errorf("url = %s", res.URL)
	}
	if len(builder.built) != 1 || builder.built[0] != "antariksh/web:latest" {
		t.Errorf("built tags = %v, want [antariksh/web:latest]", builder.built)
	}
	if len(runner.booted) != 1 || runner.booted[0].RootfsPath != "/tmp/web.ext4" {
		t.Errorf("boot spec rootfs = %q, want /tmp/web.ext4", runner.booted[0].RootfsPath)
	}
}

func TestDeployBuildFailureReleasesNothingAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	runner := &fakeRunner{}
	builder := &fakeBuilder{buildErr: errors.New("docker build failed")}
	o := New(ipam.New(), edgeproxy.New(srv.URL), runner, "local").WithBuilder(builder)

	if _, err := o.Deploy(context.Background(), Request{Service: "web", SourceDir: "/src/web"}); err == nil {
		t.Fatal("expected build error")
	}
	if len(runner.booted) != 0 {
		t.Errorf("no VM should boot when build fails, booted=%d", len(runner.booted))
	}
}
