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
go run main.go seed             # Populate database with test data
go run main.go seed --reset     # Clear and repopulate test data

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
# Generate SSL certificates (required for database security)
cd ../config/ssl/postgres
chmod +x create-certs.sh
./create-certs.sh
cd ../../../backend

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
- `DB_DSN`: PostgreSQL connection string (use sslmode=require for GDPR compliance)
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

## Critical BUN ORM Patterns

### Schema-Qualified Table Expressions

CRITICAL: Always use quotes around table aliases in PostgreSQL schema-qualified queries:

```go
// CORRECT - Quotes around alias
ModelTableExpr(`users.teachers AS "teacher"`)

// WRONG - Will cause "TeacherResult does not have column 'id'" errors
ModelTableExpr(`users.teachers AS teacher`)
```

### Loading Nested Relationships

When loading nested relationships (e.g., Teacher → Staff → Person), use explicit JOINs with column aliasing:

```go
type teacherResult struct {
    Teacher *users.Teacher `bun:"teacher"`
    Staff   *users.Staff   `bun:"staff"`
    Person  *users.Person  `bun:"person"`
}

err := r.db.NewSelect().
    Model(result).
    ModelTableExpr(`users.teachers AS "teacher"`).
    // IMPORTANT: Explicit column mapping for each table
    ColumnExpr(`"teacher".id AS "teacher__id"`).
    ColumnExpr(`"staff".id AS "staff__id"`).
    ColumnExpr(`"person".first_name AS "person__first_name"`).
    Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
    Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
    Where(`"teacher".id = ?`, id).
    Scan(ctx)
```

### Model BeforeAppendModel Hook

Models should implement `BeforeAppendModel` when using schemas:

```go
func (g *Group) BeforeAppendModel(query any) error {
    if q, ok := query.(*bun.SelectQuery); ok {
        q.ModelTableExpr(`education.groups AS "group"`)
    }
    // Handle other query types...
    return nil
}
```

## Repository and Service Factories

```go
// Create repository factory
repoFactory := repositories.NewFactory(db)
userRepo := repoFactory.NewUserRepository()

// Create service factory
serviceFactory := services.NewFactory(repoFactory, mailer)
authService := serviceFactory.NewAuthService()
```

## Transaction Context Pattern

Pass transactions via context:

```go
// Start transaction and add to context
ctx = base.ContextWithTx(ctx, &tx)

// Repositories check for transaction in context
if tx, ok := base.TxFromContext(ctx); ok {
    // Use transaction
}
```

## QueryOptions Pattern

Use the base QueryOptions for consistent query building:

```go
options := base.NewQueryOptions()
filter := base.NewFilter()
filter.Equal("name", "value")
filter.ILike("field", "%pattern%")
filter.In("id", []int64{1, 2, 3})
options.Filter = filter
options.WithPagination(1, 50)
```

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
- Seed data documentation in `docs/seed-data.md`

## SSL Security

Database connections use SSL encryption for GDPR compliance:
- SSL certificates in `../config/ssl/postgres/certs/` (generated by create-certs.sh)
- Connection string uses `sslmode=require` for development
- Production should use `sslmode=verify-full` with proper CA certificates
- Certificate files are excluded from git (.gitignore)

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