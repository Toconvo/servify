# Servify Makefile

.PHONY: help build build-cli build-weknora run run-cli run-weknora migrate migrate-seed test clean docker-build docker-run docker-up-weknora docker-down docker-logs-weknora docker-up-observ docker-down-observ dev-setup fmt lint update-deps docs

# Default target
help:
	@echo "Available commands:"
	@echo "  build         - Build the application"
	@echo "  build-cli     - Build CLI (standard)"
	@echo "  build-weknora - Build CLI with WeKnora tag"
	@echo "  run           - Run the application"
	@echo "  run-cli       - Run CLI (standard)"
	@echo "  run-weknora   - Run CLI with WeKnora tag"
	@echo "  migrate       - Run database migrations"
	@echo "  migrate-seed  - Run database migrations with seed data"
	@echo "  test          - Run tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run with Docker Compose"
	@echo "  docker-up-weknora - Up WeKnora compose (server+weknora+db)"
	@echo "  docker-down      - Down compose services"
	@echo "  docker-logs-weknora - Tail servify logs"
	@echo "  docker-up-observ    - Up OTel Collector + Jaeger"
	@echo "  docker-down-observ  - Down observability stack"
	@echo "  docker-stop   - Stop Docker Compose services"

# Build the application
build:
	@echo "Building Servify..."
	$(MAKE) _build-with-ldflags

# Build CLI targets
build-cli:
	@echo "Building CLI (standard)..."
	$(MAKE) _build-cli-with-ldflags

build-weknora:
	@echo "Building CLI (weknora)..."
	$(MAKE) _build-cli-weknora-with-ldflags

# Run the application
run:
	@echo "Starting Servify server..."
	go -C apps/server run ./cmd/server

run-cli:
	@echo "Running CLI (standard)..."
	go -C apps/server run ./cmd/cli -c $(or $(CONFIG),../../config.yml) run

run-weknora:
	@echo "Running CLI (weknora)..."
	go -C apps/server run -tags weknora ./cmd/cli -c $(or $(CONFIG),../../config.weknora.yml) run

# Run database migrations
migrate:
	@echo "Running database migrations..."
	DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD=$(DB_PASSWORD) DB_NAME=$(DB_NAME) DB_SSLMODE=$(or $(DB_SSLMODE),disable) DB_TIMEZONE=$(or $(DB_TIMEZONE),UTC) \
	go -C apps/server run ./cmd/migrate $(MIGRATE_ARGS)

# Run database migrations with seed data
migrate-seed:
	@echo "Running database migrations with seed data..."
	DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD=$(DB_PASSWORD) DB_NAME=$(DB_NAME) DB_SSLMODE=$(or $(DB_SSLMODE),disable) DB_TIMEZONE=$(or $(DB_TIMEZONE),UTC) \
	go -C apps/server run ./cmd/migrate --seed $(MIGRATE_ARGS)

# Run tests
test:
	@echo "Running tests via scripts/run-tests.sh..."
	./scripts/run-tests.sh || true

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	go clean

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t servify:latest .

# Run with Docker Compose
docker-run:
	@echo "Starting services with Docker Compose..."
	docker-compose up -d

docker-up-weknora:
	@echo "Starting WeKnora stack..."
	docker-compose -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.weknora.yml up -d

docker-down:
	@echo "Stopping services..."
	docker-compose -f infra/compose/docker-compose.yml down

docker-logs-weknora:
	@echo "Tailing servify logs..."
	docker-compose -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.weknora.yml logs -f servify

docker-up-observ:
	@echo "Starting observability stack (OTel Collector + Jaeger)..."
	docker-compose -f infra/compose/docker-compose.observability.yml up -d

docker-down-observ:
	@echo "Stopping observability stack..."
	docker-compose -f infra/compose/docker-compose.observability.yml down -v

# Stop Docker Compose services
docker-stop:
	@echo "Stopping Docker Compose services..."
	docker-compose down

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	@echo "Installing dependencies..."
	go -C apps/server mod tidy
	@echo "Creating .env file if it doesn't exist..."
	@test -f .env || cp .env.example .env
	@echo "Setup complete! Edit .env file with your configuration."

# Format code
fmt:
	@echo "Formatting code..."
	go -C apps/server fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	cd apps/server && golangci-lint run ./...

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	go -C apps/server mod tidy
	go -C apps/server mod download

# Generate API documentation (if using swag)
docs:
	@echo "Generating API documentation..."
	@command -v swag >/dev/null 2>&1 || { echo "swag is not installed. Install with: go install github.com/swaggo/swag/cmd/swag@latest"; exit 1; }
	swag init -g apps/server/cmd/server/main.go -o docs/

# Internal targets with ldflags (version info)
VERSION ?= dev
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X 'servify/apps/server/internal/version.Version=$(VERSION)' -X 'servify/apps/server/internal/version.Commit=$(GIT_COMMIT)' -X 'servify/apps/server/internal/version.BuildTime=$(BUILD_TIME)'

_build-with-ldflags:
	go -C apps/server build -ldflags "$(LDFLAGS)" -o ../../bin/servify ./cmd/server
	go -C apps/server build -ldflags "$(LDFLAGS)" -o ../../bin/migrate ./cmd/migrate
	go -C apps/server build -ldflags "$(LDFLAGS)" -o ../../bin/servify-cli ./cmd/cli

_build-cli-with-ldflags:
	go -C apps/server build -ldflags "$(LDFLAGS)" -o ../../bin/servify-cli ./cmd/cli

_build-cli-weknora-with-ldflags:
	go -C apps/server build -ldflags "$(LDFLAGS)" -tags weknora -o ../../bin/servify-cli-weknora ./cmd/cli

# Database operations
db-reset: migrate-seed
	@echo "Database reset complete with seed data"

# Show application status
status:
	@echo "Checking application status..."
	@curl -s http://localhost:8080/health | json_pp || echo "Application not running"

# View logs (for Docker Compose)
logs:
	@echo "Showing application logs..."
	docker-compose -f infra/compose/docker-compose.yml logs -f servify

# Sync SDK bundles into demo web
demo-sync-sdk:
	@echo "Syncing SDK bundles into apps/demo-web/sdk ..."
	chmod +x ./scripts/sync-sdk-to-demo.sh
	./scripts/sync-sdk-to-demo.sh

# Website (Cloudflare Worker) commands
website-dev:
	@echo "Starting Cloudflare Worker dev for website..."
	npm -C apps/website-worker run dev

website-deploy:
	@echo "Deploying website to Cloudflare Workers (requires wrangler login and secrets)..."
	npx --yes wrangler deploy --config apps/website-worker/wrangler.toml

website-pages-deploy:
	@echo "Deploying website to Cloudflare Pages..."
	@if ! command -v wrangler >/dev/null 2>&1; then echo 'wrangler not found. Install with: npm i -g wrangler'; exit 1; fi
	@CF_PAGES_PROJECT=$${CF_PAGES_PROJECT:-servify-website}; \
	echo "Using project name: $$CF_PAGES_PROJECT"; \
	wrangler pages deploy apps/website --project-name "$$CF_PAGES_PROJECT"
