# Backend Architectural Conventions

**RULE: Every new backend change must respect the conventions below.** They exist to block the per-entity copy-paste patterns that bloated the codebase. If a rule below conflicts with a reviewer's preference, the rule wins — the whole point is to remove the per-PR judgment call.

> **Verification status (2026-06-12):** all structural claims re-verified against the codebase. Two rules are now CI-enforced by ratchet tests (see Rules 1 and 11); the boilerplate counts in Rules 3, 5, and 7 are point-in-time measurements — re-run the detection commands at the bottom before relying on a specific number.

---

## 1. Layer Discipline — Handler → Service → Repository → Database

**RULE: Handlers in `api/` MUST NOT access repositories directly.** Not as struct fields, not via service `.Repository()` getters, not via direct package import.

### Forbidden

```go
// FORBIDDEN — repo as a handler struct field
type Resource struct {
    StaffRepo users.StaffRepository
}

// FORBIDDEN — handler reaching through service to grab the repo
staff, err := rs.UsersService.StaffRepository().FindByPersonID(ctx, personID)

// FORBIDDEN — direct import of repositories from api/ files (except base.go + testutil)
import "github.com/moto-nrw/project-phoenix/database/repositories/..."
```

### Required

```go
// CORRECT — handler depends on a service interface
type Resource struct {
    UsersService users.Service
}

// CORRECT — service exposes a business operation
staff, err := rs.UsersService.GetStaffByPersonID(ctx, personID)
```

### Enforcement — CI ratchet, currently at ZERO violations

`backend/test/handler_layer_ratchet_test.go` (`TestHandlerLayerRatchet`) fails any PR that reintroduces one of five patterns, all with empty allowlists:

1. Repo-typed declarations in `api/`
2. `.XxxRepository()` getter calls (backend-wide except `database/` and `test/`)
3. Query construction (`NewSelect` etc.) in `api/`
4. `database/repositories` imports in `api/` beyond `base.go` + `testutil/`
5. Test-only handler accessor wrappers in `api/`

`scripts/backend-architecture.sh check` evaluates the strict target policy in
`backend/architecture/policy.json` against production, internal-test, and
external-test imports plus the semantic ownership and contract rules. The
normal command requires exact equality with the committed finite baseline in
`backend/architecture/legacy.jsonl`; every tuple names its open migration
issue. Pull requests compare with the policy and baseline at the event's full
base-commit SHA and may only remove tuples. The required CI status is
`Backend architecture ratchet`. Run the network-dependent issue liveness audit
separately with
`scripts/backend-architecture.sh audit-issues --api-url https://api.github.com`.

### Why

Handlers are the request boundary. Putting business logic or data access there scatters knowledge across HTTP-layer files, makes services thin pass-throughs, and prevents reuse from non-HTTP entry points (CLI, scheduler, SSE).

---

## 2. Repository Generics — No Per-Field Query Methods

**RULE: New repository methods MUST extend the generic `Repository[T]` in `database/repositories/base/base.go` instead of declaring `FindByX` clusters, per-field updaters, or per-status finders.**

### What `Repository[T]` provides today (verified)

```go
type Repository[T modelBase.Entity] struct { ... }

Create(ctx, entity T) / FindByID(ctx, id) / Update(ctx, entity) / Delete(ctx, id)
List(ctx, filters map[string]any) / Count(ctx, filters) / CountWithOptions(ctx, opts)
OldestBefore(ctx, ...) / DeleteOlderThan(ctx, ...) / UpdateColumns(ctx, ...)
Transaction(ctx, fn func(tx bun.Tx) error)
```

`models/base/filters.go` provides a richer `Filter` type (`Equal`, `In`, `Like`, `DateRange`, `Or`, `And`, `WithPagination`, `WithSorting`) that compiles to `bun.SelectQuery` clauses.

### Forbidden

```go
// FORBIDDEN — N near-identical methods per repo
func (r *staffRepo) FindByEmail(ctx, email string) (*Staff, error) { ... }
func (r *staffRepo) FindByPersonID(ctx, personID int64) (*Staff, error) { ... }

// FORBIDDEN — per-field updaters / per-status finders
func (r *staffRepo) UpdateName(ctx, id int64, name string) error { ... }
func (r *staffRepo) ListActive(ctx) ([]Staff, error) { ... }
```

### Required

```go
// CORRECT — use existing List(filters) for single-field lookups
staff, err := repo.List(ctx, map[string]any{"email": email})

// CORRECT — embed Repository[T] and add genuinely-needed custom methods
type staffRepo struct {
    *base.Repository[*models.Staff]
}
```

### Exceptions

A custom method is justified only when it performs a complex join/preload the generic shape can't express, or implements a domain operation that isn't reducible to a filter (a hypothetical `ClaimNextUnsupervisedGroup`). Write a doc comment explaining why the generic isn't enough.

---

## 3. Model Base Types — No Boilerplate Getters

**RULE: New model entities MUST embed `base.Model` (and `base.TenantModel` if tenant-scoped) instead of redeclaring `ID`, `CreatedAt`, `UpdatedAt`, `TenantID`, or trivial getters for those fields.**

### What the base types provide (verified)

```go
// models/base/base.go
type Model struct {
    ID        int64     `bun:"id,pk,autoincrement"`
    CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
    UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
func (m *Model) GetID() any / GetCreatedAt() / GetUpdatedAt()  // Rule-3 getters, live here ONCE
// base.StringIDModel provides the same three getters for string-ID entities

// models/base/tenant.go
type TenantModel struct { TenantID int64 `bun:"tenant_id,notnull"` }
func (t *TenantModel) GetTenantID() int64 / SetTenantID(id int64)
```

`base.Model` and `base.StringIDModel` provide `GetID()`/`GetCreatedAt()`/`GetUpdatedAt()` — never redeclare them per entity (shadowing is allowed only for genuinely different semantics, e.g. the audit models mapping `GetUpdatedAt` to `AccessedAt`/`ChangedAt`). The same goes for GORM-style `TableName()` methods: bun never calls them; table names come from struct tags and `ModelTableExpr` strings. Both patterns are CI-ratcheted by `TestModelCeremonyRatchet` (`backend/test/model_ceremony_ratchet_test.go`) with an allowlist of the load-bearing shadow getters.

### A note on `BeforeAppendModel`

The old per-entity `BeforeAppendModel(query any) error` hooks never ran: bun's hook interface is `BeforeAppendModel(ctx context.Context, query schema.Query) error`, dispatched via a reflection `Implements()` check the one-arg signature never matched. All 93 copies were deleted in the 2026-07 audit cleanup; repositories set `ModelTableExpr` explicitly, which is the actual mechanism. If a model genuinely needs a bun append hook, use the correct two-arg signature — the dead one-arg shape is CI-ratcheted to zero by `TestModelCeremonyRatchet` (M3).

---

## 4. Thin Handlers — Cognitive Complexity ≤ 15

**RULE: No new handler may have a `gocognit` score over 15.** Branching, orchestration, and authorization logic belong in services.

```bash
gocognit -over 15 backend/api/
```

**CI-enforced since 2026-07-12** (issue #575 B0): `TestHandlerComplexityRatchet` (`backend/test/handler_complexity_ratchet_test.go`) fails any function under `api/` scoring over 15 that is not in its shrink-only allowlist (seeded with the 75 offenders existing at ratchet time; keys match `gocognit -over 15 api/` output 1:1). When a refactor lowers a score, ratchet the entry down; never raise one or add one.

Two shapes that keep handlers under the threshold without hiding logic:

- **Pure pass-through handlers** (parse → one service call → respond) should not exist as named functions at all — register them with the `api/common` handler builders (`IDAction`, `TwoIDAction`, `IDFetch`, `BindAction`) directly in `Router()`. Cutoff: a handler with per-request branching, multi-service reads, or response shaping keeps a named function; do not grow option-struct builder variants to force-fit those.
- **Error classification** belongs in a declarative rule table (`api/common/error_rules.go`: `ErrorRule` + `RulesRenderer` / `UnwrapRenderer`), not a hand-written switch. `UnwrapRenderer` covers the domain-wrapper pattern (render the inner sentinel, keep the wrapped error for 500 logs). The one sanctioned hand-written renderer is `api/iot/internal/shared.ErrorRenderer` (multi-domain dispatch + PyrePortal wire contract).

---

## 5. No Test-Export Handler Wrappers

**RULE: Do not add `*Handler() http.HandlerFunc { return rs.x }` wrappers to expose private handler methods to tests.** Use the public `Router()` instead — it exercises the same code path the production server uses and is a stronger test.

```go
// FORBIDDEN — pure ceremony to expose a private method
func (rs *Resource) ListStaffHandler() http.HandlerFunc { return rs.listStaff }

// CORRECT — test against the wired router
router := resource.Router()
router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/staff", nil))
```

All 308 such wrappers were deleted in the 2026-07 audit B3 batch; tests drive the resource's public `Router()` with minted JWTs (`api/testutil`: `SeedTestJWTConfig` + `MintTestJWT` + `WithJWTBearer`) or, for internal-package tests, call the private handler directly. `TestHandlerLayerRatchet` (R5) now fails any new niladic method returning `http.HandlerFunc` under `api/` — the shape is CI-ratcheted to zero regardless of method name.

---

## 6. API Design — Filter Parameters, Not Per-State Endpoints

**RULE: A new endpoint that varies only by a filter value MUST use a query parameter, not a separate path.** `GET /students?location=wc&status=active`, not `GET /students/wc` + `GET /students/active`. One handler + one service method + one validated filter struct instead of N copies of each.

---

## 7. Centralized Error Helpers

**RULE: Use `api/common/errors.go` for HTTP error responses.** Do not redeclare `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorInternalServer`, `ErrorForbidden`, or similar helpers in domain-specific `errors.go` files.

```go
import "github.com/moto-nrw/project-phoenix/api/common"
// helpers return a render.Renderer — wrap with RenderError:
common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid student ID")))
```

Consolidated in issue #575 B1/B2 (2026-07-12): the duplicate `ErrResponse` structs and constructor sets in `active`, `feedback`, and `suggestions` were deleted — those packages keep thin `newErrResponse` wrappers only because their wire format carries human-text `status` values (pinned by per-package `wire_format_test.go` goldens; normalizing to `"error"` is a separate frontend-audited change). `api/operator` stays deliberately divergent (`json:"message"`). Do not reintroduce local `Error*` constructor sets — declare classification in an error-rule table (see Rule 4) and let `api/common` build the responses.

---

## 8. Service Methods Encapsulate, Not Delegate

**RULE: A service method that does nothing but call one repository method is NOT a service method.** Either it adds business logic (validation, multi-repo orchestration, audit, tenant-tx wrapping, error mapping), or it shouldn't exist.

---

## 9. Auth Code Location — Audit Before Adding

**RULE: Before adding new authentication or authorization code, search both `backend/auth/` and `services/auth/` (and `services/usercontext/`) — match the existing layering rather than creating a third home.**

`backend/auth/` is NOT legacy. It contains structured low-level utility packages:

- `backend/auth/jwt/` — JWT plumbing (claims incl. MFA challenge/enrollment, tokenauth, tenant/parent middleware)
- `backend/auth/authorize/` — authorization service + permission policies
- `backend/auth/device/` — device API key + PIN authentication
- `backend/auth/userpass/` — password hashing primitives

`services/auth/` holds business-logic services (login flows, invitation flows, MFA orchestration, password reset) that depend on the lower-level `backend/auth/` packages.

- New low-level primitive (hash, parse, verify)? → `backend/auth/{subdomain}/`
- New business flow (login, invite, reset)? → `services/auth/`
- New permission decision? → `services/usercontext/` or `backend/auth/authorize/policy/`
- Handler needs to authorize? → call the service or middleware, never decide inline

---

## 10. Audit, Don't Re-Invent

**RULE: Before writing a new helper, search for existing implementations** (`rg --type go 'func.*formatStudent' backend/`). If a helper already exists in three files with three slightly different signatures, the answer is one shared helper, not a fourth variant.

---

## 11. Services Don't Bypass Repositories

**RULE: Services MUST NOT construct database queries (`NewSelect`, `NewUpdate`, `NewInsert`, `NewDelete`, `NewRaw`) directly on a `bun.DB` / `bun.IDB` / `bun.Tx` handle.** All data access goes through repositories.

### What's allowed

A service may hold `*bun.DB` for two reasons: passing it to repository constructors at wiring time, and starting transactions via the `base.TxHandler` pattern — then calling **repo methods** inside the closure.

```go
// CORRECT — service orchestrates repo calls inside a transaction
err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
    if err := s.visitRepo.End(ctx, visitID); err != nil { return err }
    return s.attendanceRepo.RecordCheckout(ctx, ...)
})
```

### Enforcement — CI ratchet

`backend/test/architecture_ratchet_test.go` (`TestServiceRepositoryRatchet`) fails any new query-construction site in `services/` — including driver-level `Exec`/`Query`/`QueryRow` calls, not just query-builder calls. Its `serviceQueryRatchetAllowlist` is the source of truth for the tolerated remainder: a shrink-only, per-file count covering exclusively transaction-control statements (SAVEPOINT triplets, `pg_advisory_xact_lock`, `LOCK TABLE`) — transaction orchestration is legitimately service-layer. Allowlist numbers may only ever go down.

### Exceptions

Cross-repo / cross-schema cleanup operations that genuinely don't fit a single repo's responsibility may use raw SQL — but the raw SQL must live in `database/repositories/{domain}/cleanup.go`, not inside the service. Document with a comment why a repo method isn't possible.

### Handler-side transactions (`tenant.WithTenantTx` in `api/`)

A `tenant.WithTenantTx` closure in a handler is usually the smell of a missing service method — multi-step writes belong in a service method that the handler's transaction wraps as ONE call (see `UpdateGroupWithDetails`, #575 B10). The exception is a genuine cross-service composition with no natural owner: `createStudent` atomically composes Guardian + Person + Student + Arrival/Pickup-schedule services (the latter live in `services/schedule`, so a `services/users` orchestrator would import-cycle), and `updateStudent`'s locked-row invariants include an in-tx re-authorization against the caller's JWT permissions — HTTP-bound policy that doesn't belong in a service. Those handler-side transactions are sanctioned; new ones need the same written justification.

### Why

The repository layer owns tenant filtering, soft-delete semantics, audit hooks, and BUN-specific quirks. When a service constructs a query directly, every one of those invariants is at risk of being silently dropped — **a forgotten `WHERE tenant_id = ?` becomes a multi-tenant data leak.** Centralizing queries in repos isolates that risk and makes RLS testing tractable.

---

## 12. Models Hold Data, Not Decisions

**RULE: Methods on model entities are limited to (a) field accessors, (b) pure derivations from existing fields, and (c) BUN persistence hooks.** State mutations, business policies, RBAC decisions, and threshold-bearing logic MUST live in services. Magic numbers in model methods MUST move to settings or constants.

### Allowed

```go
// CORRECT — derived field, no policy
func (p *Person) FullName() string { return p.FirstName + " " + p.LastName }

// CORRECT — pure boolean from existing fields, no thresholds
func (t *Token) IsExpired() bool { return !t.ExpiresAt.IsZero() && time.Now().After(t.ExpiresAt) }
```

### Forbidden (and where the old violations went)

The four violations this rule originally named were all extracted in issue #586 — the pattern to follow when you find a new one:

| Was in model | Now lives at |
|---|---|
| `Visit.EndVisit()` | `services/active/` (with logging, events, audit) |
| `Device.IsOnline()` (hardcoded 5 min) | service + `iot.device_online_window_minutes` setting |
| `Group` default duration (hardcoded 30 min) | settings registry |
| `Account.HasPermission()` | `services/usercontext` / `backend/auth/authorize` |

Remaining known hits: `RFIDCard.Activate/Deactivate` (`models/users/rfid_card.go`) — don't copy that pattern.

### Why

Business rules drift constantly. "Offline after 5 minutes" becomes "10 minutes for school A, 2 for school B." **Models hold facts (the timestamp). Services hold rules (what counts as online). Settings hold the parameters.**

---

## 13. Shared Test Doubles — No Per-Package Mock Copies

**RULE: Before hand-rolling a mock, fake, stub, or fixture in a `_test.go` file, check the shared homes.** Full-interface mocks copy-pasted across packages were ~2,000 lines of the 2026-07 audit's test-side findings (slice B14); the shared implementations now exist and new copies are review failures.

| Double | Shared home |
|---|---|
| DB fixtures (`CreateTest*` / `Cleanup*`) | `test/fixtures*.go` (the mandated catalog) |
| Pointer helpers (`StrPtr`, `IntPtr`, `Int64Ptr`, `TimePtr`) | `test/helpers.go` |
| `email.Mailer` capture | `test.CapturingMailer` (`test/mailers.go`) |
| `realtime.Broadcaster` recording fake | `test.RecordingBroadcaster` (`test/broadcaster.go`) |
| Repo mocks for `models/*` interfaces (School, Staff, suggestions) | `test/repo_mocks.go`, `test/suggestions_mocks.go` |
| `config.SettingsService` | `configtest.Mock` (`services/config/configtest`) |
| `auth.MFAService` / `auth.InvitationService` | `services/auth/authtest` |
| `users.PersonService` | `services/users/userstest` |
| API request/bootstrap helpers | `api/testutil` (`SetupAPITest`, `ExecuteWithAuth`, `ExecuteWithAuthPermissions`, `MintTestJWT`) |

Placement rules: mocks for `models/*` interfaces go in `test/` (imports models only — safe for internal test packages); mocks for a service interface go in a leaf `<domain>test` package next to the interface (usable everywhere EXCEPT that package's own internal tests — import cycle). New shared mocks follow the func-field convention (`XxxFn` fields, nil = zero-value default). Behaviorally divergent doubles (error-injection hooks, deliberate panics, channel-based capture) may stay package-local — divergence is the documented exception, copy-paste is not.

---

## 14. Calendar Fixtures Must Not Depend on the Wall Clock

**RULE: Calendar-date, date-range, weekday, and ISO-week expectations in backend tests use fixed Berlin dates or instants.** Do not derive them from `time.Now()` or `timezone.TodayDate()`: a test that is green at noon can cross midnight or Sunday/Monday in CI. Prefer `timezone.NewDate(...).BerlinMidnight()` for a Berlin instant, `timezone.NewDate(...)` for a calendar date, or `time.Date(...)` for an explicit instant.

`backend/test/calendar_fixture_ratchet_test.go` (`TestCalendarFixtureClockRatchet`) parses imports and Go syntax in `_test.go` files. It follows imported aliases, assigned values, and package-local test helpers across files. The ratchet rejects live-clock values used in date conversion, date-range calls or structs, date assertions, day/ISO-week operations, and weekly-summary fixture times. Comments, strings, shadowed import names, unrelated `Now` methods, fixed values, and non-test files are ignored. The check is part of the existing CI command:

```bash
cd backend && go test ./test -run Ratchet -count=1
```

If a test's purpose genuinely requires the system clock, inject a clock where possible. Otherwise add only its exact `path/to/file_test.go:TestFunction` key to `calendarFixtureClockExceptions`, with a specific non-empty reason explaining why the live clock is load-bearing. Every unexcepted finding fails immediately; there is no grandfathered count or fingerprint baseline. Stale exception keys fail the ratchet and must be removed.

---

## Code Review Checklist

- [ ] No repository imports/fields/getter-calls in `api/` (CI: `TestHandlerLayerRatchet`)
- [ ] New repo methods extend `Repository[T]` — no `FindByX` clusters, no per-field updaters
- [ ] New entities embed `base.Model` (+ `base.TenantModel` if tenant-scoped) — no redeclared fields or trivial getters
- [ ] New handler functions score `gocognit ≤ 15` (manual — not in CI)
- [ ] No `*Handler() http.HandlerFunc` test-export wrappers — tests use `Router()`
- [ ] State variants are filter params, not separate endpoints
- [ ] Errors use `api/common` helpers, not local copies
- [ ] Services encapsulate business logic, don't just delegate
- [ ] Auth code matches the `backend/auth/` (primitives) vs `services/auth/` (flows) layering
- [ ] No query construction in `services/` (CI: `TestServiceRepositoryRatchet`)
- [ ] Models hold data, not decisions — no `Mark*/End*/Activate*` mutations, no RBAC, no magic thresholds
- [ ] Searched for existing helpers before writing a new one (`rg` before `func`)
- [ ] No new hand-rolled mock/fixture where a shared test double exists (Rule 13 table)
- [ ] Calendar/date/week test fixtures use fixed Berlin values, or have an exact-function live-clock exception with a reviewed reason

## Detection commands (one-shot health check)

```bash
cd backend

# Rules 1 + 11 — the authoritative checks are the ratchet tests:
go test ./test/ -run 'TestHandlerLayerRatchet|TestServiceRepositoryRatchet' -v

# Rule 1 — manual sweep (should be zero)
rg --type go -g '!*_test.go' '^\s+\w+\s+[\w\.]*Repository\b|\.\w+Repository\(\)' api/

# Rule 3 — boilerplate getters (should not grow)
rg --type go -c 'func \([^)]+\) GetID\(\)' models/ | awk -F: '{s+=$2} END {print s}'

# Rule 4 — cognitive complexity over 15 in handlers
gocognit -over 15 api/

# Rule 5 — test-export wrappers
rg -U '^func \([^)]+\) \w+Handler\(\) http\.HandlerFunc \{[^}]*return [^}]+\.\w+\s*\}' api/

# Rule 7 — duplicate error helpers outside api/common
rg --type go 'func ErrorInvalidRequest|func ErrorNotFound|func ErrorForbidden' api/ -l | grep -v '/common/'

# Rule 11 — manual sweep (should be zero; the allowlisted tx-control uses ExecContext, which this regex does not match)
rg --type go -g '!*_test.go' '\.(NewSelect|NewUpdate|NewInsert|NewDelete|NewRaw)\(' services/

# Rule 12 — state-mutation / RBAC / magic-threshold methods in models
rg --type go -g '!*_test.go' '^func \([^)]+\) (Mark|End|Close|Reset|Activate|Deactivate|Cancel|Approve|Reject|Suspend|Restore|Archive)\w*\(' models/
rg --type go -g '!*_test.go' '^func \([^)]+\) (HasPermission|IsAdmin|IsOperator|CanAccess|CanEdit|CanDelete)\w*\(' models/
rg --type go -g '!*_test.go' '\d+\s*\*\s*time\.(Hour|Minute|Second)' models/
```

If any of these return non-trivial output for new code, the PR violates a rule above.

## When to deviate

Only with explicit reviewer approval recorded in the PR description, citing the specific rule and the reason. "It's faster" and "the existing code does it this way" are not reasons.
