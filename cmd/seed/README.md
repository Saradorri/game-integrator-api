# Database Seeder

Populate the database with initial data for development and testing.

## Usage

```bash
# Use default config (./config/config.development.yml)
go run cmd/seed/main.go

# Specify config directory and environment
go run cmd/seed/main.go -config ./config -env production

# Build and run
go build cmd/seed/main.go
./main -config ./config -env development
```

## Flags

- `-config`: Path to config directory (default: `./config`)
- `-env`: Environment name (default: `development`)

## What it does

- Connects to database using configuration
- Seeds initial users with test data
- Logs progress and errors

## Example Output

```
Starting database seeding...
Database seeding completed successfully
```

## Prerequisites

- Database must be running and accessible
- Migrations must be applied first
- Valid configuration file must exist 