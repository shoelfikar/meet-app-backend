# Build stage
# Use buildx to support cross-platform builds
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

# Build arguments for target platform
ARG TARGETOS
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# Use TARGETOS and TARGETARCH for cross-platform support
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-arm64} go build \
    -ldflags="-w -s" \
    -o /app/server \
    ./cmd/server/main.go

# Runtime stage
# Use target platform for runtime
FROM --platform=$TARGETPLATFORM alpine:latest

# Install runtime dependencies (including wget for healthcheck)
RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server ./server

# Verify binary exists and make it executable
RUN chmod +x /app/server && \
    ls -la /app/server

# Copy migrations (if needed at runtime)
COPY --from=builder /build/migrations ./migrations

# Set ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set default port (can be overridden by environment variable)
ENV PORT=8080

# Expose port
EXPOSE 8080

# Health check (uses PORT environment variable)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD sh -c 'wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health || exit 1'

# Run the application
CMD ["/app/server"]
