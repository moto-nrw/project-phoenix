# Phase 1: Multi-Tenancy Foundation — Implementation Summary

**Status:** Complete
**Branch:** `feature/multi-tenancy`
**Date:** 2026-02-12
**Implements:** Work Packages 1.1–1.9 from [11-implementierungsplan.md](11-implementierungsplan.md)
**Specifications:** [02-datenbank.md](02-datenbank.md) §1.1–§1.3, [03-backend.md](03-backend.md) §1.1–§1.2/§7, DEBATE decisions D2, D7, D8, D9, D10, D12, D14, D15, D16

---

## Overview

Phase 1 creates the **foundation primitives** that all subsequent multi-tenancy phases depend on. It introduces only **new code** — no existing files are modified in ways that affect behavior. All existing tests continue passing unchanged.

---

## What Was Built

### WP 1.1 — `tenant/` Package (Context + Transaction Wrappers)

**Files:**
- `backend/tenant/context.go`
- `backend/tenant/tx.go`

**Context helpers** for propagating tenant information through the request lifecycle:

| Function | Purpose |
|----------|---------|
| `WithTenantID(ctx, id)` / `FromContext(ctx)` | Set/get tenant ID (school ID) |
| `MustFromContext(ctx)` | Get tenant ID or panic (for code paths that require it) |
| `WithOrgID(ctx, id)` / `OrgFromContext(ctx)` | Set/get organization ID |
| `WithScope(ctx, scope)` / `ScopeFromContext(ctx)` | Set/get token scope ("tenant" or "platform") |
| `IsPlatformScope(ctx)` | Check if current request is platform-scoped |

**Transaction wrappers** for RLS-enforced database access:

| Function | Purpose |
|----------|---------|
| `WithTenantTx(ctx, db, tenantID, fn)` | Runs `fn` in a transaction with `SET LOCAL ROLE phoenix_tenant` and `set_config('app.current_tenant_id', ...)` |
| `WithAdminTx(ctx, db, fn)` | Runs `fn` in a transaction with `SET LOCAL ROLE phoenix_admin` (BYPASSRLS) |

**Design decisions applied:**
- Private context keys prevent collision with other packages (D8)
- `WithTenantTx` rejects zero tenant ID early (before DB interaction)
- Role and config are `LOCAL` to the transaction — automatically reset on commit/rollback

---

### WP 1.2 — TenantModel Mixin

**File:** `backend/models/base/tenant.go`

```go
type TenantModel struct {
    TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}

type TenantScoped interface {
    GetTenantID() int64
    SetTenantID(id int64)
}
```

**Design decisions applied:**
- Separate struct, NOT added to `base.Model` — platform models don't have `tenant_id` (D2)
- No `BeforeAppendModel` hook — 55/57 models shadow it; service-layer sets `tenant_id` explicitly (D10)
- In Phase 1 this is created but NOT embedded into existing models (that's Phase 3)

---

### WP 1.3 — Organizations + Schools Tables & Models

**Files:**
- `backend/database/migrations/001013001_create_organizations_and_schools.go` (V1.13.1)
- `backend/models/platform/organization.go`
- `backend/models/platform/school.go`

**Database tables:**

```
platform.organizations
├── id (BIGSERIAL PK)
├── name (VARCHAR 200, NOT NULL)
├── slug (VARCHAR 100, UNIQUE)
├── active (BOOLEAN, DEFAULT true)
├── settings (JSONB, DEFAULT '{}')
├── created_at, updated_at (TIMESTAMPTZ)

platform.schools
├── id (BIGSERIAL PK)          ← This is the tenant_id used everywhere
├── organization_id (FK → organizations)
├── name, slug, subdomain
├── active, settings
├── address, city, zip, phone, email
├── device_pin_hash
├── created_at, updated_at
├── UNIQUE(organization_id, slug)
├── UNIQUE(subdomain)
```

**Indexes:** `idx_schools_subdomain`, `idx_schools_organization`
**Triggers:** `update_modified_column()` on both tables
**No RLS** on platform tables — they are platform-scoped.

**Migration dependency:** V1.11.1 (platform schema creation)

---

### WP 1.4 — Account-Tenant Mapping

**Files:**
- `backend/database/migrations/001013002_create_account_tenants.go` (V1.13.2)
- `backend/models/auth/account_tenant.go`

**Database table:**

```
auth.account_tenants
├── id (BIGSERIAL PK)
├── account_id (FK → auth.accounts, ON DELETE CASCADE)
├── tenant_id (FK → platform.schools, ON DELETE CASCADE)
├── status ('pending' | 'active' | 'inactive')
├── invited_at, activated_at, deactivated_at
├── created_at, updated_at
├── UNIQUE(account_id, tenant_id)
```

**Indexes:** `idx_account_tenants_account`, `idx_account_tenants_tenant`, partial index on active status
**Migration dependencies:** V1.13.1 (platform.schools) + V1.0.1 (auth.accounts)

**Status lifecycle (D15):**
- `pending` → Account invited to tenant
- `active` → Account confirmed and active
- `inactive` → Account deactivated (soft disable)

---

### WP 1.5 — JWT Claims Extension

**Modified files:**
- `backend/auth/jwt/claims.go`
- `backend/auth/jwt/tokenauth.go`

**New fields in `AppClaims`:**
```go
TenantID int64 `json:"tenant_id,omitempty"` // School ID (tenant boundary)
OrgID    int64 `json:"org_id,omitempty"`    // Organization ID
```

**New field in `RefreshClaims`:**
```go
TenantID int64 `json:"tenant_id,omitempty"`
```

**Changes:**
- Added `getOptionalInt64()` helper for safe claim extraction
- `ParseClaims` extracts `tenant_id` and `org_id` from JWT claims
- `ParseStructToMap` includes `tenant_id`/`org_id` when non-zero

**Backward compatibility (FIX-5):** All new fields use `omitempty`. Existing tokens without these fields parse as zero-value (0). No existing code breaks — verified by running all 93 existing JWT tests.

---

### WP 1.6 — Tenant Middleware

**File:** `backend/auth/jwt/tenant_middleware.go`

```go
func TenantMiddleware(next http.Handler) http.Handler
```

- Placed AFTER JWT Authenticator in the middleware chain
- Extracts `TenantID`, `OrgID`, `Scope` from JWT claims
- Sets them on request context via `tenant.With*` helpers
- Returns 401 if no valid claims found (ID == 0)
- Does NOT start a transaction — that's the handler's responsibility (D14)

**In Phase 1, this middleware is created but NOT mounted in the router.** Mounting happens when handlers start using `WithTenantTx` in Phase 3.

---

### WP 1.7 — Test Fixtures

**Modified file:** `backend/test/fixtures.go`

| Fixture | Purpose |
|---------|---------|
| `CreateTestOrganization(tb, db, name)` | Creates a `platform.Organization` |
| `CreateTestSchool(tb, db, name)` | Creates a `platform.School` with auto-created org |
| `CreateTestSchoolForOrg(tb, db, orgID, name)` | Creates a school under an existing org |
| `CreateTestAccountTenant(tb, db, accountID, tenantID)` | Links an account to a tenant (active status) |
| `CleanupTenantFixtures(tb, db, ids...)` | Removes tenant test data in FK-safe order |

All fixtures follow existing patterns: `testing.TB` + `*bun.DB`, unique names with timestamp suffix, 5-second timeout, `require.NoError` assertions.

---

### WP 1.8 — `GetDB` Helper

**File:** `backend/database/repositories/base/tenant_db.go`

```go
func GetDB(ctx context.Context, db bun.IDB) bun.IDB
```

Returns the transaction from context (via `modelBase.TxFromContext`) if available, otherwise the base DB. Compatible with the existing `ContextWithTx`/`TxFromContext` pattern used throughout the codebase.

In Phase 3, repositories will replace `r.db.NewSelect()` with `GetDB(ctx, r.db).NewSelect()`.

---

### WP 1.9 — `AssertRowsAffected` Helper

**File:** `backend/database/repositories/base/helpers.go`

```go
func AssertRowsAffected(result sql.Result, expected int64) error
```

With RLS enabled, UPDATE/DELETE operations that don't check `RowsAffected()` can silently modify zero rows if the tenant can't see the target row (D16). This helper is created now; it will be wired into repositories in Phase 3.

---

## Files Summary

### New Files (15)

| File | WP |
|------|----|
| `backend/tenant/context.go` | 1.1 |
| `backend/tenant/tx.go` | 1.1 |
| `backend/tenant/context_test.go` | Tests |
| `backend/tenant/tx_test.go` | Tests |
| `backend/models/base/tenant.go` | 1.2 |
| `backend/database/repositories/base/helpers.go` | 1.9 |
| `backend/database/repositories/base/tenant_db.go` | 1.8 |
| `backend/database/migrations/001013001_create_organizations_and_schools.go` | 1.3 |
| `backend/models/platform/organization.go` | 1.3 |
| `backend/models/platform/school.go` | 1.3 |
| `backend/database/migrations/001013002_create_account_tenants.go` | 1.4 |
| `backend/models/auth/account_tenant.go` | 1.4 |
| `backend/auth/jwt/tenant_middleware.go` | 1.6 |
| `backend/auth/jwt/tenant_middleware_test.go` | Tests |
| This summary document | Docs |

### Modified Files (4)

| File | WP | Change |
|------|----|--------|
| `backend/auth/jwt/claims.go` | 1.5 | +`TenantID`, `OrgID` fields, `getOptionalInt64` helper |
| `backend/auth/jwt/tokenauth.go` | 1.5 | `ParseStructToMap` includes tenant fields |
| `backend/auth/jwt/claims_test.go` | 1.5 | +6 multi-tenancy tests |
| `backend/test/fixtures.go` | 1.7 | +5 fixture functions, +1 import |

### NOT Modified (existing code unchanged)

- `models/base/base.go` — `base.Model` is NOT touched (D2)
- `database/repositories/base/base.go` — existing CRUD methods unchanged
- `api/` handlers — no router changes
- `services/factory.go` — no changes
- `database/repositories/factory.go` — no changes
- All existing test files — no modifications

---

## Verification Results

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go run main.go migrate validate` | PASS — V1.13.1 + V1.13.2 registered with correct dependencies |
| New tenant context tests (12) | PASS |
| New tx validation test (1) | PASS |
| New JWT claims tests (6) | PASS |
| New middleware tests (4) | PASS |
| All existing JWT tests (93) | PASS — full backward compatibility confirmed |
| All existing model unit tests | PASS |

---

## What's Next (Phase 2)

Phase 2 creates the PostgreSQL roles and RLS infrastructure:

- **WP 2.1:** Create `phoenix_auth`, `phoenix_tenant`, `phoenix_admin` database roles
- **WP 2.2:** Grant appropriate privileges to each role
- **WP 2.3:** Create RLS policies on tenant-scoped tables

Once Phase 2 roles exist, the integration tests for `WithTenantTx`/`WithAdminTx` (which verify `SET LOCAL ROLE` and `set_config`) can be enabled.

---

## DEBATE Decisions Applied

| Decision | How Applied |
|----------|-------------|
| D2 | `TenantModel` is separate from `base.Model` — not all models are tenant-scoped |
| D7 | Schools table includes all fields from spec |
| D8 | `WithTenantTx` uses BUN's `RunInTx` + `SET LOCAL ROLE` + `set_config` |
| D9 | Organization/School hierarchy as specified |
| D10 | No `BeforeAppendModel` hook for tenant_id — set explicitly in service layer |
| D12 | JWT claims extended with `omitempty` for backward compatibility |
| D14 | Middleware only sets context — does NOT start transaction |
| D15 | `account_tenants` with `pending/active/inactive` status lifecycle |
| D16 | `AssertRowsAffected` helper created for Phase 3 wiring |
