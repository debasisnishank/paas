// Package main implements the serverless Postgres control plane.
//
// Sits above the Neon engine (Pageserver + Safekeeper — Apache 2.0).
// Responsibilities:
//   - Tenant lifecycle: create/drop Pageserver tenants + timelines
//   - Compute pool: spin up / suspend / resume Postgres compute nodes (Firecracker)
//   - Branch API: O(1) CoW branches per PR environment via Pageserver timeline fork
//   - PITR: continuous WAL archiving to MinIO; point-in-time restore workflows
//   - Read replica provisioning (cheap — shared storage layer)
//   - Per-tenant billing: CU-hours emitted to NATS "platform.metering.db"
//   - pgvector enabled by default on all compute images
package main

import (
	"log/slog"
	"os"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)
	log.Info("storage-cp starting",
		"pageserver_url", envOr("PAGESERVER_URL", "http://127.0.0.1:9898"),
		"safekeeper_addrs", envOr("SAFEKEEPER_ADDRS", "127.0.0.1:5454"),
		"minio_endpoint", envOr("MINIO_ENDPOINT", "http://127.0.0.1:9000"),
	)
	// TODO: Temporal worker — CreateDB, BranchDB, RestorePITR, SuspendCompute workflows
	// TODO: NATS consumer on "platform.db.>" for branch/restore events
	// TODO: WAL archiving watchdog + restore drill scheduler
	select {}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
