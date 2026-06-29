module github.com/threemates/antariksh/services/builder

go 1.23

require (
	github.com/nats-io/nats.go v1.38.0
	go.uber.org/zap v1.27.0
)
// BuildKit integration via moby/buildkit gRPC client
// Nixpacks: exec nixpacks binary, capture OCI image
// Trivy: exec trivy binary for image scanning gate
