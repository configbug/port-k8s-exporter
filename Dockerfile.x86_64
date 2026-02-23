# Build stage
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 go build -o port-k8s-exporter .

# Final stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libssl3 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/port-k8s-exporter /usr/bin/port-k8s-exporter
COPY assets/ /assets

ENTRYPOINT ["/usr/bin/port-k8s-exporter"]