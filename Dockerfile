# Dockerfile for pre-compiled binary (from GoReleaser)
# NO compilation here - binary is provided via build context

FROM debian:bookworm-slim

# TARGETARCH is auto-injected by buildx (amd64, arm64)
ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libssl3 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Copy pre-compiled binary (provided by workflow)
COPY port-k8s-exporter-${TARGETARCH} /usr/bin/port-k8s-exporter
RUN chmod +x /usr/bin/port-k8s-exporter

# Copy assets directly from source
COPY assets/ /assets/

ENTRYPOINT ["/usr/bin/port-k8s-exporter"]