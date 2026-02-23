# Dockerfile for GoReleaser - ARM64
# Binary is pre-compiled by GoReleaser, no build stage needed

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libssl3 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Copy pre-compiled binary from GoReleaser
COPY port-k8s-exporter /usr/bin/port-k8s-exporter

# Create assets directory and copy config files
# GoReleaser puts files individually in build context root
RUN mkdir -p /assets/defaults
COPY appConfig.yaml /assets/defaults/
COPY blueprints.json /assets/defaults/
COPY pages.json /assets/defaults/
COPY scorecards.json /assets/defaults/

ENTRYPOINT ["/usr/bin/port-k8s-exporter"]