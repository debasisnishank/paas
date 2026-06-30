// Package schedhttp exposes the scheduler's deploy orchestration over HTTP so
// the control-plane API can trigger it. (NATS/Temporal-driven deploys replace
// this direct call later; the orchestration behind it is unchanged.)
//
// /internal/deploy accepts either:
//   - application/json {service, image} — boot the default rootfs (no build), or
//   - multipart/form-data with fields service, image and a `source` file
//     (a .tar.gz of the project dir) — extract it and build a rootfs from source.
package schedhttp

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/threemates/antariksh/services/scheduler/internal/orchestrator"
)

// maxSourceBytes caps an uploaded source archive (decompressed) to keep a
// runaway upload from filling the build host.
const maxSourceBytes = 512 << 20 // 512 MiB

// Deployer is the orchestration entrypoint (satisfied by *orchestrator.Orchestrator).
type Deployer interface {
	Deploy(ctx context.Context, req orchestrator.Request) (orchestrator.Result, error)
}

type deployReq struct {
	Service string `json:"service"`
	Image   string `json:"image"`
}

// Handler builds the scheduler's HTTP API.
func Handler(d Deployer) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/internal/deploy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			handleMultipartDeploy(w, r, d)
			return
		}

		var req deployReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Service == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "service is required"})
			return
		}
		deploy(w, r, d, orchestrator.Request{Service: req.Service, Image: req.Image})
	})

	return mux
}

// handleMultipartDeploy reads service/image form fields and a `source` tar.gz,
// extracts the source to a temp dir, and deploys (build-from-source). The temp
// dir is removed after the deploy completes (the built rootfs lives elsewhere).
func handleMultipartDeploy(w http.ResponseWriter, r *http.Request, d Deployer) {
	file, _, err := r.FormFile("source")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing source file: " + err.Error()})
		return
	}
	defer func() { _ = file.Close() }()

	service := r.FormValue("service")
	if service == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "service is required"})
		return
	}

	srcDir, err := os.MkdirTemp("", "antariksh-src-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "temp dir: " + err.Error()})
		return
	}
	defer func() { _ = os.RemoveAll(srcDir) }()

	if err := extractTarGz(file, srcDir); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "extract source: " + err.Error()})
		return
	}

	deploy(w, r, d, orchestrator.Request{
		Service:   service,
		Image:     r.FormValue("image"),
		SourceDir: srcDir,
	})
}

func deploy(w http.ResponseWriter, r *http.Request, d Deployer, req orchestrator.Request) {
	res, err := d.Deploy(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// extractTarGz unpacks a gzipped tar stream into dir, guarding against path
// traversal (entries that would escape dir are rejected).
func extractTarGz(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(io.LimitReader(gz, maxSourceBytes))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dir, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec // bounded by maxSourceBytes above
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			// skip symlinks/devices/etc — a build context shouldn't need them
		}
	}
}

// safeJoin joins name onto dir, rejecting any path that escapes dir.
func safeJoin(dir, name string) (string, error) {
	target := filepath.Join(dir, name)
	if target != dir && !strings.HasPrefix(target, dir+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe path in archive: %q", name)
	}
	return target, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
