# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Context

**Project Name:** Project-Phoenix

**Description:** A RFID-based student attendance and room management system for educational institutions. Tracks student presence, room occupancy, and provides comprehensive management tools.

**Key Technologies:**
- Backend: Go (1.21+) with Chi router, Bun ORM for PostgreSQL
- Frontend: Next.js (v15+) with React (v19+), Tailwind CSS (v4+)
- Database: PostgreSQL (17+)
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

### Quick Start
```bash
# Start database
docker compose up -d postgres

# Run migrations
docker compose run server ./main migrate

# Start all services
docker compose up

# Frontend checks
cd frontend && npm run check
```

### Backend Environment Variables (dev.env)
```bash
# Database
DB_DSN=postgres://username:password@localhost:5432/database?sslmode=disable
DB_DEBUG=true                   # Log SQL queries

# Authentication  
AUTH_JWT_SECRET=your_jwt_secret_here  # Change in production!
AUTH_JWT_EXPIRY=15m                   # Access token expiry
AUTH_JWT_REFRESH_EXPIRY=1h            # Refresh token expiry

# Admin Account (for initial setup)
ADMIN_EMAIL=kontakt@moto.nrw
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

### Migration System
- Numbered migrations in `database/migrations/`
- Dependency tracking between migrations
- Run with `go run main.go migrate`
- Reset with `go run main.go migrate reset` (WARNING: drops all data)

## Common Issues and Solutions

### Backend Issues
- **Database Connection**: Check `DB_DSN` in dev.env and ensure PostgreSQL is running
- **JWT Errors**: Verify `AUTH_JWT_SECRET` is set and consistent
- **CORS Issues**: Ensure `ENABLE_CORS=true` for local development
- **SQL Debugging**: Set `DB_DEBUG=true` to see queries

### Frontend Issues
- **API Connection**: Verify `NEXT_PUBLIC_API_URL` points to backend
- **Auth Issues**: Check `NEXTAUTH_SECRET` and session configuration
- **Type Errors**: Run `npm run typecheck` to identify issues
- **Suspense Errors**: Components using `useSearchParams()` need Suspense boundaries

### Docker Issues
- **Database Not Ready**: Wait for health check or increase start_period
- **Permission Errors**: Check volume permissions and user context
- **Port Conflicts**: Ensure ports 3000, 8080, 5432 are available

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