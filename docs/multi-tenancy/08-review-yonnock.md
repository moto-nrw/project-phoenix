# Multi-Tenancy Plan Review — Yonnock

**Date:** 2026-02-09
**Reviewer:** @yonnock
**Scope:** Full review of docs 00-07 + DEBATE.md (D1-D17)
**Method:** Deep-read of all documents, codebase cross-reference, devil's advocate analysis

---

## TL;DR

The architecture is solid — Shared Schema + RLS, three-role model, fail-closed defaults, defense-in-depth. Good research, well-reasoned DEBATE decisions. But there are **5 blockers** that must be resolved before any implementation starts, and several high-priority gaps that will cause rework or subtle bugs if not addressed.

**Estimated fix time for all blockers: ~1 day of focused work.**

---

## CRITICAL — Will break on day one

### C1: UNIQUE constraints will reject the second tenant's data

**Affected:** 02-datenbank.md (missing entirely)

Nobody addressed existing UNIQUE constraints that assume single-tenancy. The moment a second OGS creates a group "1a" or a room "Raum 1", the database rejects the INSERT.

**Known affected constraints:**

| Table | Current Constraint | What breaks |
|---|---|---|
| `education.groups.name` | `UNIQUE(name)` | Two OGS can't have group "1a" |
| `facilities.rooms.name` | `UNIQUE(name)` | Two OGS can't have "Raum 1" |
| `activities.categories.name` | `UNIQUE(name)` | Two OGS can't have "Hausaufgaben" |
| `config.settings.key` | `UNIQUE(key)` | Two OGS can't have same setting key |
| `users.guardian_profiles.email` | `UNIQUE(email)` | Parent at two OGS = crash |

**Required fix:** All single-column UNIQUE constraints on tenant-scoped tables must become composite: `UNIQUE(tenant_id, name)`. This needs a dedicated migration step added to `02-datenbank.md`. Needs a full audit of all UNIQUE indexes and constraints across all 11+ schemas.

**Action:** Add section to 02-datenbank.md with complete list of UNIQUE constraint migrations.

---

### C2: `auth.accounts.tenant_id` contradicts D15 (global accounts)

**Affected:** 02-datenbank.md vs. DEBATE.md D15

D15 clearly states: accounts are global, email is globally UNIQUE, `account_tenants` is the N:M junction table. But 02-datenbank.md still lists `auth.accounts` as a table that gets `tenant_id`.

**The problem:** If you add `tenant_id` to `auth.accounts` and enable RLS on it, then `POST /auth/login` can't find accounts across tenants. The login flow (D6, step 3) says "Finde Account by email (global)" — but with RLS active on accounts, the query would be filtered to only the current tenant's accounts. You don't KNOW the tenant yet at that point in the login flow.

**Options:**
- **(a) Remove `tenant_id` from `auth.accounts` entirely.** Accounts stay global, `account_tenants` handles the N:M. Login uses `WithAdminTx` to find the account globally, then validates tenant membership. This is the Auth0/WorkOS pattern and aligns with D15.
- **(b) Keep it as "primary/default tenant"** but exempt `auth.accounts` from RLS. Document clearly what the field means and when it's used.

**My recommendation:** Option (a). Kill `auth.accounts.tenant_id`. It creates confusion and contradicts the decided architecture. The login handler should use `WithAdminTx` for the account lookup, then switch to `WithTenantTx` after tenant membership is confirmed.

**Action:** Decide and update 02-datenbank.md. Remove `auth.accounts` from the "WITH tenant_id" list if going with (a).

---

### C3: Org-scope (`Traeger-Buero`) has zero technical implementation

**Affected:** 00-anforderungen.md Section 3.2a, 01-architektur.md, 06-offene-punkte.md #1

Already flagged in 06-offene-punkte.md as the sole remaining blocker. I agree it's a blocker.

The business requirement is clear: Traeger-Buero staff see ALL OGS of their Traeger automatically, including new ones added later. But:

- RLS policies only handle `tenant_id = X` (single tenant per request)
- No `app.current_org_id` is set in `set_config()` or referenced in any RLS policy
- No backend code or pattern shows how to query across tenants for org-scope
- No test pattern for org-scope exists in 05-testing.md

**The three options from 06-offene-punkte.md:**

| Option | Approach | Pros | Cons |
|---|---|---|---|
| (a) Org-aware RLS | `OR tenant_id IN (SELECT id FROM platform.schools WHERE org_id = current_setting('app.current_org_id')::bigint)` | Clean, DB-enforced | Adds subquery to every RLS policy, needs `set_config` for org_id too |
| (b) Application-layer | Service uses `WithAdminTx` + manual `WHERE org_id = ?` filter | Simple, uses existing admin role | BYPASSRLS for non-platform user feels wrong, no DB-level enforcement |
| (c) Tenant-switch only | Org-scope users just switch between tenants manually | Zero new code | Terrible UX for aggregated views, no cross-OGS dashboards |

**My take:** Option (a) is the cleanest long-term but adds complexity to every RLS policy. Option (b) is pragmatic for the initial rollout — the Traeger-Buero user switches to a specific OGS for day-to-day work (`WithTenantTx`), and aggregated views use a dedicated service with `WithAdminTx` that filters by `org_id` in the application layer.

I'd lean toward **(b) for initial rollout + (a) later if needed**. Reason: the number of Traeger-Buero users is tiny compared to Betreuer. Optimizing RLS for a rare access pattern adds complexity to the most critical security layer. Better to nail the common case first.

**Action:** Add D18 decision to DEBATE.md. Pick an approach.

---

### C4: Eltern data isolation is under-specified (GDPR risk)

**Affected:** 00-anforderungen.md Section 3.4, 06-offene-punkte.md #3

RLS operates at tenant level. Once a parent has `account_tenants` access to an OGS, RLS gives them SELECT access to ALL rows in that tenant — including other families' children. The requirements say "Sehen nur Daten des eigenen Kindes", but there's no mechanism designed for this.

**This is a GDPR issue.** A parent seeing another family's child data (name, attendance, health info) is a reportable data breach under GDPR Art. 33.

**Options:**
- **(a) Policy Engine:** New `ParentChildPolicy` (analogous to existing `TeacherGroupPolicy`) that checks parent → child → student relationship before allowing access.
- **(b) Service-layer filtering:** Parent API endpoints only return data for the parent's linked children. No policy engine involvement.
- **(c) Separate RLS policy for parents:** `WHERE student_id IN (SELECT student_id FROM users.students_guardians WHERE guardian_id = current_setting('app.current_parent_id'))`

**My take:** Option (b) is simplest — parent-facing API endpoints are separate from staff endpoints anyway (different UI, different routes). The service layer filters by `students_guardians` relationship. No need to complicate the general RLS policies.

But this MUST be documented explicitly. "Eltern isolation is handled at the service layer, not RLS" needs to be a stated design decision.

**Action:** Add section to 03-backend.md or create D19 in DEBATE.md. Document the Eltern isolation mechanism.

---

### C5: 03-backend.md and 04-frontend.md are outdated — contradict DEBATE decisions

**Affected:** 03-backend.md, 04-frontend.md

These two implementation docs still describe pre-DEBATE architecture:
- **03-backend.md** still references QueryHook (eliminated by D8/D9)
- **04-frontend.md** still references header pattern (replaced by rewrite pattern in D11)

Anyone implementing from these docs will build the wrong thing.

**Note:** Flo has already partially updated these (the latest commit `ad4c866c` says "align remaining multi-tenancy docs with DEBATE decisions"), but I can see from the investigation report (07) that inconsistencies remain. Need to verify the current state matches all 17 DEBATE decisions.

**Action:** Do a final pass on 03 and 04 to ensure they match D1-D17. Cross-reference checklist:
- [ ] 03: No QueryHook references (D8/D9)
- [ ] 03: Uses `WithTenantTx`/`WithAdminTx` everywhere (D8)
- [ ] 03: No `tenant_id=0` bypass (D7)
- [ ] 03: TenantModel mixin, not base.Model extension (D2)
- [ ] 03: No BeforeAppendModel on TenantModel (D10)
- [ ] 03: Two-tier auth pattern (D14)
- [ ] 04: Rewrite pattern, not header pattern (D11)
- [ ] 04: `[tenant]/layout.tsx` validation pattern (D17)
- [ ] 04: `useTenant()` hook with TenantInfo interface (D5)
- [ ] 04: `tenant_slug` in body, not header (D6)

---

## HIGH — Will cause significant rework or subtle bugs

### H1: 72% of UPDATE/DELETE don't check RowsAffected()

**Affected:** All repositories, flagged in D16

Under RLS, an UPDATE/DELETE that targets another tenant's row silently affects 0 rows. Without checking `RowsAffected()`, the code continues as if the operation succeeded.

**Real-world scenario:** A student checkout that targets the wrong tenant? Silent. A grade transition? Silent. The student stays "checked in" forever, no error, no log. This is data corruption without any signal.

The plan mentions `assertRowsAffected` helper (D16) but doesn't include it in the migration phases. This should be Phase 1 work — it's needed BEFORE RLS goes live, not after.

**Action:** Add RowsAffected audit + fix to Phase 1 in migration plan.

---

### H2: Advisory locks are database-global, not tenant-scoped

**Affected:** `services/active/session_service.go:168`

`pg_advisory_xact_lock(activityID)` — advisory locks are global. Tenant A locking activity 42 blocks Tenant B's activity 42. Under load with multiple tenants, this becomes a cross-tenant performance bottleneck and potential deadlock source.

D16 mentions the two-argument fix (`pg_advisory_xact_lock(tenantID, activityID)`) but it's easy to miss during implementation because it's buried in a 6-item list.

**Action:** Flag this prominently in the migration checklist, not just in D16.

---

### H3: SSE hub redesign proposal is wrong

**Affected:** 03-backend.md Section 10 (SSE/Realtime)

The plan proposes tenant-level SSE partitioning:
```go
type Hub struct {
    tenantSubscriptions map[int64]map[*Subscriber]bool
}
```

But the current hub partitions by `active_group_id` — which is correct. You want group-level real-time updates (e.g., "student checked into Group A"), not all-tenant broadcasts. Tenant-level partitioning would be a regression: every user at the OGS receives every event for every group.

**The actual fix needed:** Validate tenant membership at SSE connection time (verify the connecting user's JWT `tenant_id` matches the group's `tenant_id`). Keep group-level broadcasting unchanged.

**Action:** Rewrite 03-backend.md Section 10. Tenant validation at connection, group-level broadcasting stays.

---

### H4: Rollback plan ignores NOT NULL constraint

**Affected:** 02-datenbank.md Section 8.2

The rollback plan says: "tenant_id Spalten bleiben (stoeren nicht, da nullable oder default)."

But Migration Step 6 sets `ALTER COLUMN tenant_id SET NOT NULL`. After that, rolling back the application code means old code can't INSERT without providing `tenant_id` → every INSERT fails.

**Action:** Add `ALTER COLUMN tenant_id DROP NOT NULL` to the rollback plan. Or better: keep tenant_id nullable until Phase 3 (strict enforcement), so rollback is always safe.

---

### H5: No frontend test strategy

**Affected:** 05-testing.md

The testing doc covers Go unit tests, integration tests, and Bruno API tests. Zero mention of:
- E2E tests for subdomain routing (does `altenberge.localhost:3000` actually work?)
- Component tests for tenant switching
- SWR cache isolation tests (switching tenants shows stale data from previous tenant?)
- Bruno multi-tenant test scenarios (login as Tenant A, try to access Tenant B data)

Frontend is where the user sees the data. If the SWR cache leaks across tenant switches, users see another OGS's children. That's a visible GDPR breach.

**Action:** Add frontend test section to 05-testing.md. At minimum: SWR cache isolation test, tenant-switch E2E test, subdomain routing test.

---

### H6: No local dev setup for subdomains documented

**Affected:** Missing from all docs

How do developers test `altenberge.localhost:3000`? Modern browsers (Chrome, Firefox, Edge) support `*.localhost` natively, but:
- Does Docker Compose need changes?
- Does the frontend dev server need configuration?
- What about the backend — does it need to accept requests from `*.localhost`?
- What about CORS?

This affects every developer from day one of working on multi-tenancy.

**Action:** Add dev setup instructions, either in 04-frontend.md or a new doc.

---

## MEDIUM — Address during implementation

### M1: Table count is wrong (stated 49, actually 64+)

02-datenbank.md header says "49 Tabellen" but the actual listing sums to 64+. There may also be tables in the `meta` schema (mentioned in CLAUDE.md but not in the migration plan) that need `tenant_id`. Risk: tables get missed during migration.

**Action:** Do a fresh `\dt *.*` against the actual database and reconcile with the plan.

---

### M2: `auth.roles` and `auth.permissions` in tenant_id list contradicts D13

D13 says roles are global. But some references in 02-datenbank.md imply they get `tenant_id`. These should NOT get `tenant_id` per D13.

**Action:** Verify and clean up. `auth.roles`, `auth.permissions`, `auth.role_permissions`, `auth.account_roles`, `auth.account_permissions` should all be in the "NO tenant_id" list.

---

### M3: `platform.cross_tenant_access` needs more detail

The Ferienbetreuung (D4) flow says "Admin enrollt Kinder aus anderen OGS via `platform.cross_tenant_access`". But the table schema doesn't have fields for which children are enrolled — it only tracks account-level access grants. The actual child enrollment presumably happens in `activities.student_enrollments`, but how does the cross-tenant read work? Does the active service use `WithAdminTx` to fetch specific enrolled children from other tenants?

This flow needs a more detailed sequence diagram or pseudocode.

**Action:** Expand D4 or add to 03-backend.md with a step-by-step code flow.

---

### M4: Connection pooling configuration not specified

Go's `database/sql` defaults to unlimited connections. With `SET LOCAL ROLE` per transaction, connection management matters more — each transaction holds a connection for the duration of the role switch + queries + commit. No `SetMaxOpenConns()`, `SetMaxIdleConns()`, or `SetConnMaxLifetime()` is specified.

At 100+ tenants with concurrent requests, this could exhaust connections.

**Action:** Add connection pool configuration to 01-architektur.md or 03-backend.md.

---

## Things done well (genuinely good decisions)

- **Shared Schema + RLS** — correct for 100-500 tenants with BUN ORM's limitations
- **Three-role model** (`phoenix_auth` NOINHERIT → `SET LOCAL ROLE`) — battle-tested PostgREST/Supabase pattern since 2014
- **No `tenant_id=0` bypass** (D7) — fail-closed is the only acceptable default for children's data
- **Defense-in-depth** (4 layers) — RLS + WHERE + Policy + RowsAffected exceeds industry norms
- **Global accounts + `account_tenants`** (D15) — Auth0/WorkOS pattern, correct
- **Rewrite pattern** (D11) for Next.js — official Vercel approach, avoids dynamic rendering trap
- **TenantModel mixin** (D2) — clean separation, compile-time safety
- **DEBATE.md process** — structured decision-making with alternatives, sources, and rationale. This is how architecture decisions should be documented.

---

## Summary: Pre-implementation checklist

| # | Action | Blocker? | Est. time |
|---|---|---|---|
| C1 | Audit + plan all UNIQUE constraint migrations | **Yes** | 2-3 hours |
| C2 | Decide `auth.accounts.tenant_id` — kill it or define it | **Yes** | 30 min |
| C3 | Design org-scope mechanism (D18 decision) | **Yes** | 2-4 hours |
| C4 | Design Eltern data isolation mechanism (D19) | **Yes** | 1-2 hours |
| C5 | Final pass on 03-backend.md + 04-frontend.md vs DEBATE | **Yes** | 4-6 hours |
| H1 | Add RowsAffected audit to Phase 1 | No but important | 30 min (planning) |
| H2 | Flag advisory lock fix prominently | No | 15 min |
| H3 | Fix SSE hub redesign proposal | No | 30 min |
| H4 | Fix rollback plan for NOT NULL | No | 15 min |
| H5 | Add frontend test strategy | No | 1-2 hours |
| H6 | Document local dev subdomain setup | No | 1 hour |

**Total for blockers: ~1 day of focused work.**

After that, the plan is solid and ready to execute.

---

*Review by @yonnock, 2026-02-09*
