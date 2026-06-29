# Sample Antariksh app: a tiny HTTP server, built from source and packaged into
# a microVM rootfs by `builder build`. Multi-stage so the final image is small
# but still has /bin/sh (alpine) for the injected /init.
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY server.go .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /server server.go

FROM alpine:3.20
COPY --from=build /server /server
EXPOSE 80
CMD ["/server"]
