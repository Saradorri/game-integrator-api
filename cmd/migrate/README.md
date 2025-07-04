# Database Migration Tool

A simple migration tool using `golang-migrate/migrate` for PostgreSQL database migrations.

## Usage

### Build
```bash
go build -o migrate cmd/migrate/main.go
```

### Command-line flags
- `-config`: Path to config directory (default: `./config`)
- `-env`: Environment configuration file (default: `development`)
- `-action`: Migration action: `up`, `down` (default: `up`)

### Examples

```bash
# Apply all pending migrations
./migrate -action=up

# Rollback all migrations
./migrate -action=down

# Use production environment
./migrate -env=production -action=up
```

## Migration Files

Expected format in `./migrations/`:
- `000001_create_users_table.up.sql`
- `000001_create_users_table.down.sql`
- `000002_create_transactions_table.up.sql`
- `000002_create_transactions_table.down.sql`

## Configuration

Uses the same config files as the main app:
- `config/config.development.yml`
- `config/config.production.yml`

## Error Handling

- The tool will exit with a non-zero code if migrations fail
- `migrate.ErrNoChange`