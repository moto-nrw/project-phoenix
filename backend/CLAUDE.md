# Backend — Agent Context

Go backend for Project Phoenix (Chi router, BUN ORM, PostgreSQL multi-schema). Read `docs/agents/contracts.md` before tenant-scoping, auth, enrollment, or IoT changes. File references start at the repo root unless a backend-relative path is clear from context.

## Commands and verification

Services and cleanup commands: [operations](../docs/agents/operations.md).
Focused tests run from `backend/` through `../scripts/run-go-toolchain.sh`;
[test commands and fixture lifecycle](../docs/agents/backend-testing.md) cover
full runs, clones, and diagnosis. Add `-count=1` for time-dependent failures.
Before committing backend code, run relevant tests and
`../scripts/run-go-toolchain.sh golangci-lint run --timeout 10m`.
Format changed Go files with the repo toolchain; run `go mod tidy` only when
dependencies change. Architecture and before-push checks are below.

## Database safety

HTTP requests use least-privilege roles and tenant-scoped transactions;
CLI/migrations use a superuser connection. Missing DSNs fail fast.
For role setup, BUN queries, soft deletion, or migrations, read
[backend persistence and migrations](../docs/agents/backend-data.md).

## Architecture

### Active Architecture Migration (#2580) — TEMPORARY

Until #2580's exit criteria are met, the capability-first target in
`backend/architecture/policy.json` takes precedence over legacy examples.
The Handler → Service → Repository layout is migration state, not the target.

Before backend design, implementation, or review:

1. Read `backend/architecture/README.md` and inspect affected owners, packages,
   data objects, projections, and rules in `backend/architecture/policy.json`.
   For a new or moved boundary, also read issue #2580.
2. Put new behavior behind an existing target owner's public capability.
   Dependencies use consumer-owned ports; cross-module writes use application
   workflows; cross-module reads use named tenant-safe projections.
3. Treat `repositories.Factory`, `services.Factory`, `api.API`, scheduler
   setters, `SetupAPITest`, and broad legacy composition as shrink-only.
   Do not add fields, setters, callers, or wrappers that still build that graph.
4. Assign new writable data objects to existing target owners in the same diff.
   A new owner requires an architecture decision linked from #2580 first.
5. Keep temporary dependencies out of target-allowed rules. Remove each legacy
   edge in the cutover or track that exact edge in an open #2580 subissue.

Before completing a backend code change:

1. Run `scripts/backend-architecture.sh check` from the root. Inspect policy,
   baseline, and composition-inventory diffs; green only proves detected rules.
2. Run `scripts/test-changed.sh origin/development` without `--fast` before push.
3. Record owner/capability and before/after counts for touched shrink-only
   surfaces in the PR. Counts must stay equal or decrease.

If a feature cannot fit these boundaries, stop its structural change and
create or link an urgent #2580 subissue for explicit review. Remove this
section only after the migration's exit criteria are met.

### Legacy layer safety

Read `.claude/rules/backend-conventions.md` for unmigrated code's layer rules,
repository generics, model conventions, and CI ratchets. These prevent new
legacy damage; they do not define the target module boundary.

**Query budgets (#2940)**: every list endpoint has a scenario in `test/query_budgets.go` and a test that counts its statements with `testpkg.CaptureQueries` and calls `testpkg.AssertQueryBudget`. The register is shrink-only; `TestQueryBudgetRatchet` rejects unreferenced entries and hand-rolled `BeforeQuery` hooks in tests. Rule 15 in `.claude/rules/backend-conventions.md` has the pattern.

## Calendar Dates: timezone.Date (MANDATORY)

Every model field mapped to a `DATE` column MUST be `timezone.Date` (or `*timezone.Date`), never `time.Time` — bun binds `time.Time` as UTC and Berlin-midnight dates land one day behind. `TestDateColumnTypes` fails CI on violations. Full API and rules: `.claude/rules/calendar-dates.md`.

Clock values mapped to PostgreSQL `TIME` must pass through
`timezone.NormalizeWallClock()`. `TestActivityInstanceWallClockRatchet` blocks raw
current-time values in `ActivityInstance.StartTime`/`EndTime`; do not extend an
allowlist around it.

## Tenant-Scoped Settings

Per-school config resolves tenant DB override → registry default only. Consumers
must not append env fallbacks, including legacy compatibility chains. Use
`Resolve*(ctx, key)` inside tenant middleware and
`Resolve*ForTenant(ctx, tenantID, key)` outside it (device auth, scheduler loops).
Handle resolution errors; missing service wiring is a configuration error.
Registry, permissions, and workflows: `.claude/rules/settings-system.md`.

## Request-Scoped Identity Memoization (#2099)

The identity chain (Account → Person → Staff → Teacher → `GetMyGroups`/`GetSubstitutedGroupIDs`) is memoized per request, mirroring the #2065 settings cache: `RequestIdentityCacheMiddleware` (mounted router-wide in `api/base.go` and group-wide in `ProtectedTenantGroup`) attaches an empty cache; `services/usercontext` consults it keyed by `(tenant_id, account_id)`. Only successes and clean not-found outcomes are memoized — never DB errors or partial `GetMyGroups` results. Self-writes (`UpdateCurrentProfile`, `UpdateAvatar`) drop the caller's entry before their trailing re-read; writes to *other* accounts' chains (group transfer, admin substitutions, offboarding) are deliberately exempt. Without the cache in context (scheduler, CLI, device auth, plain tests) behavior is unchanged. Full contract: doc comment in `services/usercontext/identity_request_cache.go`.

## Domain Knowledge

### Shifts vs. timetable — one recurrence engine

A shift (`schedule.staff_shifts`) is planned staff presence; a timetable block
(`activities.groups` template → `schedule.activity_instances`) is a task within
it. Neither writes into the other or double-counts time (#1873).

`staff_shift_series` materializes concrete shifts upfront, never at read time.
Reuse the timetable's `schedule.CalendarPeriod`, `week_pattern` 0/1/2,
`ShouldMaterializeWeekPattern`, and cap-`valid_until` / successor / `series_root_id`
split shape; do not create a second recurrence engine (#1888/#1889).
A single-week edit sets `staff_shifts.detached` and is skipped by replans;
a single-occurrence delete records `staff_shift_series_exceptions`. Splits
repoint both to the successor. Materialization/replans never touch dates
`<= today`.

### RFID/IoT Integration
- Two-layer auth: Device API key (`Authorization: Bearer`) + Staff PIN (`X-Staff-PIN`); devices authenticate without tenant JWTs but are scoped to one school (hence `Resolve*ForTenant` in device auth)
- The `X-Staff-PIN` header is checked against the per-tenant `security.ogs_device_pin` setting via constant-time compare; optional kiosk attribution requires `X-Staff-ID` plus an `X-Staff-Auth-PIN` verified against that account's Argon2id-hashed PIN (`X-Staff-ID` alone is ignored, and binary attendance remains attributed to the authenticated device)
- Real-time location comes from `active.visits` + `active.attendance`; scheduled statuses (sick/excused/class trip) in `active.student_status_days`
- **Error strings returned by `/api/iot/*` are a cross-repo contract** — PyrePortal maps them to German UI text (see `docs/agents/contracts.md` Ecosystem and IoT)

### GDPR/Privacy Patterns
- Student data visibility is permission-scoped: admins and verified staff see full data for every child of the tenant (#2329 removed the per-group scope); guest/guardian accounts stay redacted. The wire keeps separate `has_full_access` (read) vs `has_write_access` (write) flags
- Per-student retention: `DataRetentionDays int` (notnull) — 1-31 days, default 30 via the `DefaultDataRetentionDays` const (`models/users/privacy_consent.go`)
- Automated cleanup is scheduled per tenant via the `gdpr.data_cleanup_*` settings; manual dry-run: `go run . cleanup preview|stats` (see Development Commands for the exact CLI shapes — they differ per domain)
- All deletions logged in `audit.data_deletions`
- **Logging: no student names at Info level or above** (IDs only; names at Debug) — CI-enforced by `TestGDPRLogPIIRatchet` (`test/gdpr_log_pii_ratchet_test.go`): no log call at Info+ may read `FirstName`/`LastName`/`GreetingMsg`

### Guardian Parent-Portal Permissions

Membership does not authorize child access. Check the matching relationship's
explicit `parent_portal.*` permissions. Read
`.claude/rules/guardian-parent-permissions.md` before parent-child access changes.

## Testing

Before adding, changing, or diagnosing tests, read
[backend test fixtures and lifecycle](../docs/agents/backend-testing.md).
Use real fixture IDs, per-test tenants, `t.Parallel()`, and package-owned pools.
Preserve assertion strength and report failed checks accurately.

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

## Real-time updates

Events are lossy notification triggers, not payloads. Read
[the SSE contract](../docs/agents/realtime.md) before changing producers,
event types, streaming endpoints, or client refetch behavior.

## Email

SMTP config via `EMAIL_SMTP_*`, `EMAIL_FROM_*`, `FRONTEND_URL`/`PARENTS_URL` (link bases). With SMTP unset, local development uses `email.NewMockMailer()` to log metadata (to/subject/template); staging and production fail startup. HTML templates live in `backend/templates/email/` (shared chrome: `styles.html`, `header.html`, `footer.html`; feature templates for invitations, password reset, MFA codes, enrollment notifications, operator flows). Most email sends use async `Dispatcher.Dispatch`; fail-closed sends such as MFA challenges use synchronous `Dispatcher.Deliver` and return success only after transport acceptance. Password hashing/strength helpers: `services/auth/password_helpers.go` — reuse, don't duplicate.

**Password-reset rate limit is a cross-layer contract**: 3 requests/hour per email; the backend's `429` + `Retry-After` header drives the live countdown in the frontend's password-reset modal (localStorage-persisted). Changing the window or header silently breaks that UX.

## Environment Variables

Local dev config lives in `dev.env` (template: `dev.env.example`); Docker maps vars via the compose `environment:` block. **Gotcha**: code using `os.Getenv()` directly (migrations, scheduler, CORS) sees only the compose block, not `dev.env` — see `.claude/rules/env-docker-sync.md`. Useful dev flags: `DB_DEBUG=true` (SQL logging), `LOG_LEVEL=debug`. Per-tenant runtime behavior belongs in the settings system, not env vars.
