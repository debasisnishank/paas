// Package build turns a project directory into a bootable Firecracker rootfs:
// `docker build` → `docker export` the flattened filesystem → inject an /init
// that runs the image's command → `mkfs.ext4 -d` into an ext4 image.
//
// Uses Docker (already present on build hosts) and mke2fs's -d flag, so no
// privileged mount is needed. Linux in practice; the package compiles anywhere.
package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ImageConfig is the subset of an OCI image config we need to make it bootable.
type ImageConfig struct {
	Entrypoint []string `json:"Entrypoint"`
	Cmd        []string `json:"Cmd"`
	Env        []string `json:"Env"`
	WorkingDir string   `json:"WorkingDir"`
}

// Result reports what was built.
type Result struct {
	ImageRef   string
	RootfsPath string
}

// BuildRootfs builds projectDir into an ext4 rootfs at outExt4, tagged `tag`.
func BuildRootfs(ctx context.Context, projectDir, tag, outExt4 string) (Result, error) {
	if err := run(ctx, "docker", "build", "-t", tag, projectDir); err != nil {
		return Result{}, fmt.Errorf("docker build: %w", err)
	}

	cfg, err := inspectConfig(ctx, tag)
	if err != nil {
		return Result{}, err
	}

	cid, err := output(ctx, "docker", "create", tag)
	if err != nil {
		return Result{}, fmt.Errorf("docker create: %w", err)
	}
	cid = strings.TrimSpace(cid)
	defer func() { _ = run(ctx, "docker", "rm", "-f", cid) }()

	rootDir, err := os.MkdirTemp("", "antariksh-rootfs-")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(rootDir) }()

	if err := exportFS(ctx, cid, rootDir); err != nil {
		return Result{}, fmt.Errorf("export rootfs: %w", err)
	}

	initPath := filepath.Join(rootDir, "init")
	if err := os.WriteFile(initPath, []byte(buildInitScript(cfg)), 0o755); err != nil {
		return Result{}, fmt.Errorf("write init: %w", err)
	}

	if err := run(ctx, "mkfs.ext4", "-q", "-F", "-L", "rootfs", "-d", rootDir, outExt4, sizeFor(rootDir)); err != nil {
		return Result{}, fmt.Errorf("mkfs.ext4: %w", err)
	}
	return Result{ImageRef: tag, RootfsPath: outExt4}, nil
}

// buildInitScript renders a /init that applies the image's working dir + env and
// execs its entrypoint/command as PID 1. (eth0 is configured by the kernel `ip=`
// boot arg, so we don't touch networking here.)
func buildInitScript(cfg ImageConfig) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	if cfg.WorkingDir != "" {
		fmt.Fprintf(&b, "cd %s\n", shQuote(cfg.WorkingDir))
	}
	for _, e := range cfg.Env {
		fmt.Fprintf(&b, "export %s\n", shQuote(e))
	}
	argv := append(append([]string{}, cfg.Entrypoint...), cfg.Cmd...)
	if len(argv) == 0 {
		argv = []string{"/bin/sh"}
	}
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = shQuote(a)
	}
	fmt.Fprintf(&b, "exec %s\n", strings.Join(quoted, " "))
	return b.String()
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func inspectConfig(ctx context.Context, tag string) (ImageConfig, error) {
	out, err := output(ctx, "docker", "inspect", "--format", "{{json .Config}}", tag)
	if err != nil {
		return ImageConfig{}, fmt.Errorf("docker inspect: %w", err)
	}
	var cfg ImageConfig
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		return ImageConfig{}, fmt.Errorf("parse image config: %w", err)
	}
	return cfg, nil
}

// exportFS streams `docker export` into a tar extracted under dir.
func exportFS(ctx context.Context, cid, dir string) error {
	exp := exec.CommandContext(ctx, "docker", "export", cid)
	untar := exec.CommandContext(ctx, "tar", "-x", "-C", dir)
	pipe, err := exp.StdoutPipe()
	if err != nil {
		return err
	}
	untar.Stdin = pipe
	if err := untar.Start(); err != nil {
		return err
	}
	if err := exp.Run(); err != nil {
		return err
	}
	return untar.Wait()
}

// sizeFor returns an ext4 size argument (du of dir + 64MiB headroom, min 64M).
func sizeFor(dir string) string {
	mib := 64
	if out, err := output(context.Background(), "du", "-sm", dir); err == nil {
		if n, perr := parseLeadingInt(out); perr == nil {
			mib = n + 64
		}
	}
	return fmt.Sprintf("%dM", mib)
}

func parseLeadingInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("no leading int in %q", s)
	}
	n := 0
	for _, c := range s[:i] {
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, string(out))
	}
	return nil
}

func output(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	return string(out), err
}
