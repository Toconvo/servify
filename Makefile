# Servify Makefile

.PHONY: help build build-cli build-weknora run run-cli run-weknora migrate migrate-seed test clean docker-build docker-run docker-up-weknora docker-down docker-logs-weknora

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
	@echo "  docker-stop   - Stop Docker Compose services"

# Build the application
build:
	@echo "Building Servify..."
	go build -o bin/servify cmd/server/main.go
	go build -o bin/migrate cmd/migrate/main.go
	go build -o bin/servify-cli ./cmd/cli

# Build CLI targets
build-cli:
	@echo "Building CLI (standard)..."
	go build -o bin/servify-cli ./cmd/cli

build-weknora:
	@echo "Building CLI (weknora)..."
	go build -tags weknora -o bin/servify-cli-weknora ./cmd/cli

# Run the application
run:
	@echo "Starting Servify server..."
	go run cmd/server/main.go

run-cli:
	@echo "Running CLI (standard)..."
	go run ./cmd/cli -c $(or $(CONFIG),./config.yml) run

run-weknora:
	@echo "Running CLI (weknora)..."
	go run -tags weknora ./cmd/cli -c $(or $(CONFIG),./config.weknora.yml) run

# Run database migrations
migrate:
	@echo "Running database migrations..."
	DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD=$(DB_PASSWORD) DB_NAME=$(DB_NAME) DB_SSLMODE=$(or $(DB_SSLMODE),disable) DB_TIMEZONE=$(or $(DB_TIMEZONE),UTC) \
	go run cmd/migrate/main.go $(MIGRATE_ARGS)

# Run database migrations with seed data
migrate-seed:
	@echo "Running database migrations with seed data..."
	DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD=$(DB_PASSWORD) DB_NAME=$(DB_NAME) DB_SSLMODE=$(or $(DB_SSLMODE),disable) DB_TIMEZONE=$(or $(DB_TIMEZONE),UTC) \
	go run cmd/migrate/main.go --seed $(MIGRATE_ARGS)

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
	docker-compose -f docker-compose.yml -f docker-compose.weknora.yml up -d

docker-down:
	@echo "Stopping services..."
	docker-compose down

docker-logs-weknora:
	@echo "Tailing servify logs..."
	docker-compose -f docker-compose.yml -f docker-compose.weknora.yml logs -f servify

# Stop Docker Compose services
docker-stop:
	@echo "Stopping Docker Compose services..."
	docker-compose down

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	@echo "Installing dependencies..."
	go mod tidy
	@echo "Creating .env file if it doesn't exist..."
	@test -f .env || cp .env.example .env
	@echo "Setup complete! Edit .env file with your configuration."

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	go mod tidy
	go mod download

# Generate API documentation (if using swag)
docs:
	@echo "Generating API documentation..."
	@command -v swag >/dev/null 2>&1 || { echo "swag is not installed. Install with: go install github.com/swaggo/swag/cmd/swag@latest"; exit 1; }
	swag init -g cmd/server/main.go -o docs/

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
	docker-compose logs -f servify
