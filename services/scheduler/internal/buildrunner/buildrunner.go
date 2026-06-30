// Package buildrunner is the orchestrator's real Builder: it shells out to the
// `builder` binary (`builder build <srcDir> <out.ext4> <tag>`) to turn a project
// source directory into a bootable ext4 rootfs. Mirrors how runner shells out to
// `fc-driver` — the heavy lifting (Docker, mkfs.ext4) lives in the builder.
//
// Phase 0 collapses build + deploy onto one host (the rootfs must be local to
// the fc-driver that boots it). NATS/Temporal will separate builder and
// scheduler later; this direct call is the spike path.
package buildrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Exec runs the builder binary to produce rootfs images under OutDir.
type Exec struct {
	Bin    string // path to the `builder` binary
	OutDir string // directory to write built .ext4 images into
}

// NewExec returns a builder that invokes bin and writes images under outDir.
// outDir defaults to the OS temp dir when empty.
func NewExec(bin, outDir string) *Exec {
	if outDir == "" {
		outDir = os.TempDir()
	}
	return &Exec{Bin: bin, OutDir: outDir}
}

// Build builds srcDir into an ext4 rootfs tagged `tag` and returns its path.
func (e *Exec) Build(ctx context.Context, srcDir, tag string) (string, error) {
	if err := os.MkdirAll(e.OutDir, 0o755); err != nil {
		return "", fmt.Errorf("create out dir: %w", err)
	}
	dir, err := os.MkdirTemp(e.OutDir, "rootfs-")
	if err != nil {
		return "", fmt.Errorf("temp out dir: %w", err)
	}
	out := filepath.Join(dir, sanitizeTag(tag)+".ext4")

	cmd := exec.CommandContext(ctx, e.Bin, "build", srcDir, out, tag)
	// Builds run as the scheduler's user (in the docker group); ensure sbin is
	// on PATH so mkfs.ext4 (often /usr/sbin) is found under a non-login process.
	cmd.Env = append(os.Environ(), "PATH="+ensureSbin(os.Getenv("PATH")))
	if combined, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("builder build: %w: %s", err, strings.TrimSpace(string(combined)))
	}
	return out, nil
}

// sanitizeTag turns an OCI tag (e.g. "antariksh/web:latest") into a filename-safe
// stem ("antariksh-web-latest").
func sanitizeTag(tag string) string {
	r := strings.NewReplacer("/", "-", ":", "-", "@", "-")
	return r.Replace(tag)
}

func ensureSbin(path string) string {
	for _, p := range strings.Split(path, ":") {
		if p == "/usr/sbin" {
			return path
		}
	}
	if path == "" {
		return "/usr/sbin:/sbin"
	}
	return path + ":/usr/sbin:/sbin"
}
