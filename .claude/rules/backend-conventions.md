---
paths:
  - "backend/**"
---

# Backend Architectural Conventions

**Scope: code inside an owner.** Read the target architecture in
`backend/CLAUDE.md` first. The capability-first policy in
`backend/architecture/policy.json` takes precedence when choosing boundaries;
where an example below shows the old Handler → Service → Repository layout,
the target wins.

Current paths and enforcement were checked against `development` @ `8aca1925f7`
(2026-10-23). Historical seed counts below are not current budgets. ADR 0083
keeps the gates until tested successors cover their invariants; green shape
counts alone do not establish correct command effects or runtime protocols.

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

`backend/test/handler_layer_ratchet_test.go` (`TestHandlerLayerRatchet`) fails any PR that reintroduces one of three patterns, all with empty allowlists:

1. Repo-typed declarations in `api/`, outside the root's test composition hooks (`api/*_test_helpers.go`, #4005), which build the retained repositories for tests and serve no request
2. Query construction (`NewSelect` etc.) in `api/`
3. Test-only handler accessor wrappers in `api/`

The `.XxxRepository()` getters and the production `database/repositories` packages no longer exist, so neither shape compiles and the ratchet no longer scans for them (#4427).

All three patterns also scan the handlers #2580 moved out of `api/` (`modules/*/http`, `modules/*/inbound`, `workflows/*/http`, nested layouts included) with registers of their own, all empty when #4071 added them; no file there is exempt.

`scripts/backend-architecture.sh check` evaluates the strict target policy in
`backend/architecture/policy.json` against production, internal-test, and
external-test imports plus the semantic ownership and contract rules. The
normal command requires exact equality with the empty baseline in
`backend/architecture/legacy.jsonl`. Pull requests compare with the policy and
baseline at the event's full base-commit SHA; no tuple may be added. Branch
protection requires `ci-gate`, which includes the `backend-architecture-ratchet`
job (`Backend architecture ratchet`) when the workflow selects it. See
`.github/workflows/main.yml` for selection and skipped-job handling. Run the network-dependent issue liveness audit
separately with
`scripts/backend-architecture.sh audit-issues --api-url https://api.github.com`.

### Why

Handlers are the request boundary. Putting business logic or data access there scatters knowledge across HTTP-layer files, makes services thin pass-throughs, and prevents reuse from non-HTTP entry points (CLI, scheduler, SSE).

---

## 2. Owner-Private Persistence — Reuse Existing Queries

**RULE: Extend the target owner's existing persistence adapter and its
consumer-owned port; reuse its typed filters and query helpers before adding
another query method.** Inspect that owner's entries in
`backend/architecture/policy.json` before choosing the package.

The former shared generic repository and filter DSL were removed by #2580.
There is no shared CRUD base to extend. Owner adapters now live under their
owner's `internal` tree; a method expressing a required domain operation or
lookup is valid. Avoid near-identical per-field/per-status methods when an
existing typed filter already expresses the operation. Preserve tenant scope,
RLS, locking and error semantics; do not expose an adapter or BUN query shape
through a public capability merely to reuse code across owners.

Cross-owner callers use public capabilities bound to their own narrow ports,
not foreign persistence implementations (ADR 0067).

---

## 3. Owner-Local Row Types — No Boilerplate Getters

**RULE: Reuse the owner's existing row identity/timestamp/tenant shape where
it fits; preserve BUN column tags and tenant attribution.** There is no shared
model base to import. For examples, inspect
`backend/modules/peopledirectory/internal/rows/model.go` and
`backend/modules/auditlog/records/model.go`; neither is a cross-owner base.

BUN maps fields, not accessor methods. Do not add trivial identity/timestamp
getters or GORM-style `TableName()` methods for CRUD: table names come from
struct tags and `ModelTableExpr`. Keep meaningful audit timestamps
(`AccessedAt`, `DeletedAt`, `OccurredAt`, `ChangedAt`) rather than inventing
conventional created/updated fields. Tenant accessors still needed by an
owner's transaction protocol stay on its existing tenant shape.

If a row needs a BUN append hook, use the actual interface:
`BeforeAppendModel(ctx context.Context, query schema.Query) error`.
The old one-argument hooks were never dispatched and must not be copied.

---

## 4. Thin Handlers — Cognitive Complexity ≤ 15

**RULE: No new handler may have a `gocognit` score over 15.** Branching, orchestration, and authorization logic belong in services.

```bash
scripts/run-go-toolchain.sh go -C backend test -tags=testhooks ./test -run '^(TestHandlerComplexityRatchet|TestModuleComplexityRatchet)$' -count=1
```

**CI-enforced since 2026-07-12** (issue #575 B0): `TestHandlerComplexityRatchet` (`backend/test/handler_complexity_ratchet_test.go`) fails any function under `api/` scoring over 15 that is not in its shrink-only allowlist; the root's test composition hooks (`api/*_test_helpers.go`, #4005) are no handlers and stay outside the scan (seeded with the 75 offenders existing at ratchet time; keys match `gocognit -over 15 api/` output 1:1). When a refactor lowers a score, ratchet the entry down; never raise one or add one. The handlers under `modules/*/http`, `modules/*/inbound` and `workflows/*/http` are held to the same threshold by `TestModuleComplexityRatchet`, which scans all of `modules/` and `workflows/`.

Two shapes that keep handlers under the threshold without hiding logic:

- **Pure pass-through handlers** (parse → one service call → respond) should not exist as named functions at all — register them with the `api/common` handler builders (`IDAction`, `TwoIDAction`, `IDFetch`, `BindAction`) directly in `Router()`. Cutoff: a handler with per-request branching, multi-service reads, or response shaping keeps a named function; do not grow option-struct builder variants to force-fit those.
- **Error classification** belongs in a declarative rule table (`api/common/error_rules.go`: `ErrorRule` + `RulesRenderer` / `UnwrapRenderer`), not a hand-written switch. `UnwrapRenderer` covers the domain-wrapper pattern (render the inner sentinel, keep the wrapped error for 500 logs). IoT session routes now classify the public `modules/devicescan` failure contract in `modules/devicescan/http/sessions/runtime.go`; device authentication keeps its envelope in `modules/devicefleet/internal/adapters/deviceauth/errors.go`. Preserve their PyrePortal mappings rather than recreating the removed multi-domain rule table.

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

Every error response has one form, the RFC 9457 envelope of ADR 0006 (#2507, #2520): `type`, `title`, `status` (the HTTP status number), `detail` (developer diagnostics), `instance`, and the registered `code`, with `errors[]` and `details{}` as extensions; `error` and `message` members do not exist. `ProblemResponseMiddleware` normalizes whatever a handler writes and logs each repair as "error body outside the shared envelope" with its route; `checkErrorEnvelopeOnEveryRoute` (`TestFullProductionRouterGolden`) walks every route and requires a registered code, and `routetest.AssertErrorResponse` rejects a handler body without the status number, detail and registered code or with an `error` or `message` member. `TestErrorWritesUseTheSharedEnvelope` (`api/common/error_envelope_scan_test.go`) fails on `http.Error`/`http.NotFound` and on an error body type that neither embeds `common.ErrResponse` nor declares the envelope's members; a package that may not import `api/common` writes its own envelope struct with a registered code constant. The operator's `Operator*` constructors in `api/common/operator_errors.go` build the shared `ErrResponse` (#3232). Do not reintroduce local `Error*` constructor sets or a status text of your own — declare classification in an error-rule table (see Rule 4) and let `api/common` build the responses.

---

## 8. Service Methods Encapsulate, Not Delegate

**RULE: A service method that does nothing but call one repository method is NOT a service method.** Either it adds business logic (validation, multi-repo orchestration, audit, tenant-tx wrapping, error mapping), or it shouldn't exist.

`TestModulePassthroughRatchet` counts the shape, not the verdict. [ADR 0075](../../docs/adr/0075-pass-throughs-keep-a-per-owner-floor.md) records the shapes that may stay (transaction and observation wrapper, ADR 0067 consumer-port adapter, the #4480 section 5 Timetable adapters, scanner limitation 5, and since its amendment the owner-edge translation that validates or maps onto the public contract), the four named exceptions, and the floor per owner. Every budget equals its floor; a pure forward or a binding without translation is no floor shape and goes.

---

## 9. Auth Code Location — Audit Before Adding

**RULE: Search the current owners before adding authentication or
authorization code.** Match their public contracts and private implementations:

- Password/hash/verify primitives and permission decisions:
  `backend/modules/securityruntime/`, implemented in
  `backend/modules/securityruntime/internal/application/authorize/` and
  `backend/modules/securityruntime/internal/domain/userpass/`.
  Consumers use public capabilities or root-bound ports, never those internals.
- Staff permission constants/catalogue:
  `backend/auth/authorize/permissions/`. This is the remaining auth utility
  package, not the home of authorization services or device middleware.
- Device API key/PIN middleware:
  `backend/modules/devicefleet/deviceauth/`, with its private implementation in
  `backend/modules/devicefleet/internal/adapters/deviceauth/`.
- Login, invitation, MFA, reset and operator identity flows:
  `backend/modules/identityaccess/`. Caller identity and reach are resolved by
  `identityaccess.CallerContext`; consumers declare narrow ports bound at the root.
- JWT claims, tokenauth and portal/tenant middleware:
  `backend/modules/identityaccess/sessiontoken/` (ADR 0054). Keep it independent
  of process configuration, tenant runtime and persistence.

Handlers call the appropriate capability or middleware, not inline policies.

---

## 10. Audit, Don't Re-Invent

**RULE: Before writing a new helper, search for existing implementations** (`rg --type go 'func.*formatStudent' backend/`). If a helper already exists in three files with three slightly different signatures, the answer is one shared helper, not a fourth variant.

---

## 11. Services Don't Bypass Repositories

**RULE: Services MUST NOT construct database queries (`NewSelect`, `NewUpdate`, `NewInsert`, `NewDelete`, `NewRaw`) directly on a `bun.DB` / `bun.IDB` / `bun.Tx` handle.** All data access goes through repositories.

### What's allowed

Owner compositions may pass database handles into their adapters. Application
code participates through the owner's existing transaction port, then calls
persistence ports inside that unit of work. The example below illustrates the
shape; it is not a shared transaction-base API.

```go
// CORRECT — service orchestrates repo calls inside a transaction
err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
    if err := s.visitRepo.End(ctx, visitID); err != nil { return err }
    return s.attendanceRepo.RecordCheckout(ctx, ...)
})
```

### Enforcement — architecture policy

`scripts/backend-architecture.sh check` enforces this through `backend/architecture/policy.json`: a package may import the `orm-sql` class (`github.com/uptrace/bun`, `database/sql`) only when a production rule grants it. The `postgres`, `migration` and `test-support` roles have one, `compose`, `adapter` and `application` packages only where a named owner rule grants it, and the `http` role has none. Without the import, a package cannot build a query. The former text ratchet `TestServiceRepositoryRatchet` scanned only `modules/schedulerruntime/internal/application`, which has no grant, and was removed in #4427.

### Exceptions

Cross-owner cleanup follows ADR 0067: participating owner commands keep SQL in
their own policy-authorized persistence adapters; coordinating protocols belong
in a workflow. A cross-schema operation is not permission to bypass ownership.
Document why a custom persistence operation is needed.

### Handler-side transactions (`tenant.WithTenantTx` in `api/`)

A handler transaction is not a general exception to owner/workflow boundaries.
Keep retained HTTP child creation and in-lock update authorization within
ADR 0067's named E3/E4 protocols and the
[cross-owner write inventory](../../docs/architecture/4538-cross-owner-write-inventory.md).
A new coordinator belongs in a workflow; a participating leaf command must
meet the ADR's scope, transaction, authorization and lock-order conditions.
Do not infer a new exception from the location of an existing handler.

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
| `Visit.EndVisit()` | `modules/studentpresence/internal/application/presence/` (with logging, events, audit) |
| `Device.IsOnline()` (hardcoded 5 min) | service + `iot.device_online_window_minutes` setting |
| `Group` default duration (hardcoded 30 min) | settings registry |
| `Account.HasPermission()` | `modules/identityaccess` caller context / `modules/securityruntime` |

### Why

Business rules drift constantly. "Offline after 5 minutes" becomes "10 minutes for school A, 2 for school B." **Models hold facts (the timestamp). Services hold rules (what counts as online). Settings hold the parameters.**

---

## 13. Shared Test Doubles — No Per-Package Mock Copies

**RULE: Before hand-rolling a mock, fake, stub, or fixture in a `_test.go` file, check the shared homes.** Full-interface mocks copy-pasted across packages were ~2,000 lines of the 2026-07 audit's test-side findings (slice B14); the shared implementations now exist and new copies are review failures.

| Double | Shared home |
|---|---|
| DB fixtures (`CreateTest*` / `Cleanup*`) | `test/fixtures*.go` (the mandated catalog) |
| Pointer helpers (`StrPtr`, `IntPtr`, `Int64Ptr`, `TimePtr`) | `test/helpers.go` |
| `email.Mailer` capture | `emailtest.CapturingMailer` (`modules/delivery/emailtest`; alias `test.CapturingMailer`) |
| `realtime.Broadcaster` recording fake | `realtimetest.RecordingBroadcaster` (`realtime/realtimetest`; alias `test.RecordingBroadcaster`) |
| Delivery mail-outbox capture | `api.CapturingOutbox` (`api/capturing_outbox_test_helpers.go`, build tag `testhooks`) |
| Audit `Command` that appends one event type into a trail | `test.NewAuditCommand(trail.Create)` |
| Owner-neutral repository-shaped doubles | `test/repo_mocks.go`; owner-specific doubles stay with their owner or root test hooks |
| Consumer-owned settings ports (`ResolveBool`, `ResolveString`, …) | a package-local func-field `settingsDouble` with only the resolvers the ports need (#3965) |
| `identityaccess.AccountMFA` | a package-local func-field double; the behaviour suites keep theirs in `modules/identityaccess/behavior` |
| Person-directory lookups of the acting account's staff identity | `api.TimetableTestStaffAccountPeople` (`api/timetable_operation_people_test_helpers.go`) |
| API request helpers | `api/testutil` and `api/testutil/routetest` (`ExecuteWithAuth`, `ExecuteWithAuthPermissions`); `MintTestJWT` in `test` |

Placement follows the architecture policy, not a global model layer. Owner-specific doubles belong in an allowed owner test-support package or the root's test hooks (`api/*_test_helpers.go`, #4005); owner-neutral helpers may stay in `test/`. Mocks for a public interface may live in a leaf `<domain>test` package beside it (not usable by that package's own internal tests because of the import cycle). New shared mocks follow the func-field convention (`XxxFn` fields, nil = zero-value default). Behaviorally divergent doubles (error-injection hooks, deliberate panics, channel-based capture) may stay package-local — divergence is the documented exception, copy-paste is not. Settings doubles are the second exception: each consumer declares its own narrow settings port, so its `settingsDouble` implements exactly that port; a shared double for the settings service's full method set would pull the Settings Platform's schema types into test support (#3965).

---

## 14. Calendar Fixtures Must Not Depend on the Wall Clock

**RULE: Calendar-date, date-range, weekday, and ISO-week expectations in backend tests use fixed Berlin dates or instants.** Do not derive them from `time.Now()` or `calendar.TodayDate()`: a test that is green at noon can cross midnight or Sunday/Monday in CI. Prefer `calendar.NewDate(...).BerlinMidnight()` for a Berlin instant, `calendar.NewDate(...)` for a calendar date, or `time.Date(...)` for an explicit instant.

`backend/test/calendar_fixture_ratchet_test.go` (`TestCalendarFixtureClockRatchet`) parses imports and Go syntax in `_test.go` files. It follows imported aliases, assigned values, and package-local test helpers across files. The ratchet rejects live-clock values used in date conversion, date-range calls or structs, date assertions, day/ISO-week operations, and weekly-summary fixture times. Comments, strings, shadowed import names, unrelated `Now` methods, fixed values, and non-test files are ignored. The check is part of the existing CI command:

```bash
cd backend && ../scripts/run-go-toolchain.sh go test -tags=testhooks ./test -run Ratchet -count=1
```

If a test's purpose genuinely requires the system clock, inject a clock where possible. Otherwise add only its exact `path/to/file_test.go:TestFunction` key to `calendarFixtureClockExceptions`, with a specific non-empty reason explaining why the live clock is load-bearing. Every unexcepted finding fails immediately; there is no grandfathered count or fingerprint baseline. Stale exception keys fail the ratchet and must be removed.

---

## 15. Query Budgets — Every List Endpoint Has One (#2940)

**RULE: A new list endpoint (or list-shaped service method) ships with a query-budget test and a register entry in `backend/test/query-budgets/`.** N+1 loops show up at three fixtures already, so the test is cheap: create 3 rows, run the request, create more, run it again, assert the statement count did not move, then hand the statements to the register.

```go
counter := testpkg.CaptureQueries(t, db)          // bun.QueryHook, disabled at cleanup
rr := exec(...)                                    // the request under test
testpkg.AssertQueryBudget(t, "api.students.list", counter.Queries())
```

- **One counter.** `testpkg.QueryCounter` (`backend/test/query_counter.go`) is the only bun hook tests may define: `CaptureQueries(t, db)` attaches it to a DB, `NewQueryCounter()` feeds `db.WithQueryHook` for a private clone. It buckets by operation (`Operation("SELECT")`), table (`Selects("config.setting_values")`) or predicate (`Matching`). Tests that count own their database (`SetupIsolatedTestDB`) or a `WithQueryHook` clone before they run in parallel.
- **The register is the budget.** `queryBudgets` maps a scenario name to a statement count: `max` entries are ceilings, `exact` entries pin a dedup contract (one bulk load, one settings snapshot). Shrink-only: lower a number when a fix removes statements, never raise one. A scenario that needs more statements is an N+1 until proven otherwise; batch-load by ID set (`modules/timetable/internal/application/exception_conflicts.go` is the reference shape).
- **Two CI halves.** `TestQueryBudgetRatchet` (source-level, runs in the no-database ratchet step) fails on register entries no test references, scenario names no entry defines, and any `_test.go` file defining its own `BeforeQuery` hook. The budget tests themselves run with the full backend suite.

---

## 16. Backend Source and Owner Ratchets

**RULE: File size, function length and complexity are measured across the whole backend. Facade width, pass-throughs and legacy LOC keep their specific owner scopes below.** Migration #2580 moved 228,306 of 638,717 production LOC into those trees, where no gate in this document reached them: a file could pass every threshold above by being moved rather than by being written.

Six ratchets freeze the state measured on **2026-09-18 at merge commit `ecf0003369`**, and #4071 extended three of them on 2026-10-15 (development `c49b81c3a9`) to where code went instead of where it left. The first seed was taken at `19feca2822`; five of them were re-measured after the merge of `origin/development` (`a47f77b2c3`), which brought in PR #3408 (issue #3350) and with it the new legacy subtree `modules/careplan/legacy/carelifecycle`. They repair nothing today. **Every entry may only fall — lower a number when a refactor earns it, delete it when the offender is gone; never raise one and never add one.** Each test file's header records its exact measurement command and the semantics of its four failure cases. `scripts/backend-architecture.sh check` enforces this against the pull request's base for every ratchet file in `backend/test/`: a raised integer constant, a raised `map[string]int` value or sum, a grown `map[string]string` exception register, and a measurement that disappears or can no longer be evaluated fail unless the change adds a new-numbered ADR the entry cites. Rename a register only with `renamed from <old name>` in the new declaration's comment (ADR 0057 point 5; `backend/architecture/README.md`, "Source ratchet budgets").

| Rule | Threshold | Test — `backend/test/…` | Seed on 2026-09-18 |
|---|---|---|---|
| File size | 800 lines per file | `TestModuleFileSizeRatchet` — `module_file_size_ratchet_test.go` | 55 files of 1,655; largest 3,196 lines |
| Function length | 60 body lines per named function | `TestModuleFunctionLengthRatchet` — `module_function_length_ratchet_test.go` | 299 functions of 14,623 |
| Cognitive complexity (Rule 4, extended) | `gocognit` ≤ 15 | `TestModuleComplexityRatchet` — `module_complexity_ratchet_test.go` | 444 functions |
| Facade width | 12 methods per exported interface in a module's public package | `TestModuleFacadeWidthRatchet` — `module_facade_width_ratchet_test.go` | 36 interfaces of 421; widest 33 |
| Pass-through methods (Rule 8) | budget per module directory over its application layer and its compositions (`*/compose/**`, `*/composition/**`) together (ADR 0061), with a total | `TestModulePassthroughRatchet` — `module_passthrough_ratchet_test.go` | 704 methods in 21 modules; compositions 2,394 in 34 (2026-10-15); merged per module 3,109 (ADR 0061) |
| Legacy LOC budget | per topmost `legacy/` tree, per `legacy_*.go` file outside one, per owner composition package, each with a total | `TestModuleLegacyBudgetRatchet` — `module_legacy_budget_ratchet_test.go` | 12 trees, 92,928 LOC; on 2026-10-15 22 `legacy_*.go` files, 3,862 LOC, and 61 compose packages, 90,627 LOC (#4084) |
| Root structs of several owners (ADR 0059) | no key the base lacks | composition surface in `scripts/backend-architecture.sh check` — `internal/architecture/composition_ratchet.go` | 139 → 1,732 keys on 2026-10-15 when #4071 counted every `api`, `cmd` and `.` struct holding values of two owners; 0 since #4080 |

The seed column is history. Current budgets are the constants in each test
file. #3423 is closed; zero is not the target for every register. ADR 0075
records the pass-through floors. ADR 0083 defines tested successor cuts for
shape accounting and compose LOC; keep the existing gates until then.

Scope: file size, named function length and cognitive complexity scan **all
of `backend/` recursively** (ADRs 0082, 0087), independently of build tags and
platform. Only `_test.go` and hidden directories are excluded. Migrations,
tools, hooks, legacy code, test helpers, `testdata` and generated sources stay
measured; moving code never silences these gates. Generated error-code output
is split into bounded chunks, not exempted; see the
[ADR 0087 addendum for #4746](../../docs/adr/0087-every-backend-source-tree-stays-in-the-ratchets.md#addendum-2026-10-23-bounded-error-code-generation-4746).
Facade width keeps its public-module-interface scope. Pass-throughs scan the
owners' application layers and compositions, without `legacy/`, plus the exact
moved paths in their register. The `legacy_*.go` register scans the whole
backend outside `legacy/` trees. All budgets remain shrink-only; the measured
root pass-through classification adds no allowance.

Two former rows are gone (#4427). The `bun` import in `*/http` and `*/inbound` is an `orm-sql` import the `http` role has no rule for, so `scripts/backend-architecture.sh check` rejects it. Temporary allow-rules in `policy.json` (a transition marker or an `issue` field) are rejected by the evaluator when it loads the policy (`validateTargetOnlyRule`, `internal/architecture/target_rule.go`, ADR 0057).

---

## Code Review Checklist

- [ ] No repository imports/fields/getter-calls in `api/` (CI: `TestHandlerLayerRatchet`)
- [ ] Persistence extends the owner's adapter/port and reuses existing typed filters; no duplicate query clusters
- [ ] Rows reuse fitting owner-local shapes and preserve BUN/tenant columns; no unnecessary getters
- [ ] New handler functions score `gocognit ≤ 15` (CI: `TestHandlerComplexityRatchet` and `TestModuleComplexityRatchet`)
- [ ] No `*Handler() http.HandlerFunc` test-export wrappers — tests use `Router()`
- [ ] State variants are filter params, not separate endpoints
- [ ] Errors use `api/common` helpers, not local copies
- [ ] Services encapsulate business logic, don't just delegate
- [ ] Auth code uses Security Runtime, Device Fleet and Identity & Access at their existing boundaries (Rule 9)
- [ ] No query construction outside packages the policy grants `orm-sql` (CI: `scripts/backend-architecture.sh check`)
- [ ] Models hold data, not decisions — no `Mark*/End*/Activate*` mutations, no RBAC, no magic thresholds
- [ ] Searched for existing helpers before writing a new one (`rg` before `func`)
- [ ] No new hand-rolled mock/fixture where a shared test double exists (Rule 13 table)
- [ ] Calendar/date/week test fixtures use fixed Berlin values, or have an exact-function live-clock exception with a reviewed reason
- [ ] New list endpoint → query-budget test via `testpkg.CaptureQueries` + register entry in `test/query-budgets/`; no hand-rolled `BeforeQuery` hooks (CI: `TestQueryBudgetRatchet`)
- [ ] All backend source stays inside Rule 16: no new file over 800 lines, function over 60 lines or `gocognit` 15; owner-scoped facade/pass-through/legacy budgets also stay unchanged or decrease

## Detection commands (one-shot health check)

```bash
cd backend

# Rules 1 + 11 — the authoritative checks are the ratchet tests:
../scripts/run-go-toolchain.sh go test -tags=testhooks ./test/ -run 'TestHandlerLayerRatchet' -v

# Rule 16 — source gates are backend-wide; other ratchets keep their named scopes
../scripts/run-go-toolchain.sh go test -tags=testhooks ./test/ -run 'TestModule' -count=1

# Rule 1 — manual sweep (should be zero)
rg --type go -g '!*_test.go' '^\s+\w+\s+[\w\.]*Repository\b|\.\w+Repository\(\)' api/

# Rule 3 — review unnecessary getters in owner-private implementations
rg --type go -c 'func \([^)]+\) GetID\(\)' modules/ workflows/ | awk -F: '{s+=$2} END {print s}'

# Rule 4 — cognitive complexity over 15 in handlers
../scripts/run-go-toolchain.sh go test -tags=testhooks ./test -run '^(TestHandlerComplexityRatchet|TestModuleComplexityRatchet)$' -count=1

# Rule 5 — test-export wrappers
rg -U '^func \([^)]+\) \w+Handler\(\) http\.HandlerFunc \{[^}]*return [^}]+\.\w+\s*\}' api/

# Rule 7 — duplicate error helpers outside api/common
rg --type go 'func ErrorInvalidRequest|func ErrorNotFound|func ErrorForbidden' api/ -l | grep -v '/common/'

# Rule 11 — owner/role grants are authoritative, not a directory-name grep
../scripts/run-go-toolchain.sh ../scripts/backend-architecture.sh check

# Rule 12 — inspect owner-private row/domain candidates; hits require review
rg --type go -g '!*_test.go' '^func \([^)]+\) (Mark|End|Close|Reset|Activate|Deactivate|Cancel|Approve|Reject|Suspend|Restore|Archive)\w*\(' modules/ workflows/
rg --type go -g '!*_test.go' '^func \([^)]+\) (HasPermission|IsAdmin|IsOperator|CanAccess|CanEdit|CanDelete)\w*\(' modules/ workflows/
rg --type go -g '!*_test.go' '\d+\s*\*\s*time\.(Hour|Minute|Second)' modules/ workflows/
```

The test and architecture commands are gates. The `rg` commands are review
leads, not verdicts: they include owner-private implementations and domain
operations. An empty grep does not establish compliance (ADR 0083).

## When to deviate

Only with explicit reviewer approval recorded in the PR description, citing the specific rule and the reason. "It's faster" and "the existing code does it this way" are not reasons.
