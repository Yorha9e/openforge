.PHONY: all build build-cli build-server build-frontend build-nodejs test test-go test-frontend test-nodejs lint lint-go lint-frontend clean docker-up docker-down help

# Default target
all: lint test build

# ============================================================================
# Build targets
# ============================================================================

build: build-cli build-server build-frontend build-nodejs

build-cli:
	@echo "==> Building CLI..."
	go build -o bin/openforge.exe ./cmd/openforge

build-server:
	@echo "==> Building Web Server..."
	go build -o bin/server.exe ./cmd/server

build-frontend:
	@echo "==> Building frontend..."
	cd frontend && npm run build

build-nodejs:
	@echo "==> Building Node.js IO layer..."
	cd nodejs-io && npm run build

# ============================================================================
# Test targets
# ============================================================================

test: test-go test-frontend test-nodejs

test-go:
	@echo "==> Running Go tests..."
	go test ./internal/... -v -count=1

test-frontend:
	@echo "==> Running frontend tests..."
	cd frontend && npx vitest run

test-nodejs:
	@echo "==> Running Node.js IO tests..."
	cd nodejs-io && npm test

# ============================================================================
# Lint targets
# ============================================================================

lint: lint-go lint-frontend

lint-go:
	@echo "==> Linting Go code..."
	golangci-lint run ./...

lint-frontend:
	@echo "==> Linting frontend..."
	cd frontend && npx tsc --noEmit

# ============================================================================
# Docker targets
# ============================================================================

docker-up:
	@echo "==> Starting services..."
	docker-compose up -d

docker-down:
	@echo "==> Stopping services..."
	docker-compose down

# ============================================================================
# Clean targets
# ============================================================================

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf bin/
	rm -rf frontend/dist/
	rm -rf nodejs-io/dist/

# ============================================================================
# Help
# ============================================================================

help:
	@echo "OpenForge Development Commands:"
	@echo ""
	@echo "  make all          - Run lint, test, and build (default)"
	@echo "  make build        - Build all components"
	@echo "  make build-cli    - Build CLI tool"
	@echo "  make build-server - Build Web Server"
	@echo "  make build-frontend - Build frontend"
	@echo "  make build-nodejs - Build Node.js IO layer"
	@echo "  make test         - Run all tests"
	@echo "  make test-go      - Run Go tests"
	@echo "  make test-frontend - Run frontend tests"
	@echo "  make test-nodejs  - Run Node.js IO tests"
	@echo "  make lint         - Run all linters"
	@echo "  make lint-go      - Run Go linter"
	@echo "  make lint-frontend - Run TypeScript check"
	@echo "  make docker-up    - Start Docker services"
	@echo "  make docker-down  - Stop Docker services"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make help         - Show this help"
