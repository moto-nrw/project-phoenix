<!-- OPENSPEC:START -->
# OpenSpec Instructions

These instructions are for AI assistants working in this project.

Always open `@/openspec/AGENTS.md` when the request:
- Mentions planning or proposals (words like proposal, spec, change, plan)
- Introduces new capabilities, breaking changes, architecture shifts, or big performance/security work
- Sounds ambiguous and you need the authoritative spec before coding

Use `@/openspec/AGENTS.md` to learn:
- How to create and apply change proposals
- Spec format and conventions
- Project structure and guidelines

Keep this managed block so 'openspec update' can refresh the instructions.

<!-- OPENSPEC:END -->

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Context

**Project Name:** Project-Phoenix

**Description:** A GDPR-compliant RFID-based student attendance and room management system for educational institutions. Implements strict privacy controls for student data access.

**Key Technologies:**
- Backend: Go (1.23+) with Chi router, Bun ORM for PostgreSQL
- Frontend: Next.js (v15+) with React (v19+), Tailwind CSS (v4+)
- Database: PostgreSQL (17+) with SSL encryption (GDPR compliance)
- Authentication: JWT-based auth system with role-based access control (token cleanup on login)
- RFID Integration: Custom API endpoints for device communication
- Deployment: Docker/Docker Compose

**Security Notice:**
- All sensitive configuration uses example templates (never commit real .env files)
- SSL certificates must be generated locally using setup scripts
- Real configuration files (.env, certificates) are git-ignored
- See [Security Guidelines](docs/security.md) for complete security practices

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

## Critical Patterns & Gotchas ⚠️

**Read these first to avoid common mistakes:**

1. **BUN ORM Schema-Qualified Tables** - MUST quote aliases:
   ```go
   ModelTableExpr(`education.groups AS "group"`)  // ✓ CORRECT
   ModelTableExpr(`education.groups AS group`)    // ✗ WRONG - causes "column not found"
   ```

2. **Docker Backend Rebuild** - Go code changes require rebuild:
   ```bash
   docker compose build server  # REQUIRED after Go changes
   docker compose up -d server
   ```

3. **Frontend Quality Check** - Zero warnings policy enforced:
   ```bash
   npm run check  # MUST PASS before committing
   ```

4. **Type Mapping** - Backend int64 → Frontend string:
   ```typescript
   id: data.id.toString()  // Always convert IDs
   ```

5. **Git Workflow** - PRs target `development`, NOT `main`:
   ```bash
   gh pr create --base development  # Correct
   ```

6. **Student Location** - Use `active.visits`, NOT deprecated flags:
   ```go
   // ✓ CORRECT: active.visits + active.attendance
   // ✗ WRONG: users.students (in_house, wc, school_yard) - broken!
   ```

## Development Commands

### Quick Setup (New Development Environment)
```bash
# Automated setup with SSL certificates and secure configuration
./scripts/setup-dev.sh          # Creates configs and SSL certs automatically
docker compose up -d            # Start all services

# Manual setup alternative
cd config/ssl/postgres && ./create-certs.sh && cd ../../..
cp backend/dev.env.example backend/dev.env
cp frontend/.env.local.example frontend/.env.local
# Edit environment files with your values
```

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
go run main.go seed             # Populate database with test data
go run main.go seed --reset     # Clear ALL test data and repopulate

# Testing
go test ./...                   # Run all tests
go test -v ./api/auth           # Run specific package with verbose output
go test -race ./...             # Run tests with race condition detection
go test ./api/auth -run TestLogin  # Run specific test

# Documentation and API Discovery
go run main.go gendoc           # Generate both routes.md and OpenAPI spec
go run main.go gendoc --routes  # Generate only routes documentation
go run main.go gendoc --openapi # Generate only OpenAPI specification

# Advanced API Documentation System
# The gendoc command provides powerful API extraction and documentation capabilities:

# 1. Route Discovery and Analysis
go run main.go gendoc --routes  # Creates routes.md with complete API surface mapping
# - Shows all available endpoints with their HTTP methods
# - Displays middleware chains and permission requirements
# - Reveals handler function mappings for each route
# - Useful for understanding the complete API architecture

# 2. OpenAPI Specification Generation  
go run main.go gendoc --openapi # Creates docs/openapi.yaml for external tools
# - Generates OpenAPI 3.0.3 specification from live router
# - Includes authentication schemes (Bearer JWT + API keys)
# - Extracts path parameters automatically from route patterns
# - Can be used with Swagger UI, Postman, or other API tools

# 3. API Testing Integration
# Generated documentation integrates with Bruno API testing:
cd bruno && bru run --env Local 0*.bru  # Test all endpoints (~270ms)

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

# Code Quality (REQUIRED before committing - zero warnings policy!)
npm run check                   # Run both lint and typecheck (MUST PASS)
npm run lint                    # ESLint check (max-warnings=0)
npm run lint:fix                # Auto-fix linting issues
npm run typecheck               # TypeScript type checking

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

# CRITICAL: Backend code changes require rebuild
docker compose build server     # MUST run after any Go code changes
docker compose up -d server     # Restart with new build

# Database Operations
docker compose run server ./main migrate  # Run migrations
docker compose run server ./main seed     # Populate with test data
docker compose logs postgres             # Check database logs

# Frontend Operations
docker compose run frontend npm run lint # Run lint checks in container
docker compose logs frontend            # Check frontend logs

# API Testing
cd bruno
bru run --env Local 0*.bru              # Run all tests (~270ms, 59 scenarios)
bru run --env Local 05-sessions.bru    # Run specific test file
bru run --env Local 0[1-5]-*.bru       # Run tests 01-05 only

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

# Automated Cleanup (GDPR Compliance)
CLEANUP_SCHEDULER_ENABLED=true  # Enable automatic visit data cleanup
CLEANUP_SCHEDULER_TIME=02:00    # Daily cleanup time (24-hour format)
CLEANUP_SCHEDULER_TIMEOUT_MINUTES=30  # Maximum cleanup duration
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

## API Architecture and Documentation Patterns

### Chi Router Organization
The project uses Chi router with domain-based route mounting for clear API organization:

```go
// In api/base.go - Domain-based route mounting
r.Route("/api", func(r chi.Router) {
    r.Mount("/auth", a.Auth.Router())           // Authentication endpoints
    r.Mount("/active", a.Active.Router())       // Real-time session management
    r.Mount("/groups", a.Groups.Router())       // Educational group management
    r.Mount("/users", a.Users.Router())         // User management
    r.Mount("/iot", a.IoT.Router())            // RFID device integration
    // ... other domain routers
})
```

### Route Documentation Generation
The `gendoc` command uses Chi's router introspection to extract API documentation:

```bash
# Generate complete route documentation showing:
go run main.go gendoc --routes
# - All HTTP endpoints with methods (GET, POST, PUT, DELETE)
# - Middleware chains per route (auth, permissions, rate limiting)
# - Handler function mappings
# - Path parameter extraction from patterns like /users/{id}

# Example generated route structure:
# /api/active/analytics/dashboard
#   - _GET_
#     - [RequiresPermission(GroupsRead)]
#     - [getDashboardAnalytics]
```

### OpenAPI Specification Features
The auto-generated OpenAPI spec provides machine-readable API documentation:

```yaml
# Generated in docs/openapi.yaml
paths:
  /api/active/analytics/dashboard:
    get:
      summary: "GET /api/active/analytics/dashboard"
      tags: ["Active"]
      security:
        - bearerAuth: []
      parameters: []
      responses:
        200:
          description: "Successful operation"
```

**Key Features:**
- **Automatic Path Parameter Detection**: Extracts `{id}` patterns from routes
- **Security Scheme Integration**: Includes JWT Bearer and API Key authentication
- **Tag Generation**: Organizes endpoints by domain (Active, Users, Groups, etc.)
- **Response Schema**: Basic HTTP status code responses

### API Development Workflow

**1. API Discovery and Understanding:**
```bash
# Start with route generation to understand API surface
go run main.go gendoc --routes
# Review routes.md to see:
# - Available endpoints and their purposes
# - Permission requirements per endpoint
# - Handler function names for code navigation
```

**2. Testing API Endpoints:**
```bash
# Use Bruno API tests with generated documentation
cd bruno
bru run --env Local 0*.bru              # Run all tests (~270ms)
bru run --env Local 05-sessions.bru    # Test session lifecycle
bru run --env Local 06-checkins.bru    # Test check-in/out flows
bru run --env Local 10-schulhof.bru    # Test Schulhof auto-create
```

**3. Schema Enhancement:**
The base OpenAPI generation can be enhanced by:
- Adding response schema definitions in `cmd/gendoc.go`
- Implementing request/response models in API handlers
- Using Go struct tags for automatic schema generation

**4. Permission-Based Route Analysis:**
```bash
# Routes.md shows permission requirements like:
# [RequiresPermission(permissions.GroupsRead)]
# Use this to understand:
# - Which endpoints require authentication
# - What permission levels are needed
# - How to test with appropriate user roles
```

### Integration with Development Tools

**Bruno API Testing Integration:**
- Generated routes map directly to Bruno test collections
- Authentication examples use tokens compatible with gendoc endpoints
- Test timing (~252ms for full suite) enables rapid development feedback

**External Tool Integration:**
```bash
# Use generated OpenAPI spec with external tools:
# - Import docs/openapi.yaml into Postman
# - Serve with Swagger UI for interactive documentation
# - Generate client SDKs using OpenAPI generators
```

### Understanding API Endpoints Through gendoc

**Key API Endpoints Discovered:**
```bash
# Dashboard and Analytics (Real-time data)
/api/active/analytics/dashboard       # GET - Main dashboard metrics
/api/active/analytics/counts          # GET - Basic counts
/api/active/analytics/room/{roomId}/utilization  # GET - Room utilization
/api/active/analytics/student/{studentId}/attendance  # GET - Student attendance

# Session Management
/api/active/sessions                  # GET, POST - Active session management
/api/active/visits                    # GET, POST - Student visit tracking
/api/active/combined                  # POST - Combined group operations

# Authentication and Authorization
/auth/login                          # POST - JWT token generation
/auth/refresh                        # POST - Token refresh
/auth/logout                         # POST - Session termination

# RFID Device Integration
/iot/devices/{deviceId}/ping         # POST - Device health check
/iot/checkin                         # POST - Student check-in via RFID
/iot/checkout                        # POST - Student check-out via RFID
```

**API Response Pattern Analysis:**
```json
// Standard API response structure (from gendoc analysis)
{
  "status": "success|error",
  "data": { /* actual response data */ },
  "message": "Human-readable message"
}

// Dashboard analytics example:
{
  "status": "success",
  "data": {
    "students_present": 3,
    "students_in_rooms": 0,
    "capacity_utilization": 0.0,
    "active_activities": 0,
    // ... other metrics
  },
  "message": "Dashboard analytics retrieved successfully"
}
```

**Authentication Pattern:**
```bash
# Get JWT token for API testing
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"Test1234%"}'

# Use token in subsequent requests  
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/active/analytics/dashboard
```

#### Important BUN ORM Pattern for Schema-Qualified Tables
When working with PostgreSQL schemas, BUN requires explicit table expressions in repository methods:

```go
// In repository methods, always set ModelTableExpr with quotes around alias
func (r *GroupRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*education.Group, error) {
    var groups []*education.Group
    query := r.db.NewSelect().
        Model(&groups).
        ModelTableExpr(`education.groups AS "group"`)  // Critical: quotes around alias!
    
    // Apply options and execute query
}
```

**CRITICAL**: Always include table alias with quotes to prevent SQL errors:
```go
// CORRECT - Will generate: SELECT "group".* FROM education.groups AS "group"
ModelTableExpr(`education.groups AS "group"`)

// WRONG - Will cause "missing FROM-clause entry for table" errors
ModelTableExpr(`education.groups`)
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
- **Schema-qualified tables**: Always use `ModelTableExpr` with quoted aliases in repository methods
- **"missing FROM-clause entry" errors**: Ensure table aliases are quoted in `ModelTableExpr`
- **Student location data**: Use `active.visits` and `active.attendance` tables, NOT deprecated boolean flags
- **Rebuild requirement**: Docker backend container must be rebuilt after Go code changes (`docker compose build server`)

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
- **Backend Code Changes**: MUST rebuild backend container after Go code changes (`docker compose build server`)

## RFID Integration

The system integrates with RFID readers for student tracking:
- Devices authenticate via API endpoints with two-layer auth (device API key + teacher PIN)
- Student check-in/check-out tracked in `active_visits` table
- Room occupancy calculated from active sessions
- Implementation guide: `/RFID_IMPLEMENTATION_GUIDE.md` (comprehensive workflows and API specs)
- Device setup docs: `backend/docs/rfid-integration-guide.md`
- Example flows: `backend/docs/rfid-examples.md`

### PIN Architecture (Simplified)
The system uses a simplified PIN architecture for RFID device authentication:
- **PIN Storage**: All PINs stored in `auth.accounts` table (not in `users.staff`)
- **Authentication Flow**: Device API key + staff PIN → validates against account PIN
- **Management**: Staff can set/update PINs via `/api/staff/pin` endpoints
- **Security**: Uses Argon2id hashing with attempt limiting and account lockout

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

### API Testing with Bruno
Bruno provides a consolidated, hermetic API test suite optimized for reliability:

```bash
cd bruno

# Run all tests (recommended)
bru run --env Local 0*.bru              # 59 scenarios across 11 files (~270ms)

# Run specific test files
bru run --env Local 01-smoke.bru        # Health checks
bru run --env Local 05-sessions.bru    # Session lifecycle (10 tests)
bru run --env Local 06-checkins.bru    # Check-in/out flows (8 tests)
bru run --env Local 10-schulhof.bru    # Schulhof auto-create workflow

# Run subset of tests
bru run --env Local 0[1-5]-*.bru       # Run tests 01-05 only

# Clean up before tests (if needed)
docker compose exec -T postgres psql -U postgres -d postgres \
  -c "DELETE FROM active.groups WHERE end_time IS NULL;"

# Bruno GUI (optional)
# Open Bruno app → Open Collection → Select bruno/ directory
```

**Bruno Implementation Features:**
- **Hermetic Testing**: Each file self-contained with setup and cleanup
- **Consolidated Structure**: 11 numbered test files (62 → 11 file reduction)
- **Fast Execution**: Complete test suite runs in ~270ms (59 test scenarios)
- **No External Dependencies**: Pure Bruno CLI, no shell scripts
- **RFID Testing**: Two-layer device authentication (API key + PIN)
- **Test Accounts**: admin@example.com / Test1234%, andreas.arndt@schulzentrum.de / Test1234% (Staff ID: 1, PIN: 1234)

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

5. **Import grouping** (goimports):
   ```go
   // Group imports: stdlib, external, internal
   import (
       "context"
       "fmt"
       
       "github.com/go-chi/chi/v5"
       
       "github.com/moto-nrw/project-phoenix/models"
   )
   ```

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

**BREAKING CHANGE**: Next.js 15 made `params` async - route wrappers handle this automatically:

```typescript
// Next.js 15: params are now Promise<Record<string, string | string[] | undefined>>
export const GET = createGetHandler(async (request, token, params) => {
    // Route wrapper automatically awaits params and extracts values
    // Access params directly: params.id, params.groupId, etc.
    const response = await apiGet(`/api/resources`, token);
    return response.data;
});

// If writing custom route handlers without wrappers:
export async function GET(
    request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> }
) {
    const { id } = await context.params;  // MUST await!
    // ...
}
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
7. **Generate API documentation**: `go run main.go gendoc --routes`
8. **Test API endpoints**: `cd bruno && bru run --env Local 0*.bru`
9. Write tests for repository and service layers
10. Run linter: `golangci-lint run --timeout 10m`
11. Test with seed data: `go run main.go seed`

### Frontend Development Flow
1. **Review API endpoints**: Check `routes.md` for available backend endpoints
2. Define TypeScript interfaces in `lib/{domain}-helpers.ts`
3. Create API client in `lib/{domain}-api.ts`
4. **Test API integration**: Use Bruno or curl with generated documentation
5. Implement service layer in `lib/{domain}-service.ts`
6. Create UI components in `components/{domain}/`
7. Build pages in `app/{domain}/`
8. Always run `npm run check` before committing

### Creating New Features
1. Create feature branch from `development`: `git checkout -b feature/feature-name`
2. **Analyze existing APIs**: `go run main.go gendoc --routes` to understand current endpoints
3. Implement backend first if API changes needed
4. **Update API documentation**: Re-run `gendoc` after backend changes
5. Update frontend to consume new/changed APIs
6. **Test integration**: Use Bruno tests to verify API functionality
7. Test end-to-end with both services running
8. Create PR targeting `development` branch (NEVER `main`)

## Domain-Specific Details

### Active Sessions (Real-time tracking)
- Groups can have active sessions with room assignments
- Visit tracking for students entering/leaving rooms
- **Multiple supervisor support**: Groups can have multiple supervisors assigned via `active.group_supervisors` table
- Supervisor assignments for active groups with role-based assignments (supervisor, assistant, etc.)
- Combined groups can contain multiple regular groups
- Device tracking: `device_id` is now optional in `active.groups` (for RFID integration)

**CRITICAL - Student Location Tracking System Status:**
- **Real tracking system**: `active.visits` + `active.attendance` tables (CORRECT, functional)
- **Deprecated system**: Manual boolean flags in `users.students` (`in_house`, `wc`, `school_yard`) (BROKEN, being phased out)
- **Current issue**: Frontend still displays deprecated flags instead of real tracking data
- **Bus flag meaning**: Administrative permission flag only ("Buskind"), NOT location
- **Transition needed**: Student API must use `active.visits` for current location, not deprecated flags

### Education Domain
- Groups have teachers and representatives
- Teachers are linked through `education.group_teacher` join table
- Groups can be assigned to rooms
- Substitution system for temporary staff changes
- **No backdating rule**: Substitutions must start today or in the future

### User Management
- Person → Staff → Teacher hierarchy
- Students linked to guardians through join tables
- RFID cards associated with persons
- Privacy consent tracking for students
- Staff PIN management: 4-digit PINs for device authentication (stored in `auth.accounts`)

### IoT/Device Management
- Devices authenticate with API keys (stored in `iot.devices`)
- Two-layer authentication: Device API key + Teacher PIN
- Device health monitoring via ping endpoints
- RFID tag assignments tracked per person

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

3. **Data Retention Policy** (Implemented):
   - Individual retention settings per student (1-31 days)
   - Default: 30 days if no consent specified
   - Automated cleanup via scheduler or manual CLI commands
   - Only completed visits deleted (active sessions preserved)

4. **Audit Logging** (Implemented):
   - All data deletions logged in `audit.data_deletions` table
   - Tracks who deleted data, when, and why
   - Required for GDPR Article 30 compliance

5. **Right to Erasure**:
   - Hard delete all student data
   - Cascade deletion through database constraints
   - Students removed from groups/activities with history deleted

6. **Data Portability**:
   - Export functionality in long-term backlog
   - Not currently prioritized

### Privacy-Critical Code Locations

- **Privacy Consent Model**: `backend/models/users/privacy_consent.go`
- **Cleanup Service**: `backend/services/active/cleanup_service.go`
- **Cleanup Commands**: `backend/cmd/cleanup.go`
- **Audit Logging**: `backend/models/audit/data_deletion.go`
- **Student Data Access**: `backend/services/usercontext/usercontext_service.go`
- **Permission System**: `backend/auth/authorize/`
- **Security Headers**: `backend/middleware/security_headers.go`
- **SSL Configuration**: `config/ssl/postgres/`

## IMPORTANT: Pull Request Guidelines

**DEFAULT PR TARGET BRANCH: development**

NEVER create pull requests to the `main` branch unless EXPLICITLY instructed to do so. All pull requests should target the `development` branch by default. Only create PRs to `main` when the user specifically says "create a PR to main" or similar explicit instruction.