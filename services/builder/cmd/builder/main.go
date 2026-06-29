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
	"log/slog"
	"os"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)
	log.Info("builder starting",
		"registry", envOr("REGISTRY_URL", "http://127.0.0.1:5000"),
		"nats_url", envOr("NATS_URL", "nats://127.0.0.1:4222"),
		"buildkit_addr", envOr("BUILDKIT_ADDR", "unix:///run/buildkit/buildkitd.sock"),
	)
	// TODO: NATS consumer on "platform.build.>"
	// TODO: BuildKit client for layer-cached builds
	// TODO: Nixpacks/Buildpack detection logic
	// TODO: Trivy scan gate before registry push
	select {}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
