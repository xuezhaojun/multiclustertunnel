# Multi-stage build for MultiClusterTunnel Server
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

# Build the server binary
# CGO_ENABLED=0 for static binary
# -ldflags="-w -s" to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty)" \
    -a -installsuffix cgo \
    -o server \
    ./cmd/server/

# Stage 2: Create the runtime image
FROM gcr.io/distroless/static:nonroot

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /workspace/server /server

# Use nonroot user (uid=65532, gid=65532)
USER 65532:65532

# Expose ports
EXPOSE 8080 8443 8090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/server", "--help"]

# Set entrypoint
ENTRYPOINT ["/server"]

# Default command arguments
CMD ["--grpc-address=:8443", "--http-address=:8080", "--v=2"]

# Labels for metadata
LABEL maintainer="MultiClusterTunnel Team"
LABEL description="MultiClusterTunnel Server for E2E Testing"
LABEL version="latest"
LABEL component="server"
