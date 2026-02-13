# Phase 2: Database Expand — Implementation Summary

**Status:** Complete
**Branch:** `feature/multi-tenancy-phase-2`
**Date:** 2026-02-13
**Implements:** Work Packages 2.1–2.9 from [11-implementierungsplan.md](11-implementierungsplan.md)
**Specifications:** [02-datenbank.md](02-datenbank.md) §2–§4, [03-backend.md](03-backend.md) §12, DEBATE decisions D7, D8, D13, D15, D16, recommendations FIX-9

---

## Overview

Phase 2 is the **Database Expand** step of the Expand-Contract migration pattern. It adds a `tenant_id` column to all 58+1 tenant-scoped tables with `DEFAULT 1`, so existing code continues working unchanged. No behavioral changes — existing tests, seeds, and API calls are fully backward-compatible.

The "Contract" step (dropping `DEFAULT 1`, enabling RLS, requiring explicit `tenant_id`) happens in Phase 4, after all code paths have been updated in Phase 3.

---

## What Was Built

### WP 2.1 — PostgreSQL Roles (Migration V1.14.1)

**File:** `backend/database/migrations/001014001_create_tenant_roles.go`

Creates the three-role architecture specified by DEBATE decisions D7 and D8:

| Role | Type | Purpose |
|------|------|---------|
| `phoenix_auth` | LOGIN, NOINHERIT | Connection role — zero privileges by default, switches to tenant/admin via `SET ROLE` |
| `phoenix_tenant` | NOLOGIN | Subject to RLS — CRUD on all tenant-scoped schemas, SELECT on `platform.schools` |
| `phoenix_admin` | NOLOGIN, BYPASSRLS | Operator routes, migrations, seeds, cross-tenant operations |

**Grants:**
- `phoenix_auth` → can `SET ROLE` to either `phoenix_tenant` or `phoenix_admin`
- `phoenix_tenant` → USAGE + CRUD on 12 schemas (`auth`, `users`, `education`, `facilities`, `activities`, `active`, `schedule`, `iot`, `feedback`, `config`, `suggestions`, `audit`) + SELECT on `platform.schools` + SEQUENCE usage
- `phoenix_admin` → ALL on all 14 schemas (including `platform` and `meta`)
- `ALTER DEFAULT PRIVILEGES` set for both roles to cover future tables/sequences

**Note:** The application does NOT connect as `phoenix_auth` yet — that migration happens in Phase 3/4. The `phoenix_auth` password is read from the `PHOENIX_AUTH_PASSWORD` environment variable at migration time, falling back to `phoenix_auth_dev` for local development.

**Rollback:** Revokes all grants and default privileges, then drops all three roles.

---

### WP 2.2–2.4 — Add `tenant_id` to 58+1 Tables (Migration V1.14.2)

**File:** `backend/database/migrations/001014002_add_tenant_id_to_all_tables.go`

The largest migration in the project. For each of 58 tables, uses the **safe 5-step pattern** to add a NOT NULL column without blocking:

```
Step 1: ADD COLUMN tenant_id BIGINT DEFAULT 1 REFERENCES platform.schools(id)
        (Instant metadata change in PG 11+)
Step 2: UPDATE SET tenant_id = 1 WHERE tenant_id IS NULL
        (Instant since column was just added with DEFAULT)
Step 3: ADD CONSTRAINT chk_{table}_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID
        (Instant, no table scan)
Step 4: VALIDATE CONSTRAINT chk_{table}_tenant_id_not_null
        (Scan, but no exclusive lock)
Step 5: ALTER COLUMN tenant_id SET NOT NULL
        (Instant when valid CHECK exists — PG 12+)
```

**Special case — `auth.roles` (Decision D13):**
- Gets NULLABLE `tenant_id` (no DEFAULT, no NOT NULL)
- System roles (admin, user, guest, guardian) keep `NULL` = globally visible
- Tenant-specific custom roles will have `tenant_id` set when created

**Complete table breakdown (58 NOT NULL + 1 NULLABLE):**

| Schema | Tables | Count |
|--------|--------|-------|
| auth | tokens, invitation_tokens, accounts_parents, guardian_invitations, account_roles, account_permissions | 6 |
| users | rfid_cards, persons, profiles, staff, teachers, guests, persons_guardians, students, guardian_profiles, students_guardians, privacy_consents, guardian_phone_numbers | 12 |
| education | groups, group_teacher, group_substitution, grade_transitions, grade_transition_mappings, grade_transition_history | 6 |
| facilities | rooms | 1 |
| activities | categories, groups, schedules, supervisors_planned, student_enrollments | 5 |
| active | groups, visits, group_supervisors, combined_groups, group_mappings, attendance, scheduled_checkouts, work_sessions, work_session_breaks, staff_absences | 10 |
| schedule | timeframes, dateframes, recurrence_rules, student_pickup_schedules, student_pickup_exceptions, student_pickup_notes | 6 |
| iot | devices | 1 |
| feedback | entries | 1 |
| config | settings | 1 |
| suggestions | posts, votes, comments, comment_reads, post_reads | 5 |
| audit | data_deletions, auth_events, data_imports, work_session_edits | 4 |
| **Total NOT NULL** | | **58** |
| auth | roles (NULLABLE) | **+1** |

**Tables NOT getting `tenant_id` (verified per spec):**
- `auth.accounts` — global identity, mapped via `account_tenants` (D15)
- `auth.permissions` — system-wide permission definitions
- `auth.role_permissions` — scoped by role FK
- `auth.password_reset_tokens` — per-account, not per-tenant
- `auth.password_reset_rate_limits` — per-account rate limiting
- `platform.*` — platform tables are above the tenant boundary
- `suggestions.operator_comments` — operator is platform-scope (already replaced by unified comments)
- `meta.migration_metadata` — infrastructure

**Rollback:** `ALTER TABLE ... DROP COLUMN IF EXISTS tenant_id CASCADE` for each table.

---

### WP 2.5 — Migrate UNIQUE Constraints (Migration V1.14.3)

**File:** `backend/database/migrations/001014003_migrate_unique_constraints.go`

Migrates existing UNIQUE constraints to include `tenant_id`, enabling the same name/key/email to exist in different tenants.

#### 13 Functionally Necessary Changes (§2.4.1)

These prevent cross-tenant collisions for user-facing identifiers:

| # | Table | Old Constraint | New Index |
|---|-------|---------------|-----------|
| 1 | facilities.rooms | `rooms_name_key` | `idx_rooms_tenant_name(tenant_id, name)` |
| 2 | education.groups | `groups_name_key` | `idx_groups_tenant_name(tenant_id, name)` |
| 3 | activities.categories | `categories_name_key` | `idx_categories_tenant_name(tenant_id, name)` |
| 4 | config.settings | `settings_key_key` | `idx_settings_tenant_key(tenant_id, key)` |
| 5 | users.persons | `persons_account_id_key` | `idx_persons_tenant_account(tenant_id, account_id)` |
| 6 | users.persons | `persons_tag_id_key` | `idx_persons_tenant_tag(tenant_id, tag_id)` |
| 7 | users.profiles | `profiles_account_id_key` | `idx_profiles_tenant_account(tenant_id, account_id)` |
| 8 | users.guardian_profiles | `guardian_profiles_email_key` | `idx_guardian_profiles_tenant_email(tenant_id, email)` |
| 9 | users.guardian_profiles | `guardian_profiles_account_id_key` | `idx_guardian_profiles_tenant_account(tenant_id, account_id)` |
| 10 | auth.accounts_parents | `idx_accounts_parents_email` | `idx_accounts_parents_tenant_email(tenant_id, email)` |
| 11 | auth.accounts_parents | `accounts_parents_username_key` | `idx_accounts_parents_tenant_username(tenant_id, username)` |
| 12 | auth.account_roles | `idx_account_roles_account_role` | `idx_account_roles_tenant(account_id, role_id, tenant_id)` |
| 13 | auth.account_permissions | `idx_account_permissions_account_permission` | `idx_account_permissions_tenant(account_id, permission_id, tenant_id)` |

#### auth.roles Special Case (D13 — Nullable tenant_id)

```sql
-- System roles (NULL tenant_id) must be globally unique
CREATE UNIQUE INDEX idx_roles_name_system ON auth.roles(name) WHERE tenant_id IS NULL;
-- Tenant-specific roles must be unique per tenant
CREATE UNIQUE INDEX idx_roles_name_tenant ON auth.roles(tenant_id, name) WHERE tenant_id IS NOT NULL;
```

#### 18 Defense-in-Depth Changes (§2.4.2)

These add `tenant_id` to uniqueness for cross-tenant safety, even though FKs already scope the data:

| # | Table | Old Constraint | New Index |
|---|-------|---------------|-----------|
| 1 | users.staff | `staff_person_id_key` | `idx_staff_tenant_person` |
| 2 | users.teachers | `teachers_staff_id_key` | `idx_teachers_tenant_staff` |
| 3 | users.guests | `guests_staff_id_key` | `idx_guests_tenant_staff` |
| 4 | users.students | `students_person_id_key` | `idx_students_tenant_person` |
| 5 | users.students_guardians | `unique_student_guardian` | `idx_students_guardians_tenant` |
| 6 | users.persons_guardians | `unique_person_guardian_relationship` | `idx_persons_guardians_tenant` |
| 7 | education.group_teacher | `uk_group_teacher` | `idx_group_teacher_tenant` |
| 8 | education.group_substitution | `idx_no_duplicate_group_transfers` (partial) | `idx_no_duplicate_group_transfers_tenant` (partial) |
| 9 | education.grade_transition_mappings | `grade_transition_mappings_transition_id_from_class_key` | `idx_grade_transition_mappings_tenant` |
| 10 | activities.student_enrollments | `uk_student_activity_enrollment` | `idx_student_enrollments_tenant` |
| 11 | activities.supervisors_planned | `uq_activity_supervisors_staff_group` | `idx_supervisors_planned_tenant` |
| 12 | activities.schedules | `idx_activity_schedules_unique` (partial) | `idx_activity_schedules_tenant_unique` (partial) |
| 13 | active.group_supervisors | `unique_active_staff_group_role` (partial) | `unique_active_staff_group_role_tenant` (partial) |
| 14 | active.group_mappings | `uq_active_group_mappings` | `idx_group_mappings_tenant` |
| 15 | active.scheduled_checkouts | `idx_scheduled_checkouts_unique_pending` (partial) | `idx_scheduled_checkouts_tenant_pending` (partial) |
| 16 | schedule.student_pickup_schedules | `unique_student_weekday` | `idx_pickup_schedules_tenant` |
| 17 | schedule.student_pickup_exceptions | `unique_student_exception_date` | `idx_pickup_exceptions_tenant` |
| 18 | suggestions.votes | `votes_post_id_voter_id_key` | `idx_votes_tenant` |

#### FIX-9: IoT device_id Per-Tenant

```sql
-- device_id becomes per-tenant unique; api_key stays globally unique
ALTER TABLE iot.devices DROP CONSTRAINT IF EXISTS devices_device_id_key;
CREATE UNIQUE INDEX idx_devices_tenant_device_id ON iot.devices(tenant_id, device_id);
```

**Rollback:** All original constraints/indexes are restored in reverse order.

---

### WP 2.6 — Composite PK Indexes (Migration V1.14.4)

**File:** `backend/database/migrations/001014004_create_composite_pk_indexes.go`

Preparation for composite foreign keys in Phase 4. Creates `UNIQUE(tenant_id, id)` on 18 target tables — tables that are referenced by FKs from other tenant-scoped tables:

| # | Table |
|---|-------|
| 1 | users.persons |
| 2 | users.staff |
| 3 | users.students |
| 4 | users.teachers |
| 5 | users.guardian_profiles |
| 6 | users.rfid_cards |
| 7 | education.groups |
| 8 | education.grade_transitions |
| 9 | facilities.rooms |
| 10 | activities.categories |
| 11 | activities.groups |
| 12 | active.groups |
| 13 | active.combined_groups |
| 14 | active.work_sessions |
| 15 | iot.devices |
| 16 | schedule.timeframes |
| 17 | suggestions.posts |
| 18 | auth.accounts_parents |

This enables Phase 4 to create composite FKs like `FOREIGN KEY (tenant_id, person_id) REFERENCES users.persons(tenant_id, id)`, ensuring referential integrity cannot cross tenant boundaries.

---

### WP 2.7 — Tenant Indexes (Migration V1.14.5)

**File:** `backend/database/migrations/001014005_create_tenant_indexes.go`

**Standard indexes (59 tables):** `idx_{table}_tenant ON {schema}.{table}(tenant_id)` — ensures RLS filter `WHERE tenant_id = ?` can use an index scan.

**Composite (tenant_id, id) indexes (41 tables):** `idx_{table}_tenant_id ON {schema}.{table}(tenant_id, id)` — for queries filtering on both tenant and primary key. The 18 FK-target tables from V1.14.4 already have `UNIQUE(tenant_id, id)` and are skipped here.

**Performance composite indexes (4):**

| Index | Table | Purpose |
|-------|-------|---------|
| `idx_students_tenant_class` | users.students(tenant_id, school_class) | Student list by class within tenant |
| `idx_visits_tenant_active` | active.visits(tenant_id, exit_time) WHERE exit_time IS NULL | Active visits lookup (hot path) |
| `idx_attendance_tenant_date` | active.attendance(tenant_id, date) | Daily attendance queries |
| `idx_devices_tenant_status` | iot.devices(tenant_id, status) | Device status checks |

---

### WP 2.8 — BUN Model Tag Changes (Code-only)

Removed `unique` from BUN struct tags in 11 model files. Uniqueness is now enforced by composite database indexes (tenant_id + column), not by single-column BUN ORM tags.

**Functionally necessary (§2.4.1 — names/keys that collide across tenants):**

| File | Field | Old Tag | New Tag |
|------|-------|---------|---------|
| `models/facilities/room.go` | Name | `bun:"name,notnull,unique"` | `bun:"name,notnull"` |
| `models/education/group.go` | Name | `bun:"name,notnull,unique"` | `bun:"name,notnull"` |
| `models/activities/category.go` | Name | `bun:"name,notnull,unique"` | `bun:"name,notnull"` |
| `models/auth/role.go` | Name | `bun:"name,notnull,unique"` | `bun:"name,notnull"` |
| `models/config/settings.go` | Key | `bun:"key,notnull,unique"` | `bun:"key,notnull"` |
| `models/users/profile.go` | AccountID | `bun:"account_id,notnull,unique"` | `bun:"account_id,notnull"` |
| `models/auth/account_parent.go` | Username | `bun:"username,unique"` | `bun:"username"` |

**Defense-in-depth (§2.4.2 — 1:1 FKs where composite uniqueness adds safety):**

| File | Field | Old Tag | New Tag |
|------|-------|---------|---------|
| `models/users/staff.go` | PersonID | `bun:"person_id,notnull,unique"` | `bun:"person_id,notnull"` |
| `models/users/teacher.go` | StaffID | `bun:"staff_id,notnull,unique"` | `bun:"staff_id,notnull"` |
| `models/users/guest.go` | StaffID | `bun:"staff_id,notnull,unique"` | `bun:"staff_id,notnull"` |
| `models/iot/device.go` | DeviceID | `bun:"device_id,notnull,unique"` | `bun:"device_id,notnull"` |

**Note:** `iot.Device.APIKey` correctly keeps its `unique` tag — `api_key` stays globally unique per FIX-9.

**Why:** BUN's `unique` tag creates a single-column UNIQUE constraint via `CreateTable`. Since uniqueness is now composite (tenant_id + column), the single-column tag would conflict. The composite indexes created in Migration V1.14.3 enforce the correct constraint.

---

### WP 2.9 — Populate account_tenants (Migration V1.14.6)

**File:** `backend/database/migrations/001014006_populate_account_tenants.go`

Maps all existing accounts to the default tenant (school_id=1):

1. Ensures default organization exists (`id=1, slug='default'`)
2. Ensures default school exists (`id=1, slug='default', subdomain='default'`)
3. Inserts `account_tenants` entries for all accounts not yet mapped to tenant 1

Uses `ON CONFLICT DO NOTHING` for idempotency and `NOT IN (SELECT ...)` to skip already-mapped accounts.

**Rollback:** Deletes tenant 1 mappings and the default org/school.

---

## Files Summary

### New Files (7)

| File | WP |
|------|----|
| `backend/database/migrations/001014001_create_tenant_roles.go` | 2.1 |
| `backend/database/migrations/001014002_add_tenant_id_to_all_tables.go` | 2.2–2.4 |
| `backend/database/migrations/001014003_migrate_unique_constraints.go` | 2.5 |
| `backend/database/migrations/001014004_create_composite_pk_indexes.go` | 2.6 |
| `backend/database/migrations/001014005_create_tenant_indexes.go` | 2.7 |
| `backend/database/migrations/001014006_populate_account_tenants.go` | 2.9 |
| `docs/multi-tenancy/15-phase2-implementation-summary.md` | Docs |

### Modified Files (11)

| File | WP | Change |
|------|----|--------|
| `backend/models/facilities/room.go` | 2.8 | Remove `unique` from `name` tag |
| `backend/models/education/group.go` | 2.8 | Remove `unique` from `name` tag |
| `backend/models/activities/category.go` | 2.8 | Remove `unique` from `name` tag |
| `backend/models/auth/role.go` | 2.8 | Remove `unique` from `name` tag |
| `backend/models/config/settings.go` | 2.8 | Remove `unique` from `key` tag |
| `backend/models/users/staff.go` | 2.8 | Remove `unique` from `person_id` tag |
| `backend/models/users/teacher.go` | 2.8 | Remove `unique` from `staff_id` tag |
| `backend/models/users/guest.go` | 2.8 | Remove `unique` from `staff_id` tag |
| `backend/models/users/profile.go` | 2.8 | Remove `unique` from `account_id` tag |
| `backend/models/iot/device.go` | 2.8 | Remove `unique` from `device_id` tag (FIX-9; `api_key` keeps `unique`) |
| `backend/models/auth/account_parent.go` | 2.8 | Remove `unique` from `username` tag |

### NOT Modified

- No handler, service, or repository code changed
- No existing test files modified
- No router or middleware changes
- `base.Model` untouched — `TenantModel` mixin (Phase 1) is not yet embedded

---

## Verification Results

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| `go run main.go migrate validate` | PASS — 6 new migrations (V1.14.1–V1.14.6) registered with correct dependencies |
| Migration dependency chain | V1.14.1 → V1.14.2 → V1.14.3, V1.14.4, V1.14.5 (parallel) → V1.14.6 |
| Existing tests | Unaffected — `DEFAULT 1` absorbs missing `tenant_id` |
| Backward compatibility | INSERT without `tenant_id` gets `DEFAULT 1` automatically |

---

## Key Design Decisions Applied

| Decision | How Applied |
|----------|-------------|
| D7 | Three-role architecture: `phoenix_auth` (NOINHERIT), `phoenix_tenant` (RLS), `phoenix_admin` (BYPASSRLS) |
| D8 | Roles created for `SET LOCAL ROLE` pattern; not used by the application yet in Phase 2 |
| D13 | `auth.roles` gets NULLABLE `tenant_id`; system roles keep NULL = globally visible; `account_roles`/`account_permissions` get NOT NULL |
| D15 | `auth.accounts` stays global (no `tenant_id`); mapping via `account_tenants` (populated in V1.14.6) |
| D16 | SEQUENCE grants included for `phoenix_tenant`/`phoenix_admin` |
| FIX-9 | `iot.devices.device_id` changed from globally unique to per-tenant unique; `api_key` stays globally unique |
| Expand-Contract | `DEFAULT 1` stays in place; old code works unchanged; Contract (DROP DEFAULT, enable RLS) deferred to Phase 4 |

---

## Migration Dependency Graph

```
V1.13.1 (platform tables) ─────────────────┐
V1.13.2 (account_tenants) ──────────────────┤
                                             │
V1.14.1 (PostgreSQL roles) ← depends on 1.13.1
    │                                        │
    ▼                                        │
V1.14.2 (tenant_id on 58+1 tables) ← depends on 1.14.1 + 1.13.1
    │
    ├──► V1.14.3 (UNIQUE constraints) ← depends on 1.14.2
    ├──► V1.14.4 (composite PK indexes) ← depends on 1.14.2
    ├──► V1.14.5 (tenant indexes) ← depends on 1.14.2
    │
    └──► V1.14.6 (populate account_tenants) ← depends on 1.14.2 + 1.13.2
```

---

## What's NOT in Phase 2 (Deferred)

| Item | Phase | Reason |
|------|-------|--------|
| Composite FKs (64 FKs) | Phase 4 | Requires all code to set `tenant_id` correctly first |
| RLS policies (`CREATE POLICY`) | Phase 4 | Code must be migrated in Phase 3 |
| `DROP DEFAULT 1` | Phase 4 | Safety net stays until RLS is active |
| Trigger `SECURITY INVOKER` audit | Phase 4 | Only matters when RLS is active |
| Connect as `phoenix_auth` | Phase 3/4 | App continues using current DB superuser |
| Embed `TenantModel` in Go structs | Phase 3 | Service layer changes needed first |

---

## What's Next (Phase 3)

Phase 3 is the **Backend Code Migration** — modifying service and repository layers to explicitly set `tenant_id`:

- Embed `TenantModel` mixin into all 58+1 model structs
- Update repositories to use `GetDB(ctx, r.db)` for tenant-scoped transactions
- Update services to call `tenant.WithTenantTx()` for write operations
- Add `AssertRowsAffected` checks to UPDATE/DELETE operations
- Mount `TenantMiddleware` in the router
- Inject `tenant_id` from JWT context into all write paths
