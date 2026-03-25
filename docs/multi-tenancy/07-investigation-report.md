# Multi-Tenancy Plan Investigation Report

**Date:** 2026-02-09
**Scope:** Feasibility and fit analysis of the multi-tenancy plan (docs 00–06 + DEBATE.md D1–D17)
**Method:** 4 parallel investigation agents — Backend (Go/BUN), Frontend (Next.js), Database (PostgreSQL/RLS), Industry Research

---

## Executive Summary

| Dimension | Rating | Notes |
|-----------|--------|-------|
| **Architecture Quality** | 8.5 / 10 | Decisions align with industry best practices (Supabase, PostgREST, Auth0, Vercel patterns) |
| **Feasibility** | Feasible with revisions | 5 critical/high blockers must be resolved before implementation starts |
| **Project Fit** | Excellent | Shared Schema + RLS is the right choice for 100–500 OGS at this codebase size |
| **Effort Estimate** | Underestimated | Plan says 14–18 weeks; realistic estimate is **18–24 weeks solo dev**, 14–18 weeks for 2–3 devs |
| **Document Consistency** | Needs update | 03-backend.md and 04-frontend.md contradict later DEBATE.md decisions |

**Bottom line:** The plan is architecturally sound and well-researched. Five items must be resolved before implementation begins. Once those are addressed, the plan is ready to execute.

---

## Critical Findings (Must Fix Before Implementation)

### C1: UNIQUE Constraints Will Break Multi-Tenancy

**Severity:** CRITICAL — Data integrity failure on day one of second tenant

The plan does not address existing UNIQUE constraints that assume single-tenancy. When a second tenant creates a group named "1a", the database will reject it.

**Affected constraints:**
| Table | Column | Current Constraint |
|-------|--------|-------------------|
| `education.groups` | `name` | UNIQUE(name) |
| `facilities.rooms` | `name` | UNIQUE(name) |
| `activities.categories` | `name` | UNIQUE(name) |
| `config.settings` | `key` | UNIQUE(key) |
| `users.guardian_profiles` | `email` | UNIQUE(email) |

**Required fix:** All single-column UNIQUE constraints on tenant-scoped tables must become composite: `UNIQUE(tenant_id, name)`. This needs a dedicated migration step added to `02-datenbank.md`.

---

### C2: 03-backend.md Contradicts D8 Decision (QueryHook vs SET LOCAL ROLE)

**Severity:** CRITICAL — Implementing the wrong pattern

`03-backend.md` describes a BUN `QueryHook` that injects `tenant_id` WHERE clauses at the ORM level. However, **D8 in DEBATE.md explicitly eliminates QueryHook** in favor of `SET LOCAL ROLE` per transaction (the PostgREST/Supabase pattern).

| Document | Approach | Status |
|----------|----------|--------|
| `03-backend.md` | BUN QueryHook on connection pool | **OUTDATED — contradicts D8** |
| `DEBATE.md D8` | `SET LOCAL ROLE phoenix_tenant` per transaction | **CURRENT decision** |

**Required fix:** Rewrite `03-backend.md` sections on RLS integration to use the `WithTenantTx` / `SET LOCAL ROLE` pattern from D8. Remove all QueryHook references.

---

### C3: Application Connects as `postgres` Superuser — RLS Has Zero Effect

**Severity:** CRITICAL — Security model is defeated at the connection level

The current application connects to PostgreSQL as the `postgres` superuser. PostgreSQL superusers **bypass all RLS policies** regardless of configuration. The entire RLS strategy is inert until the connection role changes.

**Current state (`database/database_config.go`):** DSN uses `postgres:postgres@...`

**Required fix:** This is partially addressed in the plan (three-role model: `phoenix_auth`, `phoenix_tenant`, `phoenix_admin`), but it must be the **first migration step**, not a later phase. The connection must use `phoenix_auth` (LOGIN, NOINHERIT) with `SET LOCAL ROLE` to activate `phoenix_tenant` per transaction.

---

### C4: Org-Scope Has No Technical Implementation

**Severity:** CRITICAL — Acknowledged blocker (also listed in `06-offene-punkte.md`)

Träger-Büro users need to see data across ALL OGS belonging to their carrier. The plan describes the business need but provides **no RLS policy, no middleware, no API pattern** for this scope.

**Recommended approach (from research agent):** Org-aware RLS policy:
```sql
-- Extend RLS policies with OR clause for org-scope
CREATE POLICY tenant_isolation ON education.groups
  USING (
    tenant_id = current_setting('app.current_tenant_id')::bigint
    OR tenant_id IN (
      SELECT school_id FROM platform.operator_organizations
      WHERE org_id = current_setting('app.current_org_id')::bigint
    )
  );
```

This must be designed as a **D18 decision** before implementation begins.

---

### C5: IoT Device Auth Has Schema Gaps

**Severity:** HIGH — IoT check-in flow will fail

The plan references `platform.schools` for device-to-tenant mapping, but:
- `platform.schools` table is not yet created (it's in `02-datenbank.md` but has no migration)
- `iot.devices` table has no `tenant_id` column currently
- The device auth service (`services/iot/device_service.go`) has no tenant resolution logic

**Required fix:** Add IoT device migration to Phase 1 (before any multi-tenant testing). The device must resolve to a tenant via `platform.schools.id` → `tenant_id`.

---

## High-Priority Findings

### H1: Table Count is Wrong — 64 Tables Need tenant_id, Not 49

The plan heading says "49 Tables" but actual analysis found **69 tables across 14 schemas**, of which **64 need tenant_id**. The plan's body partially acknowledges more tables but the implementation scope is under-counted.

**Schemas found:** auth, users, education, facilities, activities, active, schedule, iot, feedback, config, meta, audit, suggestions, platform (new)

**Impact:** Migration effort, testing scope, and timeline are all underestimated.

---

### H2: 04-frontend.md Uses Header Approach, D11 Decided Rewrite Pattern

`04-frontend.md` describes injecting tenant info via HTTP headers from Next.js middleware. **D11 in DEBATE.md** decided on the **rewrite pattern** (Vercel Platforms Starter Kit): subdomain → `/[tenant]/path` rewrite.

**Required fix:** Rewrite `04-frontend.md` to use the rewrite pattern. Key changes:
- Middleware rewrites `ogs-name.moto-app.de/dashboard` → `/ogs-name/dashboard`
- Route structure changes from `(protected)/` to `[tenant]/`
- ~40+ pages need path restructuring
- `useTenant()` hook reads from route params, not headers

---

### H3: 713 Direct BUN Queries Need Manual Modification

Only ~10 repositories use the base repository methods. **86% of repositories** make direct BUN query calls (`r.db.NewSelect()`, etc.). Each must be individually converted to use `WithTenantTx`.

| Pattern | Count | Conversion |
|---------|-------|------------|
| `r.db.NewSelect()` | ~300 | Must use tx from `WithTenantTx` |
| `r.db.NewInsert()` | ~150 | Must use tx from `WithTenantTx` |
| `r.db.NewUpdate()` | ~130 | Must use tx from `WithTenantTx` |
| `r.db.NewDelete()` | ~80 | Must use tx from `WithTenantTx` |
| `r.db.RunInTx()` | ~50 | Must nest inside `WithTenantTx` |
| **Total** | **~713** | |

**Impact:** This is the largest single work item. Mechanical but error-prone. Each conversion is a potential data leak if done incorrectly.

---

### H4: Base Repository Bypasses Transaction Context

The base repository (`database/repositories/base_repository.go`) uses `r.DB` directly instead of extracting a transaction from context. This means even after adding `WithTenantTx` wrappers, the base methods won't participate in the tenant transaction.

**Required fix:** The base repository must be refactored to accept context-based transactions:
```go
func (r *BaseRepository) getDB(ctx context.Context) bun.IDB {
    if tx := tenant.TxFromContext(ctx); tx != nil {
        return tx
    }
    return r.DB
}
```

---

### H5: View `users.expired_privacy_consents` Lacks `security_invoker`

The view `users.expired_privacy_consents` runs with the view owner's permissions, not the calling role's. Without `security_invoker = true` (PostgreSQL 15+), RLS policies are bypassed when querying this view.

**Required fix:** Add `ALTER VIEW users.expired_privacy_consents SET (security_invoker = true)` to the migration.

---

### H6: RowsAffected() Not Checked on 72% of UPDATE/DELETE

With RLS active, an UPDATE/DELETE that targets another tenant's data will silently affect 0 rows instead of raising an error. Currently, ~72% of UPDATE/DELETE operations don't check `RowsAffected()`.

**Impact:** Silent failures where operations appear to succeed but change nothing. This is a data integrity and debugging nightmare.

**Required fix:** Add `RowsAffected()` checks to all UPDATE/DELETE operations as part of the repository conversion (H3).

---

## Medium-Priority Findings

### M1: SSE Hub Design Mismatch

The current SSE hub partitions subscriptions by `active_group_id` (correct for real-time group updates). The plan proposes tenant-level partitioning, which would be a **regression** — broadcasting all tenant events to all connected users regardless of group.

**Recommendation:** Keep group-level partitioning. Add tenant validation at connection time (verify the connecting user belongs to the tenant), but don't change the broadcast granularity.

---

### M2: Advisory Lock Cross-Tenant Blocking

`services/active/session_service.go:168` uses PostgreSQL advisory locks with student IDs. Advisory locks are **database-global** — a lock on student ID 42 in Tenant A blocks student ID 42 in Tenant B.

**Required fix:** Namespace advisory locks by tenant:
```go
lockKey := tenantID*1_000_000 + studentID
```

---

### M3: `reset.go` Missing Schemas

The database reset function doesn't include `audit`, `suggestions`, or `platform` schemas. After reset, these schemas will retain stale data.

---

### M4: `auth.accounts.tenant_id` Purpose Unclear

`02-datenbank.md` adds `tenant_id` to `auth.accounts`, but D15 decided accounts are global (single account, multiple tenants via `auth.account_tenants` junction). The column on `auth.accounts` may be a "default tenant" or may be vestigial.

**Recommendation:** Clarify in the plan — is this the default/last-used tenant? If so, document it. If not, remove it.

---

### M5: No Frontend Tests Planned

`05-testing.md` covers Go tests and Bruno API tests but has no frontend test strategy (no component tests, no E2E tests for tenant switching, no subdomain routing tests).

---

### M6: Contradictory Treatment of `auth.roles` / `auth.permissions`

D13 says roles and permissions are global (YAGNI — no per-tenant custom roles). But `02-datenbank.md` adds `tenant_id` to these tables. These contradict each other.

**Recommendation:** Follow D13 — keep roles/permissions global. Remove `tenant_id` from `auth.roles` and `auth.permissions` in the migration plan.

---

### M7: Parent Isolation Under-Specified

Parents can have children in different OGS (different tenants). The plan acknowledges this scenario but doesn't specify:
- How parent login resolves to a tenant
- Whether parents see a tenant picker
- How cross-tenant parent data is isolated

---

## Low-Priority Findings

### L1: No Connection Pooling Configured

Go's `database/sql` defaults to unlimited connections. With RLS and `SET LOCAL ROLE` per transaction, connection management becomes more important. Consider configuring `SetMaxOpenConns()`, `SetMaxIdleConns()`, and `SetConnMaxLifetime()`.

### L2: Migration Framework Well-Suited

The existing migration framework (versioned, dependency-based) is well-suited for the multi-tenancy migration. No changes needed to the framework itself.

### L3: Factory Pattern Compatible

The factory pattern (`repositories.NewFactory`, `services.NewFactory`) naturally accommodates tenant context through context propagation. No architectural changes needed.

### L4: 41 Seed File Insert Calls Need tenant_id

13 seed files contain 41 direct `NewInsert()` calls that will need tenant_id values. D16 already addresses this (6 measures for raw SQL + seed fixes).

---

## Industry Best Practices Alignment

| Decision | Pattern Used | Industry Standard | Verdict |
|----------|-------------|-------------------|---------|
| Shared Schema + RLS | PostgreSQL RLS | Supabase, Nile, Citus | ALIGNED |
| Three-Role Model | `phoenix_auth` → `SET LOCAL ROLE` | PostgREST (since 2014) | ALIGNED |
| Single Account Multi-Tenant | `auth.account_tenants` junction | Auth0, WorkOS, Clerk | ALIGNED |
| Next.js Rewrite Pattern | Subdomain → `/[tenant]/path` | Vercel Platforms Starter Kit | ALIGNED |
| Defense-in-Depth (4 layers) | RLS + WHERE + Policy + NOT NULL | Exceeds OWASP guidance | ALIGNED |
| Phased Migration | Permissive → Logging → Enforced | Standard RLS rollout | ALIGNED |
| Fail-Closed Security | No `tenant_id=0` bypass | OWASP Multi-Tenancy Guide | ALIGNED |

**Notable:** The plan exceeds industry norms for children's data protection (GDPR Art. 8), which is appropriate given the domain.

---

## Revised Effort Estimate

| Phase | Plan Estimate | Revised Estimate (Solo) | Notes |
|-------|---------------|------------------------|-------|
| Phase 1: Foundation | 4–5 weeks | 6–8 weeks | C1, C3, C5 add scope; H3 is larger than estimated |
| Phase 2: Core Migration | 6–8 weeks | 8–10 weeks | 713 queries (not ~350 files), H4 base repo refactor |
| Phase 3: Frontend | 2–3 weeks | 3–4 weeks | D11 rewrite pattern requires 40+ page restructuring |
| Phase 4: Testing & Hardening | 2 weeks | 3–4 weeks | M5 frontend tests missing, RowsAffected audit |
| **Total** | **14–18 weeks** | **20–26 weeks (solo)** | |
| **With 2–3 devs** | — | **14–18 weeks** | Parallelizable: DB + Backend + Frontend |

---

## Recommended Pre-Implementation Actions

Before writing any code, resolve these 5 items:

| # | Action | Blocker? | Effort |
|---|--------|----------|--------|
| 1 | Design org-scope RLS policy (D18 decision) | Yes — CRITICAL | 2–4 hours design |
| 2 | Add UNIQUE constraint migration to `02-datenbank.md` | Yes — CRITICAL | 1 hour |
| 3 | Rewrite `03-backend.md` to match D8 (remove QueryHook) | Yes — prevents wrong implementation | 2–3 hours |
| 4 | Rewrite `04-frontend.md` to match D11 (rewrite pattern) | Yes — prevents wrong implementation | 2–3 hours |
| 5 | Clarify `auth.accounts.tenant_id` vs D15 (global accounts) | No — but causes confusion | 30 min |

---

## Conclusion

The multi-tenancy plan is **architecturally excellent** (8.5/10) and aligns with proven industry patterns. The team has clearly done thorough research and made well-reasoned decisions in the DEBATE.md process.

The main risks are:
1. **Document drift** — Two key implementation docs (03, 04) haven't been updated to reflect DEBATE decisions
2. **Scope underestimation** — The actual migration scope is ~30% larger than documented
3. **One undesigned feature** — Org-scope access is a business-critical requirement with no technical design

Once the 5 pre-implementation actions are completed, the plan is ready to execute. The architecture is sound, the patterns are proven, and the defense-in-depth approach is appropriate for GDPR-regulated children's data.
