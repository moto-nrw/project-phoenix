# Settings System v1: Implementation Plan

**Status:** Agreed — synthesis of Chris's RFC + Yannick's reviews
**Date:** 2026-03-27 (updated after v1 plan review)
**Branch:** `feature/settings-system`

---

## Principle

Same backbone as Chris's RFC (registry + schema-driven rendering). Scoped to what we need today: **12 tenant-level settings migrated from `.env`**, with a strong core that makes adding more settings trivial.

Yannick's reviews correctly identified that (1) the RFC was too ambitious for v1, and (2) the scheduler refactor and compat wrapper are critical path items. This plan keeps the architecture but cuts to the real deliverable: **tenant-specific operational settings that actually work in production.**

---

## What's In v1

| Feature | Status | Notes |
|---------|--------|-------|
| Registry pattern (`Register()` at startup) | **In** | Core DX — adding a setting = 1 function call |
| Schema endpoint with tabs/categories | **In** | Good organization as settings grow |
| DB table `config.setting_values` | **In** | Stores tenant overrides |
| Audit table `config.setting_audit` | **In** | Writes on every change, no API/UI yet |
| Scope: system + tenant | **In** | Tenant override → registry default fallback |
| Field types: boolean, number, time, text, password, select | **In** | Covers all 12 settings + PIN masking + static selects |
| Dependencies (`DependsOn`) | **In** | 3 setting groups need it (enabled toggle → show/hide config fields) |
| Permissions (read/write per setting) | **In** | Already wired via existing middleware |
| 3 API endpoints (schema, set, reset) | **In** | Minimal but complete |
| 12 real settings from `.env` migration | **In** | The actual deliverable |
| Compat wrapper for existing ConfigService | **In (Phase 1)** | Existing `GetTimeoutSettings()` etc. must keep working |
| Scheduler refactor (tenant-aware, minute-polling) | **In (Phase 2)** | Without this, settings are a lie |
| Frontend settings page | **In (Phase 3)** | Schema-driven, 6 field components |

## What's Cut (Add Later If Needed)

| Feature | Why cut | How to add later |
|---------|---------|-----------------|
| Extra scopes (user, device, group, room, org) | No setting needs them yet | Add 1 constant + 1 chain entry (~10 lines Go, 0 migrations) |
| Scope cascade / inherited-from UI | Only system→tenant needed | Extend resolver + add UI badge |
| Action buttons in settings | Admin actions belong on their domain pages | Add `ActionDefinition` registry + handler dispatch |
| Dynamic select options (API-backed) | No setting needs API-populated dropdown yet | Frontend-only: ~15 lines in select-field.tsx to check `options.endpoint` and fetch. No backend/migration changes. |
| `date`, `color`, `textarea`, `json` field types | No current use case | Add 1 constant + 1 frontend component each |
| Custom renderers | No exotic settings yet | Add `registerCustomField()` on frontend |
| Bulk operations (set/get many at once) | Single value CRUD is enough | Add `PUT /values/bulk` endpoint |
| Audit trail API + UI | Table writes are enough for now | Add `GET /audit/{key}` + frontend history viewer |
| Resolve-single endpoint | Schema includes resolved values | Add `GET /resolve/{key}` if needed |
| Import/export, search, i18n keys | Premature | Purely additive when needed |

---

## The 12 Settings (from `.env` migration)

Source: `docs/settings-suggestions.md` by Yannick + review fix #5

Settings are categorized by **consumption pattern** (review fix #4), which determines the refactoring difficulty in Phase 2:

### Per-Request (trivial to swap — just replace `os.Getenv()` with resolver call)

| Key | Type | Default | Env Var | Code Location |
|-----|------|---------|---------|---------------|
| `security.ogs_device_pin` | password | `""` | `OGS_DEVICE_PIN` | `auth/device/device_auth.go:224` |
| `operations.student_daily_checkout_time` | time | `15:00` | `STUDENT_DAILY_CHECKOUT_TIME` | `api/iot/checkin/helpers.go:17` |

### Runtime (re-read every execution — straightforward swap)

| Key | Type | Default | Env Var | Code Location |
|-----|------|---------|---------|---------------|
| `operations.session_end_enabled` | boolean | `true` | `SESSION_END_SCHEDULER_ENABLED` | `services/scheduler/scheduler.go:548` |
| `operations.session_end_time` | time | `18:00` | `SESSION_END_TIME` | `services/scheduler/scheduler.go:548` |
| `operations.session_end_timeout_minutes` | number | `10` | `SESSION_END_TIMEOUT_MINUTES` | `services/scheduler/scheduler.go:652` |
| `operations.session_abandoned_threshold_minutes` | number | `60` | `SESSION_ABANDONED_THRESHOLD_MINUTES` | `services/scheduler/scheduler.go:719` |
| `gdpr.data_cleanup_enabled` | boolean | `true` | `CLEANUP_SCHEDULER_ENABLED` | `services/scheduler/scheduler.go:192` |
| `gdpr.data_cleanup_time` | time | `02:00` | `CLEANUP_SCHEDULER_TIME` | `services/scheduler/scheduler.go:198` |
| `gdpr.data_cleanup_timeout_minutes` | number | `30` | `CLEANUP_SCHEDULER_TIMEOUT_MINUTES` | `services/scheduler/scheduler.go:302` |

### Init-Time (stored in struct fields, captured by goroutines — needs structural refactor)

| Key | Type | Default | Env Var | Code Location |
|-----|------|---------|---------|---------------|
| `operations.session_cleanup_enabled` | boolean | `true` | `SESSION_CLEANUP_ENABLED` | `services/scheduler/scheduler.go:712` |
| `operations.session_cleanup_interval_minutes` | number | `15` | `SESSION_CLEANUP_INTERVAL_MINUTES` | `services/scheduler/scheduler.go:712` |
| `operations.break_auto_end_interval_seconds` | number | `60` | `BREAK_AUTO_END_INTERVAL_SECONDS` | `services/scheduler/scheduler.go:830` |

**Init-time settings** are parsed once at startup and stored in scheduler struct fields. Changing them per-tenant means the goroutine's tick interval must become tenant-aware. This is the hardest part of Phase 2.

---

## Phase 1: Backend Core

### 1.1 Registry (`models/config/registry.go`)

Definition struct with: Key, Label, Description, Type, Default, Validation, ReadPermission, WritePermission, Tab, Category, SortOrder, DependsOn, Options.

6 field types: `boolean`, `number`, `time`, `text`, `password`, `select`.

`DependsOn` for conditional visibility (e.g., session_end_time only shown when session_end_enabled = true). `Options` for static select choices (e.g., language picker). No dynamic selects in v1.

No scopes array in the Definition (everything is tenant-scoped in v1). System defaults come from the registry.

Registry: `Register()`, `GetDefinition()`, `AllDefinitions()`, `ResetRegistry()`.

### 1.2 DB Model (`models/config/setting_value.go`)

`SettingValue` struct: tenant_id (NOT NULL — no system scope in DB), setting_key, value (JSONB), updated_by, timestamps.

Unique constraint: `(tenant_id, setting_key)`.

### 1.3 Audit Model (`models/config/setting_audit.go`)

`SettingAuditEntry`: tenant_id, setting_key, old_value, new_value, action, changed_by, changed_at.

Append-only. No API endpoint in v1.

### 1.4 Repository Interfaces (`models/config/setting_value_repository.go`)

```
SettingValueRepository:
  FindByTenantAndKey(ctx, tenantID, key) → (*SettingValue, error)
  FindByTenant(ctx, tenantID) → ([]*SettingValue, error)
  Upsert(ctx, *SettingValue) → error
  Delete(ctx, tenantID, key) → error

SettingAuditRepository:
  Create(ctx, *SettingAuditEntry) → error
```

### 1.5 Migration (`database/migrations/001015019_...`)

Create `config.setting_values` and `config.setting_audit`. RLS on both. Simpler than RFC: no scope_type/scope_id columns, just tenant_id + setting_key.

### 1.6 Repository Implementations

`database/repositories/config/setting_value.go` — CRUD with BUN ORM, tenant-scoped.
`database/repositories/config/setting_audit.go` — INSERT only.

### 1.7 Service (`services/config/settings_service.go`)

```go
type SettingsService interface {
    GetSchema(ctx, userPermissions []string) (*SettingsSchema, error)
    Resolve(ctx, key string) (any, error)
    ResolveString(ctx, key string) (string, error)
    ResolveBool(ctx, key string) (bool, error)
    ResolveInt(ctx, key string) (int, error)
    SetValue(ctx, key string, value any, changedBy *int64) error
    ResetValue(ctx, key string, changedBy *int64) error
}
```

Resolver: check tenant override → fall back to registry default.

**PIN handling (review fix #3):** The OGS device PIN is stored as **plain text** in the DB (not hashed). A 4-digit numeric PIN has only 10,000 possibilities — Argon2id hashing provides no meaningful security. The real security improvement is making it per-tenant behind RLS. The `password` field type controls **display only**: the schema response shows `"••••••"` instead of the actual value. Device auth continues to use `SecureCompareStrings()` for constant-time comparison.

### 1.8 Schema Builder (`services/config/schema_builder.go`)

Groups definitions by Tab → Category. Resolves values. Filters by permissions. Evaluates `DependsOn` conditions to set `visible` flag. Sorts by SortOrder. Returns structured `SettingsSchema`. Frontend can re-evaluate dependencies client-side for instant toggle without API round-trip.

### 1.9 Defaults Registration (`services/config/defaults/`)

- `operations.go` — 8 operations settings (session end, checkout, cleanup, break auto-end)
- `gdpr.go` — 3 GDPR settings
- `security.go` — 1 device PIN setting

### 1.10 API (`api/config/settings_api.go`)

3 endpoints:
- `GET /api/settings/schema` — full schema with resolved values
- `PUT /api/settings/values/{key}` — set tenant override
- `DELETE /api/settings/values/{key}` — reset to default

### 1.11 Compat Wrapper (review fix #2)

**This must be in Phase 1, not Phase 4.** The existing `config.settings` table is actively used in production:
- `GetTimeoutSettings()` at `services/config/config_service.go:538-579` reads session timeout/warning/check-interval on every call
- IoT resource injects `ConfigService` at `api/iot/api.go:44,176`

The compat wrapper is a thin layer: existing `GetStringValue()` / `GetBoolValue()` / `GetTimeoutSettings()` methods delegate to the new settings resolver instead of the old repository. Existing callers keep working without changes.

File: `services/config/compat.go`

### 1.12 Factory Wiring

Add to repo factory, service factory, API base. Blank import of defaults package.

### 1.13 Tests

Registry unit tests. Service resolver tests. Compat wrapper tests. API integration tests.

---

## Phase 2: Scheduler + Code Migration

**This is the hardest part** (Yannick's key insight). Without this, the settings UI is a lie.

### 2.1 Per-Tenant Scheduling Design (review fix #1)

**Decision: Minute-polling (option A).**

The scheduler currently registers a single cron trigger per job. Per-tenant times break this model. The solution:

```
Every 60 seconds:
  for each active tenant:
    resolve tenant's configured time for each job
    if now matches tenant's time (within the 60s window):
      execute job for that tenant
```

Why minute-polling:
- Simplest to implement and reason about
- 60s delay is acceptable for session-end and cleanup jobs
- No dynamic cron registration/deregistration complexity
- `forEachTenant()` already exists at `scheduler.go:125-159` using `schoolRepo.ListActive()` + `tenant.WithTenantTx()`

The per-tenant execution path exists. This adds per-tenant **trigger timing**.

### 2.2 Setting Consumption Patterns (review fix #4)

**Per-request settings** (trivial swap):
- `security.ogs_device_pin` — replace `os.Getenv()` in `device_auth.go:224` with `settingsService.ResolveString(ctx, "security.ogs_device_pin")`. Add TTL cache (30-60s) per tenant. Fallback to env var during transition.
- `operations.student_daily_checkout_time` — replace `os.Getenv()` in `checkin/helpers.go:17` with resolver call.

**Runtime settings** (straightforward swap):
- Session end time/enabled/timeout, cleanup time/enabled/timeout, abandoned threshold — replace `os.Getenv()` calls in scheduler with `settingsService.Resolve()` calls inside the per-tenant execution loop.

**Init-time settings** (structural refactor):
- `session_cleanup_interval_minutes` and `break_auto_end_interval_seconds` are parsed once at startup and stored in scheduler struct fields, captured by goroutines.
- These goroutines run on a fixed tick interval. Making them per-tenant means either:
  - **(a)** Running a single goroutine at the minimum interval across all tenants, then skipping tenants whose interval hasn't elapsed (simpler, wastes some CPU)
  - **(b)** Running one goroutine per tenant per job (more precise, more goroutines)
- **Recommendation:** Option (a) for v1. Use the smallest configured interval as the tick, resolve per-tenant on each tick, track last-execution-time per tenant.

### 2.3 Env Var Cleanup

After all code reads from settings: remove migrated vars from `.env.example`, `docker-compose.example.yml`, `backend/dev.env.example`. Keep env vars as commented-out documentation for a release cycle.

---

## Phase 3: Frontend

### 3.1 Files

```
frontend/src/
├── lib/settings-api.ts           # fetch schema, put/delete values
└── components/settings/
    ├── settings-page.tsx          # fetches schema, renders tabs
    ├── settings-category.tsx      # category heading + fields
    ├── settings-field.tsx         # single row: label + component + reset + dependency visibility
    └── fields/
        ├── boolean-field.tsx      # toggle
        ├── number-field.tsx       # number input
        ├── time-field.tsx         # time picker (HH:MM)
        ├── text-field.tsx         # text input
        ├── password-field.tsx     # masked input
        └── select-field.tsx       # static select dropdown
```

~9 files total.

### 3.2 Integration

Injected via `extraTabs` on existing `SettingsLayout`. Tabs: Operations, GDPR, Security.

### 3.3 Behavior

- Fetch schema on mount (one API call)
- Field type → component mapping
- **Save behavior**: Booleans/selects save immediately (one click). Text/number/time fields use local state with auto-save after 3 seconds of idle OR on blur (click away). Green ring on field for 2s after successful save, red ring on error.
- Reset button → DELETE, **then re-fetch schema** (dependencies may change)
- "Default" badge when `is_default: true`
- Password fields: write-only, show "••••••" when set, admin can set new value via explicit save button
- `DependsOn` evaluation: client-side visibility toggle when parent value changes (no API round-trip)

---

## Phase 4: Follow-Up (Not in v1)

| Feature | Trigger | Effort |
|---------|---------|--------|
| Confirm-save button for sensitive settings | v2 — for passwords, PINs, or settings where accidental changes are dangerous. Instead of auto-save, show an explicit "Speichern" button that requires confirmation. Register via `requireConfirmation: true` in the Definition. | ~30 lines frontend, 1 field in Definition |
| More scopes (user, device, room, org) | When a setting genuinely needs per-room/device values | ~10 lines Go, 0 migrations |
| Scope cascade + inherited-from UI | When multi-level resolution is needed | Extend resolver + add UI badge |
| Action buttons | When admin actions are formalized | Add registry + handler dispatch |
| Dynamic select options (API-backed) | When a setting needs a dropdown populated from API | ~15 lines in select-field.tsx, 0 backend changes |
| Audit trail API + UI | When compliance requires change history review | Add `GET /audit/{key}` + frontend viewer |
| `date`, `color`, `textarea`, `json` field types | When a setting needs them | 1 constant + 1 frontend component each |
| Custom renderers | When a setting needs exotic UI | `registerCustomField()` on frontend |
| Bulk operations | When managing settings across many tenants | Add `PUT /values/bulk` endpoint |
| Import/export, search, i18n keys | When scale demands it | Purely additive |

---

## Review Fixes Applied

Fixes from Yannick's review (`docs/settings-v1-plan-review.md`):

| # | Issue | Severity | Resolution |
|---|-------|----------|------------|
| 1 | Per-tenant scheduling design missing | **High** | Added minute-polling design to Phase 2.1 |
| 2 | Compat wrapper belongs in Phase 1 | **High** | Moved to Phase 1.11 with `services/config/compat.go` |
| 3 | PIN hashing pointless for 4-digit codes | Medium | Store plain text, mask in UI via password field type only |
| 4 | Init-time vs runtime env var patterns | Medium | Settings categorized by consumption pattern in settings table |
| 5 | `BREAK_AUTO_END_INTERVAL_SECONDS` missing | Low | Added as 12th setting (`operations.break_auto_end_interval_seconds`) |
| 6 | Frontend re-fetch after DELETE | Low | Added to Phase 3.3 behavior spec |

---

## Key Differences from Original RFC

| Aspect | RFC | v1 |
|--------|-----|-----|
| Scopes | 7 (system→org→tenant→user→device→group→room) | 2 (system default in registry, tenant override in DB) |
| Field types | 10 | 6 (boolean, number, time, text, password, select) |
| Settings registered | Placeholder examples | 12 real settings from `.env` migration |
| Actions | Full action framework | None |
| Dependencies | Full dependency engine | **In** — needed for enabled-toggle → config-fields pattern |
| Dynamic selects | API-backed dropdowns | Cut (static selects only) |
| DB columns | scope_type, scope_id, nullable tenant_id | Just tenant_id + setting_key |
| API endpoints | 8 | 3 |
| Scheduler | Not addressed | **Phase 2 — minute-polling, per-tenant execution** |
| Audit | Full table + API + UI | Table + write-on-change only |
| Compat wrapper | Phase 6 | **Phase 1** — existing config.settings must keep working |
| PIN security | Argon2id hashing | Plain text behind RLS, masked in UI only |

---

## Key Differences from Yannick's Trimmed Plan

| Aspect | Yannick's plan | This plan |
|--------|---------------|-----------|
| Audit table | "No audit table for now" | **Keep** — writes are cheap, important for critical settings like PIN |
| Schema builder | Simplified | **Keep** — tabs/categories matter as settings grow |
| Password field type | Cut (4 types only) | **Keep** — OGS PIN must not be visible in plain text in schema response |
| Select field type | Cut | **Keep** — needed for language picker and similar static choices |
| Dependencies | Not mentioned | **Keep** — 3 of the 12 settings groups use the enabled→config pattern |
| Field types | 4 (boolean, number, time, text) | 6 (+ password, select) |
| Compat wrapper | Not in v1 | **In Phase 1** — per Yannick's own v1 plan review |
| Scope columns in DB | None | None (agree — tenant-only is simpler) |
