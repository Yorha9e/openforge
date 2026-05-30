# Multi-stage build for Go backend
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY gen/ ./gen/
COPY migrations/ ./migrations/
COPY config/ ./config/

# Build binaries
RUN CGO_ENABLED=1 GOOS=linux go build -o /openforge ./cmd/server
RUN CGO_ENABLED=1 GOOS=linux go build -o /openforge-cli ./cmd/openforge

# Runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates bash curl

WORKDIR /app

# Copy binaries and assets
COPY --from=builder /openforge .
COPY --from=builder /openforge-cli .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/config ./config

# Default listen port (server binary defaults to :8030, override here)
EXPOSE 8080

# Run migrations then start server
COPY docker-entrypoint.sh .
RUN chmod +x docker-entrypoint.sh
ENTRYPOINT ["./docker-entrypoint.sh"]
