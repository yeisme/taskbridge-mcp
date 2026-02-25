# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files first for better cache
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with version info
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o taskbridge .

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 taskbridge && \
    adduser -u 1000 -G taskbridge -s /bin/sh -D taskbridge

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/taskbridge .
COPY --from=builder /build/configs ./configs
COPY --from=builder /build/templates ./templates

# Create data directory
RUN mkdir -p /app/data /app/credentials && chown -R taskbridge:taskbridge /app

# Switch to non-root user
USER taskbridge

# Set environment variables
ENV TASKBRIDGE_CONFIG=/app/configs/config.yaml \
    TZ=Asia/Shanghai

# Expose port (for MCP TCP mode)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Labels
LABEL org.opencontainers.image.title="TaskBridge" \
      org.opencontainers.image.description="TaskBridge MCP Server" \
      org.opencontainers.image.source="https://github.com/taskbridge/taskbridge-mcp"

# Entry point
ENTRYPOINT ["./taskbridge"]
CMD ["serve"]
