# syntax=docker/dockerfile:1

# =============================================================================
# Stage 1: Builder
# =============================================================================
FROM golang:1.25.7-alpine AS builder

# Build arguments for version info
ARG VERSION=dev
ARG COMMIT=unknown

# Install build dependencies
RUN apk add --no-cache \
    ca-certificates \
    git \
    tzdata

WORKDIR /build

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the binary with optimizations
# TARGETARCH is automatically set by Docker buildx for multi-platform builds
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT}" \
    -trimpath \
    -o mockd \
    ./cmd/mockd

# =============================================================================
# Stage 2: Production
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

# Labels for container metadata
LABEL org.opencontainers.image.title="mockd" \
      org.opencontainers.image.description="High-performance API mocking engine" \
      org.opencontainers.image.vendor="mockd" \
      org.opencontainers.image.source="https://github.com/getmockd/mockd"

# Copy timezone data from builder (for time-based operations)
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /build/mockd /usr/local/bin/mockd

# Expose mock server and admin API ports
EXPOSE 4280 4290

# Health check against the admin API health endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/mockd", "health", "--admin-port", "4290"]

# Run as nonroot user (provided by distroless)
USER nonroot:nonroot

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/mockd"]

# Default command: start server with default ports
CMD ["start", "--port", "4280", "--admin-port", "4290"]
