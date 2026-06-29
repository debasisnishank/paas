// Package main implements the DNS management service.
//
// Manages records in PowerDNS (anycast) via its HTTP API.
// Auto-provisions: <app>.<region>.antariksh.in on deploy
// Custom domains: CNAME + TXT ownership proof → ACME DNS-01 for wildcard TLS
package main

import (
	"log/slog"
	"os"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)
	log.Info("dns-mgr starting",
		"pdns_api", envOr("PDNS_API_URL", "http://127.0.0.1:8081"),
		"nats_url", envOr("NATS_URL", "nats://127.0.0.1:4222"),
	)
	// TODO: NATS consumer on "platform.deploy.live" → upsert A/CNAME in PowerDNS
	// TODO: NATS consumer on "platform.domain.attach" → custom-domain flow
	// TODO: ACME DNS-01 challenge responder (TXT records via PowerDNS API)
	select {}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
