# Multi-stage build for MultiClusterTunnel Agent
# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /workspace

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the agent binary (used for both e2e and production)
# CGO_ENABLED=0 for static binary
# -ldflags="-w -s" to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty)" \
    -a -installsuffix cgo \
    -o agent \
    ./cmd/agent/

# Stage 2: Create the runtime image
FROM gcr.io/distroless/static:nonroot

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the agent binary
COPY --from=builder /workspace/agent /agent

# Use nonroot user (uid=65532, gid=65532)
USER 65532:65532

# Create directory for Unix Domain Socket (with proper permissions)
# Note: This will be overridden by emptyDir volume in Kubernetes
VOLUME ["/tmp"]

# Health check (check if socket exists)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["test", "-S", "/tmp/multiclustertunnel.sock"]

# Set entrypoint to agent (used for both e2e and production)
ENTRYPOINT ["/agent"]

# Default command arguments for e2e testing
CMD ["--hub-address=mctunnel-server.mctunnel-hub.svc.cluster.local:8443", \
     "--cluster-name=test-cluster", \
     "--uds-socket-path=/tmp/multiclustertunnel.sock", \
     "--hub-kubeconfig=/etc/kubeconfig/config", \
     "--insecure", \
     "--v=2"]

# Labels for metadata
LABEL maintainer="MultiClusterTunnel Team"
LABEL description="MultiClusterTunnel Agent for E2E Testing"
LABEL version="latest"
LABEL component="agent"
