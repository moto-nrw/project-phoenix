# CLAUDE.md

## Project Overview

**Project Phoenix** - GDPR-compliant RFID student attendance and room management system.

| Component | Technology |
|-----------|------------|
| Backend | Go 1.23+, Chi router, BUN ORM |
| Frontend | Next.js 15+, React 19+, Tailwind 4+ |
| Database | PostgreSQL 17+ (multi-schema, SSL) |
| Auth | JWT (15min access, 1hr refresh) |

## Core Architecture

**Handler → Service → Repository → Database** (always, no exceptions)

- `api/{domain}/` — HTTP handlers (thin, no business logic)
- `services/{domain}/` — Business logic, orchestration, transactions
- `database/repositories/{domain}/` — Data access only (BUN ORM)
- `models/{domain}/` — Domain entities, shared across layers
- Factory pattern for DI: `repositories.NewFactory(db)` → `services.NewFactory(repoFactory, db)`

## Critical Patterns

### 1. BUN ORM: Quote Aliases (MANDATORY)
```go
ModelTableExpr(`education.groups AS "group"`)   // CORRECT — quoted
ModelTableExpr(`education.groups AS group`)     // WRONG — runtime error
// Nested: ColumnExpr(`"teacher".id AS "teacher__id"`)
```

### 2. Docker: Rebuild After Go Changes
```bash
docker compose build server && docker compose up -d server
```

### 3. Frontend: Zero Warnings Policy
```bash
pnpm run check  # MUST PASS before committing
```

### 4. Type Mapping: int64 → string
Backend `int64` IDs become frontend `string`. Use `data.id.toString()` and `snake_case → camelCase` mapping helpers in `lib/{domain}-helpers.ts`.

### 5. PRs Target `development`
```bash
gh pr create --base development  # NEVER target main unless explicitly asked
```

### 6. Student Location: Use `active.visits`
- `active.visits` + `active.attendance` — real-time, correct
- `users.students` boolean flags (`in_house`, `wc`, `school_yard`) — DEPRECATED, broken

### 7. Next.js 15: Async Params
```typescript
const { id } = await context.params;  // MUST await
```

### 8. Backend Logging: slog Only
Use injected `*slog.Logger` with key-value pairs. Never logrus/log.Printf. GDPR: no student names at Info level.

### 9. Devbox Environment
```bash
devbox search <tool>     # Find packages
devbox add <tool>@latest # Add to devbox.json — never rely on global installs
```

## Essential Commands

| Task | Command |
|------|---------|
| Start backend | `cd backend && go run main.go serve` |
| Start frontend | `cd frontend && pnpm run dev` |
| Run tests | `cd backend && go test ./...` |
| Quality check | `cd frontend && pnpm run check` |
| Rebuild backend (Docker) | `docker compose build server` |
| Run migrations | `cd backend && go run main.go migrate` |
| Reset + seed DB | `cd backend && go run main.go migrate reset && go run main.go seed` |
| Generate docs | `cd backend && go run main.go gendoc --routes` |

### Test Database (port 5433)
```bash
docker compose --profile test up -d postgres-test  # Start (isolated network)
docker compose --profile test down                 # Stop (plain `down` won't work)
APP_ENV=test go run main.go migrate reset          # Setup
```

## Git Conventions

**Commit types**: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `style`

**CRITICAL**: Never include "Co-Authored-By: Claude" in commits.

## Database Schemas

`auth` · `users` · `education` · `facilities` · `activities` · `active` · `schedule` · `iot` · `feedback` · `config` · `meta` · `audit`

---

@CLAUDE.local.md
