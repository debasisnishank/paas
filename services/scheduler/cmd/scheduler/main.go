// Package main implements the scheduler service.
//
// Responsibilities:
//   - Bridge between the control-plane API and Nomad
//   - Accept deploy intents from NATS JetStream (subject: platform.deploy.>)
//   - Translate platform.toml service specs into Nomad job HCL
//   - Submit, drain, and stop Nomad jobs via the Firecracker task driver
//   - Emit lifecycle events back onto NATS (deploy.started, deploy.live, deploy.failed)
//   - Handle autoscale signals: scale-out, scale-in, scale-to-zero suspend/resume
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/threemates/antariksh/services/scheduler/internal/buildrunner"
	"github.com/threemates/antariksh/services/scheduler/internal/edgeproxy"
	"github.com/threemates/antariksh/services/scheduler/internal/ipam"
	"github.com/threemates/antariksh/services/scheduler/internal/orchestrator"
	"github.com/threemates/antariksh/services/scheduler/internal/runner"
	"github.com/threemates/antariksh/services/scheduler/internal/schedhttp"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	// `scheduler deploy <service> [image-or-sourceDir]` boots a microVM via
	// fc-driver and registers its route with edge-proxy. If the 2nd arg is an
	// existing directory it is built into a rootfs first (build-from-source);
	// otherwise it's treated as an image ref and the default rootfs is booted.
	// (NATS/Temporal-driven deploys are still TODO; this is the direct path.)
	if len(os.Args) > 1 && os.Args[1] == "deploy" {
		if err := runDeploy(os.Args[2:]); err != nil {
			log.Error("deploy failed", "err", err)
			os.Exit(1)
		}
		return
	}

	// Default mode: serve the deploy orchestration API for the control plane.
	addr := envOr("SCHEDULER_ADDR", ":7070")
	log.Info("scheduler starting",
		"addr", addr,
		"edge_proxy_admin", envOr("EDGE_PROXY_ADMIN", "http://127.0.0.1:9901"),
	)
	// TODO: NATS JetStream consumer on "platform.deploy.>" + Temporal workflows
	if err := http.ListenAndServe(addr, schedhttp.Handler(buildOrchestrator())); err != nil {
		log.Error("scheduler http server stopped", "err", err)
		os.Exit(1)
	}
}

// buildOrchestrator wires the orchestrator from environment configuration. It
// always attaches a Builder (shelling out to BUILDER_BIN) so deploys carrying
// source are built into a fresh rootfs; deploys without source boot FC_ROOTFS.
func buildOrchestrator() *orchestrator.Orchestrator {
	run := runner.NewExec(
		envOr("FC_DRIVER_BIN", "fc-driver"),
		envOr("FIRECRACKER_BIN", "firecracker"),
		envOr("FC_KERNEL", os.Getenv("HOME")+"/fc-assets/vmlinux-5.10.223"),
		envOr("FC_ROOTFS", os.Getenv("HOME")+"/fc-assets/alpine-http.ext4"),
		envOr("TAP_USER", os.Getenv("USER")),
	)
	builder := buildrunner.NewExec(
		envOr("BUILDER_BIN", "builder"),
		envOr("BUILD_OUT_DIR", ""),
	)
	return orchestrator.New(
		ipam.New(),
		edgeproxy.New(envOr("EDGE_PROXY_ADMIN", "http://127.0.0.1:9901")),
		run,
		envOr("DEPLOY_DOMAIN", "local"),
	).WithBuilder(builder)
}

func runDeploy(args []string) error {
	req := orchestrator.Request{Service: "web", Image: "local-rootfs"}
	if len(args) > 0 {
		req.Service = args[0]
	}
	if len(args) > 1 {
		// An existing directory → build from source; anything else → image ref.
		if fi, err := os.Stat(args[1]); err == nil && fi.IsDir() {
			req.SourceDir = args[1]
			req.Image = ""
		} else {
			req.Image = args[1]
		}
	}
	res, err := buildOrchestrator().Deploy(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("deployed %s → %s (guest %s, tap %s)\n", req.Service, res.URL, res.GuestIP, res.TapDev)
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
