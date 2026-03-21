# 12 — Devil's Advocate Analysis (4-Agent Adversarial Audit)

> Four specialized agents simultaneously attacked the multi-tenancy plan
> from different angles: Database/RLS, Backend Go/BUN, Frontend/Auth,
> and Business/GDPR. This document consolidates their findings.
>
> Stand: 2026-02-11

---

## Executive Summary

**4 agents, 4 attack dimensions, 40+ unique findings.**

| Agent | CRITICAL | HIGH | MEDIUM | LOW |
|-------|----------|------|--------|-----|
| Agent 1: Database/RLS | 3 | 4 | 3 | 1 |
| Agent 2: Backend Go/BUN | 3 | 5 | 2 | 0 |
| Agent 3: Frontend/Auth | 2 | 5 | 2 | 0 |
| Agent 4: Business/GDPR | 7 | 6 | 0 | 0 |
| **Deduplicated Total** | **10** | **14** | **5** | **1** |

After deduplication (several agents independently discovered the same issues), there are **30 unique findings**: 10 CRITICAL, 14 HIGH, 5 MEDIUM, 1 LOW.

**Top 3 Showstoppers:**
1. **BUN vs base TxFromContext key mismatch** — migration CANNOT be incremental (Agent 2, AV8)
2. **Trigger function `ensure_single_primary_supervisor` bypasses RLS** — confirmed cross-tenant data corruption (Agent 1, AV1)
3. **Wildcard cookie `.moto-app.de` without CSP** — session theft from any compromised subdomain (Agent 3, AV1)

---

## Part 1: CRITICAL Findings

### CRIT-1: BUN vs base.TxFromContext Use Different Context Keys (Agent 2)

**The single most dangerous finding across all 4 agents.**

The planned `WithTenantTx` uses `db.RunInTx()` (BUN's method), which stores the transaction using BUN's internal context key. But the existing `TxHandler.GetTx()` at `models/base/transaction.go:74` checks `base.TxFromContext(ctx)` which uses a **different** key (`txKey{}`).

This means: if a handler wraps a call in `WithTenantTx`, and the service inside still uses `s.txHandler.RunInTx()`, the service will NOT find the handler's transaction. It creates a NEW transaction on `r.DB` (as `phoenix_auth`, which has zero privileges) and hard-fails.

**Impact:** All 52 `RunInTx` + 110 `WithTx` calls must be refactored **atomically** with the handler migration. Partial/incremental migration creates hard failures at every seam between old and new transaction patterns.

**Fix Required:** Either:
- (A) Modify `base.TxFromContext` to check BOTH keys (backward-compatible bridge), OR
- (B) Rewrite `WithTenantTx` to store tx under both keys during migration, OR
- (C) Accept that the migration is atomic — all 52+110+538 call sites in one sprint

---

### CRIT-2: Trigger Function Bypasses RLS — Confirmed Cross-Tenant Data Corruption (Agent 1)

`activities.ensure_single_primary_supervisor()` at `database/migrations/001003004_activities_supervisors_planned.go:96-109`:

```sql
UPDATE activities.supervisors
SET is_primary = FALSE
WHERE group_id = NEW.group_id AND id != NEW.id;
```

PostgreSQL trigger functions execute with the **trigger owner's** privileges (the migration superuser), NOT the calling session's role. This UPDATE hits ALL rows across ALL tenants where `group_id` matches, bypassing RLS entirely.

**Concrete Exploit:** Tenant A inserts a primary supervisor. The trigger fires with superuser privileges and demotes Tenant B's primary supervisor for any group with the same `group_id`.

**Fix Required:** Add `AND tenant_id = NEW.tenant_id` to all trigger WHERE clauses, or use `SECURITY INVOKER` (PostgreSQL 17+).

Other triggers analyzed and found safe: `set_privacy_consent_expiration()`, `update_modified_column()`, `education.update_grade_transitions_updated_at()` (all only modify `NEW`).

---

### CRIT-3: View Without `security_invoker` Bypasses RLS (Agent 1)

`users.expired_privacy_consents` view at `database/migrations/001003007_users_privacy_consents.go:146-158` was created WITHOUT `security_invoker = true`. The plan doc 02-datenbank.md section 5.3 identifies this problem but the implementation plan does NOT schedule the fix in any phase.

Without `security_invoker`, the view executes as the owner (superuser with BYPASSRLS). Any query against this view returns ALL tenants' expired consent data including guardian names, emails, and phones.

**Fix Required:** Update view with `security_invoker = true` BEFORE Phase 1 RLS rollout, not during Phase 3.

---

### CRIT-4: Base Repository Uses `r.DB` Directly — 45+ Repos Affected (Agents 1 + 2)

`database/repositories/base/base.go` uses `r.DB` directly in ALL 7 methods (Create, FindByID, Update, Delete, List, Count, Transaction). The planned `getDB(ctx)` migration covers the 531 domain-repository `r.db.` calls but does not address the 7 base methods that serve as the foundation for 45+ repositories.

Additionally, `base.Repository.Transaction()` at line 231 starts its own `RunInTx` on `r.DB`, creating a non-tenant-scoped transaction.

**Impact:** After role switch to `phoenix_auth` (NOINHERIT, zero privileges), every call to `base.Repository.Create()`, `FindByID()`, etc. will hard-fail with "permission denied". This is a total service outage, not a data leak.

---

### CRIT-5: 18 Direct `s.db.` Calls in Service Layer Bypass Transactions (Agent 2)

18 non-test occurrences of `s.db.` in service files bypass both the repository layer and the context-based transaction:

| File | Count | Risk |
|------|-------|------|
| `services/active/cleanup_service.go` | **10** | GDPR cleanup queries cross-tenant |
| `services/facilities/facility_service.go` | 3 | Room queries unscoped |
| `services/active/session_service.go` | 2 | Orphaned supervisor cleanup |
| `services/active/dashboard_helpers.go` | 2 | Dashboard queries |
| `services/activities/supervisor_operations.go` | 1 | Supervisor operations |

The `cleanup_service.go` chain is worst: Scheduler -> `EndDailySessions()` -> `cleanupOrphanedSupervisors()` -> `s.db.NewSelect()` — even if wrapped in `WithTenantTx`, the `s.db` call at the innermost level bypasses it.

---

### CRIT-6: Wildcard Cookie Without CSP Creates Exploitable Window (Agent 3)

The plan proposes cookie domain `.moto-app.de` (04-frontend.md Section 6) — intentionally shared across all tenant subdomains. But:

1. **No CSP exists today**: `middleware.ts` has zero CSP headers
2. **CSP is not scheduled**: Implementation plan (`11-implementation-plan-2-devs.md`) does not mention CSP in any week
3. **No subdomain takeover monitoring**: Finding H-7 from audit remains OPEN

Any XSS on any subdomain or subdomain takeover (e.g., deactivated tenant) steals ALL user sessions. The cookie is architecturally designed to be readable by every subdomain.

**Operator cookies** at `lib/operator/cookies.ts` are also unprotected — no domain scoping mentioned in the plan.

---

### CRIT-7: `admin:*` Wildcard Bypasses ALL Permission Checks (Agent 4)

`auth/authorize/permission.go` lines 110-118: `hasAdminWildcard()` returns `true` unconditionally — no tenant check. Combined with the `Subject` struct at `auth/authorize/policy/interface.go:25-29` having no `TenantID` field, an admin account can bypass ALL authorization regardless of tenant.

The `Resource` struct (lines 19-22) also has no `TenantID` field. Only `StudentVisitPolicy` is registered in the policy registry — all other resource types have no Tier 2 authorization.

---

### CRIT-8: Cross-Tenant Consent Revocation Not Propagated — Ferienbetreuung (Agent 4)

When a guardian revokes photo/video consent at their home OGS (Tenant A), the `PrivacyConsent.Revoke()` method sets `Accepted = false` on the local record. But if the student is enrolled in a Ferienbetreuung group at Tenant B via `platform.cross_tenant_access`, Tenant B still holds a stale `Accepted = true` consent — or has no consent record at all (the plan doesn't specify how consent propagates cross-tenant).

**GDPR Risk:** Ferienbetreuung staff at Tenant B photograph a child whose guardian just revoked consent at Tenant A. This is a DSGVO violation that creates liability for both Traeger.

---

### CRIT-9: No Cascade Deactivation for Traeger Bankruptcy (Agent 4)

If a Traeger (carrier organization managing multiple schools) goes bankrupt or loses their contract, there is no specification for:
- Deactivating ALL tenants under that Traeger simultaneously
- Revoking all JWT tokens for affected accounts
- Handling cross-tenant access records that reference the bankrupt Traeger's schools
- Data retention/deletion obligations under DSGVO Art. 17

The plan specifies Traeger-level OrgScope (D18) but not Traeger-level deactivation.

---

### CRIT-10: Art. 17 Erasure Doesn't Cover Cross-Tenant Visit Records (Agent 4)

When a student's data retention period expires at their home OGS, the cleanup service at `services/active/cleanup_service.go` deletes their visit records. But if the student attended a Ferienbetreuung session at another tenant, those visit records (created via `WithAdminTx`) exist in the other tenant's scope. The cleanup service operates per-tenant — it will never find or delete cross-tenant records.

---

## Part 2: HIGH Findings

### HIGH-1: IoT Device Authentication Has No Tenant Boundary (Agents 1 + 2)

The device auth at `auth/device/device_auth.go` uses a single global `OGS_DEVICE_PIN` from environment. After multi-tenancy, each tenant needs its own PIN. The plan's D20 "Two-Phase Lookup" is mentioned but unspecified.

**RFID Cross-Tenant Risk:** If an RFID tag from Tenant B's student is scanned at Tenant A's device, the system could create a visit record linking Tenant B's student to Tenant A's group — cross-tenant data corruption.

**Additionally (Agent 2):** Phase 1 uses `WithAdminTx` (BYPASSRLS) for device lookup. This grants full BYPASSRLS access during the device authentication phase — far more privilege than needed.

---

### HIGH-2: Cross-Schema JOINs During Phased RLS Rollout (Agent 1)

Multiple repository methods perform JOINs across schemas. During Phase 1-2, if `active.visits` gets its real RLS policy before `users.privacy_consents`, the GDPR retention query (`GetVisitRetentionStats`) correctly filters visits by tenant but joins against ALL students' privacy consents.

The implementation plan does NOT specify the order in which 58 tables get their real RLS policies in Phase 2. The order matters for every cross-schema join.

---

### HIGH-3: SSE Hub Has Zero Tenant Isolation (Agents 1 + 2 + 3)

All three agents independently identified this. The Hub at `realtime/hub.go` is a global singleton with no `TenantID` field on `Client`. Key issues:

1. **No `verifyGroupTenant()` function defined** — the plan proposes it but the Hub has no DB access (circular dependency with factory)
2. **SSE connections are long-lived** — holding a DB transaction open for SSE duration exhausts the connection pool; the plan does not address this
3. **Frontend**: `useGlobalSSE()` at `lib/hooks/use-global-sse.ts` connects once with no tenantId dependency — no reconnection on tenant switch
4. **SSE cache invalidation** uses raw `mutate()` with string prefix matching that will break when keys become tenant-prefixed

---

### HIGH-4: Goroutine Context Escape — Auth Event Logging + Guardian Email (Agent 2)

Two fire-and-forget goroutine patterns will break:

1. **Auth event logging** at `services/auth/token_cleanup.go:79-89`: Uses `context.WithoutCancel(ctx)` which preserves the dead transaction reference. After the handler returns and tx commits, the goroutine tries to use a dead transaction.

2. **Guardian email** at `services/users/guardian_service.go:315+`: Uses `context.Background()` with no tenant context. Three database operations (get student names, update email status, dispatch email) all hit `phoenix_auth` (no rights).

---

### HIGH-5: Migration Race Condition — `DEFAULT 1` Window (Agent 1)

Between adding `tenant_id NOT NULL DEFAULT 1` (Weeks 1-2) and removing the default (Phase 3, Week 6), any code path that forgets to set `tenant_id` silently assigns data to Tenant 1.

**Worse:** If RLS is enabled BEFORE `DEFAULT 1` is removed, an INSERT that relies on the default creates the row in Tenant 1, but the session (with `app.current_tenant_id` set to a different tenant) cannot see the row it just created. Silent data loss.

---

### HIGH-6: Frontend/Backend Timeline Gap Breaks Integration (Agent 3)

Backend changes JWT and login API in Weeks 1-3; frontend changes start Week 4. During Weeks 1-3:
- `performLogin()` at `config.ts:93-135` sends `{ email, password }` with no `tenant_slug`
- Backend will require `tenant_slug` after Week 2
- Application is unusable for 2-3 weeks unless backward compatibility is maintained

---

### HIGH-7: Two-Tab Attack — Cross-Tenant JWT Desynchronization (Agent 3)

Wildcard cookie means all tabs share one session. If a user opens `tenant-a.moto-app.de` in Tab 1 and `tenant-b.moto-app.de` in Tab 2:
- Tab 1 triggers token refresh, writes Tenant A's JWT to session
- Tab 2's next `getSession()` call returns Tenant A's token
- Tab 2 makes API calls with Tenant A's JWT while displaying Tenant B's URL

Token refresh race condition: during tenant switch, if an in-flight request triggers `handleAuthFailure()` (at `lib/auth-api.ts:91-179`), it refreshes with the OLD refresh token, writing the old tenant's JWT — racing with the new tenant's `signIn`.

---

### HIGH-8: Session Cache and SWR Lack Tenant Awareness (Agent 3)

1. **Session cache** at `lib/session-cache.ts` is a module-level singleton with no tenant field. 15 API client files import it.
2. **`keepPreviousData: true`** in SWR config (at `lib/swr/config.ts:38`) shows OLD tenant's data while fetching new tenant's data — cross-tenant data flash.
3. **408 `mutate()` calls** across 42 files need audit for tenant-prefixed key patterns.

---

### HIGH-9: Redirect Count Undercounted 10x (Agent 3)

The plan documents 6 redirect locations that need tenant-prefix updates. Actual count:
- **54 `router.push`/`router.replace` calls** across 30 `.tsx` files
- **8 `window.location.href`/`assign`/`replace` calls** across 5 files
- **Total: 62 redirect locations**, not 6

---

### HIGH-10: Scheduler Creates `context.Background()` — No Tenant Context (Agent 2)

`services/scheduler/scheduler.go` uses `context.Background()` at 5 call sites. After migration, every scheduler-invoked service method hits `phoenix_auth` (no rights).

The plan says scheduler should iterate tenants with `WithTenantTx` each, but `EndDailySessions()` operates globally — lists ALL active groups, not per-tenant. Refactoring to per-tenant is required but not specified.

---

### HIGH-11: `WithTx` Pattern Doesn't Propagate Cross-Service Dependencies (Agent 2)

`services/active/active_service.go:157-245`: `service.WithTx()` creates a transactional clone but propagates `educationService` and `usersService` WITHOUT wrapping them in the transaction. After migration, calls to `s.educationService` from within the active service use the non-transactional pool connection.

This disappears only if the `WithTx` pattern is **fully abandoned** in favor of `getDB(ctx)`. Any service still using `WithTx` while another uses `getDB(ctx)` creates split-brain: some repos on tx, others on pool.

---

### HIGH-12: Ferienbetreuung Teachers See Full Student Profiles via WithAdminTx (Agent 4)

The plan specifies that Ferienbetreuung uses `WithAdminTx` for privileged cross-tenant reads. But `WithAdminTx` bypasses RLS entirely, meaning Ferienbetreuung teachers can see full student profiles including `HealthInfo`, `GuardianName`, `GuardianContact`, `GuardianEmail`, `GuardianPhone`, `ExtraInfo`, `SupervisorNotes` (all from `models/users/student.go`).

DSGVO data minimization principle requires only the fields necessary for the Ferienbetreuung purpose — name, class, emergency contact at most.

---

### HIGH-13: Active Domain Timing Conflict in Implementation Plan (Agent 4)

Dev 2's active domain (Week 3) depends on Dev 1's Policy Engine (also Week 3). The policy engine provides tenant-scoped authorization that the active service needs. If Dev 1 hasn't completed the policy engine by the time Dev 2 starts the active domain, Dev 2 is blocked.

---

### HIGH-14: Cross-Carrier AVV Not Specified for Ferienbetreuung (Agent 4)

Ferienbetreuung requires a data processing agreement (Auftragsverarbeitungsvertrag/AVV) between the two Traeger. The technical implementation exists (`platform.cross_tenant_access`) but the legal/contractual framework is completely unspecified. Without this, the cross-tenant access is illegal under DSGVO Art. 28.

---

## Part 3: MEDIUM Findings

### MED-1: Raw `ExecContext` SQL Bypasses BUN Model Mapping (Agent 1)

Two raw SQL paths will miss `tenant_id` filters:
1. `database/repositories/auth/token.go:116` — `DELETE FROM auth.tokens WHERE expiry < ?` (no tenant filter, uses `r.db` directly)
2. `database/repositories/education/grade_transition.go:520-528` — Raw SQL UPDATE joins `users.students` with `education.grade_transition_mappings` without tenant filter

The plan's "Repository: Alle Queries + WHERE tenant_id = ?" covers BUN queries but may miss raw `ExecContext` calls.

---

### MED-2: `iot.devices` UNIQUE Constraints Need Tenant Scoping (Agent 1)

`device_id TEXT NOT NULL UNIQUE` at `database/migrations/001003009_iot_devices.go:72` prevents re-registering a device at a new tenant after decommissioning at the old tenant. The plan classifies this as "OK — Keine Aenderung noetig" but `device_id` should be `UNIQUE(tenant_id, device_id)`.

---

### MED-3: Advisory Lock Without Tenant ID (Agent 2)

`services/active/session_service.go:168`: `SELECT pg_advisory_xact_lock(?)` uses single-argument form. Two tenants with the same `activityID` block each other. Only 1 call site, but migration checklist doesn't mention it explicitly.

---

### MED-4: Tenant Slug Collision with Existing Routes (Agent 3)

The `RESERVED_SUBDOMAINS` list (`['www', 'api', 'admin', 'operator', 'app']`) is incomplete. Missing: `reset-password`, `invite`, `_next`, `static`, `favicon.ico`, `images`, and all route group names. A tenant named "reset-password" would collide with `app/reset-password/page.tsx`.

---

### MED-5: Connection Pool Poisoning via Panic — Low Residual Risk (Agent 1)

Analyzed in detail. If `fn()` panics after `SET LOCAL ROLE phoenix_tenant`, BUN calls `Rollback()`. If rollback succeeds, connection returns clean. If rollback fails, the next `WithTenantTx` overrides stale state. The NULLIF fail-closed policy protects against empty `app.current_tenant_id`. **Residual risk is low.**

---

## Part 4: LOW Findings

### LOW-1: Sequence/ID Leakage via BIGSERIAL Gaps (Agents 1 + 4)

Global sequences allow tenants to infer each other's activity volume from ID gaps. For German Landesdatenschutzbeauftragte (state data protection authorities), the ability to infer attendance patterns of other schools may be a concern. The plan dismisses this as acceptable.

---

## Part 5: Observations (Not Attack Vectors, But Risks)

### OBS-1: Migration Scope Is Larger Than Documented

| Item | Plan's Count | Actual Count | Status |
|------|-------------|-------------|--------|
| `r.db.` in repositories | 538 | 531 + 7 base = 538 | Correct |
| `RunInTx` in services | ~51 | 52 | Correct |
| `WithTx` calls | ~110 | 110+ | Correct |
| `s.db.` direct SQL in services | **Not counted** | **18** | **MISSING** |
| `context.Background()` in services | **Not counted** | **10** | **MISSING** |
| Goroutines with wrong context | **Not counted** | **3** | **MISSING** |
| SWR hook calls (actual) | 821 | **27** | **Overcounted** |
| `mutate()` calls needing audit | **Not counted** | **408** | **MISSING** |
| Redirect locations | 6 | **62** | **Undercounted 10x** |
| Session cache consumers | **Not counted** | **15** | **MISSING** |

### OBS-2: `platform.announcements.target_school_ids` Is BIGINT[]

PostgreSQL RLS cannot filter array elements. An announcement targeted to `{1, 3}` appears for Tenant 2 because `platform.announcements` has no RLS.

### OBS-3: Composite FK Ordering Dependency Between Devs

Dev 2 creates composite FKs in Week 2 that require `UNIQUE(tenant_id, id)` on target tables that Dev 1 owns. If Dev 1 hasn't created these indexes yet, Dev 2's migrations fail. The plan says "Merge-Reihenfolge: Stream A vor Stream B" but doesn't explicitly assign the composite UNIQUE indexes.

### OBS-4: Operator Pages Need SWR Exemption

4 files import `useSWR` directly from `"swr"` (3 operator pages + 1 announcements hook). These are platform-scoped, NOT tenant-scoped. The proposed ESLint `no-restricted-imports` rule would either break the build or require exemptions that undermine the rule.

---

## Cross-Agent Convergence (Independently Discovered by Multiple Agents)

These findings were independently identified by 2+ agents, increasing confidence:

| Finding | Agents | Confidence |
|---------|--------|------------|
| SSE Hub has zero tenant isolation | 1, 2, 3 | **Very High** |
| Base repository `r.DB` bypasses transactions | 1, 2 | **Very High** |
| IoT device auth has no tenant boundary | 1, 2, 4 | **Very High** |
| Cleanup service `s.db` bypasses transactions | 2, 4 | **Very High** |
| Policy engine Subject/Resource has no TenantID | 2, 4 | **High** |
| Sequence/ID leakage | 1, 4 | **Medium** |

---

## Prioritized Fix List

### Before ANY Code Is Written (Blocker)

| # | Finding | Fix |
|---|---------|-----|
| 1 | CRIT-1: TxFromContext key mismatch | Design bridge pattern or accept atomic migration |
| 2 | CRIT-2: Trigger RLS bypass | Add `AND tenant_id = NEW.tenant_id` to trigger |
| 3 | CRIT-3: View without security_invoker | Add `security_invoker = true` to view migration |
| 4 | CRIT-4: Base repo `r.DB` | Add `getDB(ctx)` to base repo 7 methods |
| 5 | CRIT-6: Wildcard cookie + CSP | Add CSP to implementation timeline |
| 6 | CRIT-7: admin:* bypass | Add tenant check to `hasAdminWildcard()` |
| 7 | HIGH-5: DEFAULT 1 ordering | Specify exact RLS-before-default-removal order |
| 8 | HIGH-6: Frontend/backend timeline gap | Add backward-compatible login or parallel frontend work |

### During Phase 0 (Foundation, Before Parallel Work)

| # | Finding | Fix |
|---|---------|-----|
| 9 | CRIT-5: 18 `s.db.` in services | Inventory and assign to dev streams |
| 10 | HIGH-1: IoT device auth | Specify D20 two-phase lookup in detail |
| 11 | HIGH-2: Cross-schema JOIN order | Document exact RLS rollout order for 58 tables |
| 12 | HIGH-3: SSE Hub isolation | Design tenant-aware Hub (address circular dependency) |
| 13 | HIGH-10: Scheduler tenant iteration | Design per-tenant scheduler pattern |
| 14 | MED-4: Reserved subdomains | Complete the list with ALL root-level routes |

### During Implementation

| # | Finding | Fix |
|---|---------|-----|
| 15 | HIGH-4: Goroutine context escape | Fix 3 goroutine patterns (auth event, guardian email) |
| 16 | HIGH-7: Two-tab attack | Design per-tab tenant isolation or document as known limitation |
| 17 | HIGH-8: Session cache | Add tenantId to session cache and all 15 consumers |
| 18 | HIGH-9: 62 redirect locations | Audit all 62, not just 6 |
| 19 | HIGH-11: WithTx cross-service | Ensure full WithTx abandonment in migration |
| 20 | MED-1: Raw ExecContext | Add tenant_id to 2 raw SQL queries |
| 21 | MED-2: iot.devices UNIQUE | Change to UNIQUE(tenant_id, device_id) |
| 22 | MED-3: Advisory lock | Use two-argument form |

### Requires Legal/Business Decision

| # | Finding | Fix |
|---|---------|-----|
| 23 | CRIT-8: Cross-tenant consent | Define consent propagation for Ferienbetreuung |
| 24 | CRIT-9: Traeger deactivation | Specify cascade deactivation process |
| 25 | CRIT-10: Cross-tenant erasure | Extend cleanup to cross-tenant records |
| 26 | HIGH-12: Data minimization | Define minimal field set for Ferienbetreuung |
| 27 | HIGH-14: AVV for Ferienbetreuung | Create legal framework template |

---

## Verdict

The **architectural foundation is sound**: Shared Schema + RLS, three-role PostgreSQL model, NOINHERIT defaults, NULLIF fail-closed policies. The security model correctly identifies the major attack surfaces.

**The gap is between the plan and the actual codebase.** The most dangerous issues are:

1. **CRIT-1 (TxFromContext key mismatch)** — This makes incremental migration impossible. The plan assumes gradual rollout but the two transaction systems (`base.TxHandler` vs `bun.RunInTx`) use incompatible context keys. The entire 52+110+538 call site migration may need to be atomic.

2. **CRIT-2 + CRIT-3 (Trigger + View)** — These are existing code that will be carried forward into multi-tenancy. They are particularly insidious because queries "work" but return cross-tenant data. Standard testing won't catch them (you need multi-tenant test data to detect the leak).

3. **CRIT-6 + HIGH-7 (Cookie + Two-Tab)** — The wildcard cookie is a design requirement for tenant switching UX, but without CSP, subdomain monitoring, and tab isolation, it becomes a systemic vulnerability.

**Recommendation:** Resolve the 8 "Before ANY Code" items (~3-5 days of documentation and design work) before starting Phase 0 implementation. The TxFromContext bridge pattern (CRIT-1) is the highest-priority architectural decision — it determines whether the migration can be phased or must be atomic.

---

## Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-11 | 4-Agent adversarial audit: Database/RLS, Backend Go/BUN, Frontend/Auth, Business/GDPR |
