# V2V Blockchain Node Dockerfile
# Task 13.1: Implement Dockerfile

# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev linux-headers

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the node binary
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o v2v-node ./cmd/v2v-node

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 v2v && \
    adduser -u 1000 -G v2v -s /bin/sh -D v2v

# Create data directory
RUN mkdir -p /data && chown -R v2v:v2v /data

# Copy binary from builder
COPY --from=builder /build/v2v-node /usr/local/bin/v2v-node

# Set ownership
RUN chown v2v:v2v /usr/local/bin/v2v-node

# Switch to non-root user
USER v2v

# Expose ports
# 8080: API server
# 10000: P2P network
EXPOSE 8080 10000

# Volume for persistent data
VOLUME ["/data"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["v2v-node"]
CMD ["start", "--data-dir", "/data", "--api-port", "8080"]
