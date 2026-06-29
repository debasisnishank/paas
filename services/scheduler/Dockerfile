# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum* ./
COPY services/ services/
COPY cli/ cli/
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd services/scheduler && go build -o /app/scheduler ./cmd/scheduler
FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/scheduler /scheduler
USER nonroot:nonroot
ENTRYPOINT ["/scheduler"]
