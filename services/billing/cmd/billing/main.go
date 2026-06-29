// Package main implements the billing & metering service.
//
// Pipeline:
//   NATS JetStream ("platform.metering.>") → rate via Lago → Postgres ledger
//   → Razorpay gateway for recurring debits (RBI e-mandate compliant)
//   → GST e-invoicing (IRP/IRN) for B2B above threshold
//
// Jurisdiction adapters are pluggable: swap gateway + tax profile per region.
package main

import (
	"log/slog"
	"os"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)
	log.Info("billing starting",
		"lago_url", envOr("LAGO_URL", "http://127.0.0.1:3000"),
		"nats_url", envOr("NATS_URL", "nats://127.0.0.1:4222"),
	)
	// TODO: NATS consumer on "platform.metering.>" → Lago ingest events
	// TODO: Razorpay mandate webhook handler (24-hr pre-debit notify, AFA step-up)
	// TODO: GST e-invoice generation via IRP sandbox
	// TODO: Per-tenant real-time spend tracker → budget alert NATS publish
	select {}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
