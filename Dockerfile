# Multi-stage Dockerfile for port-k8s-exporter
# Compiles from source - works independently of GoReleaser

ARG GO_VERSION=1.24

# Build stage
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with version info
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -ldflags="-X main.Version=${VERSION}" -o port-k8s-exporter .

# Final stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libssl3 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Copy binary from builder
COPY --from=builder /app/port-k8s-exporter /usr/bin/port-k8s-exporter

# Copy assets directly from source
COPY assets/ /assets/

ENTRYPOINT ["/usr/bin/port-k8s-exporter"]