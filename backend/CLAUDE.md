# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Context

**Project Name:** Project-Phoenix Backend

**Description:** Go API server for managing student attendance and location tracking using RFID technology in educational institutions. Provides RESTful APIs for tracking student presence, room occupancy, and comprehensive management of educational resources.

**Key Technologies:**
- Go 1.23+
- Chi router for HTTP routing
- Bun ORM for PostgreSQL interactions
- JWT authentication with lestrrat-go/jwx
- Cobra CLI framework
- Docker for containerization

## Common Development Commands

### Backend (Go) Commands
```bash
# Server and database
go run main.go serve                      # Start the backend server on port 8080
go run main.go migrate                    # Run database migrations
go run main.go migrate reset              # Reset database and run all migrations (WARNING: destroys all data)
go run main.go migrate status             # Show migration status
go run main.go migrate validate           # Validate migration dependencies

# Documentation
go run main.go gendoc                     # Generate both API docs (routes.md and OpenAPI spec)
go run main.go gendoc --routes            # Generate only routes documentation
go run main.go gendoc --openapi           # Generate only OpenAPI specification

# Testing
go test ./...                             # Run all tests
go test -v ./...                          # Run tests with verbose output
go test ./api/users -run TestFunction     # Run specific test
go test -cover ./...                      # Run tests with coverage

# Dependencies
go mod tidy                               # Clean up and organize dependencies
go get -u ./...                           # Update all dependencies
```

### Docker Commands
```bash
# Quick start development
docker compose up -d postgres             # Start only the database
docker compose run server ./main migrate  # Run migrations
docker compose up                         # Start all services (frontend, backend, database)

# Other useful commands
docker compose down                       # Stop and remove all containers
docker compose logs -f server            # Follow server logs
docker compose logs postgres             # View database logs
docker compose exec postgres psql -U postgres  # Access database directly
```

## Code Architecture

### High-Level Architecture

The backend follows a layered architecture with clear separation of concerns:

1. **API Layer** (`/api/`): HTTP handlers and route definitions
   - Each domain has its own package (auth, students, rooms, etc.)
   - Handlers use `render.Bind` for request parsing and `render.JSON` for responses
   - Error responses use domain-specific error types

2. **Service Layer** (`/services/`): Business logic and orchestration
   - Services encapsulate complex operations
   - Handle transactions and cross-domain operations
   - Return domain-specific errors

3. **Model Layer** (`/models/`): Data models and validation
   - Each model embeds `base.Model` for common fields
   - Validation using ozzo-validation
   - Repository interfaces for data access patterns

4. **Database Layer** (`/database/`): Database operations
   - Migrations organized by domain and order
   - Repositories implement data access
   - Uses Bun ORM for PostgreSQL operations

### Database Schema Organization

The database uses multiple schemas to organize tables by domain:

- **auth**: Authentication (accounts, tokens, permissions, roles)
- **users**: User profiles (persons, students, teachers, staff)
- **education**: Educational structures (groups, substitutions)
- **activities**: Student activities and schedules
- **facilities**: Physical locations (rooms)
- **schedule**: Time management (timeframes, recurrence rules)
- **active**: Real-time tracking (visits, active groups)
- **iot**: Device management (RFID devices)
- **feedback**: User feedback entries
- **config**: System configuration

### Key Patterns

**Repository Pattern**:
```go
type StudentRepository interface {
    base.Repository[Student]
    FindByPersonID(ctx context.Context, personID int64) (*Student, error)
}
```

**Service Pattern**:
```go
type UserService interface {
    GetStudent(ctx context.Context, id int64) (*Student, error)
    CreateStudent(ctx context.Context, student *Student) error
}
```

**Error Handling**:
```go
var (
    ErrStudentNotFound = NewHTTPError(http.StatusNotFound, "student not found", "STU001")
    ErrInvalidStudentData = NewHTTPError(http.StatusBadRequest, "invalid student data", "STU002")
)
```

**API Handler Pattern**:
```go
func handleGetStudent(svc services.StudentService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")
        student, err := svc.GetStudent(r.Context(), id)
        if err != nil {
            render.Render(w, r, ErrInvalidRequest(err))
            return
        }
        render.JSON(w, r, student)
    }
}
```

## Authentication & Authorization

- JWT-based authentication using lestrrat-go/jwx
- Role-based access control (RBAC) with permissions
- Policy-based authorization for complex rules
- API key authentication for IoT devices

Key roles:
- `admin`: Full system access
- `teacher`: Educational resource management
- `student`: Limited personal data access
- `user`: Basic authenticated access

## Environment Configuration

Copy `dev.env.example` to `dev.env` and configure:

Essential variables:
- `DB_DSN`: PostgreSQL connection string
- `AUTH_JWT_SECRET`: Secret for JWT signing (generate securely)
- `ADMIN_EMAIL`/`ADMIN_PASSWORD`: Initial admin account
- `LOG_LEVEL`: Set to `debug` for development
- `ENABLE_CORS`: Set to `true` for frontend development

## Testing Strategy

The project uses table-driven tests with testify for assertions:

```go
func TestStudentValidation(t *testing.T) {
    tests := []struct {
        name    string
        student Student
        wantErr bool
    }{
        // Test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.student.Validate()
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Key API Endpoints

### Authentication
- `POST /api/auth/login` - User login
- `POST /api/auth/register` - User registration  
- `POST /api/auth/refresh` - Refresh JWT tokens
- `GET /api/auth/debug-token` - Debug JWT tokens (dev only)

### Core Resources
- `/api/students` - Student management
- `/api/rooms` - Room tracking
- `/api/activities` - Activity management
- `/api/groups` - Group management
- `/api/active/visits` - Real-time visit tracking
- `/api/active/groups` - Active group sessions

### RFID Integration
- `/api/iot/devices` - Device management
- `/api/iot/authenticate` - Device authentication
- Special endpoints for RFID event processing

## Migration Management

Migrations follow a structured naming convention:
```
XXXYYY_schema_description.go
```
Where:
- `XXX`: Schema order (000-999)
- `YYY`: Migration order within schema
- `schema`: Target schema name
- `description`: Migration purpose

Migration commands:
```bash
go run main.go migrate                    # Run pending migrations
go run main.go migrate status            # Check migration status
go run main.go migrate validate          # Validate dependencies
go run main.go migrate reset            # Reset and re-run all
```

## Code Style Guidelines

- Follow standard Go conventions (gofmt, golint)
- Use meaningful variable names
- Keep functions small and focused
- Return early for error conditions
- Use context for cancellation and timeouts
- Log errors at appropriate levels
- Document public APIs with godoc comments

## Debugging Tips

1. Enable debug logging: `LOG_LEVEL=debug`
2. Enable SQL logging: `DB_DEBUG=true`
3. Check JWT tokens: Use `/api/auth/debug-token` endpoint
4. Monitor database: `docker compose logs postgres`
5. Use `httputil.DumpRequest` for request debugging
6. Check migration status if database errors occur

## Common Patterns to Follow

### Error Response
```go
render.Render(w, r, ErrInvalidRequest(err))
```

### Context Usage
```go
ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
defer cancel()
```

### Transaction Handling
```go
err := svc.db.WithTx(ctx, func(ctx context.Context, tx bun.Tx) error {
    // Operations within transaction
    return nil
})
```

### Validation
```go
func (s *Student) Validate() error {
    return validation.ValidateStruct(s,
        validation.Field(&s.PersonID, validation.Required),
        validation.Field(&s.GroupID, validation.Required),
    )
}
```