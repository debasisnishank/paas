# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum* ./
COPY services/ services/
COPY cli/ cli/
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd services/storage-cp && go build -o /app/storage-cp ./cmd/storage-cp
FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/storage-cp /storage-cp
USER nonroot:nonroot
ENTRYPOINT ["/storage-cp"]
