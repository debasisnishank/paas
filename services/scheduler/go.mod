module github.com/threemates/antariksh/services/scheduler

go 1.23

// NOTE: github.com/hashicorp/nomad/api will be added (with a pinned version)
// when the Nomad bridge is implemented. The latest nomad/api requires Go 1.25;
// pin a Go 1.23-compatible pseudo-version at that time, or bump the workspace.
require (
	github.com/nats-io/nats.go v1.38.0
	go.temporal.io/sdk v1.32.0
	go.uber.org/zap v1.27.0
)
