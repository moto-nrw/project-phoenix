# Backend — Agent Context

Go backend for Project Phoenix (Chi router, BUN ORM, PostgreSQL multi-schema). Read the root `CLAUDE.md` first — it owns the stack versions, multi-tenancy model, commands table, and cross-repo contracts. This file covers backend-specific knowledge only.

## Development Commands

Day-to-day run/build/migrate commands are Docker-Compose-first — see the root `CLAUDE.md` Essential Commands table. Backend-specific:

```bash
# Testing — self-initializing lifecycle (ADR 0004): SetupTestDB starts the
# postgres-test container if needed, builds the template for this branch's
# migrations hash (phoenix_test_<hash>, so parallel worktrees never share
# one), and gives each package binary a run-stamped clone.
../scripts/run-go-toolchain.sh ../scripts/test-backend.sh  # Full suite via gotestsum + immediate clone sweep (preferred full run)
../scripts/run-go-toolchain.sh go test ./...                # All tests (works standalone; each binary drops its own clone at exit)
PHX_TEST_LEFTOVERS=1 ../scripts/run-go-toolchain.sh go test -v ./services/active  # Also print tolerated leftovers
PHX_TEST_LEFTOVERS=test ../scripts/run-go-toolchain.sh go test -parallel 1 ./services/active  # Name the leaking test
PHX_TEST_KEEP_CLONE=1 ../scripts/run-go-toolchain.sh go test ./services/active  # Keep the clone for a post-mortem
../scripts/run-go-toolchain.sh go run ./internal/testdb/cmd/sweep  # Drop this/dead runs' clones manually
../scripts/run-go-toolchain.sh go test -short ./...             # Fast inner loop; NEVER in CI — guts coverage.
../scripts/run-go-toolchain.sh go test ./services/active/... -v # Specific package
../scripts/run-go-toolchain.sh go test -race ./...              # Race detection
../scripts/run-go-toolchain.sh go test ./api/auth -run TestLogin # Specific test

# Code Quality (run before committing!)
../scripts/run-go-toolchain.sh golangci-lint run --timeout 10m
../scripts/run-go-toolchain.sh go tool goimports -w .
../scripts/run-go-toolchain.sh go mod tidy

# CLI (run inside the container via `docker compose run server go run . <cmd>`)
go run . migrate status|validate|reset
go run . seed --email <op-email> --password <pw> --pin 1234   # flags required; seeds via the HTTP API, server must be running
go run . cleanup preview|stats      # visit-retention dry-run / statistics
go run . cleanup visits             # REAL deletion — there is no `cleanup visits preview`; extra args are silently ignored
go run . cleanup timetable|time-tracking [preview|stats]      # nested dry-runs exist only for these two
go run . cleanup tokens|invitations|rate-limits|attendance|sessions|supervisors
go run . gendoc                     # Generates routes.md + docs/openapi.yaml
```

The Go result cache can replay an earlier green test after the calendar day
changes. When diagnosing date- or clock-dependent behavior, add `-count=1` to
the focused `go test` command before trusting the result.

## Database Configuration

DSN resolution is **fail-fast** (`database/database_config.go`) — there are no localhost fallbacks:

1. `APP_ENV=test` — requires `TEST_DB_DSN` and never falls through to `DB_DSN` (test DB on port 5433)
2. Every other environment — requires `DB_DSN`; CLI commands connect as the `postgres` **superuser** (the seeder is API-based and opens no DB connection itself)
3. Missing config exits with an error

The HTTP server (`serve`) connects as the least-privilege **`phoenix_auth`** role instead (NOINHERIT; can `SET ROLE` to `phoenix_tenant`/`phoenix_admin` per request). `PHOENIX_AUTH_PASSWORD` is mandatory — the server refuses to start without it. This split is what makes RLS enforcement real: request queries run under the tenant role, never as superuser.

## Architecture

### Active Architecture Migration (#2580) — TEMPORARY

This section applies to every change under `backend/` until #2580 is closed. It
overrides legacy architecture examples elsewhere in the agent documentation.
The old Handler → Service → Repository layout remains safety guidance for
unmigrated code; it is not the target design.

Before writing or reviewing backend code:

1. Read `architecture/README.md` and inspect the affected owners, packages,
   data objects, projections, and rules in `architecture/policy.json`. For a new
   or moved boundary, also read GitHub issue #2580.
2. Put new behavior behind an existing target owner's public capability. Use a
   consumer-owned port for a dependency, an application workflow for a
   cross-module write, and a named tenant-safe projection for a cross-module
   read.
3. Treat `repositories.Factory`, `services.Factory`, `api.API`, scheduler
   setters, `SetupAPITest`, and legacy composition as **shrink-only surfaces**.
   A change may remove or redirect an existing use. It must not add a field,
   setter, caller, or a narrow wrapper that still constructs the broad graph.
4. Assign every new writable database object to an existing target owner in
   the same change. A new domain owner requires an explicit architecture
   decision linked from #2580 before implementation.
5. Keep migration-only dependencies out of target-allowed policy rules. Remove
   the temporary edge in the same cutover, or track the exact legacy edge with
   an open #2580 subissue.

Before declaring the change complete:

1. Run `scripts/backend-architecture.sh check` from the repository root and
   inspect every policy, baseline, and composition-inventory diff. A green
   check proves only what the current evaluator detects; it is not proof that
   the target architecture was preserved.
2. Run `scripts/test-changed.sh origin/development` without `--fast` before the
   push.
3. Record the affected owner/capability and before/after counts for every
   touched shrink-only surface in the PR description. Each count must stay
   equal or decrease.

If a requested feature cannot satisfy these rules, stop the structural change
and create or link an urgent subissue of #2580 for explicit review. Remove this
temporary section only after #2580's exit criteria are met.

### Legacy Layer Safety

`.claude/rules/backend-conventions.md` is the enforcement-level reference for
unmigrated code (layer discipline, `Repository[T]` generics, model conventions,
and the CI ratchet tests `TestHandlerLayerRatchet` +
`TestServiceRepositoryRatchet`). These checks prevent additional damage inside
the legacy layout; they do not define the target module boundary.

**Query budgets (#2940)**: every list endpoint has a scenario in `test/query_budgets.go` and a test that counts its statements with `testpkg.CaptureQueries` and calls `testpkg.AssertQueryBudget`. The register is shrink-only; `TestQueryBudgetRatchet` rejects unreferenced entries and hand-rolled `BeforeQuery` hooks in tests. Rule 15 in `.claude/rules/backend-conventions.md` has the pattern.

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
Open tenant transactions through `tenant.TransactionRunner` (or the composition root's `tenant.UnitOfWork`). The runtime propagates the active transaction through context; repositories pick it up through `base.GetDB(ctx, db)`. For query filters and the generic repository API (`Repository[T]`, `base.Filter` with `Equal`/`ILike`/`In`/pagination), see `.claude/rules/backend-conventions.md` Rule 2 — don't invent per-field finder methods.

### Soft Delete
`users.Person`, `users.Staff`, and `users.Teacher` carry `deleted_at` with bun's `soft_delete` tag: normal queries auto-filter soft-deleted rows. Staff deletion runs an offboarding service (not a bare delete). Keep this in mind when counting rows or writing raw SQL against these tables.

## Calendar Dates: timezone.Date (MANDATORY)

Every model field mapped to a `DATE` column MUST be `timezone.Date` (or `*timezone.Date`), never `time.Time` — bun binds `time.Time` as UTC and Berlin-midnight dates land one day behind. `TestDateColumnTypes` fails CI on violations. Full API and rules: `.claude/rules/calendar-dates.md`.

Clock values mapped to PostgreSQL `TIME` must pass through
`timezone.NormalizeWallClock()`. `TestActivityInstanceWallClockRatchet` blocks raw
current-time values in `ActivityInstance.StartTime`/`EndTime`; do not extend an
allowlist around it.

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
- **No explicit `Cleanup*` calls**: `cleanupCallBaseline` is empty and the
  AST-based gate rejects fixture cleanup through any import alias. The package
  clone owns tenant rows. Tenantless fixture builders register their lifecycle
  internally; schema-migration tests use `testpkg.OwnTenantRows`; subtests that
  need isolated state use `testpkg.OwnTenant` / `testpkg.OwnCtx`. If a missing
  row is the test arrangement, use a production delete operation or reserve an
  unused sequence ID through a fixture helper.
- **Every test owns its tenant (#2419)**: a package opts in once, from `TestMain`, with `testpkg.PerTestTenants()`. From then on each top-level test gets its own tenant, every `CreateTest*` fixture it creates lands there, and JWT claims minted through `api/testutil` follow it — so no fixture call and no claims helper needs a tenant argument. Inside a test, `testpkg.Ctx(t)` is the context (the replacement for `TenantContext(1)`) and `testpkg.Tenant(t)` the ID. Subtests share their parent's tenant — which is right when the parent builds the fixtures they read, and wrong for a table of subtests that each create the same kind of row and then assert something tenant-wide about it. Those call `testpkg.OwnTenant(t)` / `testpkg.OwnCtx(t)` as their first line and get a tenant of their own. One edge to know: the rebase happens when claims are *used* (`MintTestJWT`, `WithClaims`), so reading `claims.TenantID` straight off the struct still yields the bootstrap value — inside a test, take the tenant from `testpkg.Tenant(t)`, never from the claims you just built. Two gates hold the line: `db_packages_opt_into_per_test_tenants` fails any package that opens the test database without opting in, and `bootstrap_tenant_ratchet` counts every remaining spelling (`TenantContext(1)`, `WithTenantID(ctx, 1)`, `TenantID: 1`, `SetTenantID(1)`, `…ForTenant(…, 1, …)`, and literal `tenant_id` filters in raw SQL) per package, shrink-only.
- **Every top-level test is parallel (#2851)**: start it with `t.Parallel()`.
  The `tests_run_in_parallel` gate has an empty baseline and rejects every
  exception. Inject process configuration and output; use per-test database
  clones for schema changes, sweeps, query measurements, and lock tests.
- **Concurrency is pinned, not inherited**: `scripts/test-backend.sh` runs
  `-p 10 -parallel 8` (local postgres-test has `max_connections=300`);
  post-merge CI runs `-p 6 -parallel 8`, changed-only PRs `-p 4 -parallel 8`
  (CI's service container keeps the stock 100 connections). `-parallel` stays
  at 8 everywhere on purpose: `-test.parallel` is part of the Go test cache
  key and sizes the per-binary pool.
  The pool per binary is derived from `-test.parallel` plus
  headroom, because a test holding a tenant transaction that opens a second one
  needs two connections at once — without headroom those tests deadlock and
  every one of them fails on its own 5s deadline, which looks nothing like a
  pool problem.
- **Leftovers are a gate, not a report (#2419)**: every test binary compares its
  clone against the start state it recorded for itself and fails the PACKAGE
  when rows are left in SHARED state — rows outside the tenants its own tests
  created. Rows in a test's own tenant are not leftovers. The gate runs from
  `TestMain` via `testpkg.Run(m)` (gate: `db_packages_run_the_leftover_gate`),
  so `../scripts/run-go-toolchain.sh go test ./...` is gated exactly like a
  full wrapper run; it costs the package one query at exit (~30-70ms measured).
  `PHX_TEST_LEFTOVERS=1 ../scripts/run-go-toolchain.sh go test -v` also prints
  the pairs `testdb.LeftoverAllowlist` still tolerates;
  `PHX_TEST_LEFTOVERS=test ../scripts/run-go-toolchain.sh go test -parallel 1 ./pkg` checks after every test
  and names the culprit instead of the package.
- **Parallel + bootstrap tenant is the combination to avoid.** Tests sharing tenant 1 may run in parallel only while every assertion is scoped to IDs the test created; the moment one asserts something tenant-wide (a count, a "list all"), it becomes order-dependent. The remaining files where both meet are frozen by the `parallel_on_bootstrap_tenant_ratchet` gate — do not add a new one, opt the package into per-test tenants instead.
- The fixture catalog lives in `test/fixtures.go` (`CreateTest*` helpers, including `*ForTenant` variants for multi-tenant tests and auth chains like `CreateTestTeacherWithAccount`). Search it before writing a new fixture.
- Tests hitting the DB go in external test packages (`package active_test`); pure model tests stay internal.
- Run the gate locally before pushing: `cd backend && ../scripts/run-go-toolchain.sh go test ./test/ -run TestHermeticTestPatterns -v`
- Never modify existing tests to make new code pass — see `.claude/rules/no-test-modifications.md`.

## Seed Coverage (MANDATORY)

The demo seeder (`seed/api`) plus `simulate full-day` are the widest end-to-end
path in this repo, roughly a hundred real API calls across every module, and
the only source of dev and demo data. Two rules follow from that:

1. **A user-facing feature ships with seed data in the same PR.** A screen that
   is empty on every dev machine is a screen nobody reviews.
2. **The coverage ratchet is not optional.** `TestSeedCoverageRatchet`
   (`seed/api/coverage_ratchet_test.go`) fails when a table ends up with no
   seeded rows and is not allowlisted, and just as loudly when an allowlisted
   table starts holding data (then delete the entry). The `seed-smoke` CI job
   runs it, and by running the seeder at all it also proves every API contract
   the seeder drives still holds.

Allowlist entries reading `GAP: prod has N rows` are the measured backlog: 58
tables that production tenants fill and the seeder does not. Shrink that list,
never grow it.

```bash
# Against a seeded local stack
docker compose exec server sh -c 'SEED_COVERAGE_DSN="$DB_DSN" go test ./seed/api/ -run TestSeedCoverageRatchet -v'
```

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

SMTP config via `EMAIL_SMTP_*`, `EMAIL_FROM_*`, `FRONTEND_URL`/`PARENTS_URL` (link bases). With SMTP unset, local development uses `email.NewMockMailer()` to log metadata (to/subject/template); staging and production fail startup. HTML templates live in `backend/templates/email/` (shared chrome: `styles.html`, `header.html`, `footer.html`; feature templates for invitations, password reset, MFA codes, enrollment notifications, operator flows). Most email sends use async `Dispatcher.Dispatch`; fail-closed sends such as MFA challenges use synchronous `Dispatcher.Deliver` and return success only after transport acceptance. Password hashing/strength helpers: `services/auth/password_helpers.go` — reuse, don't duplicate.

**Password-reset rate limit is a cross-layer contract**: 3 requests/hour per email; the backend's `429` + `Retry-After` header drives the live countdown in the frontend's password-reset modal (localStorage-persisted). Changing the window or header silently breaks that UX.

## Environment Variables

Local dev config lives in `dev.env` (template: `dev.env.example`); Docker maps vars via the compose `environment:` block. **Gotcha**: code using `os.Getenv()` directly (migrations, scheduler, CORS) sees only the compose block, not `dev.env` — see `.claude/rules/env-docker-sync.md`. Useful dev flags: `DB_DEBUG=true` (SQL logging), `LOG_LEVEL=debug`. Per-tenant runtime behavior belongs in the settings system, not env vars.
