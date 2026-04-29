# Backend Architectural Conventions

**RULE: Every new backend change must respect the conventions below.** They exist to prevent the bloat documented in `docs/architecture-cleanup-deep-dive.md`. The codebase is ~40% above its natural function-count floor today. Most of that bloat came from per-entity copy-paste that nobody blocked at review time. These rules block the patterns that produce it.

If a rule below conflicts with a reviewer's preference, the rule wins. The whole point is to remove the per-PR judgment call.

> **Verification status (2026-04-28):** All claims about base types, generic helpers, and existing infrastructure below have been cross-checked against the codebase. Existence claims (e.g., `Repository[T]`, `base.Model`, `api/common/errors.go`) are verified. Numeric counts (76 GetID copies, 67 BeforeAppendModel hooks, 310 test-export wrappers, 37 duplicate error helpers) are verified by `rg` sweep. Re-run the detection commands at the bottom of this file before relying on any specific count — they may shift with the codebase.

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

### Why

Handlers are the request boundary. Putting business logic or data access there scatters knowledge across HTTP-layer files, makes services thin pass-throughs, and prevents reuse from non-HTTP entry points (CLI, scheduler, SSE). Every layer skip becomes a future cleanup ticket.

### Detection

```bash
rg --type go -g '!*_test.go' '^\s+\w+\s+[\w\.]*Repository\b' backend/api/
rg --type go -g '!*_test.go' '\.\w+Repository\(\)' backend/api/
```

CI should fail on any new violation.

---

## 2. Repository Generics — No Per-Field Query Methods

**RULE: New repository methods MUST extend the generic `Repository[T]` in `database/repositories/base/base.go` instead of declaring `FindByX` clusters, per-field updaters, or per-status finders.**

### What `Repository[T]` provides today (verified)

```go
// In database/repositories/base/base.go:
type Repository[T modelBase.Entity] struct { ... }

func (r *Repository[T]) Create(ctx, entity T) error
func (r *Repository[T]) FindByID(ctx, id any) (T, error)
func (r *Repository[T]) Update(ctx, entity T) error
func (r *Repository[T]) Delete(ctx, id any) error
func (r *Repository[T]) List(ctx, filters map[string]any) ([]T, error)
func (r *Repository[T]) Count(ctx, filters map[string]any) (int, error)
func (r *Repository[T]) Transaction(ctx, fn func(tx bun.Tx) error) error
```

`models/base/filters.go` also provides a richer `Filter` type (`Equal`, `In`, `Like`, `DateRange`, `Or`, `And`, `WithPagination`, `WithSorting`) that compiles to `bun.SelectQuery` clauses.

### Forbidden

```go
// FORBIDDEN — N near-identical methods per repo
func (r *staffRepo) FindByEmail(ctx, email string) (*Staff, error) { ... }
func (r *staffRepo) FindByPersonID(ctx, personID int64) (*Staff, error) { ... }
func (r *staffRepo) FindByExternalID(ctx, externalID string) (*Staff, error) { ... }

// FORBIDDEN — per-field updaters
func (r *staffRepo) UpdateName(ctx, id int64, name string) error { ... }
func (r *staffRepo) UpdateEmail(ctx, id int64, email string) error { ... }

// FORBIDDEN — per-status finders
func (r *staffRepo) ListActive(ctx) ([]Staff, error) { ... }
func (r *staffRepo) ListInactive(ctx) ([]Staff, error) { ... }
```

### Required

```go
// CORRECT — use existing List(filters) for single-field lookups
staff, err := repo.List(ctx, map[string]any{"email": email})

// CORRECT — use base.Filter for richer queries
filter := base.NewFilter().Equal("status", "active").In("role_id", roleIDs)
staff, err := repo.ListWithFilter(ctx, filter)

// CORRECT — embed Repository[T] and add genuinely-needed custom methods
type staffRepo struct {
    *base.Repository[*models.Staff]
}

// CORRECT — patch-based update for partial mutations
func (r *staffRepo) UpdatePatch(ctx, id int64, patch map[string]any) error { ... }
```

### Exceptions

A custom method is justified only when:
- It performs a complex join or preload that the generic shape can't express
- It implements a domain operation that isn't reducible to a filter (e.g., `ClaimNextUnsupervisedGroup`)

Write a doc comment explaining why the generic isn't enough.

### Why

The repository layer has ~130 over-specified methods today. A new entity adds 10–15 by default. The generics infrastructure (`Repository[T]` + `Filter`) already exists; adoption is the only blocker.

---

## 3. Model Base Types — No Boilerplate Getters

**RULE: New model entities MUST embed `base.Model` (and `base.TenantModel` if tenant-scoped) instead of redeclaring `ID`, `CreatedAt`, `UpdatedAt`, `TenantID`, or trivial getters for those fields.**

### What the base types actually provide today (verified)

```go
// In models/base/base.go:
type Model struct {
    ID        int64     `bun:"id,pk,autoincrement"`
    CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
    UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
func (m *Model) BeforeAppend() error { ... }   // BUN hook (note: NOT BeforeAppendModel)

// In models/base/tenant.go:
type TenantModel struct {
    TenantID int64 `bun:"tenant_id,notnull"`
}
func (t *TenantModel) GetTenantID() int64       { return t.TenantID }
func (t *TenantModel) SetTenantID(id int64)     { t.TenantID = id }
```

`base.Model` does NOT currently provide `GetID()`, `GetCreatedAt()`, or `GetUpdatedAt()` methods, nor `BeforeAppendModel`. If an interface needs them, add them ONCE on `base.Model` so every embedder gets them for free — do not redeclare per entity.

### Forbidden

```go
// FORBIDDEN — redeclaring fields the base types own
type Staff struct {
    bun.BaseModel `bun:"users.staff"`
    ID            int64     `bun:",pk,autoincrement"`   // already in base.Model
    TenantID      int64     `bun:",notnull"`            // already in base.TenantModel
    CreatedAt     time.Time                             // already in base.Model
    UpdatedAt     time.Time                             // already in base.Model
}

// FORBIDDEN — trivial getters on entity (currently 76 GetID copies, 152 GetCreatedAt/UpdatedAt copies)
func (s *Staff) GetID() int64                 { return s.ID }
func (s *Staff) GetCreatedAt() time.Time      { return s.CreatedAt }
func (s *Staff) GetUpdatedAt() time.Time      { return s.UpdatedAt }
```

### Required

```go
type Staff struct {
    base.Model           // ID, CreatedAt, UpdatedAt + BeforeAppend
    base.TenantModel     // TenantID + GetTenantID/SetTenantID
    bun.BaseModel `bun:"users.staff"`
    // entity-specific fields below
}

// If an interface requires GetID() etc, add them ONCE in models/base/base.go:
//   func (m *Model) GetID() int64           { return m.ID }
//   func (m *Model) GetCreatedAt() time.Time { return m.CreatedAt }
// — never per entity.
```

### A note on `BeforeAppendModel`

There are 67 per-entity `BeforeAppendModel` hooks in `models/` today. They implement BUN's `BeforeAppendModelHook` interface and may contain entity-specific logic (defaults, derived fields, etc.) — they are NOT automatically redundant. Audit each hook before deleting; many can collapse to a shared helper if they only set timestamps or tenant_id, but some hold real logic.

### Why

The models/ layer has ~225 functions of pure passthrough boilerplate today. Every new entity adds 5–7 more. The base types exist; embedding them and centralizing getters once removes the rot at the source.

---

## 4. Thin Handlers — Cognitive Complexity ≤ 15

**RULE: No new handler may have a `gocognit` score over 15.** Branching, orchestration, and authorization logic belong in services.

### Detection

```bash
gocognit -over 15 backend/api/
```

CI should fail on any handler regression. New code at score ≤ 15 lands cleanly. Existing handlers above the cap are tracked in `docs/architecture-layer-violations.md` (Cat 3) and addressed in dedicated refactor tickets, not bundled with feature work.

### Why

Today the worst handlers score 64, 42, 41, 33, 31. They contain authorization branching, transaction orchestration, and multi-step business workflows that belong in `services/`. Capping at 15 forces extraction-as-you-go.

---

## 5. No Test-Export Handler Wrappers

**RULE: Do not add `*Handler() http.HandlerFunc { return rs.x }` wrappers to expose private handler methods to tests.** Use the public `Router()` (or `Routes()`) instead.

### Forbidden

```go
// FORBIDDEN — pure ceremony to expose a private method
func (rs *Resource) ListStaffHandler() http.HandlerFunc {
    return rs.listStaff
}
```

### Required

In tests:

```go
// CORRECT — test against the wired router
router := resource.Router()
req := httptest.NewRequest("GET", "/staff", nil)
w := httptest.NewRecorder()
router.ServeHTTP(w, req)
```

Or capitalize the method directly if it must be exported.

### Why

The `*Handler()` pattern produces ~310 throwaway functions today (verified by regex sweep). They're flagged as dead by `deadcode` because they're only called by tests. Routing via `Router()` exercises the same code path the production server uses, and is also a stronger test. Every Resource already has a `Router() chi.Router` method available.

---

## 6. API Design — Filter Parameters, Not Per-State Endpoints

**RULE: A new endpoint that varies only by a filter value MUST use a query parameter, not a separate path.**

### Forbidden

```
GET /students/in-house
GET /students/wc
GET /students/school-yard
GET /students/active
GET /students/picked-up
```

Each is a separate handler + service method + repo method = 5 funcs × 5 endpoints = 25 funcs.

### Required

```
GET /students?location=in_house
GET /students?location=wc
GET /students?status=picked_up
GET /students?status=active
```

One handler, one service method, one filter struct. Filter values are validated against an enum.

### Why

Per-state endpoints multiply functions linearly with state count. A `students` resource with three location states + two attendance states + four guardian states = 9 endpoints today, vs 1 with filters.

---

## 7. Centralized Error Helpers

**RULE: Use `api/common/errors.go` for HTTP error responses.** Do not redeclare `ErrorInvalidRequest`, `ErrorNotFound`, `ErrorInternalServer`, `ErrorForbidden`, or similar helpers in domain-specific `errors.go` files.

### Forbidden

```go
// FORBIDDEN — copy of api/common/errors.go in another package
package staff

func ErrorInvalidRequest(w http.ResponseWriter, msg string) { ... }
func ErrorNotFound(w http.ResponseWriter, msg string) { ... }
```

### Required

```go
// CORRECT — call the common helper
import "github.com/moto-nrw/project-phoenix/api/common"

common.ErrorInvalidRequest(w, "invalid student ID")
```

### Why

Today ~30 functions across `api/groups/errors.go`, `api/staff/errors.go`, `api/feedback/errors.go`, `api/rooms/errors.go`, `api/config/errors.go`, `api/auth/errors.go`, `api/operator/errors.go` redeclare what `api/common` already provides. Most are pure passthroughs.

---

## 8. Service Methods Encapsulate, Not Delegate

**RULE: A service method that does nothing but call one repository method is NOT a service method.** Either it adds business logic (validation, multi-repo orchestration, audit, tenant-tx wrapping, error mapping), or the handler should call the repository through a domain helper instead of through a thin service shim.

### Forbidden

```go
// FORBIDDEN — service that just forwards
func (s *staffService) GetByID(ctx, id int64) (*Staff, error) {
    return s.repo.FindByID(ctx, id)
}
```

### Required

```go
// CORRECT — service that does something a repo can't
func (s *staffService) GetByID(ctx, id int64) (*StaffWithPermissions, error) {
    staff, err := s.repo.FindByID(ctx, id)
    if err != nil { return nil, mapErr(err) }
    perms, err := s.permRepo.ForStaff(ctx, staff.ID)
    if err != nil { return nil, mapErr(err) }
    return &StaffWithPermissions{Staff: staff, Permissions: perms}, nil
}
```

### Why

Today many service methods are 1-line forwards. They inflate the function count without adding value, and they make handlers reach for `.Repository()` because the service offers nothing the repo doesn't already.

---

## 9. Auth Code Location — Audit Before Adding

**RULE: Before adding new authentication or authorization code, search both `backend/auth/` and `services/auth/` (and `services/usercontext/`) — match the existing layering rather than creating a third home.**

`backend/auth/` is NOT legacy. It contains structured low-level utility packages:

- `backend/auth/jwt/` — JWT plumbing (claims, tokenauth, authenticator, tenant_middleware)
- `backend/auth/authorize/` — authorization service + permission policies
- `backend/auth/device/` — device API key + PIN authentication
- `backend/auth/userpass/` — password hashing primitives

`services/auth/` holds business-logic services (login flows, invitation flows, MFA orchestration, password reset) that depend on the lower-level `backend/auth/` packages.

### Forbidden

- Inline permission/role decisions in handlers (use `services/usercontext/` or `backend/auth/authorize/`)
- A third copy of JWT/password/device helpers in `api/` or `middleware/`
- Bypassing the existing layering by importing repositories or DB clients directly into auth code

### Required

- New low-level primitive (hash, parse, verify)? → `backend/auth/{subdomain}/`
- New business flow (login, invite, reset)? → `services/auth/`
- New permission decision? → `services/usercontext/` or `backend/auth/authorize/policy/`
- Handler needs to authorize? → call the service or middleware, never decide inline

### Open question (audit needed, not a rule yet)

Some files in `backend/auth/authorize/` look service-shaped (`authorization_service.go`) and may belong under `services/auth/` instead. This is not currently enforced — flag for review when touching auth code, but do not move files speculatively.

---

## 10. Audit, Don't Re-Invent

**RULE: Before writing a new helper, search for existing implementations.**

```bash
# Before writing yet another formatStudentName, check
rg --type go 'func.*formatStudent' backend/

# Before writing yet another containsIgnoreCase, check
rg --type go 'func.*containsIgnoreCase|func.*caseInsensitive' backend/
```

If a helper already exists in three files with three slightly different signatures, the answer is one shared helper, not a fourth variant.

### Why

Today ~250 functions are trivial helpers duplicated across packages. The cause is reviewers not running a 5-second `rg`.

---

## 11. Services Don't Bypass Repositories

**RULE: Services MUST NOT construct database queries (`NewSelect`, `NewUpdate`, `NewInsert`, `NewDelete`, `NewRaw`) directly on a `bun.DB` / `bun.IDB` / `bun.Tx` handle.** All data access goes through repositories.

### What's allowed

A service may hold `*bun.DB` for two reasons:

- Passing it to repository constructors at wiring time
- Starting transactions via the `base.TxHandler` pattern, then calling **repo methods** inside the closure

Calling repo methods inside `RunInTx` is the correct pattern. Constructing queries inside a service — even via `repoBase.GetDB(ctx, s.db).NewSelect(...)` — is not.

### Forbidden (real examples in the codebase today)

```go
// services/active/session_service.go:1096 — service issues direct SELECT
err := s.db.NewSelect().Model(&groups).Where(...).Scan(ctx)

// services/active/cleanup_service.go:181 — raw SQL in service
err = s.db.NewRaw("DELETE FROM ... WHERE ...").Scan(ctx)

// services/facilities/facility_service.go:77 — query constructed via tenant helper
err := repoBase.GetDB(ctx, s.db).NewSelect().Model(&rooms).Where(...).Scan(ctx)
```

### Required

```go
// CORRECT — repo owns the query
groups, err := s.groupRepo.ListByRoomID(ctx, roomID)

// CORRECT — service orchestrates repo calls inside a transaction
err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
    if err := s.visitRepo.End(ctx, visitID); err != nil { return err }
    return s.attendanceRepo.RecordCheckout(ctx, ...)
})
```

### Exceptions

Cross-repo / cross-schema cleanup operations that genuinely don't fit a single repo's responsibility may use raw SQL — but the raw SQL must live in `database/repositories/{domain}/cleanup.go`, not inside the service. Document with a comment why a repo method isn't possible.

### Detection

```bash
rg --type go -g '!*_test.go' '\.(NewSelect|NewUpdate|NewInsert|NewDelete|NewRaw)\(' backend/services/
```

Today returns ~50 hits across ~15 service files (the worst offenders: `services/active/cleanup_service.go` with 10, `services/schedule/timetable_cleanup_service.go` and `services/platform/operator_provisioning_service.go` with 8 each). New code MUST NOT add to this count.

### Why

The repository layer owns tenant filtering, soft-delete semantics, audit hooks, and BUN-specific quirks. When a service constructs a query directly, every one of those invariants is at risk of being silently dropped — a forgotten `WHERE tenant_id = ?` becomes a multi-tenant data leak. Centralizing queries in repos isolates that risk and makes RLS testing tractable.

---

## 12. Models Hold Data, Not Decisions

**RULE: Methods on model entities are limited to (a) field accessors, (b) pure derivations from existing fields, and (c) BUN persistence hooks (`BeforeAppendModel`, `AfterScan`, etc.).** State mutations, business policies, RBAC decisions, and threshold-bearing logic MUST live in services. Magic numbers in model methods MUST move to settings or constants.

### Allowed

```go
// CORRECT — derived field, no policy
func (p *Person) FullName() string {
    return p.FirstName + " " + p.LastName
}

// CORRECT — pure boolean from existing fields, no thresholds
func (t *Token) IsExpired() bool {
    return !t.ExpiresAt.IsZero() && time.Now().After(t.ExpiresAt)
}

// CORRECT — BUN persistence hook
func (s *Staff) BeforeAppendModel(ctx context.Context, query bun.Query) error {
    // tenant_id / timestamp wiring
}
```

### Forbidden (real examples in the codebase today)

```go
// FORBIDDEN — state mutation with implicit policy
// models/active/visit.go:97
func (v *Visit) EndVisit() { ... }

// FORBIDDEN — business decision with magic-number threshold
// models/iot/device.go:140
func (d *Device) IsOnline() bool {
    return time.Since(*d.LastSeen) <= 5*time.Minute  // why 5? policy belongs elsewhere
}

// FORBIDDEN — magic number that should be a tenant setting
// models/active/group.go:156
return 30 * time.Minute  // "Default 30 minutes" — should be tenant-configurable

// FORBIDDEN — RBAC decision in a model
// models/auth/account.go:83
func (a *Account) HasPermission(permission string) bool { ... }
```

### Required relocations

| Was in model | Goes to |
|---|---|
| `Visit.EndVisit()` | `services/active/active_service.go` (with logging, events, audit) |
| `Device.IsOnline()` (5-minute threshold) | `services/iot/iot_service.go`, threshold from settings registry |
| `Group.DefaultDuration() = 30 * time.Minute` | settings registry (per-tenant configurable) |
| `Account.HasPermission(p)` | `services/usercontext` or `backend/auth/authorize` |

### Detection

```bash
# State-mutation verbs in model methods (Mark*/End*/Activate*/etc.)
rg --type go -g '!*_test.go' \
  '^func \([^)]+\) (Mark|End|Close|Reset|Activate|Deactivate|Cancel|Approve|Reject|Promote|Demote|Suspend|Restore|Archive)\w*\(' \
  backend/models/

# RBAC decisions in models
rg --type go -g '!*_test.go' \
  '^func \([^)]+\) (HasPermission|IsAdmin|IsOperator|CanAccess|CanEdit|CanDelete)\w*\(' \
  backend/models/

# Magic time-window literals in models
rg --type go -g '!*_test.go' \
  '\d+\s*\*\s*time\.(Hour|Minute|Second|Day)' \
  backend/models/
```

Today the first command returns ~12 hits, the second returns 5 (all should move to services), and the third surfaces ~3 bona-fide policy thresholds (5-minute device offline, 30-minute default duration, 15-minute account lockout) plus a few benign `Truncate(24*time.Hour)` calls.

### Why

Business rules drift constantly. Yesterday's "device is offline after 5 minutes" becomes "10 minutes for school A, 2 minutes for school B." If that logic lives in a model, every drift requires touching the data layer — and worse, every consumer that calls `device.IsOnline()` becomes a hidden policy enforcer. **Models hold facts (the timestamp). Services hold rules (what counts as online). Settings hold the parameters.**

---

## Code Review Checklist

Apply this before approving any backend PR:

- [ ] No repository imports in `api/` (other than `base.go` + `testutil/`)
- [ ] No `Repository` fields on handler structs
- [ ] No `.XxxRepository()` getter calls in handlers
- [ ] New repo methods extend `Repository[T]` — no `FindByEmail/FindByID/FindByX` clusters, no per-field updaters
- [ ] New entities embed `base.Model` (and `base.TenantModel` if tenant-scoped) — no redeclared ID/CreatedAt/UpdatedAt fields, no trivial `GetID/GetCreatedAt` methods
- [ ] New handler functions score `gocognit ≤ 15`
- [ ] No `*Handler() http.HandlerFunc` wrappers — tests use `Router()`
- [ ] State variants live as filter params, not separate endpoints
- [ ] Errors use `api/common` helpers, not local copies
- [ ] Services encapsulate business logic, don't just delegate to repos
- [ ] Auth code matches the existing `backend/auth/` (primitives) vs `services/auth/` (flows) layering
- [ ] **Services don't construct queries** — no `NewSelect/NewUpdate/NewInsert/NewDelete/NewRaw` in `services/`
- [ ] **Models hold data, not decisions** — no state-mutation methods (`Mark*/End*/Activate*`), no RBAC (`HasPermission/IsAdmin`), no magic-number thresholds in model code
- [ ] Searched for existing helpers before writing a new one (`rg` before `func`)

---

## Detection commands (one-shot health check)

```bash
cd backend

# Rule 1 — handler layer skips
rg --type go -g '!*_test.go' '^\s+\w+\s+[\w\.]*Repository\b|\.\w+Repository\(\)' \
   api/ -c | sort -t: -k2 -nr

# Rule 4 — cognitive complexity over 15 in handlers
gocognit -over 15 api/

# (project-wide) — dead code
deadcode ./...

# Rule 5 — test-export wrappers
rg -U '^func \([^)]+\) \w+Handler\(\) http\.HandlerFunc \{[^}]*return [^}]+\.\w+\s*\}' api/

# Rule 7 — duplicate error helpers outside api/common
rg --type go 'func ErrorInvalidRequest|func ErrorNotFound|func ErrorForbidden' api/ -l \
  | grep -v '/common/'

# Rule 11 — services constructing queries directly
rg --type go -g '!*_test.go' '\.(NewSelect|NewUpdate|NewInsert|NewDelete|NewRaw)\(' services/

# Rule 12a — state-mutation methods in models
rg --type go -g '!*_test.go' \
  '^func \([^)]+\) (Mark|End|Close|Reset|Activate|Deactivate|Cancel|Approve|Reject|Promote|Demote|Suspend|Restore|Archive)\w*\(' \
  models/

# Rule 12b — RBAC decisions in models
rg --type go -g '!*_test.go' \
  '^func \([^)]+\) (HasPermission|IsAdmin|IsOperator|CanAccess|CanEdit|CanDelete)\w*\(' \
  models/

# Rule 12c — magic time-window literals in models
rg --type go -g '!*_test.go' '\d+\s*\*\s*time\.(Hour|Minute|Second|Day)' models/
```

If any of these return non-trivial output for new code, the PR violates a rule above.

---

## When to deviate

Only with explicit reviewer approval recorded in the PR description, citing the specific rule and the reason. "It's faster" and "the existing code does it this way" are not reasons. The whole codebase pays the cost of every deviation.
