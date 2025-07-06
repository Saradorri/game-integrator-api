# Game Integrator API Makefile

.PHONY: help build run test migrate seed clean deps

# Default target
help:
	@echo "Available commands:"
	@echo "  build    - Build all binaries"
	@echo "  run      - Run the API server"
	@echo "  test     - Run tests"
	@echo "  migrate  - Run database migrations"
	@echo "  seed     - Seed database with initial data"
	@echo "  clean    - Clean build artifacts"
	@echo "  deps     - Download dependencies"
	@echo "  start    - Migrate + seed + run (all in one)"

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

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run database migrations
migrate:
	@echo "Running migrations..."
	go run cmd/migrate/main.go

# Seed database
seed:
	@echo "Seeding database..."
	go run cmd/seed/main.go

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