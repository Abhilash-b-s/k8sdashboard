# Frontend build stage
FROM node:22-alpine AS web-builder

WORKDIR /web

# Install deps with cache-friendly layer
COPY package.json package-lock.json* ./
RUN npm ci

# Build static assets (vite outputs to /web/static/)
COPY vite.config.js ./
COPY web/ ./web/
RUN npm run build

# Go build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# Replace any committed static/ with the freshly built assets
RUN rm -rf static
COPY --from=web-builder /web/static ./static

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o k8s-dashboard ./cmd/server

# Final stage - using distroless for minimal attack surface
FROM gcr.io/distroless/static:nonroot

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/k8s-dashboard .

# Copy built static files
COPY --from=builder /app/static ./static/

# Expose port
EXPOSE 8080

# Run as non-root user (distroless default)
USER nonroot:nonroot

# Run the application
ENTRYPOINT ["/app/k8s-dashboard"]
