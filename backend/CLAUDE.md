# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Start the backend server
go run main.go serve            # Starts server on port 8080 (or PORT env var)

# Database operations
go run main.go migrate          # Run database migrations
go run main.go migrate status   # Show migration status
go run main.go migrate validate # Validate migration dependencies
go run main.go migrate reset    # WARNING: Reset database and run all migrations

# Documentation generation
go run main.go gendoc           # Generate both routes.md and OpenAPI spec
go run main.go gendoc --routes  # Generate only routes documentation
go run main.go gendoc --openapi # Generate only OpenAPI specification

# Testing
go test ./...                   # Run all tests
go test -v ./...               # Run tests with verbose output
go test -race ./...            # Run tests with race condition detection
go test ./api/auth -run TestLogin  # Run specific test

# Linting
golangci-lint run --timeout 10m  # Run linter (install: brew install golangci-lint)
golangci-lint run --fix         # Auto-fix some linting issues
go fmt ./...                    # Format code
/Users/yonnock/go/bin/goimports -w .  # Organize imports

# Dependencies
go mod tidy                     # Clean up dependencies
go get -u ./...                # Update all dependencies
```

## Docker Development

```bash
# Start PostgreSQL only
docker compose up -d postgres

# Run migrations in docker
docker compose run server ./main migrate

# Start all services
docker compose up

# View logs
docker compose logs -f server
docker compose logs postgres
```

## Environment Setup

The backend uses `dev.env` file for local configuration. Copy `dev.env.example` to `dev.env` and configure:

```bash
cp dev.env.example dev.env
```

Key environment variables:
- `DB_DSN`: PostgreSQL connection string
- `AUTH_JWT_SECRET`: JWT secret key (change in production)
- `ENABLE_CORS`: Set to `true` for cross-origin requests during development
- `LOG_LEVEL`: Options: debug, info, warn, error
- `DB_DEBUG`: Set to `true` to log SQL queries

## Architecture Overview

The backend follows a layered architecture:

```
api/            # HTTP handlers and routing
├── active/     # Active sessions management
├── auth/       # Authentication endpoints
├── users/      # User management
└── ...

services/       # Business logic layer
├── auth/       # Authentication services
├── active/     # Active session services
└── ...

models/         # Data models and interfaces
├── base/       # Base model and repository interfaces
├── auth/       # Authentication models
└── ...

database/
├── migrations/ # Database migration files
└── repositories/ # Data access layer

auth/           # Authentication subsystem
├── authorize/  # Authorization and permissions
├── jwt/        # JWT token handling
└── userpass/   # Username/password auth
```

### Key Patterns

1. **Repository Pattern**: Each domain has a repository interface in `models/` and implementation in `database/repositories/`

2. **Service Layer**: Business logic is in `services/`, keeping HTTP handlers thin

3. **Factory Pattern**: Both services and repositories use factories for dependency injection

4. **Migration System**: Numbered migrations with dependency tracking in `database/migrations/`

5. **Error Handling**: Each package has its own `errors.go` with domain-specific errors

6. **JWT Authentication**: Access tokens (15min) and refresh tokens (1hr) with role-based permissions

## Testing

```go
// Example test structure
func TestUserLogin(t *testing.T) {
    // Test setup uses test database
    db := setupTestDB(t)
    defer cleanupTestDB(db)
    
    // Create test data
    user := createTestUser(t, db)
    
    // Test the functionality
    result, err := authService.Login(ctx, user.Email, password)
    require.NoError(t, err)
    assert.NotEmpty(t, result.AccessToken)
}
```

Test helpers are in `test/helpers.go`. Integration tests use a real test database.

## Common Linting Issues

From `docs/linting-issues.md`:

1. **Unchecked errors** (errcheck):
   ```go
   // Fix by checking error returns
   if _, err := w.Write(data); err != nil {
       log.Printf("write failed: %v", err)
   }
   ```

2. **Context key type** (staticcheck):
   ```go
   // Define proper context keys
   type contextKey string
   const userContextKey = contextKey("user")
   ```

3. **Ineffective assignments**: Remove unused variable assignments

4. **Empty branches**: Add implementation or remove unnecessary conditions

## API Documentation

- Routes are documented in `routes.md` (generated)
- OpenAPI spec in `docs/openapi.yaml` (generated)
- RFID integration guides in `docs/rfid-*.md`

## Database Schema

The database uses multiple PostgreSQL schemas:
- `auth`: Authentication and authorization
- `users`: User profiles and identity
- `education`: Groups and educational structures
- `activities`: Student activities
- `facilities`: Rooms and locations
- `active`: Real-time session tracking
- `schedule`: Scheduling system
- `iot`: Device management
- `feedback`: User feedback
- `config`: System configuration

## RFID Integration

The system integrates with RFID readers for tracking:
- Device authentication via API endpoints
- Real-time student location tracking
- Room occupancy monitoring
- See `docs/rfid-integration-guide.md` for details