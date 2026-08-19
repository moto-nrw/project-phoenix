# Backend — Agent Context

Go backend for Project Phoenix (Chi router, BUN ORM, PostgreSQL multi-schema). Read the root `CLAUDE.md` first — it owns the stack versions, multi-tenancy model, commands table, and cross-repo contracts. This file covers backend-specific knowledge only.

## Development Commands

Day-to-day run/build/migrate commands are Docker-Compose-first — see the root `CLAUDE.md` Essential Commands table. Backend-specific:

```bash
# Testing — self-initializing lifecycle (ADR 0004): SetupTestDB starts the
# postgres-test container if needed, rebuilds the phoenix_test template when
# migrations changed, and gives each package binary a run-stamped clone.
../scripts/test-backend.sh          # Full suite via gotestsum + immediate clone sweep (preferred full run)
go test ./...                       # All tests (works standalone; clones are GC'd by the next run)
PHX_TEST_LEFTOVERS=1 ../scripts/test-backend.sh   # Opt-in: report rows tests left behind (diagnosis, not a gate)
go run ./internal/testdb/cmd/sweep  # Drop this/dead runs' phx_test_pkg_* clones manually
go test -short ./...                # Fast inner loop: skips every DB integration test (SetupTestDB t.Skip). NEVER in CI — guts coverage.
go test ./services/active/... -v    # Specific package
go test -race ./...                 # Race detection
go test ./api/auth -run TestLogin   # Specific test

# Code Quality (run before committing!)
golangci-lint run --timeout 10m
go fmt ./... && goimports -w . && go mod tidy

# CLI (run inside the container via `docker compose run server go run . <cmd>`)
go run . migrate status|validate|reset
go run . seed --email <op-email> --password <pw> --pin 1234   # flags required; seeds via the HTTP API, server must be running
go run . cleanup preview|stats      # visit-retention dry-run / statistics
go run . cleanup visits             # REAL deletion — there is no `cleanup visits preview`; extra args are silently ignored
go run . cleanup timetable|time-tracking [preview|stats]      # nested dry-runs exist only for these two
go run . cleanup tokens|invitations|rate-limits|attendance|sessions|supervisors
go run . gendoc                     # Generates routes.md + docs/openapi.yaml
```

## Database Configuration

DSN resolution is **fail-fast** (`database/database_config.go`) — there are no localhost fallbacks:

1. `APP_ENV=test` — requires `TEST_DB_DSN` and never falls through to `DB_DSN` (test DB on port 5433)
2. Every other environment — requires `DB_DSN`; CLI commands connect as the `postgres` **superuser** (the seeder is API-based and opens no DB connection itself)
3. Missing config exits with an error

The HTTP server (`serve`) connects as the least-privilege **`phoenix_auth`** role instead (NOINHERIT; can `SET ROLE` to `phoenix_tenant`/`phoenix_admin` per request). `PHOENIX_AUTH_PASSWORD` is mandatory — the server refuses to start without it. This split is what makes RLS enforcement real: request queries run under the tenant role, never as superuser.

## Architecture

**Handler → Service → Repository → Database** — the root `CLAUDE.md` states the rule; `.claude/rules/backend-conventions.md` is the enforcement-level reference (layer discipline, `Repository[T]` generics, model conventions, and the CI ratchet tests `TestHandlerLayerRatchet` + `TestServiceRepositoryRatchet` that fail PRs on violations).

## Critical BUN ORM Patterns

### Schema-Qualified Tables (MUST USE QUOTES!)
```go
// CORRECT - Quotes around alias prevent "column not found" errors
ModelTableExpr(`users.teachers AS "teacher"`)

// WRONG - Missing quotes causes BUN mapping failures
ModelTableExpr(`users.teachers AS teacher`)
```

### Loading Nested Relationships
```go
// For Teacher → Staff → Person relationships
type teacherResult struct {
    Teacher *users.Teacher `bun:"teacher"`
    Staff   *users.Staff   `bun:"staff"`
    Person  *users.Person  `bun:"person"`
}

err := r.db.NewSelect().
    Model(result).
    ModelTableExpr(`users.teachers AS "teacher"`).
    // Explicit column mapping required for each table
    ColumnExpr(`"teacher".id AS "teacher__id"`).
    ColumnExpr(`"staff".id AS "staff__id"`).
    ColumnExpr(`"person".* AS "person__*"`).
    Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
    Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
    Where(`"teacher".id = ?`, id).
    Scan(ctx)
```

### Transactions and Filters
Transactions propagate via context (`base.ContextWithTx` / `base.TxFromContext`); repositories pick them up through `base.GetDB(ctx, db)`. For query filters and the generic repository API (`Repository[T]`, `base.Filter` with `Equal`/`ILike`/`In`/pagination), see `.claude/rules/backend-conventions.md` Rule 2 — don't invent per-field finder methods.

### Soft Delete
`users.Person`, `users.Staff`, and `users.Teacher` carry `deleted_at` with bun's `soft_delete` tag: normal queries auto-filter soft-deleted rows. Staff deletion runs an offboarding service (not a bare delete). Keep this in mind when counting rows or writing raw SQL against these tables.

## Calendar Dates: timezone.Date (MANDATORY)

Every model field mapped to a `DATE` column MUST be `timezone.Date` (or `*timezone.Date`), never `time.Time` — bun binds `time.Time` as UTC and Berlin-midnight dates land one day behind. `TestDateColumnTypes` fails CI on violations. Full API and rules: `.claude/rules/calendar-dates.md`.

## Tenant-Scoped Settings

Per-school config resolves tenant DB override → registry default; the service does **not** check env vars. Consumers needing env var backward compatibility must check `HasTenantOverride()` first, then fall back to `os.Getenv()` manually. Use `Resolve*(ctx, key)` inside tenant middleware, `Resolve*ForTenant(ctx, tenantID, key)` outside it (device auth, scheduler loops). Everything else — registry, field types, permissions, add/edit/delete workflows: `.claude/rules/settings-system.md`.

## Request-Scoped Identity Memoization (#2099)

The identity chain (Account → Person → Staff → Teacher → `GetMyGroups`/`GetSubstitutedGroupIDs`) is memoized per request, mirroring the #2065 settings cache: `RequestIdentityCacheMiddleware` (mounted router-wide in `api/base.go` and group-wide in `ProtectedTenantGroup`) attaches an empty cache; `services/usercontext` consults it keyed by `(tenant_id, account_id)`. Only successes and clean not-found outcomes are memoized — never DB errors or partial `GetMyGroups` results. Self-writes (`UpdateCurrentProfile`, `UpdateAvatar`) drop the caller's entry before their trailing re-read; writes to *other* accounts' chains (group transfer, admin substitutions, offboarding) are deliberately exempt. Without the cache in context (scheduler, CLI, device auth, plain tests) behavior is unchanged. Full contract: doc comment in `services/usercontext/identity_request_cache.go`.

## Domain Knowledge

### RFID/IoT Integration
- Two-layer auth: Device API key (`Authorization: Bearer`) + Staff PIN (`X-Staff-PIN`); devices authenticate without tenant JWTs but are scoped to one school (hence `Resolve*ForTenant` in device auth)
- The `X-Staff-PIN` header is checked against the per-tenant `security.ogs_device_pin` setting via constant-time compare; optional kiosk attribution requires `X-Staff-ID` plus an `X-Staff-Auth-PIN` verified against that account's Argon2id-hashed PIN (`X-Staff-ID` alone is ignored, and binary attendance remains attributed to the authenticated device)
- Check-in/out tracked in `active.visits`; scheduled statuses (sick/excused/class trip) in `active.student_status_days`
- **Error strings returned by `/api/iot/*` are a cross-repo contract** — PyrePortal maps them to German UI text (see root `CLAUDE.md` Ecosystem)

### GDPR/Privacy Patterns
- Student data visibility is permission-scoped: admins and verified staff see full data for every child of the tenant (#2329 removed the per-group scope); guest/guardian accounts stay redacted. The wire keeps separate `has_full_access` (read) vs `has_write_access` (write) flags
- Per-student retention: `DataRetentionDays int` (notnull) — 1-31 days, default 30 via the `DefaultDataRetentionDays` const (`models/users/privacy_consent.go`)
- Automated cleanup is scheduled per tenant via the `gdpr.data_cleanup_*` settings; manual dry-run: `go run . cleanup preview|stats` (see Development Commands for the exact CLI shapes — they differ per domain)
- All deletions logged in `audit.data_deletions`
- **Logging: no student names at Info level or above** (IDs only; names at Debug) — CI-enforced by `TestGDPRLogPIIRatchet` (`test/gdpr_log_pii_ratchet_test.go`): no log call at Info+ may read `FirstName`/`LastName`/`GreetingMsg`

### Guardian Parent-Portal Permissions

Parent portal guardian permissions are relationship-scoped, not normal tenant
account permissions. Staff/admin authorization still uses `auth.roles`,
`auth.permissions`, JWT permissions, and `authorize.RequiresPermission`, but
parents app access for a child must be decided from the matching
`users.students_guardians` row.

Never authorize parent portal child visibility or writes only from
`auth.account_tenants`, `guardian_profiles.account_id`, or the existence of a
guardian link. Those facts prove membership/relationship only; they do not prove
permission. Operational fields such as `can_pickup`, `is_emergency_contact`,
`relationship_type`, and `is_primary` may inform defaults but must not replace
explicit `parent_portal.*` permission checks.

Detailed rule and implementation guidance:
`.claude/rules/guardian-parent-permissions.md`.

## Migration System

One file per migration, named with the **zero-padded numeric version prefix** — `001015124_my_feature.go` for version `1.15.124` (the collision scanner in `00_migrations.go` only recognizes `000`/`001`-prefixed filenames; never use the dotted version in the filename):

```go
const (
    myFeatureVersion     = "1.15.124"
    myFeatureDescription = "What this migration does"
)

func init() {
    MigrationRegistry.Register(&Migration{
        Version:     myFeatureVersion,
        Description: myFeatureDescription,
        DependsOn:   []string{"1.15.119"},
    })
    Migrations.MustRegister(upFunc, downFunc)
}
```

`MigrationRegistry` is a `SafeMigrationMap` — duplicate versions **panic at init**, so the binary won't start on a collision. `go run main.go migrate validate` checks the dependency graph in-memory. RLS never needs disabling in migrations (superuser connection — see root `CLAUDE.md` Critical Pattern 9).

## Testing — Hermetic Pattern (MANDATORY)

All backend tests use real database fixtures, never hardcoded IDs. The CI gate `TestHermeticTestPatterns` (`backend/test/hermetic_verification_test.go`) fails on `int64(1)`-style IDs; mock-based test files must be added to its `skipPatterns` allowlist.

```go
import testpkg "github.com/moto-nrw/project-phoenix/test"

func TestExample(t *testing.T) {
    db := testpkg.SetupTestDB(t)   // shared package pool against the package clone — NEVER close it

    // ARRANGE: real fixtures — reference returned IDs, never literals.
    // No cleanup calls: the package clone is dropped after the run (#2419).
    student := testpkg.CreateTestStudent(t, db, "First", "Last", "1a")
    staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Name")

    // ACT + ASSERT
    result, err := service.DoSomething(ctx, student.ID)
    require.NoError(t, err)
}
```

- **One pool per package (#2419)**: `SetupTestDB` returns the same `*bun.DB` for every test in the binary. Never `db.Close()` it (gate: `no_shared_pool_close`). Tests that close their DB on purpose to force error paths use `testpkg.SetupClosableTestDB(t)`.
- **No `Cleanup*` calls in new tests**: the clone-per-package lifecycle owns cleanup. Per-package counts are ratcheted shrink-only (`cleanupCallBaseline`); the leftover-tolerant packages are already at zero.
- **New tests create their own tenant** via `testpkg.NewTenantScope(t, db)` instead of `TenantContext(1)` — the bootstrap tenant is being phased out (ratchet: `tenantContext1Baseline`, which counts `TenantContext(1)`, `WithTenantID(ctx, 1)` and `TenantID: 1` alike).
- **Parallel + bootstrap tenant is the combination to avoid.** Tests sharing tenant 1 may run in parallel only while every assertion is scoped to IDs the test created; the moment one asserts something tenant-wide (a count, a "list all"), it becomes order-dependent. The files where both already meet are frozen by the `parallel_on_bootstrap_tenant_ratchet` gate — do not add a new one, give the test its own tenant.
- The fixture catalog lives in `test/fixtures.go` (`CreateTest*` helpers, including `*ForTenant` variants for multi-tenant tests and auth chains like `CreateTestTeacherWithAccount`). Search it before writing a new fixture.
- Tests hitting the DB go in external test packages (`package active_test`); pure model tests stay internal.
- Run the gate locally before pushing: `cd backend && go test ./test/ -run TestHermeticTestPatterns -v`
- Never modify existing tests to make new code pass — see `.claude/rules/no-test-modifications.md`.

## Logging: slog Only (MANDATORY)

All backend code uses `log/slog` via injected loggers — never logrus or `log.Printf`. `sloglint` enforces key-value style (`no-mixed-args`, `key-naming-case: snake`, `args-on-sep-lines`). Use the `backend-structured-logging` skill for detailed patterns.

```go
s.logger.Info("visit recorded", "student_id", sid, "group_id", gid)  // snake_case keys
```

- Loggers flow through the factory: `services.NewFactory(repos, db, logger)`; services scope with `logger.With("service", "active")`
- Structs that tests construct bare use the nil-safe pattern: `getLogger()` returning `slog.Default()` when nil
- **GDPR: student names never at Info level** — IDs only; names at Debug
- Known exceptions (intentional `log.Printf`): `auth/jwt/tokenauth.go` startup logging; `cmd/` and `simulator/` route through slog default at WARN

## Real-Time Updates (SSE)

- **Hub**: `backend/realtime/` (dependency-neutral package, `*slog.Logger` with nil-safe `getLogger()`). Single instance wired in `services.Factory`, injected into the active service (broadcasting) and the SSE API resource (connections).
- **Endpoint**: `/api/sse/events` — JWT-authenticated, auto-subscribes the client to the active groups they supervise, 30s heartbeat.
- **Broadcasting**: services fire events after data changes via `realtime.NewEvent(...)` + `BroadcastToGroup` — fire-and-forget, broadcast errors are logged and never block the operation. Per-client buffers are small and lossy (events drop when a client's channel is full), which is why clients refetch instead of trusting delivery. Broadcast points live in `services/active/` (visits, sessions, attendance).
- **Event types**: authoritative list in `realtime/events.go` (student check-in/out, activity lifecycle, instance lifecycle, dashboard counts, supervision/arrival-schedule/settings changes). Frontend types mirror it in `frontend/src/lib/sse-types.ts` — keep both in sync.
- Events are notification triggers, not payloads — clients refetch via bulk endpoints.

## Email

SMTP config via `EMAIL_SMTP_*`, `EMAIL_FROM_*`, `FRONTEND_URL`/`PARENTS_URL` (link bases). With SMTP unset, the factory falls back to `email.NewMockMailer()` which logs metadata (to/subject/template) instead of sending — local dev needs no SMTP. HTML templates live in `backend/templates/email/` (shared chrome: `styles.html`, `header.html`, `footer.html`; feature templates for invitations, password reset, MFA codes, enrollment notifications, operator flows). Email sends are async fire-and-forget; failures are logged, never block API responses. Password hashing/strength helpers: `services/auth/password_helpers.go` — reuse, don't duplicate.

**Password-reset rate limit is a cross-layer contract**: 3 requests/hour per email; the backend's `429` + `Retry-After` header drives the live countdown in the frontend's password-reset modal (localStorage-persisted). Changing the window or header silently breaks that UX.

## Environment Variables

Local dev config lives in `dev.env` (template: `dev.env.example`); Docker maps vars via the compose `environment:` block. **Gotcha**: code using `os.Getenv()` directly (migrations, scheduler, CORS) sees only the compose block, not `dev.env` — see `.claude/rules/env-docker-sync.md`. Useful dev flags: `DB_DEBUG=true` (SQL logging), `LOG_LEVEL=debug`. Per-tenant runtime behavior belongs in the settings system, not env vars.
