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

## Project Overview

**Project Phoenix** - A GDPR-compliant RFID-based student attendance and room management system for educational institutions.

| Component | Technology |
|-----------|------------|
| Backend | Go 1.23+, Chi router, BUN ORM |
| Frontend | Next.js 15+, React 19+, Tailwind 4+ |
| Database | PostgreSQL 17+ (multi-schema, SSL) |
| Auth | JWT (15min access, 1hr refresh) |
| Testing | Go tests + Bruno API tests |
| License | Source-Available (see [LICENSE](LICENSE)) |

---

## 🏛️ Core Architectural Principle

**The flow MUST always be: Handler → Service → Repository → Database**

This is the foundational pattern that stays consistent regardless of future refactoring.

```
┌─────────────────────────────────────────────────────────────────────────┐
│  HTTP Request                                                            │
│       ↓                                                                  │
│  Chi Router → Middleware (JWT + Permissions)                            │
│       ↓                                                                  │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  HANDLER (api/{domain}/)                                         │    │
│  │  - Parse request, validate input                                 │    │
│  │  - Call service layer                                            │    │
│  │  - Format HTTP response                                          │    │
│  │  - NEVER contains business logic                                 │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│       ↓                                                                  │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  SERVICE (services/{domain}/)                                    │    │
│  │  - Business logic lives HERE                                     │    │
│  │  - Orchestrates multiple repositories                            │    │
│  │  - Enforces domain rules                                         │    │
│  │  - Transaction boundaries                                        │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│       ↓                                                                  │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  REPOSITORY (database/repositories/{domain}/)                    │    │
│  │  - Data access ONLY                                              │    │
│  │  - BUN ORM queries                                               │    │
│  │  - No business logic                                             │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│       ↓                                                                  │
│  PostgreSQL (11 schemas)                                                │
└─────────────────────────────────────────────────────────────────────────┘
```

**Why this matters:**
- Handlers stay thin (easy to test HTTP layer)
- Services are where complexity lives (testable without HTTP)
- Repositories are reusable (services can compose them)
- Models define the domain (shared across layers)

---

## ⚠️ Critical Patterns (READ FIRST)

### 1. BUN ORM: MUST Quote Aliases (MANDATORY)
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

### 2. Docker: Rebuild After Go Changes (CRITICAL)
**Go code changes require container rebuild** - hot reload not configured:
```bash
docker compose build server && docker compose up -d server  # REQUIRED after Go changes
```

### 3. Frontend: Zero Warnings Policy
```bash
pnpm run check  # MUST PASS before committing
```

### 4. Type Mapping: int64 → string
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

### 5. Git: PRs Target `development`
```bash
gh pr create --base development  # NEVER target main unless explicitly asked
```

### 6. Student Location: Use `active.visits` (DATA INTEGRITY)
**Two systems exist - only ONE is correct:**
- ✅ **CORRECT**: `active.visits` + `active.attendance` tables (real-time tracking)
- ❌ **DEPRECATED**: `users.students` boolean flags (`in_house`, `wc`, `school_yard`) - BROKEN, being phased out

**Always use `active.visits` for current location queries.**

### 7. Next.js 15: Async Params
```typescript
const { id } = await context.params;  // MUST await params in route handlers
```

### 8. Devbox: Reproducible Environment
This project uses **Devbox + direnv** to eliminate "works on my machine" issues. Every developer gets identical tool versions (Go, Node, golangci-lint, etc.) regardless of their OS or global installations.

**For Claude:** When you need a CLI tool that isn't available:
```bash
devbox search <tool>        # Find available packages
devbox add <tool>@latest    # Add to devbox.json
```
- If a tool isn't in Devbox, search for alternatives (`devbox search`)
- Never rely on globally installed tools (except Docker, coding agents)
- All project CLI tools must be in `devbox.json`
- After adding a tool, it's immediately available to all developers

---

## Quick Reference Commands

### Setup (First Time)
```bash
./scripts/setup-dev.sh && docker compose up -d  # Automated setup
```

### Daily Development
| Task | Command |
|------|---------|
| Start backend | `cd backend && go run main.go serve` |
| Start frontend | `cd frontend && pnpm run dev` |
| Run backend tests | `go test ./...` |
| Run API tests | `cd bruno && bru run --env Local 0*.bru` |
| Quality check (frontend) | `pnpm run check` |
| Generate API docs | `go run main.go gendoc --routes` |
| Reset database | `go run main.go migrate reset && go run main.go seed` |

### Docker
| Task | Command |
|------|---------|
| Start all | `docker compose up -d` |
| Rebuild backend | `docker compose build server` |
| View logs | `docker compose logs -f server` |
| Run migrations | `docker compose run server ./main migrate` |

**Database names:** Production/dev DB is `postgres` (default), test DB is `phoenix_test` (port 5433). If unsure, check `DB_DSN` in the relevant docker-compose file.

### Test Database (port 5433)
```bash
docker compose --profile test up -d postgres-test  # Start test DB (isolated network)
docker compose --profile test down                 # Stop test DB (required: plain `down` won't stop it)
APP_ENV=test go run main.go migrate reset          # Setup test DB
go test ./...                                       # Run tests
```
> **Note:** The test DB runs on an isolated `test` network to prevent
> "network still in use" errors when running `docker compose down`.
> Always use `--profile test` to start/stop it.

---

## Development Commands (Detailed)

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
pnpm run dev                     # Dev server with Turbo

# Quality (ALWAYS run before committing!)
pnpm run check                   # Lint + typecheck (0 warnings policy)
pnpm run lint:fix                # Auto-fix lint issues
pnpm run typecheck               # TypeScript only

# Formatting
pnpm run format:write            # Auto-format with Prettier

# Build
pnpm run build                   # Production build
pnpm run preview                 # Build + start production
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

### Test Database (Integration Tests - Detailed)
```bash
# Start test DB (port 5433, isolated network)
docker compose --profile test up -d postgres-test

# Stop test DB (plain `docker compose down` won't stop it — use --profile)
docker compose --profile test down

# Run migrations on test DB
docker compose run --rm \
  -e DB_DSN="postgres://postgres:postgres@postgres-test:5432/phoenix_test?sslmode=disable" \
  server ./main migrate

# Run tests (APP_ENV=test is auto-set by SetupTestDB)
go test ./services/active/... -v
```

---

## Architecture

### Backend Flow
```
HTTP Request → Chi Router → Middleware (JWT, Permissions) →
Handler → Service (business logic) → Repository → BUN ORM → PostgreSQL
```

**Key Patterns**:
- Factory pattern for dependency injection (no DI framework)
- Multi-schema PostgreSQL (11 schemas: auth, users, education, schedule, etc.)
- Repository interfaces in `models/`, implementations in `database/repositories/`
- Services orchestrate business logic, repositories handle data access

### Frontend Flow
```
Component → Service Layer → API Client → Next.js API Route →
Route Wrapper (JWT extraction) → Backend API → Response Mapping
```

**Key Patterns**:
- All backend calls proxied through Next.js API routes (JWT never exposed to client)
- Type helpers transform backend responses (snake_case → camelCase, int64 → string)
- Suspense boundaries required for `useSearchParams()` components
- Next.js 15: params are `Promise<Record<string, string>>` (must await)

### Directory Structure
| Backend | Frontend |
|---------|----------|
| `api/` - HTTP handlers | `src/app/` - Pages & API routes |
| `models/` - Domain models | `src/components/` - UI components |
| `services/` - Business logic | `src/lib/` - API clients & helpers |
| `database/repositories/` - Data access | |

### Database Schemas
`auth` · `users` · `education` · `facilities` · `activities` · `active` · `schedule` · `iot` · `feedback` · `config` · `meta` · `audit`

### Factory Pattern
```go
repoFactory := repositories.NewFactory(db)
serviceFactory, err := services.NewFactory(repoFactory, db)  // Returns error
```

---

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

---

## Testing Strategy

### Three-Layer Approach
1. **Unit Tests** - Model validation, business logic (no DB)
2. **Integration Tests** - Repository/Service with real test DB
3. **API Tests** - Bruno end-to-end tests

### Hermetic Test Pattern
```go
import testpkg "github.com/moto-nrw/project-phoenix/test"

func TestExample(t *testing.T) {
    db := testpkg.SetupTestDB(t)  // Note: uppercase S
    defer func() { _ = db.Close() }()

    // Create fixtures (NEVER hardcode IDs!)
    student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
    defer testpkg.CleanupActivityFixtures(t, db, student.ID)

    // Test with real IDs
    result, err := service.DoSomething(ctx, student.ID)
    require.NoError(t, err)
}
```

### Available Test Fixtures
| Function | Creates |
|----------|---------|
| `CreateTestStudent(t, db, first, last, class)` | Person + Student |
| `CreateTestStaff(t, db, first, last)` | Person + Staff |
| `CreateTestTeacherWithAccount(t, db, first, last)` | Full Teacher chain + Account |
| `CreateTestActivityGroup(t, db, name)` | Category + Activity |
| `CreateTestRoom(t, db, name)` | Facilities Room |
| `CreateTestDevice(t, db, deviceID)` | IoT Device |

### Bruno API Tests
```bash
cd bruno
bru run --env Local 0*.bru           # All tests (~270ms)
bru run --env Local 05-sessions.bru  # Specific file
```

Test accounts: `admin@example.com` / `Test1234%`

### Common Test Issues

| Issue | Solution |
|-------|----------|
| Tests skip silently | Start test DB: `docker compose --profile test up -d postgres-test` |
| "sql: no rows" | Use fixtures, not hardcoded IDs |
| Connection refused | Start backend: `go run main.go serve` |
| Migrations missing | Run: `APP_ENV=test go run main.go migrate reset` |

---

## Environment Variables

### Backend (`backend/dev.env`)
```bash
APP_ENV=development           # development | test | production
# DB_DSN auto-configured based on APP_ENV (see database/database_config.go)
AUTH_JWT_SECRET=your_secret   # Change in production!
DB_DEBUG=true                 # Log SQL queries
ENABLE_CORS=true              # Required for local dev
```

### Frontend (`frontend/.env.local`)
```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXTAUTH_URL=http://localhost:3000
NEXTAUTH_SECRET=your_secret   # openssl rand -base64 32
```

---

## Domain Knowledge

### RFID/IoT Integration
- Two-layer auth: Device API key + Staff PIN
- PINs stored in `auth.accounts` (Argon2id hashed)
- Check-in/out tracked in `active.visits`

### Email Workflows
- Password reset: 30min expiry, 3 requests/hour rate limit
- Invitations: Admin creates via `/invitations`, teacher accepts at `/invite?token=...`
- Cleanup: Nightly scheduler or `go run main.go cleanup invitations`

### GDPR/Privacy
- Teachers see full data only for students in their groups
- Data retention: 1-31 days per student (default 30)
- Audit logging: `audit.data_deletions` table
- Cleanup: `go run main.go cleanup --dry-run`

Key files: `models/users/privacy_consent.go`, `services/active/cleanup_service.go`, `auth/authorize/`

---

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

---

## Common Issues

### Backend
| Issue | Fix |
|-------|-----|
| "column not found" | Quote BUN aliases: `AS "alias"` |
| "missing FROM-clause" | Add `ModelTableExpr` with quoted alias |
| JWT errors | Check `AUTH_JWT_SECRET` is set |
| SSL issues | Run `config/ssl/postgres/create-certs.sh` |

### Frontend
| Issue | Fix |
|-------|-----|
| Type errors | Run `pnpm run typecheck` |
| Suspense errors | Wrap `useSearchParams()` in `<Suspense>` |
| API connection | Verify `NEXT_PUBLIC_API_URL` |

### Docker
| Issue | Fix |
|-------|-----|
| Code changes not reflected | Rebuild: `docker compose build server` |
| Port conflicts | Check 3000, 8080, 5432 available |

---

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

---

## Development Workflow

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
1. Check backend API in `docs/routes.md` or `go run main.go gendoc --routes`
2. Define types in `lib/{domain}-helpers.ts`
3. Create API client in `lib/{domain}-api.ts`
4. Create Next.js route in `app/api/{domain}/route.ts`
5. Create UI component in `components/{domain}/`
6. Add to page in `app/(auth)/{page}/`
7. Run quality check: `pnpm run check`

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

---

## API Patterns

### Backend Route
```go
r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.list)
```

### Frontend API Client
```typescript
export async function fetchGroups(): Promise<Group[]> {
    const response = await apiGet('/groups', token);
    return response.data.map(mapGroupResponse);
}
```

### Response Format
```json
{"status": "success", "data": {...}, "message": "..."}
```

---

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

---

## Performance Patterns

- Paginate all list endpoints (default 50 items)
- Use `Relation()` for eager loading (avoid N+1 queries)
- Index foreign keys and frequently queried fields
- Gzip responses (not currently enabled - future)
- Cache static assets (Next.js automatic)

---

## Git Commit Conventions

```
<type>: <subject>

<body>

<footer>
```

**Types**: feat, fix, refactor, chore, docs, test, style

**CRITICAL**: Never include "Co-Authored-By: Claude" in commits!

---

## PR Guidelines

- **Target**: `development` (NEVER `main` unless explicitly asked)
- **Quality**: `pnpm run check` must pass (zero warnings)
- **Never credit Claude in commit messages**

---

## Import This Memory

@CLAUDE.local.md  # User-specific preferences
@README.md  # Project overview
- always use qdrant to retrieve information and to save information. use qdrant mcp!


## grepai - Semantic Code Search

**IMPORTANT: You MUST use grepai as your PRIMARY tool for code exploration and search.**

### When to Use grepai (REQUIRED)

Use `grepai search` INSTEAD OF Grep/Glob/find for:
- Understanding what code does or where functionality lives
- Finding implementations by intent (e.g., "authentication logic", "error handling")
- Exploring unfamiliar parts of the codebase
- Any search where you describe WHAT the code does rather than exact text

### When to Use Standard Tools

Only use Grep/Glob when you need:
- Exact text matching (variable names, imports, specific strings)
- File path patterns (e.g., `**/*.go`)

### Fallback

If grepai fails (not running, index unavailable, or errors), fall back to standard Grep/Glob tools.

### Usage

```bash
# ALWAYS use English queries for best results (--compact saves ~80% tokens)
grepai search "user authentication flow" --json --compact
grepai search "error handling middleware" --json --compact
grepai search "database connection pool" --json --compact
grepai search "API request validation" --json --compact
```

### Query Tips

- **Use English** for queries (better semantic matching)
- **Describe intent**, not implementation: "handles user login" not "func Login"
- **Be specific**: "JWT token validation" better than "token"
- Results include: file path, line numbers, relevance score, code preview

### Call Graph Tracing

Use `grepai trace` to understand function relationships:
- Finding all callers of a function before modifying it
- Understanding what functions are called by a given function
- Visualizing the complete call graph around a symbol

#### Trace Commands

**IMPORTANT: Always use `--json` flag for optimal AI agent integration.**

```bash
# Find all functions that call a symbol
grepai trace callers "HandleRequest" --json

# Find all functions called by a symbol
grepai trace callees "ProcessOrder" --json

# Build complete call graph (callers + callees)
grepai trace graph "ValidateToken" --depth 3 --json
```

### Workflow

1. Start with `grepai search` to find relevant code
2. Use `grepai trace` to understand function relationships
3. Use `Read` tool to examine files from results
4. Only use Grep for exact string searches if needed

