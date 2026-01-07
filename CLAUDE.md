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

## Quick Navigation

**For quick tasks:**
- 🚀 Getting started? → [Quick Setup](#quick-setup)
- 🧪 Running tests? → [Testing Strategy](#testing-strategy)
- 🐛 Fixing bugs? → [Common Issues](#common-issues-and-solutions)
- 📚 Building features? → [Development Workflow](#development-workflow)
- ⚠️ Before committing? → [Critical Patterns & Gotchas](#critical-patterns--gotchas-)

**Full Table of Contents:**
1. [Project Context](#project-context)
2. [Quick Setup](#quick-setup) (New!)
3. [Development Commands](#development-commands)
4. [Critical Patterns & Gotchas](#critical-patterns--gotchas-)
5. [Environment Configuration](#environment-configuration)
6. [Architecture Overview](#architecture-overview)
7. [Testing Strategy](#testing-strategy) (Expanded!)
8. [Database & Migrations](#database-migration-pattern)
9. [Common Issues & Solutions](#common-issues-and-solutions)
10. [API Documentation](#api-architecture-and-documentation-patterns)
11. [Development Workflow](#development-workflow)
12. [Domain-Specific Details](#domain-specific-details)

---

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

## Quick Setup

**Get running in 5 minutes** (after first clone):

```bash
# Automated setup (recommended)
./scripts/setup-dev.sh          # Creates configs + SSL certs automatically
docker compose up -d            # Start all services (backend, frontend, postgres)

# Or manual setup if preferred
cd config/ssl/postgres && ./create-certs.sh && cd ../../..
cp backend/dev.env.example backend/dev.env
cp frontend/.env.local.example frontend/.env.local
docker compose up -d postgres   # Start database first
cd backend && go run main.go migrate  # Run migrations
cd ../frontend && npm install   # Install dependencies
```

**Verify everything works:**

```bash
# Backend health check
curl http://localhost:8080/health

# Frontend running
open http://localhost:3000

# Run tests
cd backend && go test ./...     # Should pass (test DB auto-detected)
cd ../bruno && bru run --env Local 01-smoke.bru  # API tests
```

**Common first steps:**

| Goal | Command |
|------|---------|
| Start developing backend | `cd backend && go run main.go serve` |
| Start developing frontend | `cd frontend && npm run dev` |
| See all API endpoints | `cd backend && go run main.go gendoc --routes` |
| Run quality checks | `cd frontend && npm run check` |
| Run backend tests | `go test ./...` (requires test DB running) |
| Reset database & start fresh | `go run main.go migrate reset && go run main.go seed` |

---

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

## Email & Auth Workflows

- **SMTP Configuration**: All environments load email settings from `EMAIL_SMTP_HOST`, `EMAIL_SMTP_PORT`, `EMAIL_SMTP_USER`, `EMAIL_SMTP_PASSWORD`, `EMAIL_FROM_NAME`, `EMAIL_FROM_ADDRESS`, `FRONTEND_URL`, `INVITATION_TOKEN_EXPIRY_HOURS`, and `PASSWORD_RESET_TOKEN_EXPIRY_MINUTES`. Production builds require `FRONTEND_URL` to be HTTPS; development falls back to the mock mailer.
- **Password Reset**: Reset tokens now expire after 30 minutes. The backend emits a `Retry-After` header when per-email rate limiting (3 requests/hour) is triggered, and the frontend surfaces the countdown in the modal.
- **Invitation Workflow**: Administrators can invite new users via the `/invitations` admin route. Invite acceptance runs at `/invite?token=…` where teachers set their password, satisfying the same strength policy as password reset.
- **Cleanup Operations**: Nightly scheduler runs invitation and rate-limit cleanup alongside existing token jobs. Manual CLI commands (`go run main.go cleanup invitations` and `go run main.go cleanup rate-limits`) are available for operations teams.

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
# Note: DB_DSN now auto-configured based on APP_ENV (see backend/database/database_config.go)

# Development Database Operations (localhost:5432)
go run main.go serve            # Start server (port 8080)
go run main.go migrate          # Run database migrations
go run main.go migrate status   # Show migration status
go run main.go migrate validate # Validate migration dependencies
go run main.go migrate reset    # WARNING: Reset database and run all migrations
go run main.go seed             # Populate database with test data
go run main.go seed --reset     # Clear ALL test data and repopulate

# Test Database Operations (localhost:5433)
# Start test DB: docker compose --profile test up -d postgres-test
APP_ENV=test go run main.go migrate reset  # Reset test database
APP_ENV=test go run main.go seed           # Seed test database
APP_ENV=test go test ./...                 # Run integration tests

# Testing
go test ./...                   # Run all tests (uses TEST_DB_DSN if set, else dev DB)
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
goimports -w .  # Organize imports
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
# Environment Selection
APP_ENV=development             # Options: development, test, production
# Smart DB defaults based on APP_ENV:
#   - development (default): localhost:5432/phoenix (sslmode=require)
#   - test: localhost:5433/phoenix_test (sslmode=disable)
#   - production: Requires explicit DB_DSN

# Database (Optional - only set for production or non-standard configs)
# If not set, uses APP_ENV-based smart defaults (see database/database_config.go)
# DB_DSN=postgres://username:password@localhost:5432/database?sslmode=require
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
- See Bruno API tests in `bruno/` for RFID workflow examples

### PIN Architecture (Simplified)
The system uses a simplified PIN architecture for RFID device authentication:
- **PIN Storage**: All PINs stored in `auth.accounts` table (not in `users.staff`)
- **Authentication Flow**: Device API key + staff PIN → validates against account PIN
- **Management**: Staff can set/update PINs via `/api/staff/pin` endpoints
- **Security**: Uses Argon2id hashing with attempt limiting and account lockout

## Testing Strategy

Project Phoenix uses a **three-layer testing approach**:
1. **Unit Tests** (Models + Permission Logic) - Fast, no database
2. **Integration Tests** (Repositories + Services) - Real test database
3. **API Tests** (Full End-to-End) - Bruno API testing

### Test Database Setup

All backend tests use a real PostgreSQL database (not mocks). The test database runs on **port 5433** (dev DB on 5432).

#### Option 1: Automatic (Recommended for Local Development)

```bash
# Start test database container with Docker
docker compose --profile test up -d postgres-test

# Run migrations on test DB (one-time setup)
APP_ENV=test go run main.go migrate reset

# (Optional) Seed test data
APP_ENV=test go run main.go seed

# Now tests just work - no prefix needed!
go test ./...
```

**How it works:**
- Tests using `setupTestDB()` auto-load `.env` via `godotenv.Load()`
- `TEST_DB_DSN` is extracted from `.env` automatically
- No `APP_ENV=test` prefix needed for running tests
- IDE-friendly: Click "Run Test" in GoLand/VSCode works instantly

#### Option 2: Manual Configuration (For CI/Docker)

```bash
# Start container manually
docker compose --profile test up -d postgres-test

# Set TEST_DB_DSN environment variable
export TEST_DB_DSN="postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable"

# Or in one command
TEST_DB_DSN="postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable" \
  go test ./...

# For migrations in CI
docker compose run --rm \
  -e DB_DSN="postgres://postgres:postgres@postgres-test:5432/phoenix_test?sslmode=disable" \
  server ./main migrate
```

**Environment Variable Precedence** (from `backend/database/database_config.go`):
1. Explicit `DB_DSN` (highest priority, for production/Docker overrides)
2. `APP_ENV`-based smart defaults:
   - `APP_ENV=test` → `localhost:5433/phoenix_test?sslmode=disable`
   - `APP_ENV=development` → `localhost:5432/phoenix?sslmode=require`
   - `APP_ENV=production` → Requires explicit `DB_DSN` (fails fast)
3. Legacy `TEST_DB_DSN` (backwards compatibility)
4. Development default (localhost:5432/phoenix)

### Backend Testing Patterns

#### Test Structure (Unit + Integration)

```go
// backend/models/{domain}/{file}_test.go
package {domain}

import (
	"context"
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/assert"
)

// Unit test - validates business logic (no database needed)
func TestGroupValidation(t *testing.T) {
	group := &Group{
		Name: "Valid Group",
	}

	err := group.Validate()
	require.NoError(t, err)
}

// Integration test - real database
func TestGroupRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewGroupRepository(db)

	group := &Group{
		Name:        "Test Group",
		Description: "A test group",
	}

	err := repo.Create(context.Background(), group)
	require.NoError(t, err)
	assert.NotZero(t, group.ID)
}
```

#### Test Database Helpers

See `backend/test/helpers.go` for available helpers:

```go
import "github.com/moto-nrw/project-phoenix/test"

// Setup test database connection
db := setupTestDB(t)
defer cleanupTestDB(t, db)

// Create test data with proper relationships
testData := test.CreateTestData(t)
// Provides: AdminUser, TeacherUser, StudentUser, Group1, Group2, Visit1, Visit2, Tokens

// JWT test tokens
tokenAuth, err := test.CreateTestJWTAuth()
require.NoError(t, err)
token := testData.AdminToken  // Use pre-generated tokens
```

#### Running Backend Tests

Tests using `setupTestDB()` auto-load `.env`, so no prefix needed:

```bash
# Run all tests (hermetic tests auto-detect TEST_DB_DSN from .env)
go test ./...

# Run specific package
go test ./services/active/...

# Run specific test
go test ./services/active/... -run TestSessionConflict

# Verbose output
go test -v ./...

# With race condition detection
go test -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # View in browser
```

#### Common Test Patterns

**Hermetic Test Pattern** (self-contained, no external state):

Tests create their own fixtures, execute operations, and clean up - no dependency on seed data:

```go
import testpkg "github.com/moto-nrw/project-phoenix/test"

func TestSessionCleanup(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	// ARRANGE: Create real database fixtures (not hardcoded IDs!)
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity")
	device := testpkg.CreateTestDevice(t, db, "test-device-001")
	room := testpkg.CreateTestRoom(t, db, "Test Room")
	staff := testpkg.CreateTestStaff(t, db, "Test", "Supervisor")
	student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")

	// Cleanup fixtures after test (even if test fails)
	defer testpkg.CleanupActivityFixtures(t, db,
		activityGroup.ID, device.ID, room.ID, staff.ID, student.ID)

	// ACT: Use real IDs from fixtures
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err)

	// ASSERT: Verify results
	assert.Equal(t, activityGroup.ID, session.GroupID)
}
```

**Available Test Fixtures** (in `backend/test/fixtures.go`):
| Fixture | Function | Creates |
|---------|----------|---------|
| Activity Group | `CreateTestActivityGroup(t, db, "name")` | Category + Activity |
| Device | `CreateTestDevice(t, db, "device-id")` | IoT Device |
| Room | `CreateTestRoom(t, db, "name")` | Facilities Room |
| Staff | `CreateTestStaff(t, db, "first", "last")` | Person + Staff |
| Student | `CreateTestStudent(t, db, "first", "last", "class")` | Person + Student |
| Account | `CreateTestAccount(t, db, "email")` | Auth Account |
| Person + Account | `CreateTestPersonWithAccount(t, db, "first", "last")` | Person linked to Account |
| Student + Account | `CreateTestStudentWithAccount(t, db, "first", "last", "class")` | Student with Account (for auth) |
| Teacher + Account | `CreateTestTeacherWithAccount(t, db, "first", "last")` | Full Teacher chain with Account |
| Group Supervisor | `CreateTestGroupSupervisor(t, db, staffID, groupID, "role")` | Active supervision assignment |

**⚠️ Never use hardcoded IDs** like `int64(9001)` - they cause "sql: no rows in result set" errors.

**Policy/Authorization Tests**: All policy tests use the hermetic pattern with real database. Create real users with accounts, set up actual relationships, and test actual policy decisions. This catches real bugs that mocks would miss.

**Batch Operation Testing**: For operations like `EndDailySessions()` that affect all database records, use bounds assertions:
```go
// WRONG - Exact count fails when database has other data
assert.Equal(t, 1, result.SessionsEnded)

// CORRECT - Resilient to database state
assert.GreaterOrEqual(t, result.SessionsEnded, 1)
```

**Transaction-Based Isolation** (for complex test scenarios):

```go
func TestServiceWithMultipleOperations(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback() // Auto-cleanup even if test fails

	// All operations within transaction
	// Test isolation guaranteed - rollback cleans up
	result, err := service.DoSomething(context.Background(), tx)
	require.NoError(t, err)
}
```

### API Testing with Bruno

Bruno provides consolidated, hermetic API test suite (~270ms full run):

#### Quick Start

```bash
cd bruno

# Run all tests
bru run --env Local 0*.bru

# Run specific test file
bru run --env Local 05-sessions.bru    # Session lifecycle tests

# Run multiple files
bru run --env Local 0[1-5]-*.bru       # Files 01-05

# Interactive GUI (optional)
# Open Bruno app → File → Open Collection → Select bruno/ directory
```

#### Test Files & Coverage

```
01-smoke.bru          # Health checks, basic endpoints (5 tests)
02-auth.bru           # Login, refresh, logout (8 tests)
03-users.bru          # User CRUD operations (7 tests)
04-groups.bru         # Group management (9 tests)
05-sessions.bru       # Active session lifecycle (10 tests)
06-checkins.bru       # RFID check-in/check-out (8 tests)
07-analytics.bru      # Dashboard and metrics (6 tests)
08-activities.bru     # Activity management (4 tests)
09-rooms.bru          # Room operations (4 tests)
10-schulhof.bru       # Schulhof auto-creation (5 tests)
11-invitations.bru    # Invitation workflow (3 tests)

Total: 59 test scenarios across 11 files (~270ms execution)
```

#### Test Accounts for API Testing

**Admin Account** (full system access):
```
Email: admin@example.com
Password: Test1234%
Token: Automatically obtained via login test
```

**Staff Account** (teacher/supervisor):
```
Email: andreas.krueger@example.com
Password: Test1234%
Staff ID: 1
PIN: 1234 (for RFID device authentication)
```

#### Bruno Features

- **Hermetic Testing**: Each file self-contained, no external state
- **Auto-generated Setup**: Bruno handles token refresh between tests
- **Error Assertions**: Built-in response validation
- **Environment Variables**: `{{baseUrl}}`, `{{token}}` auto-substitution
- **Two-Layer RFID Auth**: Device API key + staff PIN testing
- **Parallel Execution**: Bruno runs independent tests concurrently

#### Troubleshooting Bruno Tests

```bash
# Tests skip or fail
# → Ensure backend is running: go run main.go serve

# "invalid token" errors
# → Token might be expired, re-run from 02-auth.bru first

# Database state pollution
# → Run a single test file to reset: bru run --env Local 01-smoke.bru

# Port conflicts
# → Verify port 8080 is available: lsof -i :8080
```

### Frontend Testing (Vitest)

Frontend testing is configured but tests are optional (not required for this phase):

#### Setup

Vitest is configured in `frontend/vitest.config.ts`:
- **Test Runner**: Vitest with happy-dom
- **Coverage**: V8 provider with LCOV reporting
- **Framework**: React Testing Library support

```bash
# Run frontend tests (if written)
cd frontend
npm run test:run               # Run once
npm run test:watch             # Watch mode
npm run test:run -- --coverage # With coverage report
```

#### When Writing Tests (Optional)

```typescript
// frontend/src/components/__tests__/my-component.test.tsx
import { render, screen } from '@testing-library/react';
import { MyComponent } from '../my-component';

describe('MyComponent', () => {
  it('renders correctly', () => {
    render(<MyComponent />);
    expect(screen.getByText('Hello')).toBeInTheDocument();
  });
});
```

**Coverage Configuration**:
- Excludes: `node_modules/`, test files, types, configs
- Reports: Terminal, JSON, HTML, LCOV (for SonarCloud)
- Threshold: No minimum required (optional to define)

### CI/CD Test Execution

GitHub Actions workflow (`.github/workflows/test.yml`):

```yaml
# Runs on: Push to main/development, Pull requests
# Services: PostgreSQL 17 for integration tests
# Coverage: Uploaded to SonarCloud

# Backend tests
- name: Run Go tests with coverage
  env:
    TEST_DB_DSN: postgres://postgres:postgres@localhost:5432/phoenix_test?sslmode=disable
  run: go test -v -coverprofile=coverage.out -covermode=atomic ./...

# Frontend tests
- name: Run frontend tests with coverage
  run: npm run test:run -- --coverage

# Both coverage files uploaded to SonarCloud for analysis
```

**Key Points**:
- Test database automatically available as PostgreSQL service
- Coverage data transformed and sent to SonarCloud
- All tests must pass before PR can merge
- Failures block merge and show in PR status

### Coverage Goals

**Coverage Goals by Layer**:
- Model validations: 80%+ (most critical)
- Repository operations: 70%+
- Service logic: 60%+
- API handlers: 40%+

Current coverage is tracked in SonarCloud and updated with each PR.

### Test Development Workflow

1. **Write failing test first** (optional TDD approach)
   ```bash
   go test ./services/auth/... -run TestNewAuthService
   ```

2. **Implement the feature**
   - Make the test pass
   - Ensure all tests still pass: `go test ./...`

3. **Run linter before committing**
   ```bash
   golangci-lint run --timeout 10m
   ```

4. **Backend**: No tests required in commits, but integration tests must pass
5. **Frontend**: Type checking required (`npm run check`), tests optional

### Common Test Issues & Solutions

| Issue | Solution |
|-------|----------|
| Tests skip without error | Verify test DB on port 5433 is running: `docker compose --profile test up -d postgres-test` |
| "connection refused" | Backend not running on port 8080: `go run main.go serve` |
| Tests timeout | Migrations not run: `APP_ENV=test go run main.go migrate reset` |
| Race condition detected | Add mutex locks around shared state |
| Flaky tests | Check for time-dependent assertions, use fixed clock in tests |
| Data isolation fails | Ensure `cleanupTestDB()` called in deferred cleanup |
| "sql: no rows in result set" | Replace hardcoded IDs (`int64(1)`) with test fixtures from `testpkg` |
| Batch test assertions fail | Use `GreaterOrEqual` for batch operations, not exact counts |

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
- never credit claude in commit messages
- never credit claude
- never credit claude code
