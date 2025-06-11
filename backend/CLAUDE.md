# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Context

Backend service for Project Phoenix - a RFID-based student attendance and room management system. Built with Go 1.21+ using Chi router, Bun ORM, and PostgreSQL with multi-schema architecture.

## Development Commands

```bash
# Environment Setup
cp dev.env.example dev.env      # Create local config (edit DB_DSN and AUTH_JWT_SECRET)

# Server Operations
go run main.go serve            # Start server (port 8080)
go run main.go migrate          # Run database migrations
go run main.go migrate status   # Show migration status
go run main.go migrate validate # Validate migration dependencies
go run main.go migrate reset    # WARNING: Reset database and run all migrations

# Development Data
go run main.go seed             # Populate database with test data
go run main.go seed --reset     # Clear ALL test data and repopulate

# Data Cleanup (GDPR Compliance)
go run main.go cleanup visits   # Delete expired visit records based on privacy consent
go run main.go cleanup preview  # Preview what would be deleted (dry run)
go run main.go cleanup stats    # Show data retention statistics

# Documentation
go run main.go gendoc           # Generate routes.md and OpenAPI spec

# Testing
go test ./...                   # Run all tests
go test -v ./api/auth           # Run specific package with verbose output
go test -race ./...             # Run tests with race detection
go test ./api/auth -run TestLogin  # Run specific test

# Code Quality (Run before committing!)
golangci-lint run --timeout 10m # Run linter
golangci-lint run --fix         # Auto-fix linting issues
go fmt ./...                    # Format code
/Users/yonnock/go/bin/goimports -w .  # Organize imports
go mod tidy                     # Clean up dependencies
```

## Docker Development

```bash
# SSL Setup (Required - GDPR compliance)
cd ../config/ssl/postgres && ./create-certs.sh && cd ../../../backend

# Development with Docker
docker compose up -d postgres   # Start only database
docker compose run server ./main migrate  # Run migrations
docker compose up               # Start all services
docker compose logs -f server   # View server logs
```

## Architecture Patterns

### Domain-Driven Design Structure
```
api/{domain}/           # HTTP handlers (thin layer)
services/{domain}/      # Business logic (orchestration)
models/{domain}/        # Domain models and repository interfaces
database/repositories/{domain}/  # Data access implementation
```

### Factory Pattern for Dependency Injection
```go
// Repository factory
repoFactory := repositories.NewFactory(db)
userRepo := repoFactory.NewUserRepository()

// Service factory
serviceFactory := services.NewFactory(repoFactory, mailer)
authService := serviceFactory.NewAuthService()
```

### Authentication & Authorization
- JWT tokens: Access (15m) + Refresh (1hr)
- Role-based permissions via middleware
- Permission constants in `auth/authorize/permissions/`
- Authorization policies in `auth/authorize/policies/`

## Critical BUN ORM Patterns

### Schema-Qualified Tables (MUST USE QUOTES!)
```go
// CORRECT - Quotes around alias prevent "column not found" errors
ModelTableExpr(`users.teachers AS "teacher"`)

// WRONG - Missing quotes causes BUN mapping failures
ModelTableExpr(`users.teachers AS teacher`)
```

### Loading Nested Relationships
```go
// For Teacher → Staff → Person relationships
type teacherResult struct {
    Teacher *users.Teacher `bun:"teacher"`
    Staff   *users.Staff   `bun:"staff"`
    Person  *users.Person  `bun:"person"`
}

err := r.db.NewSelect().
    Model(result).
    ModelTableExpr(`users.teachers AS "teacher"`).
    // Explicit column mapping required for each table
    ColumnExpr(`"teacher".id AS "teacher__id"`).
    ColumnExpr(`"staff".id AS "staff__id"`).
    ColumnExpr(`"person".* AS "person__*"`).
    Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
    Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
    Where(`"teacher".id = ?`, id).
    Scan(ctx)
```

### Repository Pattern with Transactions
```go
// Pass transaction via context
ctx = base.ContextWithTx(ctx, &tx)

// Repository checks for transaction
if tx, ok := base.TxFromContext(ctx); ok {
    // Use transaction
}
```

### QueryOptions for Filtering
```go
options := base.NewQueryOptions()
filter := base.NewFilter()
filter.Equal("status", "active")
filter.ILike("name", "%pattern%")
filter.In("id", []int64{1, 2, 3})
options.Filter = filter
options.WithPagination(1, 50)
```

## Database Schema Organization

PostgreSQL schemas separate domain concerns:
- `auth`: Authentication, tokens, permissions, roles
- `users`: Persons, staff, students, teachers, guardians
- `education`: Groups, substitutions, assignments
- `facilities`: Rooms and locations
- `activities`: Student activities and enrollments
- `active`: Real-time visit and group tracking
- `schedule`: Timeframes, dateframes, recurrence
- `iot`: RFID devices
- `feedback`: User feedback entries
- `config`: System settings

## Migration System

```go
// database/migrations/{number}_{name}.go
var Dependencies = []string{
    "001000001_auth_accounts",  // Required migrations
}

var Migration = `
CREATE TABLE IF NOT EXISTS schema.table_name (...);
`

var Rollback = `DROP TABLE IF EXISTS schema.table_name CASCADE;`
```

## Testing Strategy

```go
func TestFeature(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer cleanupTestDB(db)
    
    // Create test data
    user := createTestUser(t, db)
    
    // Test functionality
    result, err := service.DoSomething(ctx, user.ID)
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

## Common Linting Fixes

```go
// 1. Check errors (errcheck)
if _, err := w.Write(data); err != nil {
    log.Printf("write failed: %v", err)
}

// 2. Context keys (staticcheck)
type contextKey string
const userContextKey = contextKey("user")

// 3. Remove unused assignments
// 4. Implement or remove empty branches
```

## API Error Response Pattern

```go
type ErrorResponse struct {
    Status  string `json:"status"`   // "error"
    Message string `json:"message"`  // Human-readable message
    Code    string `json:"code,omitempty"`  // Machine-readable code
}
```

## Environment Variables

Key variables in `dev.env`:
- `DB_DSN`: PostgreSQL connection (use `sslmode=require`)
- `AUTH_JWT_SECRET`: JWT signing key
- `DB_DEBUG=true`: Log SQL queries
- `ENABLE_CORS=true`: For frontend development
- `LOG_LEVEL=debug`: Logging verbosity

Automated Cleanup Scheduler:
- `CLEANUP_SCHEDULER_ENABLED=true`: Enable automated daily cleanup
- `CLEANUP_SCHEDULER_TIME=02:00`: Time to run cleanup (24-hour format)
- `CLEANUP_SCHEDULER_TIMEOUT_MINUTES=30`: Maximum cleanup duration

## Seed Data

Creates test data for development:
- 24 rooms across different buildings
- 25 groups (10 grade classes, 15 activities)
- 150 persons (30 staff/teachers, 120 students)
- Guardians, RFID cards, and relationships

## SSL Security

GDPR-compliant database connections:
- Certificates in `../config/ssl/postgres/certs/`
- Development: `sslmode=require`
- Production: `sslmode=verify-full`
- Run `create-certs.sh` before first use

## RFID Integration

- Device authentication endpoints
- Real-time visit tracking
- Room occupancy monitoring
- Student check-in/check-out flows