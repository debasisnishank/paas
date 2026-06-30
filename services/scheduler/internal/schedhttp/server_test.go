package schedhttp

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/threemates/antariksh/services/scheduler/internal/orchestrator"
)

type fakeDeployer struct {
	res orchestrator.Result
	err error

	mu      sync.Mutex
	lastReq orchestrator.Request
	srcSeen map[string]string // file name → contents found in SourceDir
}

func (f *fakeDeployer) Deploy(_ context.Context, req orchestrator.Request) (orchestrator.Result, error) {
	f.mu.Lock()
	f.lastReq = req
	if req.SourceDir != "" {
		f.srcSeen = map[string]string{}
		_ = filepath.Walk(req.SourceDir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(req.SourceDir, p)
			b, _ := os.ReadFile(p)
			f.srcSeen[rel] = string(b)
			return nil
		})
	}
	f.mu.Unlock()
	return f.res, f.err
}

func TestDeployEndpointReturnsResult(t *testing.T) {
	d := &fakeDeployer{res: orchestrator.Result{Host: "web.local", URL: "http://web.local", GuestIP: "172.16.0.2"}}
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
	srv := httptest.NewServer(Handler(&fakeDeployer{}))
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
	srv := httptest.NewServer(Handler(&fakeDeployer{err: errors.New("kvm down")}))
	defer srv.Close()
	resp, _ := http.Post(srv.URL+"/internal/deploy", "application/json",
		strings.NewReader(`{"service":"web"}`))
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

// makeTarGz builds an in-memory .tar.gz from name→content entries.
func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDeployEndpointBuildsFromMultipartSource(t *testing.T) {
	d := &fakeDeployer{res: orchestrator.Result{URL: "http://web.local"}}
	srv := httptest.NewServer(Handler(d))
	defer srv.Close()

	archive := makeTarGz(t, map[string]string{
		"Dockerfile":     "FROM scratch\n",
		"app/server.go":  "package main\n",
	})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("service", "web")
	_ = mw.WriteField("image", "antariksh/web:latest")
	fw, _ := mw.CreateFormFile("source", "source.tar.gz")
	if _, err := fw.Write(archive); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()

	resp, err := http.Post(srv.URL+"/internal/deploy", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lastReq.Service != "web" || d.lastReq.SourceDir == "" {
		t.Fatalf("deploy req = %+v", d.lastReq)
	}
	if d.srcSeen["Dockerfile"] != "FROM scratch\n" || d.srcSeen[filepath.Join("app", "server.go")] != "package main\n" {
		t.Errorf("extracted source = %v", d.srcSeen)
	}
}

func TestDeployEndpointMultipartRequiresService(t *testing.T) {
	srv := httptest.NewServer(Handler(&fakeDeployer{}))
	defer srv.Close()

	archive := makeTarGz(t, map[string]string{"Dockerfile": "FROM scratch\n"})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("source", "source.tar.gz")
	_, _ = fw.Write(archive)
	_ = mw.Close()

	resp, _ := http.Post(srv.URL+"/internal/deploy", mw.FormDataContentType(), &body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
