# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files first for layer caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o k8s-dashboard ./cmd/server

# Final stage - using distroless for minimal attack surface
FROM gcr.io/distroless/static:nonroot

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/k8s-dashboard .

# Copy static files
COPY static/ ./static/

# Expose port
EXPOSE 8080

# Run as non-root user (distroless default)
USER nonroot:nonroot

# Run the application
ENTRYPOINT ["/app/k8s-dashboard"]
