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

This file provides guidance to Claude Code when working with code in this repository.

## Project Overview

**Project Phoenix** - A GDPR-compliant RFID-based student attendance and room management system for educational institutions.

| Component | Technology |
|-----------|------------|
| Backend | Go 1.23+, Chi router, BUN ORM |
| Frontend | Next.js 15+, React 19+, Tailwind 4+ |
| Database | PostgreSQL 17+ (multi-schema, SSL) |
| Auth | JWT (15min access, 1hr refresh) |
| Testing | Go tests + Bruno API tests |

---

## ⚠️ Critical Patterns (READ FIRST)

### 1. BUN ORM: MUST Quote Aliases
```go
ModelTableExpr(`education.groups AS "group"`)  // ✅ CORRECT
ModelTableExpr(`education.groups AS group`)    // ❌ WRONG - "column not found"
```

For nested relationships, use explicit column mapping:
```go
ColumnExpr(`"teacher".id AS "teacher__id"`)
ColumnExpr(`"staff".id AS "staff__id"`)
Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`)
```

### 2. Docker: Rebuild After Go Changes
```bash
docker compose build server && docker compose up -d server  # REQUIRED after Go changes
```

### 3. Frontend: Zero Warnings Policy
```bash
npm run check  # MUST PASS before committing
```

### 4. Type Mapping: int64 → string
```typescript
id: data.id.toString()  // Backend int64 → Frontend string
```

### 5. Git: PRs Target `development`
```bash
gh pr create --base development  # NEVER target main unless explicitly asked
```

### 6. Student Location: Use `active.visits`
```go
// ✅ CORRECT: active.visits + active.attendance tables
// ❌ WRONG: users.students boolean flags (in_house, wc, school_yard) - DEPRECATED
```

### 7. Next.js 15: Async Params
```typescript
const { id } = await context.params;  // MUST await params in route handlers
```

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
| Start frontend | `cd frontend && npm run dev` |
| Run backend tests | `go test ./...` |
| Run API tests | `cd bruno && bru run --env Local 0*.bru` |
| Quality check (frontend) | `npm run check` |
| Generate API docs | `go run main.go gendoc --routes` |
| Reset database | `go run main.go migrate reset && go run main.go seed` |

### Docker
| Task | Command |
|------|---------|
| Start all | `docker compose up -d` |
| Rebuild backend | `docker compose build server` |
| View logs | `docker compose logs -f server` |
| Run migrations | `docker compose run server ./main migrate` |

### Test Database (port 5433)
```bash
docker compose --profile test up -d postgres-test  # Start test DB
APP_ENV=test go run main.go migrate reset          # Setup test DB
go test ./...                                       # Run tests
```

---

## Architecture

### Backend Flow
```
HTTP → Chi Router → JWT Middleware → Permission Check → Handler → Service → Repository → BUN ORM → PostgreSQL
```

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
| Type errors | Run `npm run typecheck` |
| Suspense errors | Wrap `useSearchParams()` in `<Suspense>` |
| API connection | Verify `NEXT_PUBLIC_API_URL` |

### Docker
| Issue | Fix |
|-------|-----|
| Code changes not reflected | Rebuild: `docker compose build server` |
| Port conflicts | Check 3000, 8080, 5432 available |

---

## Development Workflow

### Backend Feature
1. Define model in `models/{domain}/`
2. Create migration in `database/migrations/`
3. Implement repository in `database/repositories/{domain}/`
4. Add service in `services/{domain}/`
5. Create API handler in `api/{domain}/`
6. Test: `go test ./... && bru run --env Local 0*.bru`
7. Lint: `golangci-lint run --timeout 10m`

### Frontend Feature
1. Check API: `routes.md` or `go run main.go gendoc --routes`
2. Create types in `lib/{domain}-helpers.ts`
3. Create API client in `lib/{domain}-api.ts`
4. Build components in `components/{domain}/`
5. Add page in `app/(auth)/{page}/`
6. Verify: `npm run check`

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

## PR Guidelines

- **Target**: `development` (NEVER `main` unless explicitly asked)
- **Quality**: `npm run check` must pass (zero warnings)
- **Never credit Claude in commit messages**
