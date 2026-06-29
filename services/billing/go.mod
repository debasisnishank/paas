module github.com/threemates/antariksh/services/billing

go 1.23

require (
	github.com/jackc/pgx/v5 v5.7.2
	github.com/nats-io/nats.go v1.38.0
	go.uber.org/zap v1.27.0
)
// Lago integration: REST client, not a Go SDK — use net/http
// Razorpay: razorpay-go (unofficial) or plain net/http
