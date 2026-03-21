# Multi-Tenancy Devil's Advocate Findings

**Date:** 2026-02-09
**Reviewer:** @yonnock
**Method:** 4-agent adversarial analysis (Opus 4.6), each attacking from a different dimension
**Agents:** Database & RLS, Backend Go/BUN, Frontend & Auth, Business Logic & GDPR
**Scope:** Full codebase cross-reference against docs 00-07, DEBATE.md (D1-D17), and investigation report

---

## TL;DR

The architecture (Shared Schema + RLS, three-role model, defense-in-depth) is fundamentally sound. But a deep adversarial review against the **actual codebase** reveals **7 critical items that must be resolved before implementation**, plus 15+ high/medium issues that will cause production failures, data leaks, or GDPR violations if left unaddressed.

**Revised effort estimate: 20-26 weeks solo** (up from the plan's 14-18 weeks).

---

## Methodology

Each agent was given full access to the codebase and the multi-tenancy documentation. Instructions: find every flaw, contradiction, missing piece, and dangerous assumption. Agents were explicitly told to be hostile and assume nothing.

| Agent | Focus | Key Metric |
|---|---|---|
| Database & RLS | RLS policies, migrations, GRANT, FKs, triggers | 60+ sequences, 13 broken UNIQUE constraints, 40+ FKs |
| Backend Go/BUN | Transaction patterns, repositories, services, scheduler | 538 `r.db.` calls, 51 `RunInTx`, 110 `WithTx`, 6 scheduler jobs |
| Frontend & Auth | NextAuth, cookies, SWR cache, redirects, tenant switch | 821 SWR calls, 40+ hardcoded redirects, 194 API routes |
| Business & GDPR | Domain logic, roles, GDPR Art. 8/17/25/26, Ferienbetreuung | 5 GDPR articles implicated, 0 parent isolation mechanisms |

---

## CRITICAL -- Must resolve before implementation begins

### C1: Transaction pattern conflict -- WithTenantTx vs existing WithTx/RunInTx

**Source:** Backend agent, codebase analysis
**Affected:** All services and repositories

The plan proposes `WithTenantTx(ctx, tenantID)` as the new transaction wrapper. But the codebase already uses two transaction patterns:

- **51 `RunInTx` calls** -- BUN's built-in transaction method
- **110 `WithTx` calls** -- Custom transaction passing

These patterns are fundamentally different from `WithTenantTx` which needs to do `SET LOCAL ROLE` + `set_config('app.current_tenant_id', ...)` inside the transaction. The plan never explains:

1. How `RunInTx` callbacks get the tenant context (they receive a `bun.Tx`, not a context)
2. Whether `WithTx(tx)` chains inside `WithTenantTx` or creates a nested transaction
3. What happens to cross-service calls that pass `tx` directly (e.g., `session_service.go:77`)

**Evidence:** `backend/services/active/session_service.go:77` -- cross-service call inside `RunInTx` would start a nested transaction on a different connection if not reconciled.

**Risk:** Every service that uses `RunInTx` or `WithTx` will need manual conversion. This is not documented in any migration phase.

---

### C2: Scheduler has no tenant concept -- 6 jobs use context.Background()

**Source:** Backend agent, codebase analysis
**Affected:** `backend/services/scheduler/scheduler.go`

The scheduler runs 6+ periodic jobs using `context.Background()` with zero tenant information:

| Job | Line | What it does |
|---|---|---|
| Cleanup expired data | ~256 | Deletes across all tenants |
| Process pending checkouts | ~574 | Modifies active visits |
| Nightly attendance sync | ~697 | Reads/writes attendance records |
| Session timeout checker | ~799 | Modifies active sessions |
| + 2 more | various | Various maintenance |

With RLS active on `phoenix_tenant`, these jobs cannot run -- they have no tenant context. With `phoenix_admin` (BYPASSRLS), they bypass all tenant isolation.

**The plan mentions none of this.** There is no design for how scheduled jobs iterate over tenants, whether they use `WithAdminTx` or loop through tenants with `WithTenantTx`, or how failures in one tenant's cleanup affect others.

**Risk:** Either all scheduler jobs silently fail (no tenant context) or they run with BYPASSRLS (no tenant isolation). Both are unacceptable.

---

### C3: IoT device auth bootstrap -- chicken-and-egg problem

**Source:** Backend agent, codebase analysis
**Affected:** `backend/auth/device/device_auth.go:112-157`, `backend/models/iot/device.go:27-39`

The IoT check-in flow:
1. Device sends `device_id` + `api_key` + `staff_pin`
2. Backend looks up device to validate credentials
3. Device determines which tenant the check-in belongs to

But with RLS, the device lookup (step 2) requires a tenant context. The tenant context comes from the device (step 3). Chicken-and-egg.

**Evidence:**
- `backend/models/iot/device.go:27-39` -- Device struct has **no TenantID field**
- `backend/auth/device/device_auth.go:112-157` -- `DeviceAuthenticator` uses a single global PIN, no tenant awareness

**Options the plan doesn't address:**
- (a) Exempt `iot.devices` from RLS (devices are globally unique by `device_id`)
- (b) Use `WithAdminTx` for device lookup, then switch to `WithTenantTx` for the check-in
- (c) Encode `tenant_id` in the device's API key

**Risk:** IoT check-in (the core business function) will break entirely with RLS.

---

### C4: D13 must be reversed -- per-tenant roles are required

**Source:** Business & GDPR agent (F1)
**Affected:** DEBATE.md D13, `backend/auth/jwt/claims.go:17-31`, `backend/auth/authorize/policy/interface.go:25-29`

D13 says: "YAGNI. Per-tenant roles are not implemented. Retrofitting is trivial."

This is wrong for the OGS domain. The requirements (`00-anforderungen.md` lines 49-52) define distinct role levels that coexist within the same Trager.

**Concrete scenario:** Maria is an OGS-Buero-Mitarbeiterin at OGS Altenberge (admin permissions: `users:manage`, `config:update`) and a Betreuerin at OGS Greven (read-only permissions). Under global roles, Maria gets the same permissions at both OGS. Either she's over-privileged at Greven or under-privileged at Altenberge.

**Evidence from code:**
- `backend/auth/jwt/claims.go:17-31` -- `AppClaims.Roles` is a flat `[]string`, no tenant scoping
- `backend/auth/authorize/policy/interface.go:25-29` -- `Subject.Roles` is flat, no tenant field
- `backend/auth/authorize/permission.go:12-28` -- `RequiresPermission` middleware reads permissions from JWT with no tenant context

**Why "retrofitting is trivial" is false:**
1. Change `auth.account_roles` to include `tenant_id` or use `account_tenants.role_id`
2. Change login flow to load roles per-tenant
3. Change `AppClaims` to carry `tenant_id`
4. Change token-switch to reload roles
5. Change every test that creates accounts with roles

**Risk:** Every Trager with more than one OGS will encounter this within weeks of deployment. This is the standard operating model, not an edge case.

---

### C5: Org-scope (Trager-Buero) has no solution that preserves the security model

**Source:** Business & GDPR agent (F4), Database agent (MED-5)
**Affected:** `06-offene-punkte.md` point 1, all RLS policies

Already flagged as the sole remaining "CRITICAL BLOCKER" in the plan. But the three proposed options are all problematic:

| Option | Approach | Fatal flaw |
|---|---|---|
| (a) Org-aware RLS | Subquery in every policy | Performance: subquery on every row check, prevents index-only scans on `tenant_id` |
| (b) WithAdminTx | BYPASSRLS for non-platform users | Security: bug in org-scope code = full access to ALL tenants, not just the Trager's OGS |
| (c) Restricted Admin | Same as (b) | Same: "BYPASSRLS fuer Nicht-Platform-User fragwuerdig" (the plan's own words) |

**The business requirement is non-optional** (`00-anforderungen.md` lines 79-82): Trager-Buero needs automatic access to ALL OGS of their Trager, including new ones added later, plus cross-OGS aggregated views.

**Risk:** If solved with (b) or (c), a single application bug exposes all tenants across all Trager. The entire security model of D7/D8 is voided.

---

### C6: Parent data isolation is architecturally absent

**Source:** Business & GDPR agent (F3), Frontend agent (ATTACK 9)
**Affected:** All student-facing endpoints, `06-offene-punkte.md` point 3

RLS operates at tenant level. A parent logged into OGS Altenberge can see ALL data at Altenberge -- not just their own child. The requirements say "Sehen nur Daten des eigenen Kindes."

**Evidence:**
- The only registered policy is `StudentVisitPolicy` (`backend/auth/authorize/policies/student_visit.go`). There is NO parent-specific policy.
- No mechanism exists to filter parents to their own children's data across ALL endpoints.
- `users.guardian_profiles` has `UNIQUE(email)` -- same parent at two OGS = constraint violation.

**GDPR Art. 8** requires heightened protection for children's data. Parent A seeing Child B's attendance, health notes, or emergency contacts at the same OGS is a reportable data breach under Art. 33/34. The LDI NRW has explicitly stated "besondere Sorgfalt" for children's data in educational settings.

**Risk:** Reportable GDPR violation. Fines up to EUR 20M or 4% of annual turnover (Art. 83(5)(a)).

---

### C7: GDPR Art. 17 erasure is legally impossible for shared accounts

**Source:** Business & GDPR agent (F5)
**Affected:** DEBATE.md D15, `auth.accounts`, `auth.account_tenants`

D15 decides: one account, multiple tenants, email globally UNIQUE. The deletion process:
- OGS removes staff -> `account_tenants.status = inactive` + tenant-specific data deleted
- Staff wants full deletion -> "GDPR Request an Plattform"

**Why this fails GDPR Art. 17:**

**Scenario:** Betreuerin Lisa works at OGS A (Caritas) and OGS B (AWO). Lisa quits OGS A. Caritas, as data controller (Art. 4(7)), is legally obligated to erase Lisa's data upon request.

1. The account (email, name) is in `auth.accounts` -- **global, no tenant_id**. Caritas cannot delete the account because AWO still needs it.
2. Setting `account_tenants.status = 'inactive'` is **NOT erasure**. The ECJ (C-553/07) interprets "erasure" as actual deletion, not soft-delete flags.
3. Lisa's name and email remain in a shared table Caritas cannot control.
4. **Who is the data controller for `auth.accounts`?** Independent controllers sharing a DB row = joint controller situation (Art. 26 GDPR). The plan has NO Art. 26 joint controller agreement.
5. The "GDPR Request an Plattform" escape hatch means the data subject must go to the platform operator, not their former employer. This contradicts Art. 17 (right against the controller).

**Risk:** LDI NRW finds the platform designed a system where controllers CANNOT fulfill Art. 17 obligations. Fine targets the platform operator (Art. 83(4)(a)).

---

## HIGH -- Will cause production failures or significant rework

### H1: GRANT statements missing SEQUENCE permissions -- every INSERT will fail

**Source:** Database agent (CRIT-1)
**Affected:** `02-datenbank.md` Section 4.1

The three-role setup grants table-level permissions but **omits SEQUENCE permissions**. Every table uses `BIGSERIAL PRIMARY KEY` which creates implicit sequences. Without `USAGE` on sequences, `phoenix_tenant` cannot generate IDs.

```sql
-- Missing from the plan:
GRANT USAGE ON ALL SEQUENCES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit TO phoenix_tenant;

-- Also missing (for future migrations):
ALTER DEFAULT PRIVILEGES IN SCHEMA ...
    GRANT USAGE ON SEQUENCES TO phoenix_tenant;
```

**Evidence:** Every migration uses `BIGSERIAL PRIMARY KEY`. There are 60+ sequences across the database.

**Risk:** 100% of INSERT operations fail with "permission denied for sequence."

---

### H2: UNIQUE constraints -- 13 broken (plan only found 5)

**Source:** Database agent (CRIT-3)
**Affected:** All tenant-scoped tables with UNIQUE constraints

The investigation report found 5 broken UNIQUE constraints. The actual codebase has at least 13:

| # | Table | Constraint | Migration file | What breaks |
|---|---|---|---|---|
| 1 | `education.groups` | `UNIQUE(name)` | `001002007_education_groups.go:55` | Two OGS both name a group "1a" |
| 2 | `facilities.rooms` | `UNIQUE(name)` | `001001001_facilities_rooms.go:55` | Two OGS both have "Raum 1" |
| 3 | `activities.categories` | `UNIQUE(name)` | `001003001_activities_categories.go:55` | Two OGS both have "Basteln" |
| 4 | `config.settings` | `UNIQUE(key)` | `001006001_config_settings.go:56` | Two OGS both have "school_name" |
| 5 | `users.guardian_profiles` | `UNIQUE(email)` | `001003005001_users_guardian_profiles.go:61` | Same parent at two OGS |
| 6 | `active.work_sessions` | `UNIQUE(staff_id, date)` | `001010001_create_work_sessions.go:63` | Staff at two OGS same day (D4!) |
| 7 | `schedule.pickup_schedules` | `UNIQUE(student_id, weekday)` | `001008001_create_pickup_schedules.go:52` | Transfer student, same weekday |
| 8 | `schedule.pickup_exceptions` | `UNIQUE(student_id, exception_date)` | `001008001_create_pickup_schedules.go:79` | Same pattern |
| 9 | `active.scheduled_checkouts` | `UNIQUE(student_id, status) WHERE status='pending'` | `001006007_create_scheduled_checkouts_table.go:60` | Cross-tenant checkout |
| 10 | `users.persons.tag_id` | `UNIQUE` | `001002001_users_persons.go:57` | OK if RFID globally unique |
| 11 | `iot.devices.device_id` | `UNIQUE` | `001003009_iot_devices.go:72` | OK if device globally unique |
| 12 | `iot.devices.api_key` | `UNIQUE` | `001003009_iot_devices.go:76` | OK if API key globally unique |
| 13 | `auth.accounts_parents.email` | `UNIQUE INDEX` | `001000009_auth_accounts_parents.go:70` | Same parent at different OGS |

Items 10-12 are arguably correct (global uniqueness). Items 1-9 and 13 need `UNIQUE(tenant_id, ...)`.

**Risk:** Second tenant cannot create groups, rooms, categories, or settings with common names. Hard failure on INSERT.

---

### H3: Foreign keys between tenant-scoped tables have no tenant_id guard

**Source:** Database agent (CRIT-4)
**Affected:** 40+ FK constraints across all migrations

Every FK references only `(id)`, never `(tenant_id, id)`:

```sql
-- Example from 001004002_active_visits.go:
CONSTRAINT fk_active_visits_student FOREIGN KEY (student_id)
    REFERENCES users.students(id) ON DELETE CASCADE
```

**Scenario:** Tenant B has a service bug that constructs a visit with `student_id = 42`. FK passes because student 42 exists (in Tenant A). RLS hides this from Tenant A, but the data linkage exists.

Fix requires composite FKs: `FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id)`. This in turn requires `UNIQUE(tenant_id, id)` on every referenced table.

**Risk:** Cross-tenant data linkage through application bugs. Database provides zero protection.

---

### H4: Wildcard cookie (.moto-app.de) = platform-wide XSS blast radius

**Source:** Frontend agent (ATTACK 1)
**Affected:** `04-frontend.md` Section 6 (cookie configuration)

The plan configures NextAuth cookies with `domain: .${rootDomain}` -- shared across ALL subdomains.

**Attack vectors:**
1. **XSS blast radius is infinite.** One XSS on `altenberge.moto-app.de` steals sessions valid on `greven.moto-app.de` and every other tenant.
2. **Subdomain takeover.** A dangling CNAME for `test.moto-app.de` lets an attacker read/set cookies for the entire domain tree.
3. **Cookie fixation.** Per RFC 6265, `evil.moto-app.de` can set `next-auth.session-token` with `domain=.moto-app.de`. Victim visits `altenberge.moto-app.de` and operates in the attacker's session.
4. **`__Secure-` prefix is insufficient.** It requires `Secure` flag but does NOT prevent cross-subdomain access. `__Host-` prefix would restrict to exact origin but cannot have a `domain` attribute.

**Missing from the plan:** CSP per tenant, cookie integrity verification (HMAC binding to subdomain), subdomain takeover monitoring.

**Risk:** For a GDPR-regulated system handling children's data, a platform-wide session hijacking vulnerability is catastrophic.

---

### H5: Confused deputy -- login slug vs subdomain mismatch

**Source:** Frontend agent (ATTACK 5)
**Affected:** Login flow (`04-frontend.md` Section 8)

The login POST sends `tenant_slug` from `params.tenant` (URL path). Nothing prevents:

1. Visit `altenberge.moto-app.de/login`
2. Intercept request, change body to `{ tenant_slug: "greven" }`
3. Backend returns JWT with `tenant_id` for Greven
4. URL shows Altenberge, branding shows Altenberge (from `resolveTenant`)
5. Data comes from Greven (from JWT)

**Evidence:** No backend validation exists that `tenant_slug` matches the `Origin`/`Host` header. The backend trusts the body parameter blindly.

**Risk:** UI identifies as one tenant, data comes from another. Visible to users as a data breach.

---

### H6: NextAuth JwtPayload missing tenant fields -- data loss on token refresh

**Source:** Frontend agent (ATTACK 7)
**Affected:** `frontend/src/server/auth/config.ts:11` (JwtPayload interface)

Current `JwtPayload` interface:

```typescript
interface JwtPayload {
  id: string | number;
  sub?: string;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  roles?: string[];
  is_admin?: boolean;
  // NO tenant_id, NO org_id, NO scope
}
```

The plan says the session should include `tenantId`, `orgId`, `scope`. But `parseJwtPayload()` does not extract these. After token refresh (`auth-api.ts:140`), the session loses all tenant information.

**Risk:** Every token refresh (every 15 minutes) silently drops tenant context from the session. Any component reading tenant info from the session gets `undefined`.

---

### H7: SWR cache + module cache + sessionStorage = cross-tenant data leak

**Source:** Frontend agent (ATTACK 3)
**Affected:** `frontend/src/lib/swr/hooks.ts`, `frontend/src/lib/session-cache.ts`

**821 SWR calls** across 50 files have no tenant prefix. The plan proposes `useTenantSWR` but doesn't address:

1. **Module-level session cache** (`session-cache.ts`) -- survives tenant-switch for 10 seconds, serves the OLD tenant's JWT
2. **Browser HTTP cache** -- 194 API routes have no `Vary: Authorization` or tenant-specific `Cache-Control` headers
3. **Development on localhost** -- all tenants share the same `localStorage`/`sessionStorage` origin

**Risk:** Teacher switches tenant, sees previous tenant's student data from stale cache. Visible GDPR breach.

---

### H8: 40+ hardcoded redirect paths break after restructure

**Source:** Frontend agent (ATTACK 2)
**Affected:** Multiple frontend files

| File | Code | Problem |
|---|---|---|
| `lib/api.ts:471` | `window.location.href = "/"` | Redirects to root, not tenant |
| `lib/auth-api.ts:169` | `window.location.href = "/"` | Same |
| `server/auth/config.ts:403` | `pages: { signIn: "/" }` | NextAuth sends to root on session expiry |
| `(protected)/dashboard/page.tsx:245` | `router.replace("/")` | Goes to root, not tenant root |
| `components/dashboard/sidebar.tsx:450-483` | `router.push("/ogs-groups")` etc. | Missing tenant prefix |
| `(protected)/rooms/page.tsx:147` | `router.push("/rooms/${room.id}")` | Missing tenant prefix |

The plan says "33 pages total." Actual count is 30+ directories with nested dynamic routes, plus 40+ `router.push()`/`redirect()`/`window.location.href` calls.

**Risk:** Every expired session dumps users to a tenant selection page. Every sidebar navigation breaks.

---

### H9: Ferienbetreuung uses BYPASSRLS for regular business logic

**Source:** Business & GDPR agent (F2)
**Affected:** DEBATE.md D4

D4 says: "Service holt nur die eingeschriebenen Kinder via privilegierten Read (Admin-Connection, kein RLS)."

A regular business feature (summer holiday care) runs with `phoenix_admin` (BYPASSRLS). Any bug in the Ferienbetreuung service code exposes ALL tenant data across ALL OGS.

**Unaddressed write operations:**
- Can a Betreuer at Host-OGS check in a child from Guest-OGS?
- Which `tenant_id` goes on the visit record -- host OGS or home OGS?
- Which `tenant_id` goes on the audit record?

**Cross-Trager Ferienbetreuung** (`00-anforderungen.md` line 138) means different data controllers. A Caritas Betreuer accessing AWO children requires a legal basis (Art. 6(1)(f) or Art. 28 processing agreement). No mechanism to verify agreements exist.

**Risk:** Full data breach from a single application bug. GDPR cross-controller violation.

---

### H10: `admin:*` wildcard permission bypasses all multi-tenant controls

**Source:** Business & GDPR agent (F9)
**Affected:** `backend/auth/authorize/permission.go:86-89`, `backend/auth/authorize/permissions/constants.go:28-31`

```go
func hasAdminWildcard(permissions []string) bool {
    for _, perm := range permissions {
        if perm == "admin:*" || perm == "*:*" {
            return true
        }
    }
    return false
}
```

Combined with global roles (D13): if an account has "admin" role at ANY OGS, upon tenant-switching the JWT still carries `admin:*`. All permission middleware is bypassed at the new OGS.

In `StudentVisitPolicy` (`student_visit.go:89`):
```go
if hasRole(authCtx.Subject.Roles, "admin") {
    return true  // Bypasses ALL policy checks
}
```

**Risk:** Admin at one OGS = admin everywhere. Master key that opens every door.

---

## MEDIUM -- Address during implementation

### M1: "Zero-downtime" migration claim is false

**Source:** Database agent (HIGH-3)
**Affected:** `02-datenbank.md` Section 8.1

- Step 6 (`ALTER COLUMN tenant_id SET NOT NULL`) requires `ACCESS EXCLUSIVE` lock + full table scan
- In PG 17, `ADD COLUMN ... NOT NULL DEFAULT 1` is metadata-only (no lock). The plan's three-step dance is an anti-pattern for PG 11+.
- `CREATE INDEX CONCURRENTLY` cannot run inside BUN's transactional migrations
- `CREATE INDEX CONCURRENTLY` can fail silently (creates INVALID index)

---

### M2: Trigger functions unanalyzed for RLS interaction

**Source:** Database agent (HIGH-1)
**Affected:** 4 trigger functions in migration files

Triggers like `enforce_single_primary_student_guardian()` filter by `student_id` without `tenant_id`. Under RLS this is implicitly safe, but during `phoenix_admin` operations (BYPASSRLS), the UPDATE could affect other tenants' rows.

**Files:** `001003006_users_students_guardians.go:92-105`, `001003004_activities_supervisors_planned.go:96-109`, `001002006_users_guardians.go:84-101`, `001007006_guardian_phone_numbers.go:141-171`

---

### M3: Tenant switch does not update NextAuth session

**Source:** Frontend agent (ATTACK 4)
**Affected:** Tenant-switch flow (`04-frontend.md` Section 9)

`switchTenant()` returns a new JWT but never stores it in the NextAuth session. The wildcard cookie carries the OLD session to the new subdomain. The TenantProvider shows the new tenant branding, but API calls use the OLD JWT with the OLD `tenant_id`.

**Risk:** UI says "OGS Greven" but data is from OGS Altenberge.

---

### M4: resolveTenant() DoS via wildcard DNS

**Source:** Frontend agent (ATTACK 6)
**Affected:** D17 (no cache decision)

D17 explicitly says "Kein Cache" for tenant resolution. With wildcard DNS (`*.moto-app.de`), scripted requests to random subdomains each hit the backend. No rate limiting, no caching, no WAF.

---

### M5: CVE-2025-8713 reference appears fabricated

**Source:** Database agent (HIGH-4)
**Affected:** `02-datenbank.md` Section 6.1, DEBATE.md D16

The plan references "CVE-2025-8713 | Optimizer-Statistiken leaken RLS-versteckte Daten | PG 17.6." This CVE cannot be verified. The known relevant CVE is CVE-2024-10976 (plan cache ignoring role changes, fixed in PG 17.1). If fabricated, the minimum version requirement (PG 17.6) is based on a phantom vulnerability.

---

### M6: Sequential IDs leak tenant activity

**Source:** Database agent (MED-2)
**Affected:** All tables using BIGSERIAL

Globally sequential IDs allow tenants to infer other tenants' activity volume (student ID jumps from 100 to 150 = 49 students created by others). For a GDPR-regulated system, this should be documented as an accepted risk.

---

## Summary

| Severity | Count | Top issues |
|---|---|---|
| **CRITICAL** | 7 | Transaction reconciliation, scheduler design, IoT bootstrap, per-tenant roles, org-scope, parent isolation, GDPR Art. 17 |
| **HIGH** | 10 | Sequence GRANTs, 13 broken UNIQUEs, FK without tenant_id, wildcard cookie XSS, confused deputy, NextAuth desync, SWR cache leak, hardcoded redirects, Ferienbetreuung BYPASSRLS, admin:* bypass |
| **MEDIUM** | 6 | Migration downtime, trigger analysis, tenant-switch session, DNS DoS, CVE reference, ID leakage |

### Pre-implementation checklist (blocking)

| # | Action | Relates to |
|---|---|---|
| 1 | Reconcile transaction patterns -- design how WithTenantTx interacts with RunInTx/WithTx | C1 |
| 2 | Design scheduler tenant iteration pattern | C2 |
| 3 | Solve IoT device auth bootstrap (exempt from RLS or two-phase lookup) | C3 |
| 4 | Reverse D13 -- design per-tenant roles into JWT/permission architecture | C4 |
| 5 | Solve org-scope without BYPASSRLS for non-platform users (D18 decision) | C5 |
| 6 | Design parent-level data isolation NOW (D19 decision) | C6 |
| 7 | Design GDPR Art. 17 erasure for shared accounts (Art. 26 joint controller agreement) | C7 |
| 8 | Add SEQUENCE GRANTs + ALTER DEFAULT PRIVILEGES to migration plan | H1 |
| 9 | Full audit of all UNIQUE constraints -- convert to composite `(tenant_id, ...)` | H2 |
| 10 | Design backend validation of `tenant_slug` vs `Origin`/`Host` header | H5 |

---

*Adversarial review by @yonnock, 2026-02-09*
*Analysis method: 4x Opus 4.6 devil's advocate agents, total ~900K tokens, ~15 min runtime*
