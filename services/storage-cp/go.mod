module github.com/threemates/antariksh/services/storage-cp

go 1.23

require (
	github.com/jackc/pgx/v5 v5.7.2
	github.com/nats-io/nats.go v1.38.0
	go.temporal.io/sdk v1.32.0
	go.uber.org/zap v1.27.0
)
// Neon engine (Pageserver/Safekeeper) integration via their HTTP management APIs
// pgvector: baked into the Postgres compute node image
