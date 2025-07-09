# Game Integrator

A Go-based game integration service for handling user transactions and wallet operations.

## Quick Start

### Local Development
```bash
# Setup development environment
make dev-setup

# Start everything (migrate + seed + run)
make start
```

### Docker Development
```bash
# Build and start all services
make docker-build
make docker-up

# Run migrations and seed data
make docker-migrate
make docker-seed

# View logs
make docker-logs
```

## Makefile Commands

```bash
make help        # Show all available commands
make build       # Build all binaries
make run         # Run the API server
make test        # Run tests
make migrate     # Run database migrations
make seed        # Seed database with initial data
make setup-db    # Migrate + seed database
make start       # Migrate + seed + run (all in one)
make clean       # Clean build artifacts
make deps        # Download dependencies
make dev-setup   # Full development setup
make dev         # Run with hot reload (requires air)
make build-prod  # Build for production

# Docker commands
make docker-build    # Build Docker images
make docker-up       # Start all services with Docker Compose
make docker-down     # Stop all services
make docker-migrate  # Run migrations in Docker
make docker-seed     # Seed database in Docker
make docker-logs     # View logs
make docker-clean    # Clean Docker resources
```

## Manual Commands

```bash
# Run migrations
go run cmd/migrate/main.go

# Seed database
go run cmd/seed/main.go

# Start API server
go run cmd/api/main.go
```

## API Endpoints

### Authentication

#### Login
- **POST** `/api/v1/auth/login`
- **Description**: Authenticate user and return JWT token
- **Request Body**:
  ```json
  {
    "username": "user1",
    "password": "password123"
  }
  ```
- **Response**:
  ```json
  {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 123,
      "username": "user1",
      "balance": 1000.50,
      "currency": "USD"
    }
  }
  ```

### Users

#### Get Current User Info
- **GET** `/api/v1/users/me`
- **Description**: Get current user information from JWT token
- **Headers**: `Authorization: Bearer <token>`
- **Response**:
  ```json
  {
    "id": 123,
    "username": "user1",
    "balance": 1000.50,
    "currency": "USD"
  }
  ```

### Transactions

#### Create Withdrawal
- **POST** `/api/v1/transactions/withdraw`
- **Description**: Create a withdrawal transaction
- **Headers**: `Authorization: Bearer <token>`
- **Request Body**:
  ```json
  {
    "amount": 100.50,
    "provider_tx_id": "provider_12345",
    "currency": "USD"
  }
  ```
- **Response**:
  ```json
  {
    "transaction_id": 1,
    "user_id": 123,
    "type": "withdraw",
    "status": "pending",
    "amount": 100.50,
    "currency": "USD",
    "provider_tx_id": "provider_12345",
    "old_balance": 1000.50,
    "new_balance": 899.50,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
  ```

#### Create Deposit
- **POST** `/api/v1/transactions/deposit`
- **Description**: Create a deposit transaction
- **Headers**: `Authorization: Bearer <token>`
- **Request Body**:
  ```json
  {
    "amount": 50.25,
    "provider_tx_id": "provider_67890",
    "provider_withdrawn_tx_id": 1,
    "currency": "USD"
  }
  ```
- **Response**:
  ```json
  {
    "transaction_id": 2,
    "user_id": 123,
    "type": "deposit",
    "status": "completed",
    "amount": 50.25,
    "currency": "USD",
    "provider_tx_id": "provider_67890",
    "provider_withdrawn_tx_id": 1,
    "old_balance": 899.50,
    "new_balance": 949.75,
    "created_at": "2024-01-15T10:35:00Z",
    "updated_at": "2024-01-15T10:35:00Z"
  }
  ```

#### Cancel Transaction
- **POST** `/api/v1/transactions/cancel/:provider_tx_id`
- **Description**: Cancel a pending transaction
- **Headers**: `Authorization: Bearer <token>`
- **URL Parameters**: `provider_tx_id` - Provider transaction ID
- **Response**:
  ```json
  {
    "transaction_id": 1,
    "user_id": 123,
    "type": "withdraw",
    "status": "cancelled",
    "amount": 100.50,
    "currency": "USD",
    "provider_tx_id": "provider_12345",
    "old_balance": 899.50,
    "new_balance": 1000.00,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:40:00Z"
  }
  ```

### Error Responses

All endpoints return consistent error responses:

```json
{
  "code": "ERROR_CODE",
  "message": "Human readable error message",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

Common error codes:
- `TOKEN_MISSING` - Authorization header required
- `TOKEN_INVALID` - Invalid JWT token
- `INSUFFICIENT_BALANCE` - Not enough balance for transaction
- `TRANSACTION_CANNOT_CANCEL` - Transaction cannot be cancelled
- `USER_NOT_FOUND` - User not found
- `TRANSACTION_NOT_FOUND` - Transaction not found

## Architecture

- **Clean Architecture** with domain-driven design
- **FX** for dependency injection
- **Gin** for HTTP server
- **GORM** for database operations
- **JWT** for authentication
- **External Wallet Service** integration via kentechsp/wallet-client

## Configuration

### Environment-Based Configuration

The application uses environment-specific configuration files and environment variables for Docker deployments:

#### Local Development
- Uses `config/config.development.yml` by default
- Environment variables can override config values with `GAME_INTEGRATOR_` prefix

#### Docker/Production
- Uses `config/config.production.yml` 
- All configuration is set via environment variables in `docker-compose.yml`
- `.env` file can override default Docker env

#### Configuration Files
- `config/config.development.yml` - Local development settings
- `config/config.production.yml` - Production/Docker settings

#### Environment Variables
All environment variables use the `GAME_INTEGRATOR_` prefix:

```bash
# Database Configuration
GAME_INTEGRATOR_DATABASE_HOST=postgres
GAME_INTEGRATOR_DATABASE_PORT=5432
GAME_INTEGRATOR_DATABASE_USER=postgres
GAME_INTEGRATOR_DATABASE_PASSWORD=password
GAME_INTEGRATOR_DATABASE_NAME=game-integrator-db
GAME_INTEGRATOR_DATABASE_SSL=disable

# Server Configuration
GAME_INTEGRATOR_SERVER_PORT=8080

# Wallet Service Configuration
GAME_INTEGRATOR_WALLET_URL=http://wallet:8000
GAME_INTEGRATOR_WALLET_API_KEY=secret-key

# Environment Selection
GAME_INTEGRATOR_ENV=production
```

### Docker Services

The application includes several Docker services:

- **API** - Main application server
- **PostgreSQL** - Database
- **Wallet Service** - External wallet integration (kentechsp/wallet-client)
- **Migrate** - Database migration tool (profile: migrate)
- **Seed** - Database seeding tool (profile: seed)

## Development

```bash
# Build all commands
make build

# Run tests
make test

# API documentation
# Available at /swagger/index.html when server is running
```

## Project Structure

```
├── cmd/                   # Application entry points
│   ├── api/               # Main API server
│   ├── migrate/           # Database migration tool
│   └── seed/              # Database seeding tool
├── config/                # Configuration files
│   ├── config.development.yml
│   └── config.production.yml
├── docs/                  # API documentation
│   ├── docs.go            # Swagger documentation
│   ├── swagger.json       # OpenAPI specification
│   └── swagger.yaml       # OpenAPI specification
├── internal/              # Private application code
│   ├── app/               # FX dependency injection & application setup
│   ├── config/            # Configuration management
│   ├── domain/            # Business logic, entities & interfaces
│   ├── http/              # HTTP layer
│   │   ├── handlers/      # HTTP request handlers
│   │   ├── middleware/    # HTTP middleware (JWT, error handling)
│   │   └── server.go      # HTTP server setup
│   ├── infrastructure/    # External dependencies & implementations
│   │   ├── auth/          # JWT authentication service
│   │   ├── database/      # Database connection & configuration
│   │   ├── external/      # External service integrations
│   │   ├── repository/    # Data access layer implementations
│   │   └── seeder/        # Database seeding logic
│   └── usecase/           # Application use cases & business logic
├── migrations/            # Database migration files
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── Makefile               # Build & development commands
└── README.md              # Project documentation
```
