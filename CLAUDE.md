# Project Phoenix — Agent Context

## Project Overview

**Project Phoenix** - GDPR-compliant NFC/RFID student attendance and room management system (internal codename; the public product name is "moto").

| Component | Technology |
|-----------|------------|
| Backend | Go 1.25+, Chi router, BUN ORM |
| Frontend | Next.js 16+, React 19+, Tailwind 4+ |
| Database | PostgreSQL 17+ (15 domain schemas, SSL, RLS) |
| Auth | JWT via `AUTH_JWT_EXPIRY` / `AUTH_JWT_REFRESH_EXPIRY` (currently 15m / 168h), MFA, three isolated portals |

## Ecosystem

Project Phoenix is part of a three-repo system. All repos live side-by-side (`../`):

| Repo | Role | Relationship |
|------|------|-------------|
| **PyrePortal** (`../PyrePortal/`) | Raspberry Pi kiosk app (Tauri + React) | Consumes `/api/iot/*` endpoints with device API key + staff PIN auth |
| **moto-balenaOS** (`../moto-balenaOS/`) | Balena OS deployment layer | Runs the PyrePortal kiosk on Pi 5 hardware (the Phoenix backend itself runs on the server, never on the Pi) |

**If you change IoT endpoints, error messages, or auth headers**: PyrePortal will break silently. Backend error strings are hardcoded in `PyrePortal/src/services/api.ts` and mapped to German UI text. Coordinate changes across repos. PRs target `development` in all repos except moto-balenaOS (`main`).

### Presence mode (cross-repo contract)

`GET /api/iot/config` returns a `presence_mode: "detailed" | "binary"` field
that PyrePortal must respect. In `binary` mode the kiosk must hide room
selection, hide Raumwechsel/WC buttons, and branch the scan-result modal
based on `checkout.schulhof_enabled` (2-button door kiosk vs 3-button with
yard state). Missing or unknown values default to `detailed` so old kiosk
builds continue to work. Backend checkin semantics adapt transparently —
only the kiosk UI needs to change per mode.

## Multi-Tenancy

### Tenant Hierarchy

```
Platform Operator (moto)
 └── Organization (Träger)           → platform.organizations
      └── School (OGS) = tenant      → platform.schools (school.id = tenant_id)
```

**School ID is the tenant boundary.** All 58+ tenant-scoped tables have a `tenant_id` FK to `platform.schools`. Account-to-school mappings live in `auth.account_tenants` (lifecycle: pending → active → inactive).

### Scoping Mechanisms

| Layer | How |
|-------|-----|
| **JWT** | Claims include `tenant_id`, `org_id`, `scope` ("" = tenant, "org" = organization, "platform" = operator, "parent" = guardian) |
| **Context** | `tenant.WithTenantID(ctx, id)` / `tenant.FromContext(ctx)` propagate tenant through request lifecycle |
| **Database** | `TenantTxMiddleware` sets PostgreSQL `LOCAL ROLE` + RLS config per request; auto-rollback on 5xx |
| **Models** | `base.TenantModel` (embeds `TenantID int64`) + `TenantScoped` interface on all tenant-aware entities |
| **Repositories** | `base.GetDB(ctx, db)` picks up tenant transaction; `base.EnsureTenantID(ctx, entity)` auto-populates tenant_id |

### Frontend Routing

- **Subdomain mode**: `{slug}.localhost:3000` → proxy rewrites to `/[tenant]/*` internally
- **Operator isolation**: `operator.localhost:3000` → rewrites to `/operator/*`, separate session
- **Parents isolation**: `parents.localhost:3000` → rewrites to `/parents/*`, separate session (cross-tenant guardian portal)
- **Tenant resolution**: `[tenant]/layout.tsx` validates slug via `/auth/tenant/resolve?slug=...` (cached 5min)
- **Tenant switching**: `POST /auth/switch-tenant` returns new JWT scoped to target school

### Three Portals — Strict Session Isolation

Each portal runs as its own NextAuth (v5) instance with its own cookie + dedicated `basePath`. Operator/parents cookies are host-only; the tenant cookie is domain-scoped on purpose (shared across tenant subdomains so tenant switching works). The proxy redirects cross-host paths back to their canonical subdomain.

| Portal | Host | Cookie | basePath | JWT scope | Backend login |
|---|---|---|---|---|---|
| Tenant (staff) | `{slug}.{TENANT_DOMAIN}` | `{TENANT_DOMAIN, dots→dashes}.session-token` scoped to `.{TENANT_DOMAIN}` (localhost: `authjs.session-token`, host-only) | `/api/auth` | `""` (or `"org"`) | `POST /auth/login` |
| Operator | `{NEXT_PUBLIC_OPERATOR_HOSTNAME}` | `operator.session-token` (host-only) | `/api/operator/auth` | `"platform"` | `POST /operator/auth/login` |
| Parents | `{NEXT_PUBLIC_PARENTS_HOSTNAME}` | `parent.session-token` (host-only) | `/api/parent/auth` | `"parent"` | `POST /parent/auth/login` |

**Login policy** (enforced in `services/auth/auth_login*.go`):
- Tenant login refuses guardian-only accounts (returns `ErrParentMustUseParentPortal` → 403). Dual-role accounts (e.g. teacher AND guardian at the same school) pass through unchanged.
- Parents login requires guardian role on at least one tenant mapping (`ErrAccountNoGuardianRole` → 403).
- `auth/jwt/TenantMiddleware` rejects `scope=parent` tokens with 401 (defense-in-depth on top of cookie isolation).
- `auth/jwt/ParentMiddleware` rejects everything except `scope=parent`; mounted on `/parent/*` protected routes.
- **MFA exists and alters login flows**: tenant and operator logins can require a challenge step between credentials and session (`services/auth/mfa_service.go`, `api/auth/mfa_handlers.go`, `api/operator/mfa.go`, dedicated challenge/enrollment JWT claims in `backend/auth/jwt/`, trusted devices via `security.mfa_*` settings). Touch login flows only with MFA in mind.

**Embedded enrollment**: the parents portal serves the public enrollment form at `/parents/enroll/{slug}/{phaseId}` using the same `EnrollmentForm` component as `{slug}.TENANT_DOMAIN/enroll/{phaseId}`, via injected `profileFetcher`/`submitter`/`skipCaptcha` props. Parent-authenticated submits stamp `enrollment.requests.guardian_account_id`; the decision service prefers attaching by ID over email matching.

### Key Env Vars

| Var | Purpose |
|-----|---------|
| `TENANT_DOMAIN` | Base domain for subdomain extraction (e.g., `localhost`, `moto-app.de`) |
| `NEXT_PUBLIC_TENANT_DOMAIN` | Client-side tenant domain |
| `NEXT_PUBLIC_OPERATOR_HOSTNAME` | Operator subdomain (e.g., `operator.localhost:3000`) |
| `NEXT_PUBLIC_PARENTS_HOSTNAME` | Parents subdomain (e.g., `parents.localhost:3000`) |
| `FRONTEND_URL` | Backend-side staff/admin URL (used in admin notification emails) |
| `PARENTS_URL` | Backend-side parents-portal URL (used in every parent-facing email link). Required — the server refuses to start without it; must be `https://` in production. |

### Reserved Slugs

Both backend (`models/platform/organization.go`) and frontend (`lib/reserved-slugs.ts`) maintain matching lists of reserved slugs (www, api, operator, parents, grafana, etc.) that cannot be used as tenant subdomains. **These must stay in sync** — nothing enforces it.

## Core Architecture

**Handler → Service → Repository → Database** (always, no exceptions)

- `api/{domain}/` — HTTP handlers (thin, no business logic)
- `services/{domain}/` — Business logic, orchestration, transactions
- `database/repositories/{domain}/` — Data access only (BUN ORM)
- `models/{domain}/` — Domain entities, shared across layers
- Factory pattern for DI: `repositories.NewFactory(db)` → `services.NewFactory(repos, db, logger)`

Layer discipline, repository generics, model conventions, and the CI ratchet tests that enforce them: see `.claude/rules/backend-conventions.md`.

## Critical Patterns

### 0. Frontend: Reuse the UI Kit (MANDATORY)

Build all new UI from `frontend/src/components/ui/`; brand colors come only from `LOCATION_COLORS` in `frontend/src/lib/location-helper.ts` — never generic Tailwind hues. Full component map, hex table, and design checklist: `.claude/rules/frontend-ui-kit.md`.

### 1. BUN ORM: Quote Aliases (MANDATORY)
```go
ModelTableExpr(`education.groups AS "group"`)   // CORRECT — quoted
ModelTableExpr(`education.groups AS group`)     // WRONG — runtime error
```

### 2. Frontend: Zero Warnings Policy
```bash
pnpm run check  # MUST PASS before committing
```

### 3. Type Mapping: int64 → string
Backend `int64` IDs become frontend `string`. Use `data.id.toString()` and `snake_case → camelCase` mapping helpers in `lib/{domain}-helpers.ts`.

### 4. PRs Target `development`
```bash
gh pr create --base development  # NEVER target main unless explicitly asked
```

### 5. Student Location: Use `active.visits`
Real-time student location comes from `active.visits` + `active.attendance`. Scheduled statuses (sick / excused / class trip) live in `active.student_status_days`.

### 6. Next.js 16: Async Params
```typescript
const { id } = await context.params;  // MUST await
```

### 7. Backend Logging: slog Only
Use injected `*slog.Logger` with key-value pairs. Never logrus/log.Printf. GDPR: no student names at Info level.

### 8. Devbox Environment
```bash
devbox search <tool>     # Find packages
devbox add <tool>@latest # Add to devbox.json — never rely on global installs
```

### 9. Migrations and RLS: No Bypass Needed
CLI commands (migrate, seed, cleanup) connect via `DB_DSN` as the `postgres` **superuser**; the HTTP server connects as the least-privilege `phoenix_auth` role (`PHOENIX_AUTH_PASSWORD` required). PostgreSQL superusers always bypass Row Level Security, even with `FORCE ROW LEVEL SECURITY` enabled. This means:
- **Data migrations (UPDATE/INSERT/DELETE) do NOT need to disable RLS** — the superuser connection sees all rows across all tenants automatically
- **Never add `ALTER TABLE ... DISABLE/ENABLE ROW LEVEL SECURITY`** in migration code — it's unnecessary and can cause test failures
- **Migration version numbers must be unique** — `MigrationRegistry` is a `SafeMigrationMap` that panics at init on a duplicate version, so the binary won't start until the collision is fixed

### 10. Time Modeling: Match the Type to the Business Meaning
- **Actual instant** (created_at, checked_in_at): `TIMESTAMPTZ` ↔ `time.Time`, API ISO timestamp
- **Calendar date** (attendance day, birthday): `DATE` ↔ `timezone.Date` — NEVER `time.Time` — API `YYYY-MM-DD`
- **Clock time without date** (template start/end): `TIME WITHOUT TIME ZONE`, normalized via `timezone.WallClock()`, API `HH:MM`

bun binds every `time.Time` as UTC, so DATE columns modeled as `time.Time` land one day behind around Berlin midnight. `TestDateColumnTypes` fails CI for violations. Full guide: `.claude/rules/calendar-dates.md`.

### 11. Shifts vs. Timetable — One Recurrence Engine (#1888/#1889)

A **shift** (`schedule.staff_shifts`) is the outer planned presence of a staff member; a **timetable block** (`activities.groups` template → `schedule.activity_instances`) is a task *within* that presence. Nothing is double-counted and neither side writes into the other (#1873).

Shifts recur via `schedule.staff_shift_series` (weekdays + wall-clock window bound to a calendar period), which **materializes concrete `staff_shifts` rows upfront** — never a read-time projection, so every reader (time tracking, auto-checkout, coverage, weekly summaries) keeps seeing only concrete rows. The series REUSES the shared timetable primitives (`schedule.CalendarPeriod`, `week_pattern` 0/1/2, `ShouldMaterializeWeekPattern`, the cap-`valid_until`/successor/`series_root_id` split shape). **NEVER build a second recurrence engine or a parallel template model.** Deviations: a "Nur diese Woche" edit sets `staff_shifts.detached` (re-plans skip the row), a single-occurrence delete records a `staff_shift_series_exceptions` row; splits re-point both to the successor. Materialization and re-plans never touch rows with `date <= today`.

## Essential Commands

**RULE: Always suggest Docker Compose commands** when advising how to run, build, test, or debug services. Never default to bare `go run` or `pnpm run dev` unless the user explicitly asks for it.

| Task | Command |
|------|---------|
| Start all services | `docker compose up -d` |
| Rebuild backend (go.mod / Dockerfile changes; air hot-reloads plain Go edits) | `docker compose build server && docker compose up -d server` |
| Run migrations | `docker compose run server go run . migrate` |
| Reset DB | `docker compose run server go run . migrate reset` (then seed — see `docs/getting-started.md` for the credential flags) |
| View logs | `docker compose logs -f server` |
| Quality check (frontend) | `cd frontend && pnpm run check` |
| Run backend tests | `cd backend && go test ./...` |
| Generate docs | `docker compose run server go run . gendoc --routes` |

**Seeder is DEV-ONLY**: it creates fake test data and must NEVER run on staging or production. Production infrastructure (system rooms, categories, activities) must be created via data migrations or admin UI — never via the seeder.

**Hermetic tests are MANDATORY** for all new backend tests (no hardcoded IDs, fixtures + cleanup, `TestHermeticTestPatterns` CI gate) — see `backend/CLAUDE.md` for the fixture catalog and rules.

### Test Database (port 5433)
```bash
docker compose --profile test up -d postgres-test       # Start (isolated network)
docker compose --profile test down                       # Stop (plain `down` won't work)
cd backend && APP_ENV=test go run . migrate reset        # Setup
```

## No Fallbacks, No Defaults — Fail Fast (MANDATORY)

**ABSOLUTE RULE: NEVER use fallback defaults (`??`, `||`, `.default()`, `.optional().default()`) for environment variables or configuration values. Missing config MUST crash immediately with a clear error.** Silent fallbacks create invisible production bugs: the app boots, looks healthy, and quietly runs with `operator.localhost:3000` in production.

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

Applies to: all `process.env` reads (proxy, server, client), all Zod schemas in `env.js`, all docker-compose environment blocks (`${VAR}`, never `${VAR:-default}`). **Exception**: only `NODE_ENV` and `LOG_LEVEL` may have defaults. Document correct values in `.env.example`; if a var is required in `env.js`, it also needs `ARG`+`ENV` in `frontend/Dockerfile.prod` and `build-args` in `.github/workflows/build.yml` (the Docker build runs env validation, so CI catches missing args). This policy is enforced in code (`frontend/src/proxy.ts`, `frontend/src/lib/env-validation.js`), not aspirational.

## Environment Management (SOPS)

Deployed environments (staging, production) use **SOPS-encrypted env files** tracked in git. No manual `.env` editing via SSH.

```
1. Developer edits:  sops environments/staging.sops.env   (decrypts → $EDITOR → re-encrypts)
2. Commit + push:    push to development (staging) / main (production)
3. CI decrypts:      sops decrypt → SCP .env + compose + deploy-remote.sh to server
4. Server runs:      deploy-remote.sh (pull → backup DB → migrate → start → healthcheck, rollback on failure)
```

| File | Purpose |
|------|---------|
| `environments/{staging,production}.sops.env` | Encrypted env vars (keys plaintext, values encrypted — CI validates key sync without decrypting) |
| `environments/{staging,production}.compose.yml` | Docker Compose for deployed envs (images from GHCR, pinned to commit SHA) |
| `.sops.yaml` / `scripts/sops-setup.sh` | SOPS config + one-time age key setup |
| `scripts/env-check.sh` | Key-sync validation across all env files (CI `env-sync-check` job blocks PRs on drift; lefthook pre-commit guards staged `.sops.env` changes) |
| `scripts/deploy-remote.sh` | Runs on server: pull, backup, migrate, rollback. Exit codes: `0` ok, `1` aborted pre-migration, `10` rollback succeeded, `11` rollback FAILED (critical) |

Key rules:
1. **Edit only with the SOPS CLI** (`sops environments/staging.sops.env`) — never hand-edit encrypted values. Share the age private key via 1Password/Signal, never Slack/email. CI uses the same key as the `SOPS_AGE_KEY` GitHub secret (plus `STAGING_SSH_*` / `PRODUCTION_SSH_*` deploy secrets; deploy-failure emails go to the `DEPLOY_NOTIFY_EMAILS` repository *variable*).
2. **Both `.sops.env` files must have identical keys**, and `.env.example` must stay in sync (minus the dev-only vars whitelisted in `env-check.sh`: `COMPOSE_BAKE`, `DB_DEBUG`, `TEST_DB_*`, Docker build flags, host-port overrides) — `env-check.sh` enforces this.
3. **Shared `.env` on the server** — all services load the same `.env` via `env_file:`; use the compose `environment:` block for per-service overrides (e.g. `PORT: 3000` for frontend vs backend's `PORT=8080`).
4. Server layout: `~/{staging,production}/` (`.env`, `docker-compose.yml`, `.deploy-state`) + `~/backups/{env}/` (pg_dump retention: 3 staging, 7 production).
5. Deploy triggers: push to `development` → staging, push to `main` → production.

**Adding or changing an env var? Follow the checklists in `.claude/rules/env-docker-sync.md`** (covers local files, docker-compose, SOPS files, and Dockerfile build args).

## URL & Route Conventions

**All URL paths must use kebab-case** — both backend API routes and frontend page routes. Existing snake_case routes (`feedback_history`, `mensa_history`) are legacy; migrate them to kebab-case when touched.

## Git Conventions

**Commit types**: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `style`

**CRITICAL**: Never include "Co-Authored-By: Claude" in commits.

### After Fixing Review Findings: Re-request the Reviewer

`quorum` reviews trigger on the **review request**, not on new code, and it posts
its report as a plain PR comment — so GitHub never clears the request and a bare
`gh pr edit --add-reviewer` is a silent no-op. Once fixes for a quorum review are
pushed, the reviewer must be removed and added again:

```bash
scripts/quorum-rerequest.sh   # remove/add round trip for this branch's PR
```

A `Stop` hook blocks the end of a turn while this is owed. Details:
`.claude/rules/quorum-review-loop.md`.

### PR Screenshots and QA Evidence

Never create GitHub Releases, prereleases, tags, Gists, branches, or commits
only to host screenshots or QA images for a pull request.

For PR screenshots, use GitHub's native attachment upload in the PR
description or PR comment. The resulting URLs should look like
`https://github.com/user-attachments/assets/...`.

If native upload is not available from the current tool context, provide the
local screenshot paths and ask the user to attach them manually. Do not use
releases, prereleases, tags, or Gists as an asset host.

## Database Schemas

`platform` · `auth` · `users` · `education` · `facilities` · `activities` · `active` · `schedule` · `iot` · `feedback` · `config` · `enrollment` · `suggestions` · `meta` · `audit`

## Tenant-Scoped Settings System

**RULE: New per-tenant runtime configuration MUST use the settings system, not environment variables.** Env vars are for infrastructure (DB DSN, JWT secret, SMTP host). If a school admin should be able to configure it, it's a setting. Architecture, the `HasTenantOverride()` env-fallback pattern, and step-by-step guides: `.claude/rules/settings-system.md`.

## In-App Help Guide

**RULE: When you add a user-facing feature flow, or substantially change a flow the guide documents, update `frontend/src/components/help/guide-data.ts` (and changed screenshots) in the SAME PR.** Backend-only, operator/parents-only, and pure-styling changes are exempt. File map, data model, and PDF-render caveat: `.claude/rules/help-guide-sync.md`.

## Agent skills

### Issue tracker

GitHub Issues via `gh` (`moto-nrw/project-phoenix`). See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical Matt roles map 1:1 to tracker labels (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` + `docs/adr/` at the repo root (created lazily by domain-modeling). See `docs/agents/domain.md`.

---

@CLAUDE.local.md
