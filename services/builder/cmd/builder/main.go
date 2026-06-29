// Package main implements the build service.
//
// Build pipeline:
//  1. Receive build request from NATS ("platform.build.>")
//  2. Detect builder: Nixpacks → Buildpacks → Dockerfile (in that order)
//  3. Spawn ephemeral Firecracker build VM via Nomad, run BuildKit inside
//  4. Push OCI image to internal registry (zot)
//  5. Run Trivy scan; fail build if critical CVEs found
//  6. Emit "platform.build.done" or "platform.build.failed" on NATS
//
// Build VMs are themselves workloads on the platform (dogfooding).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/threemates/antariksh/services/builder/internal/build"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	// `builder build <projectDir> <out.ext4> [tag]` builds a project into a
	// bootable Firecracker rootfs (Docker → export → mkfs.ext4 with /init).
	if len(os.Args) > 1 && os.Args[1] == "build" {
		if err := runBuild(os.Args[2:]); err != nil {
			log.Error("build failed", "err", err)
			os.Exit(1)
		}
		return
	}

	log.Info("builder starting",
		"registry", envOr("REGISTRY_URL", "http://127.0.0.1:5000"),
		"nats_url", envOr("NATS_URL", "nats://127.0.0.1:4222"),
		"buildkit_addr", envOr("BUILDKIT_ADDR", "unix:///run/buildkit/buildkitd.sock"),
	)
	// TODO: NATS consumer on "platform.build.>" + BuildKit + Trivy gate
	select {}
}

func runBuild(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: builder build <projectDir> <out.ext4> [tag]")
	}
	projectDir, out := args[0], args[1]
	tag := "antariksh/app:latest"
	if len(args) > 2 {
		tag = args[2]
	}
	res, err := build.BuildRootfs(context.Background(), projectDir, tag, out)
	if err != nil {
		return err
	}
	fmt.Printf("built %s → %s\n", res.ImageRef, res.RootfsPath)
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
