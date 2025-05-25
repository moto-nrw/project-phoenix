# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Context

**Project Name:** Project-Phoenix

**Description:** A GDPR-compliant RFID-based student attendance and room management system for educational institutions. Implements strict privacy controls for student data access.

**Key Technologies:**
- Backend: Go (1.21+) with Chi router, Bun ORM for PostgreSQL
- Frontend: Next.js (v15+) with React (v19+), Tailwind CSS (v4+)
- Database: PostgreSQL (17+) with SSL encryption (GDPR compliance)
- Authentication: JWT-based auth system with role-based access control
- RFID Integration: Custom API endpoints for device communication
- Deployment: Docker/Docker Compose

## Architecture Overview

The project follows a layered architecture with clear domain boundaries:

### Backend Structure (Go)
- **api/**: HTTP handlers and route definitions organized by domain
- **auth/**: Authentication and authorization mechanisms
- **cmd/**: CLI commands for server, migrations, and documentation
- **database/**: Database connections and migrations
- **models/**: Data models with validation and business logic
- **services/**: Core business logic organized by domain
- **email/**: Email templating and delivery services
- **logging/**: Structured logging utilities

### Frontend Structure (Next.js)
- **src/app/**: Next.js App Router pages and API routes
- **src/components/**: Reusable UI components organized by domain
- **src/lib/**: Utility functions, API clients, and helpers
- **src/styles/**: Global CSS and Tailwind configuration

### Database Schema Organization
The database uses multiple PostgreSQL schemas to organize tables by domain:
- **auth**: Authentication, tokens, permissions, roles
- **users**: User profiles, students, teachers, staff
- **education**: Groups and educational structures
- **facilities**: Rooms and physical locations
- **activities**: Student activities and enrollments
- **active**: Real-time session tracking
- **schedule**: Time and schedule management
- **iot**: Device management
- **feedback**: User feedback
- **config**: System configuration

## Development Commands

### Backend (Go)
```bash
cd backend

# Setup
cp dev.env.example dev.env      # Create environment file from template

# Server Operations
go run main.go serve            # Start server (port 8080)
go run main.go migrate          # Run database migrations
go run main.go migrate status   # Show migration status
go run main.go migrate validate # Validate migration dependencies
go run main.go migrate reset    # WARNING: Reset database and run all migrations

# Testing
go test ./...                   # Run all tests
go test -v ./api/auth           # Run specific package with verbose output
go test -race ./...             # Run tests with race condition detection
go test ./api/auth -run TestLogin  # Run specific test

# Documentation
go run main.go gendoc           # Generate both routes.md and OpenAPI spec
go run main.go gendoc --routes  # Generate only routes documentation
go run main.go gendoc --openapi # Generate only OpenAPI specification

# Code Quality
go fmt ./...                    # Format code
golangci-lint run --timeout 10m # Run linter (install: brew install golangci-lint)
golangci-lint run --fix         # Auto-fix some linting issues
go mod tidy                     # Clean up dependencies
go get -u ./...                 # Update all dependencies
/Users/yonnock/go/bin/goimports -w .  # Organize imports
```

### Frontend (Next.js)
```bash
cd frontend

# Development
npm run dev                     # Start dev server with turbo (port 3000)
npm run build                   # Build for production
npm run start                   # Start production server
npm run preview                 # Build and preview production

# Code Quality (Run before committing!)
npm run lint                    # ESLint check (max-warnings=0)
npm run lint:fix                # Auto-fix linting issues
npm run typecheck               # TypeScript type checking
npm run check                   # Run both lint and typecheck

# Formatting
npm run format:check            # Check Prettier formatting
npm run format:write            # Fix formatting issues
```

### Docker Operations
```bash
# SSL Setup (Required before starting services)
cd config/ssl/postgres && ./create-certs.sh && cd ../../..

# Quick Start
docker compose up               # Start all services
docker compose up -d postgres   # Start only database
docker compose up -d            # Start all services in detached mode

# Database Operations
docker compose run server ./main migrate  # Run migrations
docker compose logs postgres             # Check database logs

# Frontend Operations
docker compose run frontend npm run lint # Run lint checks in container
docker compose logs frontend            # Check frontend logs

# Cleanup
docker compose down             # Stop all services
docker compose down -v          # Stop and remove volumes
```

## Environment Configuration

### Quick Start (New Development Environment)
```bash
# Option 1: Use the automated setup script (recommended)
./scripts/setup-dev.sh          # Creates configs and SSL certs automatically

# Option 2: Manual setup
# Generate SSL certificates (required for database security)
cd config/ssl/postgres
chmod +x create-certs.sh
./create-certs.sh
cd ../../..

# Copy environment files
cp backend/dev.env.example backend/dev.env
cp frontend/.env.local.example frontend/.env.local

# Start services
docker compose up -d postgres   # Start database
docker compose run server ./main migrate  # Run migrations
docker compose up               # Start all services

# Frontend checks
cd frontend && npm run check
```

### Backend Environment Variables (dev.env)
```bash
# Database
DB_DSN=postgres://username:password@localhost:5432/database?sslmode=require
DB_DEBUG=true                   # Log SQL queries
# Note: sslmode=require enables SSL for GDPR compliance and security

# Authentication  
AUTH_JWT_SECRET=your_jwt_secret_here  # Change in production!
AUTH_JWT_EXPIRY=15m                   # Access token expiry
AUTH_JWT_REFRESH_EXPIRY=1h            # Refresh token expiry

# Admin Account (for initial setup)
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=strong_password_here   # Change immediately!

# Development
LOG_LEVEL=debug                 # Options: debug, info, warn, error
ENABLE_CORS=true               # Required for local development
PORT=8080                      # Server port
```

### Frontend Environment Variables (.env.local)
```bash
# API Configuration
NEXT_PUBLIC_API_URL=http://localhost:8080  # Backend API URL

# NextAuth Configuration
NEXTAUTH_URL=http://localhost:3000         # Frontend URL for auth
NEXTAUTH_SECRET=your_secret_here           # Generate with: openssl rand -base64 32

# Docker Build
SKIP_ENV_VALIDATION=true                   # For Docker builds
```

## High-Level Architecture

### Authentication Flow
1. JWT-based authentication with access (15min) and refresh (1hr) tokens
2. Role-based permissions checked via middleware
3. Frontend uses NextAuth with JWT strategy
4. API routes proxy requests with token in Authorization header

### Key API Patterns

**Backend Route Pattern:**
```go
// In api/{domain}/api.go
func (rs *Resource) Router() chi.Router {
    r := chi.NewRouter()
    r.Use(tokenAuth.Verifier())
    r.Use(jwt.Authenticator)
    
    r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.list)
    r.With(authorize.RequiresPermission(permissions.GroupsWrite)).Post("/", rs.create)
    return r
}
```

**Frontend API Client Pattern:**
```typescript
// In lib/{domain}-api.ts
export async function fetchResources(filters?: ResourceFilters): Promise<Resource[]> {
    const response = await apiGet('/resources', token, { params: filters });
    return mapResourcesResponse(response);
}
```

**Next.js Route Handler Pattern:**
```typescript
// In app/api/{resource}/route.ts
export const GET = createGetHandler(async (request, token, params) => {
    const response = await apiGet(`/api/resources`, token);
    return response.data; // Extract data from paginated response
});
```

### Repository Pattern (Backend)
Each domain has:
- Repository interface in `models/{domain}/repository.go`
- Implementation in `database/repositories/{domain}/`
- Service layer in `services/{domain}/`

**Factory Pattern**: Both services and repositories use factories for dependency injection:
```go
// Services factory in services/factory.go
serviceFactory := services.NewFactory(repoFactory, mailer)
authService := serviceFactory.NewAuthService()

// Repository factory in database/repositories/factory.go
repoFactory := repositories.NewFactory(db)
userRepo := repoFactory.NewUserRepository()
```

#### Important BUN ORM Pattern for Schema-Qualified Tables
When working with PostgreSQL schemas, BUN requires explicit table expressions in repository methods:

```go
// In repository methods, always set ModelTableExpr
func (r *GroupRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*education.Group, error) {
    var groups []*education.Group
    query := r.db.NewSelect().
        Model(&groups).
        ModelTableExpr(`education.groups AS "group"`)  // Critical for schema-qualified tables!
    
    // Apply options and execute query
}
```

Models should implement BeforeAppendModel when using schemas:
```go
func (g *Group) BeforeAppendModel(query any) error {
    if q, ok := query.(*bun.SelectQuery); ok {
        q.ModelTableExpr(`education.groups AS "group"`)
    }
    // Handle other query types...
    return nil
}
```

### Migration System
- Numbered migrations in `database/migrations/`
- Dependency tracking between migrations
- Run with `go run main.go migrate`
- Reset with `go run main.go migrate reset` (WARNING: drops all data)

## SSL Security Setup

PostgreSQL uses SSL encryption for GDPR compliance:

```bash
# Generate SSL certificates (required before first run)
cd config/ssl/postgres
chmod +x create-certs.sh
./create-certs.sh
cd ../../..

# Check certificate expiration periodically
./config/ssl/postgres/check-cert-expiration.sh
```

**Connection String SSL Modes:**
- Development: `sslmode=require` (basic encryption)
- Deployment: Configure based on your security requirements

**SSL Configuration:**
- Minimum TLS 1.2 with strong ciphers
- Certificate files in `config/ssl/postgres/certs/` (git-ignored)
- Server enforces SSL via `pg_hba.conf`

## Common Issues and Solutions

### Backend Issues
- **Database Connection**: Check `DB_DSN` in dev.env and ensure PostgreSQL is running
- **SSL Certificate Issues**: Run `config/ssl/postgres/create-certs.sh` to generate certificates
- **SSL Verification Issues**: Ensure certificate paths are correct and certificates are valid
- **JWT Errors**: Verify `AUTH_JWT_SECRET` is set and consistent
- **CORS Issues**: Ensure `ENABLE_CORS=true` for local development
- **SQL Debugging**: Set `DB_DEBUG=true` to see queries
- **Schema-qualified tables**: Always use `ModelTableExpr` in repository methods

### Frontend Issues
- **API Connection**: Verify `NEXT_PUBLIC_API_URL` points to backend
- **Auth Issues**: Check `NEXTAUTH_SECRET` and session configuration
- **Type Errors**: Run `npm run typecheck` to identify issues
- **Suspense Errors**: Components using `useSearchParams()` need Suspense boundaries

### Docker Issues
- **Database Not Ready**: Wait for health check or increase start_period
- **Permission Errors**: Check volume permissions and user context
- **Port Conflicts**: Ensure ports 3000, 8080, 5432 are available
- **Code Changes Not Reflected**: Restart containers to pick up changes

## RFID Integration

The system integrates with RFID readers for student tracking:
- Devices authenticate via API endpoints
- Student check-in/check-out tracked in `active_visits` table
- Room occupancy calculated from active sessions
- See `backend/docs/rfid-integration-guide.md` for device setup
- Example flows in `backend/docs/rfid-examples.md`

## Testing Strategy

### Backend Testing
```go
// Example test structure
func TestUserLogin(t *testing.T) {
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

### Frontend Testing
- Component testing with React Testing Library
- API client testing with MSW (Mock Service Worker)
- Type safety with TypeScript strict mode
- Linting with ESLint (0 warnings policy)

## Performance Considerations

- Database queries use Bun ORM with eager loading
- API responses are paginated by default (50 items)
- Frontend uses React 19 Suspense for loading states
- JWT tokens cached to reduce auth overhead
- RFID check-ins optimized for bulk operations

## Common Linting Issues (Backend)

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

## Critical Backend Patterns

### Loading Nested Relationships

When loading nested relationships (e.g., Teacher → Staff → Person), BUN ORM requires explicit column mapping:

```go
// CORRECT - Use explicit JOINs with column aliasing
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

### Schema-Qualified Table Expressions

CRITICAL: Always use quotes around table aliases in PostgreSQL schema-qualified queries:

```go
// CORRECT - Quotes around alias
ModelTableExpr(`users.teachers AS "teacher"`)

// WRONG - Will cause "TeacherResult does not have column 'id'" errors
ModelTableExpr(`users.teachers AS teacher`)
```

## Critical Frontend Patterns

### Next.js 15+ Route Handlers

Route handlers must properly type the context parameter for Next.js 15 compatibility:

```typescript
// CORRECT - Properly typed params
export const GET = createGetHandler(async (request, token, params) => {
    // params: Promise<Record<string, string | string[] | undefined>>
    const response = await apiGet(`/api/resources`, token);
    return response.data;
});
```

### Type Mapping Between Backend and Frontend

Always use helper functions to transform data types:

```typescript
// In lib/{domain}-helpers.ts
export function mapGroupResponse(data: BackendGroup): Group {
    return {
        id: data.id.toString(),  // Backend uses int64, frontend uses string
        name: data.name,
        room_id: data.room_id?.toString() || '',
        // Handle nested objects carefully
        representative: data.representative 
            ? mapTeacherResponse(data.representative) 
            : undefined
    };
}
```

### Suspense Boundaries for useSearchParams

Components using `useSearchParams()` must be wrapped in Suspense:

```typescript
// pages using searchParams
export default function Page() {
    return (
        <Suspense fallback={<Loading />}>
            <PageContent />
        </Suspense>
    );
}
```

## Development Workflow

### Backend Development Flow
1. Define models in `models/{domain}/`
2. Create repository interface in model file
3. Implement repository in `database/repositories/{domain}/`
4. Create service interface in `services/{domain}/interface.go`
5. Implement service business logic
6. Create API handlers in `api/{domain}/`
7. Write tests for repository and service layers
8. Run linter: `golangci-lint run --timeout 10m`

### Frontend Development Flow
1. Define TypeScript interfaces in `lib/{domain}-helpers.ts`
2. Create API client in `lib/{domain}-api.ts`
3. Implement service layer in `lib/{domain}-service.ts`
4. Create UI components in `components/{domain}/`
5. Build pages in `app/{domain}/`
6. Always run `npm run check` before committing

## Domain-Specific Details

### Active Sessions (Real-time tracking)
- Groups can have active sessions with room assignments
- Visit tracking for students entering/leaving rooms
- Supervisor assignments for active groups
- Combined groups can contain multiple regular groups

### Education Domain
- Groups have teachers and representatives
- Teachers are linked through `education.group_teacher` join table
- Groups can be assigned to rooms
- Substitution system for temporary staff changes

### User Management
- Person → Staff → Teacher hierarchy
- Students linked to guardians through join tables
- RFID cards associated with persons
- Privacy consent tracking for students

## Database Migration Pattern

Migrations follow a dependency system:

```go
// In database/migrations/{number}_{name}.go
var Dependencies = []string{
    "001000001_auth_accounts",  // Must run before this migration
}

var Rollback = `DROP TABLE IF EXISTS table_name CASCADE;`
```

## API Error Response Pattern

All API errors should follow this structure:

```go
type ErrorResponse struct {
    Status  string `json:"status"`
    Message string `json:"message"`
    Code    string `json:"code,omitempty"`
}
```

## Session Management

Backend sessions use JWT with separate access and refresh tokens:
- Access tokens: 15 minutes
- Refresh tokens: 1 hour
- Tokens stored in HTTP-only cookies
- Frontend uses NextAuth to manage session state

## Deployment

For deployment instructions, please refer to the deployment documentation specific to your infrastructure. The project supports Docker-based deployments with:
- PostgreSQL with SSL encryption
- Health checks and restart policies
- Resource limits and persistent volumes

## Privacy & GDPR Implementation

### Core Privacy Principles

1. **Data Access Restrictions**:
   - Teachers/Staff can only see full data for students in their assigned groups
   - Other staff see only student names and responsible person
   - Admin accounts should not be used for day-to-day operations
   - Admin access is reserved for GDPR compliance tasks (exports, deletions)

2. **Privacy Consent System**:
   - Database model supports versioned privacy policies
   - Consent expiration and renewal tracking
   - Frontend UI for consent management is **planned** (not yet implemented)

3. **Data Retention Policy** (Planned):
   - Attendance/location data: Maximum 30 days
   - If no consent given: Delete same day
   - Implementation of automated cleanup is **planned**

4. **Audit Logging** (Planned):
   - Comprehensive logging of who accesses student data
   - Required for GDPR Article 30 compliance
   - To be implemented in future release

5. **Right to Erasure**:
   - Hard delete all student data
   - Cascade deletion through database constraints
   - Students removed from groups/activities with history deleted

6. **Data Portability**:
   - Export functionality in long-term backlog
   - Not currently prioritized

### Privacy-Critical Code Locations

- **Privacy Consent Model**: `backend/models/users/privacy_consent.go`
- **Student Data Access**: `backend/services/usercontext/usercontext_service.go`
- **Permission System**: `backend/auth/authorize/`
- **Security Headers**: `backend/middleware/security_headers.go`
- **SSL Configuration**: `config/ssl/postgres/`

## IMPORTANT: Pull Request Guidelines

**DEFAULT PR TARGET BRANCH: development**

NEVER create pull requests to the `main` branch unless EXPLICITLY instructed to do so. All pull requests should target the `development` branch by default. Only create PRs to `main` when the user specifically says "create a PR to main" or similar explicit instruction.