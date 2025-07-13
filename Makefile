# Game Integrator API Makefile

.PHONY: help build run test migrate seed clean deps mocks

# Default target
help:
	@echo "Available commands:"
	@echo "  build    - Build all binaries"
	@echo "  run      - Run the API server"
	@echo "  test     - Run tests"
	@echo "  mocks    - Generate mocks for testing"
	@echo "  migrate  - Run database migrations"
	@echo "  seed     - Seed database with initial data"
	@echo "  clean    - Clean build artifacts"
	@echo "  deps     - Download dependencies"
	@echo "  start    - Migrate + seed + run (all in one)"
	@echo ""
	@echo "Docker commands:"
	@echo "  docker-build    - Build Docker images"
	@echo "  docker-up       - Start all services with Docker Compose"
	@echo "  docker-down     - Stop all services"
	@echo "  docker-migrate  - Run migrations in Docker"
	@echo "  docker-seed     - Seed database in Docker"
	@echo "  docker-setup    - Start services + migrate + seed (all in one)"
	@echo "  docker-logs     - View logs"
	@echo "  docker-clean    - Clean Docker resources"

# Build all binaries
build:
	@echo "Building binaries..."
	go build -o bin/api cmd/api/main.go
	go build -o bin/migrate cmd/migrate/main.go
	go build -o bin/seed cmd/seed/main.go
	@echo "Build complete!"

# Run the API server
run:
	@echo "Starting API server..."
	go run cmd/api/main.go

# Generate mocks for testing
mocks:
	@echo "Generating mocks..."
	mockgen -source=internal/domain/wallet_service.go -destination=internal/domain/mocks/wallet_service_mock.go -package=mocks
	mockgen -source=internal/domain/transaction.go -destination=internal/domain/mocks/transaction_repository_mock.go -package=mocks
	mockgen -source=internal/domain/user.go -destination=internal/domain/mocks/user_repository_mock.go -package=mocks
	mockgen -source=internal/domain/outbox.go -destination=internal/domain/mocks/outbox_repository_mock.go -package=mocks
	@echo "Mocks generated!"

# Run tests
test: mocks
	@echo "Running tests..."
	go test -v ./...

# Run database migrations (with defaults)
migrate:
	@echo "Running migrations with defaults..."
	go run cmd/migrate/main.go -env=development -action=up

# Seed database (with defaults)
seed:
	@echo "Seeding database with defaults..."
	go run cmd/seed/main.go -env=development

# Setup database (migrate + seed)
setup-db: migrate seed
	@echo "Database setup complete!"

# Start everything (migrate + seed + run)
start: setup-db run

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	@echo "Clean complete!"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated!"

# Development setup (deps + setup-db)
dev-setup: deps setup-db
	@echo "Development environment ready!"

# Build for production
build-prod:
	@echo "Building for production..."
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api cmd/api/main.go
	@echo "Production build complete!"

# Docker commands
docker-build:
	@echo "Building Docker images..."
	docker-compose build
	@echo "Docker images built!"

docker-up:
	@echo "Starting services with Docker Compose..."
	docker-compose up -d
	@echo "Services started!"

docker-down:
	@echo "Stopping services..."
	docker-compose down
	@echo "Services stopped!"

docker-migrate:
	@echo "Running migrations in Docker with defaults..."
	docker-compose --profile migrate run --rm migrate ./migrate -env=production -action=up
	@echo "Migrations completed!"

docker-seed:
	@echo "Seeding database in Docker..."
	docker-compose --profile seed up seed
	@echo "Database seeded!"

docker-setup: docker-up docker-migrate docker-seed
	@echo "Docker setup complete! All services running with database migrated and seeded."

docker-logs:
	@echo "Viewing logs..."
	docker-compose logs -f

docker-clean:
	@echo "Cleaning Docker resources..."
	docker-compose down -v --remove-orphans
	docker system prune -f
	@echo "Docker resources cleaned!"

docker-restart:
	@echo "Docker restart..."
	docker-compose down
	docker-compose build
	docker-compose up