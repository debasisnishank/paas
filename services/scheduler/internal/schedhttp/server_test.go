package schedhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/threemates/antariksh/services/scheduler/internal/orchestrator"
)

type fakeDeployer struct {
	res orchestrator.Result
	err error
}

func (f fakeDeployer) Deploy(_ context.Context, _, _ string) (orchestrator.Result, error) {
	return f.res, f.err
}

func TestDeployEndpointReturnsResult(t *testing.T) {
	d := fakeDeployer{res: orchestrator.Result{Host: "web.local", URL: "http://web.local", GuestIP: "172.16.0.2"}}
	srv := httptest.NewServer(Handler(d))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/internal/deploy", "application/json",
		strings.NewReader(`{"service":"web","image":"img"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got orchestrator.Result
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.URL != "http://web.local" || got.GuestIP != "172.16.0.2" {
		t.Errorf("result = %+v", got)
	}
}

func TestDeployEndpointValidates(t *testing.T) {
	srv := httptest.NewServer(Handler(fakeDeployer{}))
	defer srv.Close()

	// missing service → 400
	resp, _ := http.Post(srv.URL+"/internal/deploy", "application/json", strings.NewReader(`{}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing service status = %d, want 400", resp.StatusCode)
	}
	// GET → 405
	resp2, _ := http.Get(srv.URL + "/internal/deploy")
	if resp2.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET status = %d, want 405", resp2.StatusCode)
	}
}

func TestDeployEndpointPropagatesError(t *testing.T) {
	srv := httptest.NewServer(Handler(fakeDeployer{err: errors.New("kvm down")}))
	defer srv.Close()
	resp, _ := http.Post(srv.URL+"/internal/deploy", "application/json",
		strings.NewReader(`{"service":"web"}`))
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}
