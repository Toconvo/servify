# Servify Makefile

.PHONY: help build run migrate test clean docker-build docker-run

# Default target
help:
	@echo "Available commands:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  migrate       - Run database migrations"
	@echo "  migrate-seed  - Run database migrations with seed data"
	@echo "  test          - Run tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run with Docker Compose"
	@echo "  docker-stop   - Stop Docker Compose services"

# Build the application
build:
	@echo "Building Servify..."
	go build -o bin/servify cmd/server/main.go
	go build -o bin/migrate cmd/migrate/main.go

# Run the application
run:
	@echo "Starting Servify server..."
	go run cmd/server/main.go

# Run database migrations
migrate:
	@echo "Running database migrations..."
	go run cmd/migrate/main.go

# Run database migrations with seed data
migrate-seed:
	@echo "Running database migrations with seed data..."
	go run cmd/migrate/main.go --seed

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

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