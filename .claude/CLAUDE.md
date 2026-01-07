# Project Phoenix - Claude Code Memory

## Critical Project Patterns (READ FIRST)

### ⚠️ BUN ORM Schema-Qualified Tables (MANDATORY)
**CRITICAL**: Always quote table aliases in BUN ORM queries to prevent runtime errors.

```go
// CORRECT - Quotes around alias prevent "column not found" errors
query := r.db.NewSelect().
    Model(&groups).
    ModelTableExpr(`education.groups AS "group"`)

// WRONG - Will fail with "column not found"
ModelTableExpr(`education.groups AS group`)  // NO QUOTES = ERROR
```

**For nested relationships**, use explicit column mapping:
```go
type teacherResult struct {
    Teacher *users.Teacher `bun:"teacher"`
    Staff   *users.Staff   `bun:"staff"`
    Person  *users.Person  `bun:"person"`
}

err := r.db.NewSelect().
    Model(&result).
    ModelTableExpr(`users.teachers AS "teacher"`).
    ColumnExpr(`"teacher".id AS "teacher__id"`).
    ColumnExpr(`"staff".* AS "staff__*"`).
    Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
    Scan(ctx)
```

### ⚠️ Docker Backend Rebuild (CRITICAL)
**Go code changes require container rebuild** - hot reload not configured:
```bash
docker compose build server  # MUST run after Go changes
docker compose up -d server
```

### ⚠️ Student Location Tracking (DATA INTEGRITY)
**Two systems exist - only ONE is correct:**
- ✅ **CORRECT**: `active.visits` + `active.attendance` tables (real-time tracking)
- ❌ **DEPRECATED**: `users.students` boolean flags (`in_house`, `wc`, `school_yard`) - BROKEN, being phased out

**Always use `active.visits` for current location queries.**

### ⚠️ Type Mapping Frontend ↔ Backend
Backend uses `int64`, frontend uses `string` for IDs:
```typescript
// Backend response
interface BackendGroup {
  id: number;              // int64 in Go
  room_id: number | null;
}

// Frontend type
interface Group {
  id: string;              // Convert to string
  roomId: string | null;   // camelCase + string
}

// Mapping helper
export function mapGroupResponse(data: BackendGroup): Group {
  return {
    id: data.id.toString(),
    roomId: data.room_id?.toString() ?? null,
  };
}
```

## Architecture Quick Reference

### Backend (Go + BUN ORM + Chi)
```
HTTP Request → Chi Router → Middleware (JWT, Permissions) →
Handler → Service (business logic) → Repository → BUN ORM → PostgreSQL
```

**Key Patterns**:
- Factory pattern for dependency injection (no DI framework)
- Multi-schema PostgreSQL (11 schemas: auth, users, education, schedule, etc.)
- Repository interfaces in `models/`, implementations in `database/repositories/`
- Services orchestrate business logic, repositories handle data access

### Frontend (Next.js 15 + React 19)
```
Component → Service Layer → API Client → Next.js API Route →
Route Wrapper (JWT extraction) → Backend API → Response Mapping
```

**Key Patterns**:
- All backend calls proxied through Next.js API routes (JWT never exposed to client)
- Type helpers transform backend responses (snake_case → camelCase, int64 → string)
- Suspense boundaries required for `useSearchParams()` components
- Next.js 15: params are `Promise<Record<string, string>>` (must await)

## Development Commands

### Backend (Go)
```bash
cd backend

# Development
go run main.go serve            # Start server (port 8080)
go run main.go migrate          # Run migrations
go run main.go seed             # Populate test data
go run main.go seed --reset     # Clear and re-seed
go run main.go gendoc           # Generate routes.md + OpenAPI

# Quality
golangci-lint run --timeout 10m # Lint
golangci-lint run --fix         # Auto-fix
goimports -w .                  # Organize imports (install: go install golang.org/x/tools/cmd/goimports@latest)
go fmt ./...                    # Format

# Testing
go test ./...                   # All tests
go test -race ./...             # Race detection
```

### Frontend (Next.js)
```bash
cd frontend

# Development
npm run dev                     # Dev server with Turbo

# Quality (ALWAYS run before committing!)
npm run check                   # Lint + typecheck (0 warnings policy)
npm run lint:fix                # Auto-fix lint issues
npm run typecheck               # TypeScript only

# Formatting
npm run format:write            # Auto-format with Prettier

# Build
npm run build                   # Production build
npm run preview                 # Build + start production
```

### API Testing (Bruno)
```bash
cd bruno

# Quick domain tests (recommended)
./dev-test.sh groups            # ~44ms - 25 groups
./dev-test.sh students          # ~50ms - 50 students
./dev-test.sh rooms             # ~19ms - 24 rooms
./dev-test.sh all               # ~252ms - Full suite
```

### Docker Operations
```bash
# SSL Setup (required first time)
cd config/ssl/postgres && ./create-certs.sh && cd ../../..

# Services
docker compose up -d            # Start all services
docker compose build server     # Rebuild backend (after Go changes!)
docker compose logs -f server   # Check backend logs
docker compose exec server ./main migrate  # Run migrations
```

### Test Database (Integration Tests)
```bash
# Start test DB (port 5433)
docker compose --profile test up -d postgres-test

# Run migrations on test DB
docker compose run --rm \
  -e DB_DSN="postgres://postgres:postgres@postgres-test:5432/phoenix_test?sslmode=disable" \
  server ./main migrate

# Run tests
TEST_DB_DSN="postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable" \
  go test ./services/active/... -v
```

## Critical GDPR/Privacy Patterns

### Data Retention
```go
// Student-specific retention (1-31 days, default 30)
type Student struct {
    DataRetentionDays *int `bun:"data_retention_days"`  // NULL = 30 days
}

// Automated cleanup runs daily at 2:00 AM
// Manual: go run main.go cleanup --dry-run
```

### Access Control Policies
- Teachers see FULL data for students in their assigned groups
- Other staff see ONLY names + responsible person (no birthdays, addresses)
- Admin accounts for GDPR tasks only (not day-to-day ops)

### Audit Logging
All data deletions logged in `audit.data_deletions` table:
```go
type DataDeletion struct {
    ID          int64     `bun:"id,pk,autoincrement"`
    EntityType  string    `bun:"entity_type,notnull"`
    EntityID    int64     `bun:"entity_id,notnull"`
    DeletedBy   int64     `bun:"deleted_by,notnull"`
    DeletedAt   time.Time `bun:"deleted_at,default:now()"`
    Reason      string    `bun:"reason"`
}
```

## Common Workflows

### Adding New Backend Feature
1. Define model in `models/{domain}/`
2. Create migration in `database/migrations/`
3. Implement repository in `database/repositories/{domain}/`
4. Add factory method to `repositories/factory.go`
5. Create service in `services/{domain}/`
6. Add factory method to `services/factory.go`
7. Create API handlers in `api/{domain}/`
8. Add routes to `api/{domain}/api.go` with permission middleware
9. Run migration: `go run main.go migrate`
10. Test with Bruno: `cd bruno && ./dev-test.sh {domain}`
11. Generate docs: `go run main.go gendoc`

### Adding New Frontend Feature
1. Check backend API in `docs/routes.md`
2. Define types in `lib/{domain}-helpers.ts`
3. Create API client in `lib/{domain}-api.ts`
4. Create Next.js route in `app/api/{domain}/route.ts`
5. Create UI component in `components/{domain}/`
6. Add to page in `app/(auth)/{page}/`
7. Run quality check: `npm run check`

### Database Migration
```go
// database/migrations/{version}_{name}.go
const (
    Version     = "4.5.1"
    Description = "Create class_schedules table"
)

var Dependencies = []string{
    "3.0.1",  // Must depend on existing tables
}

func init() {
    Migrations.MustRegister(
        func(ctx context.Context, db *bun.DB) error {
            // Up migration
            _, err := db.ExecContext(ctx, `CREATE TABLE ...`)
            return err
        },
        func(ctx context.Context, db *bun.DB) error {
            // Down migration (test this!)
            _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS ... CASCADE`)
            return err
        },
    )
}
```

**Migration Checklist**:
- [ ] Never edit existing migrations
- [ ] Always add new migration file
- [ ] Specify dependencies correctly
- [ ] Test rollback function
- [ ] Use CASCADE on DROP TABLE
- [ ] Run `go run main.go migrate validate`

## File Locations Quick Reference

### Backend
- **Models**: `models/{domain}/` - Domain entities + validation
- **Repos**: `database/repositories/{domain}/` - Data access
- **Services**: `services/{domain}/` - Business logic
- **API**: `api/{domain}/` - HTTP handlers
- **Migrations**: `database/migrations/` - Database schema
- **Auth**: `auth/authorize/` - Permissions + policies

### Frontend
- **API Clients**: `lib/{domain}-api.ts` - Backend calls
- **Type Helpers**: `lib/{domain}-helpers.ts` - Type mapping
- **Components**: `components/{domain}/` - UI components
- **Pages**: `app/(auth)/{page}/` - Next.js routes
- **API Routes**: `app/api/{domain}/route.ts` - Proxy handlers

### Config
- **SSL Certs**: `config/ssl/postgres/certs/` (git-ignored)
- **Backend Env**: `backend/dev.env` (git-ignored)
- **Frontend Env**: `frontend/.env.local` (git-ignored)
- **Docker Env**: `.env` (git-ignored)

## Known Issues & Workarounds

### 1. BUN ORM Column Mapping Errors
**Issue**: "Column not found" with nested relations
**Fix**: Use explicit `ColumnExpr` with table prefixes
```go
ColumnExpr(`"teacher".id AS "teacher__id"`)
```

### 2. Next.js 15 Async Params
**Issue**: `params` is now `Promise<Record<string, string>>`
**Fix**: Always await params in route handlers
```typescript
const { id } = await params;
```

### 3. Bruno Token Expiry
**Issue**: Tokens expire during test execution
**Fix**: Use `dev-test.sh` which gets fresh tokens automatically

### 4. SSL Certificate Expiration
**Issue**: Self-signed certs expire after 1 year
**Fix**: Check expiry and regenerate:
```bash
./config/ssl/postgres/check-cert-expiration.sh
./config/ssl/postgres/create-certs.sh  # If expired
```

### 5. Policy/Authorization Tests
**Pattern**: All policy tests use hermetic pattern with real database
**Fixtures**: Use `CreateTestStudentWithAccount`, `CreateTestTeacherWithAccount` to create users with auth context, then test actual policy decisions against real relationships.

## Security Checklist

- [ ] Never commit `.env*` files (except `*.example`)
- [ ] Use `git add <file>` (never `git add .`)
- [ ] Run `git diff --cached` before committing
- [ ] SSL enabled for database (`sslmode=require`)
- [ ] Passwords hashed with Argon2id
- [ ] JWT tokens expire (15min access, 1hr refresh)
- [ ] Permission checks on all API routes
- [ ] Input validation with ozzo-validation
- [ ] GDPR audit logging for deletions

## Performance Patterns

- Paginate all list endpoints (default 50 items)
- Use `Relation()` for eager loading (avoid N+1 queries)
- Index foreign keys and frequently queried fields
- Gzip responses (not currently enabled - future)
- Cache static assets (Next.js automatic)

## Git Commit Conventions

```
<type>: <subject>

<body>

<footer>
```

**Types**: feat, fix, refactor, chore, docs, test, style

**CRITICAL**: Never include "Co-Authored-By: Claude" in commits!

## Import This Memory

@CLAUDE.local.md  # User-specific preferences
@README.md  # Project overview
- always use qdrant to retrieve information and to save information. use qdrant mcp!