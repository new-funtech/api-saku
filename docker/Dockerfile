# Stage 1: Build stage
FROM golang:1.25.0-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a \
    -installsuffix cgo \
    -o /app/bin/api-hrmis \
    ./cmd/main.go

# Stage 2: Final minimal runtime image
FROM alpine:3.19

# Install runtime dependencies (curl for healthcheck)
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    && addgroup -g 1000 appuser \
    && adduser -D -u 1000 -G appuser appuser

# Set timezone
ENV TZ=Asia/Jakarta

WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=appuser:appuser /app/bin/api-hrmis /app/api-hrmis

# Copy environment files if needed (optional, better to use env vars)
COPY --from=builder --chown=appuser:appuser /app/envs ./envs

# Use non-root user for security
USER appuser

# Expose application port
EXPOSE 4000

# Health check (use curl; busybox wget flags are limited)
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=5 \
    CMD curl -fsS http://127.0.0.1:4000/health || exit 1

# Run the application
CMD ["/app/api-hrmis"]
