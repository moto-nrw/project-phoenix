# CLAUDE.md

## Project Overview

**Project Phoenix** - GDPR-compliant RFID student attendance and room management system.

| Component | Technology |
|-----------|------------|
| Backend | Go 1.23+, Chi router, BUN ORM |
| Frontend | Next.js 16+, React 19+, Tailwind 4+ |
| Database | PostgreSQL 17+ (multi-schema, SSL) |
| Auth | JWT (15min access, 1hr refresh) |

## Ecosystem

Project Phoenix is part of a three-repo system. All repos live side-by-side (`../`):

| Repo | Role | Relationship |
|------|------|-------------|
| **PyrePortal** (`../PyrePortal/`) | Raspberry Pi kiosk app (Tauri + React) | Consumes `/api/iot/*` endpoints with device API key + staff PIN auth |
| **moto-balenaOS** (`../moto-balenaOS/`) | Balena OS deployment layer | Runs PyrePortal + Phoenix backend on Raspberry Pi hardware |

**If you change IoT endpoints, error messages, or auth headers**: PyrePortal will break silently. Error messages are hardcoded in `PyrePortal/src/services/api.ts` and mapped to German UI text. Coordinate changes across repos.

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

### 7. Next.js 16: Async Params
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

**RULE: Always suggest Docker Compose commands** when advising how to run, build, test, or debug services. Never default to bare `go run` or `pnpm run dev` unless the user explicitly asks for it. The development environment runs through Docker Compose.

| Task | Command |
|------|---------|
| Start all services | `docker compose up -d` |
| Rebuild + restart backend | `docker compose build server && docker compose up -d server` |
| Run migrations | `docker compose run server ./main migrate` |
| Reset + seed DB | `docker compose run server ./main migrate reset && docker compose run server ./main seed` |
| View logs | `docker compose logs -f server` |
| Quality check (frontend) | `cd frontend && pnpm run check` |
| Run backend tests | `cd backend && go test ./...` |
| Generate docs | `docker compose run server ./main gendoc --routes` |

**Seeder is DEV-ONLY**: `go run main.go seed` creates fake test data and must NEVER run on staging or production. Production infrastructure (system rooms, categories, activities) must be created via data migrations or admin UI — never via the seeder.

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
