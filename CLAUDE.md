# CLAUDE.md

## Project Overview

**Project Phoenix** - GDPR-compliant RFID student attendance and room management system.

| Component | Technology |
|-----------|------------|
| Backend | Go 1.23+, Chi router, BUN ORM |
| Frontend | Next.js 16+, React 19+, Tailwind 4+ |
| Database | PostgreSQL 17+ (multi-schema, SSL) |
| Auth | JWT (15min access, 7 days refresh) |

## Ecosystem

Project Phoenix is part of a three-repo system. All repos live side-by-side (`../`):

| Repo | Role | Relationship |
|------|------|-------------|
| **PyrePortal** (`../PyrePortal/`) | Raspberry Pi kiosk app (Tauri + React) | Consumes `/api/iot/*` endpoints with device API key + staff PIN auth |
| **moto-balenaOS** (`../moto-balenaOS/`) | Balena OS deployment layer | Runs PyrePortal + Phoenix backend on Raspberry Pi hardware |

**If you change IoT endpoints, error messages, or auth headers**: PyrePortal will break silently. Error messages are hardcoded in `PyrePortal/src/services/api.ts` and mapped to German UI text. Coordinate changes across repos.

## Multi-Tenancy

### Tenant Hierarchy

```
Platform Operator (moto)
 └── Organization (Träger)           → platform.organizations
      └── School (OGS) = tenant      → platform.schools (school.id = tenant_id)
```

**School ID is the tenant boundary.** All 58+ tenant-scoped tables have a `tenant_id` FK to `platform.schools`. Account-to-school mappings live in `auth.account_tenants` (with lifecycle: pending → active → inactive).

### Scoping Mechanisms

| Layer | How |
|-------|-----|
| **JWT** | Claims include `tenant_id`, `org_id`, `scope` ("" = tenant, "org" = organization, "platform" = operator) |
| **Context** | `tenant.WithTenantID(ctx, id)` / `tenant.FromContext(ctx)` propagate tenant through request lifecycle |
| **Database** | `TenantTxMiddleware` sets PostgreSQL `LOCAL ROLE` + RLS config per request; auto-rollback on 5xx |
| **Models** | `base.TenantModel` (embeds `TenantID int64`) + `TenantScoped` interface on all tenant-aware entities |
| **Repositories** | `base.GetDB(ctx, db)` picks up tenant transaction; `base.EnsureTenantID(ctx, entity)` auto-populates tenant_id |

### Frontend Routing

- **Subdomain mode**: `{slug}.localhost:3000` → proxy rewrites to `/[tenant]/*` internally
- **Operator isolation**: `operator.localhost:3000` → rewrites to `/operator/*`, separate session
- **Tenant resolution**: `[tenant]/layout.tsx` validates slug via `/auth/tenant/resolve?slug=...` (cached 5min)
- **Tenant switching**: `POST /auth/switch-tenant` returns new JWT scoped to target school

### Key Env Vars

| Var | Purpose |
|-----|---------|
| `TENANT_DOMAIN` | Base domain for subdomain extraction (e.g., `localhost`, `moto-app.de`) |
| `NEXT_PUBLIC_TENANT_DOMAIN` | Client-side tenant domain |
| `NEXT_PUBLIC_OPERATOR_HOSTNAME` | Operator subdomain (e.g., `operator.localhost:3000`) |

### Reserved Slugs

Both backend (`models/platform/organization.go`) and frontend (`lib/reserved-slugs.ts`) maintain matching lists of reserved slugs (www, api, operator, grafana, etc.) that cannot be used as tenant subdomains. **These must stay in sync.**

### Cross-Repo Impact

Changing tenant resolution, auth headers, or error messages affects PyrePortal's device auth flow. The IoT API (`/api/iot/*`) uses device API keys (not tenant JWTs), but devices are scoped to schools.

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

### 10. Migrations and RLS: No Bypass Needed
Migrations connect via `DB_DSN` as the `postgres` **superuser**. PostgreSQL superusers always bypass Row Level Security, even with `FORCE ROW LEVEL SECURITY` enabled. This means:
- **Data migrations (UPDATE/INSERT/DELETE) do NOT need to disable RLS** — the superuser connection sees all rows across all tenants automatically
- **DDL migrations (CREATE TABLE, ALTER, GRANT)** are never affected by RLS regardless of role
- **Never add `ALTER TABLE ... DISABLE/ENABLE ROW LEVEL SECURITY`** in migration code — it's unnecessary and can cause test failures
- **Migration version numbers must be unique** — two migrations sharing a version in `MigrationRegistry` causes a map key collision where one silently overwrites the other

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

### Hermetic Tests (MANDATORY)

**ABSOLUTE RULE: All new backend tests MUST be hermetic and MUST pass the `TestHermeticTestPatterns` CI check.**

- **No hardcoded IDs**: Never use `int64(1)` through `int64(9)` as entity IDs. Use test fixtures (`testpkg.CreateTestStudent`, `testpkg.CreateTestStaff`, etc.) and reference the returned `.ID`.
- **Mock test files must be exempted**: If a new test file uses sqlmock/mock structs instead of real DB fixtures, add it to the `skipPatterns` in `backend/test/hermetic_verification_test.go`. Otherwise the `no_hardcoded_integer_ids` check will flag mock IDs as violations.
- **Always run the hermetic check locally before pushing**: `cd backend && go test ./test/ -run TestHermeticTestPatterns -v`
- **Each test creates its own data and cleans up**: Use `defer testpkg.Cleanup*` helpers. Never depend on seed data or shared state between tests.

### Test Database (port 5433)
```bash
docker compose --profile test up -d postgres-test  # Start (isolated network)
docker compose --profile test down                 # Stop (plain `down` won't work)
APP_ENV=test go run main.go migrate reset          # Setup
```

## No Fallbacks, No Defaults — Fail Fast (MANDATORY)

**ABSOLUTE RULE: NEVER use fallback defaults (`??`, `||`, `.default()`, `.optional().default()`) for environment variables or configuration values. Missing config MUST crash immediately.**

Silent fallbacks are unacceptable because they create invisible production bugs. A developer misconfigures one env var, the app boots fine, passes CI, deploys — and then silently runs with `operator.localhost:3000` in production. No error. No log. No alert. The bug surfaces days later as a user report: "login doesn't work." Root cause? A missing env var that three layers of `??` fallbacks quietly papered over.

Fallbacks destroy developer experience:
- They make `.env.example` a lie — "these values are just defaults anyway"
- They make `docker compose up` silently broken — the app starts, looks healthy, but half the features route to localhost
- They make debugging a nightmare — nothing errors, nothing logs, the wrong value just propagates silently through the system
- They violate the principle of least surprise — a fresh clone with no `.env` should **fail with a clear message**, not boot into a half-working state

### Rules

```typescript
// FORBIDDEN — silent fallback
const hostname = process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME ?? "operator.localhost:3000";

// FORBIDDEN — optional with default in env schema
NEXT_PUBLIC_OPERATOR_HOSTNAME: z.string().optional().default("operator.localhost:3000")

// CORRECT — fail fast with a clear error
const hostname = process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME;
if (!hostname) throw new Error("NEXT_PUBLIC_OPERATOR_HOSTNAME is not set");

// CORRECT — required in env schema, no default
NEXT_PUBLIC_OPERATOR_HOSTNAME: z.string().min(1)
```

### Where this applies
- **All `process.env` reads** in proxy, server code, and client code
- **All Zod schemas** in `env.js` for environment validation
- **All docker-compose environment blocks** — use `${VAR}` not `${VAR:-default}`
- **Exception**: Only `NODE_ENV` and `LOG_LEVEL` may have defaults (they have universally safe defaults like `"development"` and `"info"`)

### What to do instead
- Put the correct value in `.env.example` as documentation
- Let `env.js` validation crash the build if the var is missing
- In proxy (`proxy.ts`) where `env.js` can't run, throw explicitly
- If required in `env.js`: add as `ARG` + `ENV` in `frontend/Dockerfile.prod` and as `build-args` in `.github/workflows/build.yml`
- `next build` runs env validation inside the Docker container — missing build args break CI

## Environment Management (SOPS)

Deployed environments (staging, production) use **SOPS-encrypted env files** tracked in git. No more manual `.env` management via SSH.

### How It Works

```
1. Developer edits:  sops environments/staging.sops.env   (decrypts → $EDITOR → re-encrypts)
2. Commit + push:    git commit environments/staging.sops.env → push to development
3. CI decrypts:      sops decrypt staging.sops.env > /tmp/staging.env
4. CI deploys:       SCP .env + docker-compose.yml + deploy-remote.sh → server
5. Server runs:      deploy-remote.sh (pull → backup DB → migrate → start → healthcheck)
```

### File Layout

| File | Purpose |
|------|---------|
| `environments/staging.sops.env` | Encrypted env vars for staging |
| `environments/production.sops.env` | Encrypted env vars for production |
| `environments/staging.compose.yml` | Docker Compose for staging (images from GHCR) |
| `environments/production.compose.yml` | Docker Compose for production |
| `.sops.yaml` | SOPS config with age public key |
| `scripts/sops-setup.sh` | One-time setup: generate age key, encrypt files |
| `scripts/env-check.sh` | CI validation: key sync across all env files |
| `scripts/deploy-remote.sh` | Runs on server: pull, backup, migrate, rollback |

### Local SOPS Setup

```bash
# 1. One-time: generate age key and encrypt files
./scripts/sops-setup.sh

# Key location (macOS):
#   ~/Library/Application Support/sops/age/keys.txt
# Key location (Linux):
#   ~/.config/sops/age/keys.txt
#
# Share the private key with team members via 1Password/Signal — NEVER Slack/email.
# CI uses the same key as GitHub Secret: SOPS_AGE_KEY
```

### Common SOPS Commands

```bash
# Edit encrypted file (opens in $EDITOR, re-encrypts on save)
sops environments/staging.sops.env

# View decrypted values (stdout only, no file change)
sops decrypt environments/staging.sops.env

# View a single value
sops decrypt environments/staging.sops.env | grep AUTH_JWT_REFRESH_EXPIRY

# Verify all env files are in sync
./scripts/env-check.sh
```

### Key Rules

1. **Keys are plaintext, values are encrypted** — SOPS encrypts only values, so CI can validate key consistency without decryption
2. **Both `.sops.env` files must have identical keys** — `env-check.sh` enforces this in CI
3. **`.env.example` must stay in sync** with `.sops.env` keys (minus whitelisted dev-only/deploy-only vars)
4. **Shared `.env` on server** — all services (postgres, server, frontend) load the same `.env` via `env_file:`. Use the compose `environment:` block to override per-service (e.g., `PORT: 3000` for frontend to override backend's `PORT=8080`)
5. **Edit with SOPS CLI** — `sops environments/staging.sops.env` opens decrypted in `$EDITOR`, re-encrypts on save. Never manually edit encrypted values.

### Adding a New Env Var (Deployed Environments)

- [ ] Add to both `environments/*.sops.env` files via `sops` CLI
- [ ] Add to `.env.example` (for local dev parity)
- [ ] If frontend-only or needs override: add to `environment:` block in `environments/*.compose.yml`
- [ ] Run `./scripts/env-check.sh` to verify sync

### Deployment Triggers

| Environment | Trigger | Branch | SOPS File |
|-------------|---------|--------|-----------|
| Staging | Push to `development` | `development` | `staging.sops.env` |
| Production | Push to `main` | `main` | `production.sops.env` |

### Deploy Flow (CI → Server)

CI (`build.yml`) runs on push/merge:
1. **Decrypt**: `sops decrypt environments/{env}.sops.env > /tmp/{env}.env`
2. **SCP to server**: `.env.new`, `docker-compose.yml.new`, `deploy-remote.sh`
3. **`deploy-remote.sh`** runs on the server:
   - Saves rollback copies (`.env.rollback`, `docker-compose.yml.rollback`)
   - Swaps in new config, pins Docker images to commit SHA
   - `docker compose pull` (fails → restore old config, abort)
   - Stops `server` + `frontend` (postgres stays up for backup)
   - `pg_dump` backup to `~/backups/{env}/` (fails → restore, abort)
   - `docker compose run --rm server ./main migrate` (fails → rollback DB + config)
   - `docker compose up -d --wait` + healthcheck (fails → rollback)
   - On success: writes `.deploy-state`, prunes old backups

**Exit codes**: `0` = success, `1` = aborted before migration, `10` = rollback succeeded, `11` = rollback failed (CRITICAL)

### Server Directory Structure

```
~/staging/          ← staging deployment
  .env              ← decrypted from staging.sops.env (by CI)
  docker-compose.yml ← from staging.compose.yml (images pinned to SHA)
  .deploy-state     ← CURRENT_SHA, PREVIOUS_SHA, DEPLOYED_AT, BACKUP_FILE
~/production/       ← production deployment (same structure)
~/backups/{env}/    ← pg_dump backups (retention: 3 staging, 7 production)
```

### GitHub Secrets Required

| Secret | Purpose |
|--------|---------|
| `SOPS_AGE_KEY` | Age private key for decrypting `.sops.env` files |
| `STAGING_SSH_KEY` | SSH deploy key for staging server |
| `STAGING_SSH_HOST` | Staging server hostname/IP |
| `STAGING_SSH_KNOWN_HOSTS` | SSH host verification |
| `PRODUCTION_SSH_KEY` / `PRODUCTION_SSH_HOST` / `PRODUCTION_SSH_KNOWN_HOSTS` | Same for production |
| `DEPLOY_NOTIFY_*` | Email notification recipients for deploy failures |

### CI Guards

- **`env-sync-check`** job runs on every PR — blocks merge if keys are out of sync or plaintext values detected
- **Lefthook pre-commit** — runs env key sync + unencrypted secrets guard on staged `.sops.env` changes

### Whitelisted Key Exceptions

| Key | Scope | Reason |
|-----|-------|--------|
| `COMPOSE_BAKE`, `COMPOSE_DOCKER_CLI_BUILD`, `DOCKER_BUILDKIT` | dev-only | Docker build optimization, not needed on server |
| `DB_DEBUG`, `TEST_DB_DSN`, `TEST_DB_PORT` | dev-only | Local testing only |
| `AUTH_TRUST_HOST`, `TENANT_DOMAIN`, `NEXT_PUBLIC_TENANT_DOMAIN` | deploy-only | Multi-tenancy, not needed for local dev |
| `PHOENIX_AUTH_PASSWORD` | deploy-only | Server DB role password |

## URL & Route Conventions

**All URL paths must use kebab-case** — both backend API routes and frontend page routes.

```
/students/{id}/attendance-history   ✅ kebab-case
/students/{id}/room-history         ✅ kebab-case
/students/{id}/feedback_history     ❌ snake_case (legacy, migrate when touched)
```

Existing snake_case routes (`feedback_history`, `mensa_history`) are legacy. When modifying these routes, migrate them to kebab-case.

## Git Conventions

**Commit types**: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `style`

**CRITICAL**: Never include "Co-Authored-By: Claude" in commits.

## Database Schemas

`platform` · `auth` · `users` · `education` · `facilities` · `activities` · `active` · `schedule` · `iot` · `feedback` · `config` · `suggestions` · `meta` · `audit`

---

@CLAUDE.local.md
