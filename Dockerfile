# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /frontend
COPY frontend/package.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go backend
FROM golang:1.23-alpine AS go-builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY gen/ ./gen/
COPY migrations/ ./migrations/
COPY config/ ./config/

RUN CGO_ENABLED=1 GOOS=linux go build -o /openforge ./cmd/server
RUN CGO_ENABLED=1 GOOS=linux go build -o /openforge-cli ./cmd/openforge

# Stage 3: Runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates bash curl

WORKDIR /app

# Copy Go binaries
COPY --from=go-builder /openforge .
COPY --from=go-builder /openforge-cli .
COPY --from=go-builder /app/migrations ./migrations
COPY --from=go-builder /app/config ./config

# Copy built frontend
COPY --from=frontend-builder /frontend/dist ./frontend/dist

EXPOSE 8080

COPY docker-entrypoint.sh .
RUN chmod +x docker-entrypoint.sh
ENTRYPOINT ["./docker-entrypoint.sh"]
