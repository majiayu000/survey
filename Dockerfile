# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build both binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /consumer ./cmd/consumer

# Install golang-migrate for database migrations
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Final stage
FROM alpine:3.20

# Install ca-certificates for HTTPS calls
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

# Copy binaries from builder
COPY --from=builder /api /usr/local/bin/api
COPY --from=builder /consumer /usr/local/bin/consumer
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate

# Copy migrations for database migrations
COPY --from=builder /app/internal/db/migrations /migrations

# Use non-root user
USER appuser

# Expose port
EXPOSE 8080

# Default to api, override in k8s for consumer
ENTRYPOINT ["/usr/local/bin/api"]
